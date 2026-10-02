import copy
import json
from pathlib import Path
import tempfile
import unittest

from compatibility_summary import read_resources, render_preparation, render_report


class SummaryTests(unittest.TestCase):
    def test_resources_preserve_units_elapsed_and_sampled_memory(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            (directory / "resources.txt").write_text("""
User time (seconds): 120.5
System time (seconds): 3.25
Percent of CPU this job got: 126%
Elapsed (wall clock) time (h:mm:ss or m:ss): 1:02:03.50
Maximum resident set size (kbytes): 1048576
Major (requiring I/O) page faults: 17
File system inputs: 18
File system outputs: 19
Exit status: 7
""")
            samples = [dict(at="first", memory=dict(MemTotalKiB=4096, MemAvailableKiB=2048, SwapTotalKiB=1024, SwapFreeKiB=1024)),
                       dict(at="second", memory=dict(MemTotalKiB=4096, MemAvailableKiB=1024, SwapTotalKiB=1024, SwapFreeKiB=512),
                            terminatedChecker=dict(rssKiB=2048), reason="configured reserve")]
            (directory / "progress.jsonl").write_text("\n".join(json.dumps(sample) for sample in samples) + '\n{"partial":')
            result = read_resources(directory)
            self.assertEqual(result["wallSeconds"], 3723.5)
            self.assertEqual(result["cpuPercent"], 126)
            self.assertEqual(result["peakRssBytes"], 1024 ** 3)
            self.assertEqual(result["minimumAvailableRamBytes"], 1024 ** 2)
            self.assertEqual(result["peakSwapUsedBytes"], 512 * 1024)
            self.assertEqual(result["sampleCount"], 2)
            self.assertEqual(result["incompleteSamples"], 1)
            self.assertEqual(result["protection"][0]["rssBytes"], 2 * 1024 ** 2)
            self.assertEqual(result["exitCode"], 7)

    def test_missing_resources_are_unknown(self):
        with tempfile.TemporaryDirectory() as temporary:
            self.assertEqual(read_resources(Path(temporary)), {})

    def test_real_report_preserves_failed_check_and_generation_measurements(self):
        root = Path(__file__).resolve().parents[2]
        manifest = json.loads((root / "test/compatibility/regression.json").read_text())
        report = json.loads((root / "test/compatibility/regression-results.json").read_text())
        graph = next(item for item in report["documents"] if item["id"] == "microsoft-graph-beta")
        graph["typecheck"]["durationMillis"] = 2500
        graph["typecheck"]["detail"] = "</pre><script>bad</script>"
        summary = render_report(manifest, report, {graph["id"]: dict(peakRssBytes=1024 ** 3, wallSeconds=60, cpuPercent=120)})
        self.assertIn("6/7 client SDKs verified", summary)
        self.assertIn(f"{graph['generation']['artifactCount']:,}", summary)
        self.assertIn(f"{graph['generation']['artifactBytes'] / 1024 ** 3:.2f} GiB", summary)
        self.assertIn("2.50 s", summary)
        self.assertIn("1.00 GiB", summary)
        self.assertIn("runner snapshots", summary)
        self.assertNotIn("<script>", summary)
        self.assertIn("&lt;script&gt;", summary)
        self.assertFalse(graph["documentSuccess"])
        unknown = copy.deepcopy(report)
        unknown.pop("measurement")
        self.assertNotIn("Varies by runner", render_report(manifest, unknown))

    def test_preparation_counts_exact_pinned_files(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "root.json").write_bytes(b"123")
            (root / "aux.json").write_bytes(b"12345")
            manifest = dict(corpora=[dict(input="root.json")], files=[dict(input="aux.json")])
            summary = render_preparation(manifest, root, "Artifact reused", 1.5)
            self.assertIn("Artifact reused | 1 | 1 | 8 B | 1.50 s", summary)


if __name__ == "__main__":
    unittest.main()
