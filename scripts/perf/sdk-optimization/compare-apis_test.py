"""Checks for pairing failures and the fresh-process timing/RSS boundary."""
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

SCRIPT = Path(__file__).with_name('compare-apis.py')
spec = importlib.util.spec_from_file_location('compare_apis', SCRIPT)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class ComparisonTests(unittest.TestCase):
    def test_failed_and_unpaired_trials_do_not_create_improvement_samples(self):
        def row(pair, variant, seconds, status='pass'):
            return {'provider': 'example', 'target': 'golang', 'phase': 'cold',
                    'pair': pair, 'variant': variant, 'status': status, 'seconds': seconds,
                    'peakProcessRSSBytes': 100, 'peakSessionRSSBytes': 200}
        result = module.summarize([row(0, 'baseline', 10), row(0, 'final', 8),
                                   row(1, 'baseline', 1, 'timeout'), row(1, 'final', 1),
                                   row(2, 'baseline', 20), row(2, 'final', 10)])[0]
        self.assertEqual(result['pairedTimeRatios'], [0.8, 0.5])
        self.assertEqual(result['variants']['baseline']['completed'], 2)
        self.assertEqual(result['variants']['baseline']['seconds']['median'], 15)
        self.assertGreater(result['pairedGeometricTimeRatio'], 0.5)
        self.assertLess(result['pairedGeometricTimeRatio'], 0.8)

    def test_child_preserves_failure_and_records_actual_peak(self):
        with tempfile.TemporaryDirectory() as directory:
            metrics = Path(directory) / 'metrics.json'
            result = subprocess.run([sys.executable, '-B', str(SCRIPT), '_child', str(metrics),
                                     sys.executable, '-c', 'x = bytearray(32 * 1024 * 1024); raise SystemExit(7)'])
            value = json.loads(metrics.read_text())
            self.assertEqual(result.returncode, 7)
            self.assertEqual(value['exitCode'], 7)
            self.assertGreater(value['peakProcessRSSBytes'], 32 * 1024 * 1024)
            self.assertGreater(value['seconds'], 0)


if __name__ == '__main__':
    unittest.main()
