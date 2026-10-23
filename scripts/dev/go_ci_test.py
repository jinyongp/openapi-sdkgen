import unittest
import hashlib
import json
from pathlib import Path
from tempfile import TemporaryDirectory

from go_ci import PACKAGES, group_tests, load_timings, partition, refresh_timings, validate_timings, verify_reports


class TestPartition(unittest.TestCase):
    def test_every_test_runs_exactly_once(self):
        names = [f"TestCase{index}" for index in range(103)] + ["FuzzSeed", "ExampleClient"]
        shards = [partition(names, index, 4) for index in range(1, 5)]
        flat = [name for shard in shards for name in shard]
        self.assertEqual(sorted(flat), sorted(names))
        self.assertEqual(len(flat), len(set(flat)))
        self.assertLessEqual(max(map(len, shards)) - min(map(len, shards)), 1)
        self.assertEqual(shards, [partition(reversed(names), index, 4) for index in range(1, 5)])

    def test_invalid_shards_fail(self):
        for shard, count in ((0, 4), (5, 4), (1, 0), (1, -1)):
            with self.subTest(shard=shard, count=count), self.assertRaises(ValueError):
                partition(["TestOne"], shard, count)

    def test_measured_cost_balances_heavy_tests_without_changing_inventory(self):
        names = [f"Test{letter}" for letter in "ABCDEF"]
        timings = {name: 10 if name[-1] in "ACE" else 1 for name in names}
        shards = [partition(names, index, 2, timings) for index in (1, 2)]
        self.assertEqual(sorted(name for shard in shards for name in shard), names)
        self.assertEqual(len({name for shard in shards for name in shard}), len(names))
        self.assertEqual(max(sum(timings[name] for name in shard) for shard in shards), 20)
        self.assertEqual(shards, [partition(reversed(names), index, 2, timings) for index in (1, 2)])

    def test_zero_cost_and_new_tests_still_run_exactly_once(self):
        names = ["TestKnown", "TestNew", "FuzzSeed", "ExampleClient"]
        timings = {name: 0 for name in names if name != "TestNew"}
        shards = [partition(names, index, 3, timings) for index in (1, 2, 3)]
        self.assertEqual(sorted(name for shard in shards for name in shard), sorted(names))
        self.assertTrue(all(shards))

    def test_only_typescript_matrices_have_separate_owners(self):
        names = ["TestOne", "FuzzSeed", "ExampleClient",
                 "TestRuntimeFeatureNativeMatrixRegression", "TestRuntimeFeatureDeclarationMatrixRegression"]
        self.assertEqual(group_tests("golang", names), names)
        self.assertEqual(group_tests("typescript", names), names[:3])


class TestReports(unittest.TestCase):
    def reports(self, group="golang"):
        names = ["TestOne", "TestTwo", "FuzzSeed", "ExampleClient"]
        digest = hashlib.sha256("\n".join(sorted(names)).encode()).hexdigest()
        return [{"group": group, "shard": shard, "count": 2,
                 "exitCode": 0, "inventoryCount": len(names), "inventorySha256": digest,
                 "selected": partition(names, shard, 2),
                 "tests": {PACKAGES[group] + "/" + name: {"status": "pass", "seconds": 1}
                           for name in partition(names, shard, 2)}} for shard in (1, 2)]

    def verify(self, reports, group="golang"):
        with TemporaryDirectory() as temporary:
            directory = Path(temporary)
            for index, report in enumerate(reports):
                (directory / f"{group}-{index}.json").write_text(json.dumps(report))
            verify_reports(directory, group)

    def test_completed_inventory_passes(self):
        for group in PACKAGES:
            with self.subTest(group=group):
                self.verify(self.reports(group), group)

    def test_missing_duplicate_changed_or_unexecuted_inventory_fails(self):
        for change in ("missing", "duplicate", "changed", "unexecuted", "overlap", "timings"):
            with self.subTest(change=change), self.assertRaises(ValueError):
                reports = self.reports()
                if change == "missing":
                    reports.pop()
                elif change == "duplicate":
                    reports[1]["shard"] = 1
                elif change == "changed":
                    reports[1]["inventorySha256"] = "0" * 64
                elif change == "unexecuted":
                    reports[1]["tests"] = {}
                elif change == "timings":
                    reports[1]["timingsSha256"] = "different-profile"
                else:
                    reports[1]["selected"] = reports[0]["selected"]
                    reports[1]["tests"] = reports[0]["tests"]
                self.verify(reports)


class TestTimings(unittest.TestCase):
    def profile(self):
        return {"schema": 1, "groups": {group: {"TestOne": 0, "TestTwo": 2.5} for group in PACKAGES}}

    def test_bad_profiles_fail(self):
        for value in ([], {}, {"schema": 1, "groups": {}},
                      {"schema": True, "groups": self.profile()["groups"]}):
            with self.subTest(value=value), self.assertRaises(ValueError):
                validate_timings(value)
        for seconds in (-1, float("inf"), float("nan"), "1", True):
            profile = self.profile()
            profile["groups"]["golang"]["TestOne"] = seconds
            with self.subTest(seconds=seconds), self.assertRaises(ValueError):
                validate_timings(profile)
        profile = self.profile()
        profile["groups"]["golang"] = {"TestParent/child": 1}
        with self.assertRaises(ValueError):
            validate_timings(profile)

    def test_loaded_profile_has_a_content_identity(self):
        with TemporaryDirectory() as temporary:
            path = Path(temporary) / "timings.json"
            raw = json.dumps(self.profile()).encode()
            path.write_bytes(raw)
            groups, digest = load_timings(path)
            self.assertEqual(groups, self.profile()["groups"])
            self.assertEqual(digest, hashlib.sha256(raw).hexdigest())

    def write_run(self, root, run_id, seconds):
        directory = root / str(run_id)
        directory.mkdir()
        (directory / "run.json").write_text(json.dumps({
            "id": run_id, "head_sha": "a" * 40, "html_url": f"https://example.test/runs/{run_id}",
            "status": "completed", "conclusion": "success", "event": "push", "head_branch": "main",
            "path": ".github/workflows/ci.yml"}))
        reports = directory / "reports"
        reports.mkdir()
        for group in PACKAGES:
            for index, report in enumerate(TestReports().reports(group)):
                for outcome in report["tests"].values():
                    outcome["seconds"] = seconds
                (reports / f"{group}-{index}.json").write_text(json.dumps(report))
        return directory

    def test_refresh_uses_per_test_medians_and_preserves_sources(self):
        with TemporaryDirectory() as temporary:
            root = Path(temporary)
            for run_id, seconds in ((1, 1), (2, 9), (3, 2)):
                self.write_run(root, run_id, seconds)
            destination = root / "timings.json"
            refresh_timings(root, destination)
            profile = json.loads(destination.read_text())
            self.assertEqual([source["id"] for source in profile["sources"]], [1, 2, 3])
            self.assertTrue(all(seconds == 2 for tests in profile["groups"].values() for seconds in tests.values()))

    def test_failed_missing_or_incomplete_sources_do_not_replace_profile(self):
        for change in ("failed", "missing", "incomplete", "invalid"):
            with self.subTest(change=change), TemporaryDirectory() as temporary:
                root = Path(temporary)
                directory = self.write_run(root, 1, -1 if change == "invalid" else 1)
                if change == "failed":
                    run = json.loads((directory / "run.json").read_text())
                    run["conclusion"] = "failure"
                    (directory / "run.json").write_text(json.dumps(run))
                elif change == "missing":
                    (directory / "run.json").unlink()
                elif change == "incomplete":
                    (directory / "reports" / "golang-0.json").unlink()
                destination = root / "timings.json"
                destination.write_text("previous profile")
                with self.assertRaises(ValueError):
                    refresh_timings(root, destination)
                self.assertEqual(destination.read_text(), "previous profile")


if __name__ == "__main__":
    unittest.main()
