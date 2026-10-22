"""Checks for pairing failures and the fresh-process timing/RSS boundary."""
import importlib.util
from contextlib import contextmanager
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import tarfile
import unittest
from unittest import mock

SCRIPT = Path(__file__).with_name('compare-apis.py')
spec = importlib.util.spec_from_file_location('compare_apis', SCRIPT)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class ComparisonTests(unittest.TestCase):
    def test_driver_reclaims_each_go_pair_cache_on_success_and_failure(self):
        for failed_phase in (None, 'cold', 'warm'):
            with self.subTest(failed_phase=failed_phase), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                source = root / 'input.json'
                source.write_text('{}')
                compiler = root / 'test/typescript/node_modules/typescript-5-9'
                (compiler / 'lib').mkdir(parents=True)
                (compiler / 'package.json').write_text('{"version":"fixture"}')
                (compiler / 'lib/tsc.js').write_text('// fixture')
                archive = io.BytesIO()
                with tarfile.open(fileobj=archive, mode='w') as stream:
                    data = b'module example.test/fixture\n'
                    item = tarfile.TarInfo('go.mod')
                    item.size = len(data)
                    stream.addfile(item, io.BytesIO(data))
                caller = root / 'caller-cache'
                caller.mkdir()
                (caller / 'keep').write_bytes(b'caller-owned')
                output = root / 'study'
                caches, failed = [], []
                def check_output(command, **kwargs):
                    if command[:2] == ['git', 'archive']:
                        return archive.getvalue()
                    return 'fixture-tool'
                @contextmanager
                def execute(invocation, cwd, env):
                    record = Path(invocation[invocation.index('--output') + 1])
                    record.mkdir()
                    index = invocation.index('_child')
                    metrics, command = Path(invocation[index + 1]), invocation[index + 2:]
                    if command[:2] == ['go', 'build'] and '-o' in command:
                        Path(command[command.index('-o') + 1]).write_bytes(b'generator fixture')
                    if len(command) > 1 and command[1] == 'generate':
                        sdk = Path(command[command.index('--output') + 1])
                        sdk.mkdir()
                        (sdk / 'value').write_bytes(b'fixture output')
                        module.dump(sdk / '.openapi-sdkgen-manifest.json', {'files': {'value': module.sha(sdk / 'value')}})
                    status = 'pass'
                    if record.name in ('cold', 'warm'):
                        if '-golang-' in record.parent.name:
                            cache = Path(env['GOCACHE'])
                            marker = cache / 'compiled'
                            if record.name == 'cold':
                                self.assertEqual(list(cache.iterdir()), [])
                                marker.write_bytes(b'compiler cache')
                                caches.append(cache)
                            else:
                                self.assertEqual(cache, caches[-1])
                                self.assertEqual(marker.read_bytes(), b'compiler cache')
                            if record.name == failed_phase and not failed:
                                failed.append(record)
                                status = 'fail'
                        else:
                            self.assertEqual(env['GOCACHE'], str(caller))
                            self.assertEqual(list(record.parent.glob('cache-*')), [])
                    module.dump(record / 'result.json', {'status': status, 'peakSessionRSSBytes': 1})
                    module.dump(metrics, {'seconds': 1, 'peakProcessRSSBytes': 1, 'exitCode': 1 if status == 'fail' else 0})
                    yield mock.Mock(wait=lambda: 0)
                with mock.patch.object(module, 'ROOT', root), \
                     mock.patch.object(module, 'PROVIDERS', {'tiny': ('input.json', module.sha(source))}), \
                     mock.patch.object(module.subprocess, 'check_output', side_effect=check_output), \
                     mock.patch.object(module, 'owned_process', side_effect=execute), \
                     mock.patch.dict(os.environ, {'GOCACHE': str(caller)}), \
                     mock.patch.object(sys, 'argv', [str(SCRIPT), '--output', str(output), '--pairs', '2']):
                    self.assertEqual(module.main(), 0 if failed_phase is None else 1)
                self.assertTrue(caches)
                self.assertFalse(any(cache.exists() for cache in caches))
                self.assertEqual(len(caches), 4 if failed_phase is None else 2)
                self.assertEqual((caller / 'keep').read_bytes(), b'caller-owned')
                provenance = json.loads((output / 'provenance.json').read_text())
                self.assertEqual(provenance['resourcePolicySha256'], module.sha(module.POLICY_PATH))

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
