import copy
import hashlib
import json
import tomllib
from pathlib import Path
import unittest

from compatibility_gate import check_report
from compatibility_matrix import CORPORA, document_matrix

ROOT = Path(__file__).resolve().parents[2]


class CompatibilityMatrixTests(unittest.TestCase):

    def test_recorded_metadata_settings_are_validated(self):
        manifest, report = self.load("holdout"), self.load("holdout", "-results")
        for item in report["documents"]:
            item["generationAddons"] = ["metadata"]
            for profile in item.get("supportProfiles", []):
                profile["generationAddons"] = ["metadata", "server"]
        check_report(manifest, report)
        for addons in (None, ["server"], ["metadata", "metadata"], ["unknown"]):
            invalid = copy.deepcopy(report)
            invalid["documents"][0]["generationAddons"] = addons
            with self.assertRaises(ValueError):
                check_report(manifest, invalid)
        invalid = copy.deepcopy(report)
        item = next(item for item in invalid["documents"] if item.get("supportProfiles"))
        item["supportProfiles"][0]["generationAddons"] = ["server"]
        with self.assertRaises(ValueError):
            check_report(manifest, invalid)
    def load(self, name, suffix=""):
        return json.loads((ROOT / "test/compatibility" / f"{name}{suffix}.json").read_text())

    def test_every_registered_document_is_selected_once(self):
        jobs = document_matrix(ROOT)["include"]
        expected = {(name, item["id"]) for name in CORPORA for item in self.load(name)["corpora"]}
        self.assertEqual({(job["corpus"], job["document"]) for job in jobs}, expected)
        self.assertEqual(len(jobs), len(expected))

    def test_existing_client_and_server_evidence_keeps_its_outcome(self):
        for name in CORPORA:
            manifest = self.load(name)
            report = self.load(name, "-results")
            summary, failures = check_report(manifest, report)
            self.assertEqual(set(failures), {item["id"] for item in report["documents"] if not item["capabilityAdjustedSuccess"]})
            self.assertIn("Strict typecheck", summary)
            for item in report["documents"]:
                shard = dict(report, documents=[item])
                _, failed = check_report(manifest, shard, item["id"])
                self.assertEqual(bool(failed), not item["capabilityAdjustedSuccess"])

    def test_missing_duplicate_unknown_and_false_success_fail(self):
        manifest = self.load("regression")
        original = self.load("regression", "-results")
        invalid = []
        missing = copy.deepcopy(original)
        missing["documents"].pop()
        invalid.append(missing)
        duplicate = copy.deepcopy(original)
        duplicate["documents"].append(duplicate["documents"][0])
        invalid.append(duplicate)
        unknown = copy.deepcopy(original)
        unknown["documents"][0]["id"] = "unknown"
        invalid.append(unknown)
        false_success = copy.deepcopy(original)
        false_success["documents"][0]["documentSuccess"] = not false_success["documents"][0]["documentSuccess"]
        invalid.append(false_success)
        for report in invalid:
            with self.assertRaises(ValueError):
                check_report(manifest, report)

    def test_selected_scope_requires_policy_runtime_and_distinct_summary(self):
        directory = ROOT / "test/compatibility/selections"
        path = directory / "microsoft-graph-beta.toml"
        policy = tomllib.loads(path.read_text())
        report = self.load("regression", "-results")
        item = next(item for item in report["documents"] if item["id"] == policy["document"])
        routes = sorted(policy["selection"]["routes"])
        item.update(generationScope="selected", documentSuccess=True, capabilityAdjustedSuccess=True,
                    generationSelection=dict(fixtureSha256=hashlib.sha256(path.read_bytes()).hexdigest(),
                        runtimeProbeSha256=hashlib.sha256((directory / policy["runtime_probe"]).read_bytes()).hexdigest(),
                        requested=dict(routes=routes), routes=routes, dependencyRoutes=[],
                        excludedOperations=item["operationRetention"]["total"]-len(routes), runtime=dict(status="pass")))
        item["typecheck"]["status"] = "pass"
        item["operationEmission"]["count"] = len(routes)
        manifest = self.load("regression")
        summary, failures = check_report(manifest, report, selection_directory=directory)
        self.assertEqual(failures, [])
        self.assertIn("Full documents: 6/6", summary)
        self.assertIn("Selected SDKs: 1/1", summary)
        for field, value in [("fixtureSha256", "bad"), ("runtimeProbeSha256", "bad"), ("excludedOperations", 0), ("routes", routes[1:])]:
            invalid = copy.deepcopy(report)
            selected = next(item for item in invalid["documents"] if item.get("generationScope") == "selected")
            selected["generationSelection"][field] = value
            with self.assertRaises(ValueError):
                check_report(manifest, invalid, selection_directory=directory)
        item["generationSelection"]["runtime"]["status"] = "fail"
        item["documentSuccess"] = item["capabilityAdjustedSuccess"] = False
        _, failures = check_report(manifest, report, selection_directory=directory)
        self.assertEqual(failures, ["microsoft-graph-beta"])
        item.pop("generationScope")
        with self.assertRaises(ValueError):
            check_report(manifest, report, selection_directory=directory)


if __name__ == "__main__":
    unittest.main()
