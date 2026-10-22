"""Cache boundary checks without compiling the entire optional benchmark matrix."""
import importlib.util
import os
from pathlib import Path
import tempfile
import unittest
import sys
from unittest import mock

SCRIPT = Path(__file__).with_name('measure.py')
spec = importlib.util.spec_from_file_location('go_measurement', SCRIPT)
measurement = importlib.util.module_from_spec(spec)
spec.loader.exec_module(measurement)


class CacheFixture(measurement.Measurement):
    def __init__(self, output, fail=None, cache=None):
        self.output, self.fail, self.results = output, fail, []
        self.environment = {**os.environ, 'GOCACHE': str(cache or output / 'caller-cache')}
        self.caches = []
        self.max_owned_caches = 0

    def generate(self, label, input_file, module, extra=()):
        sdk = module / 'api'
        sdk.mkdir(exist_ok=True)
        self.results.append({'output': {'sha256': 'identical-fixture'}})
        return sdk

    def run(self, label, argv, cwd, env=None, **kwargs):
        if label.startswith('build-full-static-'):
            cache = Path(env['GOCACHE'])
            marker = cache / 'compiled'
            if '-cold' in label:
                if any(cache.iterdir()):
                    raise AssertionError('cold cache was not empty')
                marker.write_bytes(b'cached object')
                self.caches.append(cache)
                self.max_owned_caches = max(self.max_owned_caches,
                    len(list(self.output.glob('cache-*'))) + len(list(self.output.glob('go-cache-*'))))
            elif marker.read_bytes() != b'cached object':
                raise AssertionError('warm build did not inherit cold cache')
            if self.fail and self.fail in label:
                raise SystemExit('controlled compiler failure')
        if argv[:2] == ['go', 'build'] and '-o' in argv:
            Path(argv[argv.index('-o') + 1]).write_bytes(b'consumer')
        self.results.append({'exitCode': 0})
        return self.results[-1]


class CacheLifetimeTests(unittest.TestCase):
    def test_build_pair_cache_is_reclaimed_after_cold_or_warm_failure(self):
        for failure in ('-cold0', '-warm0'):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                runner = CacheFixture(root, failure)
                with self.assertRaises(SystemExit):
                    runner.builds('full-static')
                self.assertTrue(runner.caches)
                self.assertFalse(any(path.exists() for path in runner.caches),
                                 'failed consumer left a dedicated Go cache')

    def test_successive_pairs_start_empty_and_reclaim_their_cache(self):
        with tempfile.TemporaryDirectory() as directory:
            runner = CacheFixture(Path(directory))
            runner.builds('full-static')
            self.assertEqual(len(runner.caches), 5)
            self.assertEqual(len(set(runner.caches)), 5)
            self.assertFalse(any(path.exists() for path in runner.caches))

    def test_main_owns_default_cache_but_preserves_a_callers_cache(self):
        for borrowed in (False, True):
            with self.subTest(borrowed=borrowed), tempfile.TemporaryDirectory() as directory:
                parent = Path(directory)
                caller = parent / 'caller'
                caller.mkdir()
                sentinel = caller / 'keep'
                sentinel.write_bytes(b'caller-owned')
                environment = dict(os.environ)
                environment.pop('GOCACHE', None)
                if borrowed:
                    environment['GOCACHE'] = str(caller)
                runners = []
                def create(output, cache):
                    runner = CacheFixture(output, cache=cache)
                    runners.append(runner)
                    return runner
                with mock.patch.dict(os.environ, environment, clear=True), mock.patch.object(measurement, 'Measurement', side_effect=create):
                    measurement.main(['builds', '--variant', 'full-static', '--output', str(parent / 'study')])
                runner = runners[0]
                cache = Path(runner.environment['GOCACHE'])
                self.assertEqual(cache.exists(), borrowed)
                self.assertEqual(sentinel.read_bytes(), b'caller-owned')
                self.assertLessEqual(runner.max_owned_caches, 1 if borrowed else 2)

    def test_default_cache_is_reclaimed_when_generator_build_fails(self):
        with tempfile.TemporaryDirectory() as directory:
            environment = dict(os.environ)
            environment.pop('GOCACHE', None)
            paths = []
            def fail(output, cache):
                paths.append(Path(cache))
                (Path(cache) / 'partial').write_bytes(b'partial build')
                raise SystemExit('generator build failed')
            with mock.patch.dict(os.environ, environment, clear=True), mock.patch.object(measurement, 'Measurement', side_effect=fail):
                with self.assertRaises(SystemExit):
                    measurement.main(['runtime', '--output', str(Path(directory) / 'study')])
            self.assertTrue(paths)
            self.assertFalse(paths[0].exists())

    def test_timeout_is_recorded_before_owned_cache_cleanup(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            ready = root / 'writer'
            with measurement.go_cache(root) as cache:
                runner = measurement.Measurement.__new__(measurement.Measurement)
                runner.output, runner.results = root, []
                runner.environment = {**os.environ, 'GOCACHE': cache, 'GOMEMLIMIT': '2GiB'}
                code = ('import os,sys,time;from pathlib import Path;'
                        'Path(sys.argv[1]).write_text(str(os.getpid()));'
                        'Path(sys.argv[2]).write_text("cache data");time.sleep(60)')
                with self.assertRaises(SystemExit):
                    runner.run('timeout', [sys.executable, '-c', code, str(ready), str(Path(cache) / 'object')], root, timeout=.3)
                self.assertEqual(runner.results[-1]['exitCode'], 124)
                self.assertEqual(runner.results[-1]['stoppedReason'], 'measurement deadline')
                self.assertTrue(Path(cache).exists())
                pid = int(ready.read_text())
                status = Path('/proc') / str(pid) / 'stat'
                if status.exists():
                    self.assertEqual(status.read_text().rsplit(')', 1)[1].split()[0], 'Z')
            self.assertFalse(Path(cache).exists())


if __name__ == '__main__':
    unittest.main()
