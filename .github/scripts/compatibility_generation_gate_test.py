import copy
import hashlib
import json
from pathlib import Path
import unittest

from compatibility_generation_gate import check_generation_report

ROOT = Path(__file__).resolve().parents[2] / "test/compatibility"


class GenerationGateTests(unittest.TestCase):
    def fixture(self, scope):
        manifest = (ROOT / "regression.json").read_bytes()
        if scope == "selected":
            report = json.loads((ROOT / "graph-selected-results.json").read_text())
        else:
            baseline = json.loads((ROOT / "graph-full-baseline.json").read_text())
            original = json.loads((ROOT / "regression-results.json").read_text())
            item = next(item for item in original["documents"] if item["id"] == "microsoft-graph-beta")
            item.update(baseline["document"])
            report = dict(schemaVersion=2, manifestSha256=hashlib.sha256(manifest).hexdigest(), documents=[item])
        item = report["documents"][0]
        item["documentSuccess"] = item["capabilityAdjustedSuccess"] = False
        item["generationAddons"] = []
        for profile in [item] + item.get("supportProfiles", []):
            profile["typecheck"] = dict(status="not-run")
            if "success" in profile:
                profile["success"] = False
        if scope == "selected":
            item["generationSelection"]["runtime"] = dict(status="not-run")
        return manifest, report

    def test_both_scopes_require_generation_without_verification(self):
        for scope in ("full", "selected"):
            manifest, report = self.fixture(scope)
            check_generation_report(manifest, report, scope, ROOT / "selections")
            for mutate in (
                lambda item: item.update(inputSha256="wrong"),
                lambda item: item.update(documentSuccess=True),
                lambda item: item["typecheck"].update(status="pass"),
                lambda item: item["generation"].update(status="fail"),
                lambda item: item["generation"].update(durationMillis=float("nan")),
                lambda item: item["operationEmission"].update(count=0),
            ):
                invalid = copy.deepcopy(report)
                mutate(invalid["documents"][0])
                with self.assertRaises(ValueError):
                    check_generation_report(manifest, invalid, scope, ROOT / "selections")

    def test_missing_and_duplicate_generation_evidence_is_rejected(self):
        manifest, report = self.fixture("full")
        for documents in ([], report["documents"] * 2):
            with self.assertRaises(ValueError):
                check_generation_report(manifest, dict(report, documents=documents), "full", ROOT / "selections")


if __name__ == "__main__":
    unittest.main()
