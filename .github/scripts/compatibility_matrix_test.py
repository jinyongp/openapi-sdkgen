import copy
import json
from pathlib import Path
import unittest

from compatibility_gate import check_report
from compatibility_matrix import CORPORA, document_matrix

ROOT = Path(__file__).resolve().parents[2]


class CompatibilityMatrixTests(unittest.TestCase):
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


if __name__ == "__main__":
    unittest.main()
