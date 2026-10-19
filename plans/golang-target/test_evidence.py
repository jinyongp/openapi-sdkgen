"""Regression checks for source/log binding, without running nested Go suites."""
import json
from pathlib import Path
import tempfile
import unittest

from record_evidence import commands, digest, snapshot, summarize
from verify_plan import local_links


class EvidenceChecks(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.folder = self.root / "plans/golang-target"
        (self.folder / "probes").mkdir(parents=True)
        (self.folder / "probes/test.go").write_text("package probes\n")
        (self.root / "docs").mkdir()
        (self.root / "docs/openapi-feature-manifest.json").write_text('{"features": []}')
        self.current = snapshot(self.folder, self.root)
        self.run = self.make_run("first")

    def make_run(self, name: str) -> Path:
        run = self.root / name
        run.mkdir()
        (run / "basis.json").write_text(json.dumps({"runID": name, "startedAt": "fixture", "sourceSha256": self.current}))
        events = [
            {"Action": "output", "Test": "TestRegex", "Output": "42 ECMAScript Unicode samples; standard regexp differences: 29; raw regexp2 differences: 7; goja differences: 3; selected differences: 0\n"},
            {"Action": "pass", "Test": "TestRegex"}, {"Action": "pass", "Elapsed": 1},
        ]
        outputs = {"list": "TestRegex\nok\n", "tests": "\n".join(map(json.dumps, events)),
                   "race": "\n".join(map(json.dumps, events)), "purego": "\n".join(map(json.dumps, events)), "go": "{}", "node": "{}", "deps": '{"Standard":true}',
                   "benchmark": "\n".join(f"BenchmarkRegexEngines/{variant}-8 100 1.0 ns/op 0 B/op 0 allocs/op"
                                             for variant in ["standard", "ecmascript", "ecmascript-utf8"] for _ in range(3)) + "\nPASS\n"}
        results = {}
        for check, command in commands().items():
            (run / f"{check}.stdout").write_text(outputs.get(check, ""))
            (run / f"{check}.stderr").write_text("")
            results[check] = {"command": command, "exitCode": 0,
                              "stdoutSha256": digest(run / f"{check}.stdout"), "stderrSha256": digest(run / f"{check}.stderr")}
        (run / "results.json").write_text(json.dumps(results))
        return run

    def update_output(self, check: str, text: str) -> None:
        path = self.run / f"{check}.stdout"
        path.write_text(text)
        results = json.loads((self.run / "results.json").read_text())
        results[check]["stdoutSha256"] = digest(path)
        (self.run / "results.json").write_text(json.dumps(results))

    def test_valid_run_is_derived(self) -> None:
        evidence = summarize(self.run, self.current)
        self.assertEqual(evidence["checks"]["tests"]["topLevelTests"], 1)
        self.assertEqual(evidence["regex"]["standardEngineDifferences"], 29)

    def test_old_logs_cannot_certify_new_source(self) -> None:
        (self.folder / "probes/test.go").write_text("invalid Go syntax\n")
        with self.assertRaisesRegex(AssertionError, "source changed"):
            summarize(self.run, snapshot(self.folder, self.root))

    def test_missing_or_failed_check_is_rejected(self) -> None:
        path = self.run / "results.json"
        original = json.loads(path.read_text())
        for mode in ["missing", "failed", "replaced"]:
            with self.subTest(mode=mode):
                results = json.loads(json.dumps(original))
                if mode == "missing":
                    del results["vet"]
                elif mode == "failed":
                    results["vet"]["exitCode"] = 1
                else:
                    results["vet"]["command"] = ["true"]
                path.write_text(json.dumps(results))
                with self.assertRaises(AssertionError):
                    summarize(self.run, self.current)

    def test_missing_or_skipped_test_is_rejected(self) -> None:
        original = (self.run / "tests.stdout").read_text()
        for mode in ["skip", "missing", "package-fail"]:
            with self.subTest(mode=mode):
                events = [json.loads(line) for line in original.splitlines()]
                if mode == "skip":
                    events[1]["Action"] = "skip"
                elif mode == "missing":
                    del events[1]
                else:
                    events[-1]["Action"] = "fail"
                self.update_output("tests", "\n".join(map(json.dumps, events)))
                with self.assertRaises(AssertionError):
                    summarize(self.run, self.current)

    def test_tampered_log_is_rejected(self) -> None:
        (self.run / "race.stdout").write_text("replaced log")
        with self.assertRaisesRegex(AssertionError, "changed log"):
            summarize(self.run, self.current)

    def test_incomplete_benchmark_is_rejected(self) -> None:
        self.update_output("benchmark", "PASS\n")
        with self.assertRaises(AssertionError):
            summarize(self.run, self.current)

    def test_concurrent_runs_do_not_share_logs(self) -> None:
        other = self.make_run("second")
        (self.run / "vet.stdout").write_text("changed first run")
        self.assertEqual(summarize(other, self.current)["runID"], "second")
        with self.assertRaises(AssertionError):
            summarize(self.run, self.current)

    def test_markdown_generics_are_not_links(self) -> None:
        document = "`ValueFrom[T any](T)` and [plan](plan.md)\n\n```go\nfunc F[T any](T) {}\n```\n"
        self.assertEqual(local_links(document), ["plan.md"])


if __name__ == "__main__":
    unittest.main()
