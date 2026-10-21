import unittest
import hashlib
import json
from pathlib import Path
from tempfile import TemporaryDirectory

from go_ci import PACKAGES, group_tests, partition, verify_reports


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
        for change in ("missing", "duplicate", "changed", "unexecuted", "overlap"):
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
                else:
                    reports[1]["selected"] = reports[0]["selected"]
                    reports[1]["tests"] = reports[0]["tests"]
                self.verify(reports)


if __name__ == "__main__":
    unittest.main()
