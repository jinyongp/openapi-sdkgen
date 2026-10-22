"""Recovery keeps completed evidence and scopes each missing Go pair's cache."""
from contextlib import contextmanager
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('resume_study', HERE / 'resume-apis.py')
resume = importlib.util.module_from_spec(spec)
spec.loader.exec_module(resume)
study = resume.study


class RecoveryCacheTests(unittest.TestCase):
    def fixture(self, root, missing=False):
        source = root / 'input.json'
        source.write_text('{}')
        provenance = {'producerSha256': study.sha(resume.WORKER),
            'recorderSha256': study.sha(HERE / 'record.py'),
            'resourcePolicySha256': study.sha(resume.POLICY_PATH),
            'revisions': study.REVISIONS, 'status': 'running', 'pairs': 1,
            'environment': {}, 'tools': {'go': 'fixture-go'},
            'inputs': {'tiny': {'path': str(source), 'sha256': study.sha(source)}}}
        for variant in study.REVISIONS:
            binary = root / ('sdkgen-' + variant)
            binary.write_bytes(b'generator fixture')
            provenance[variant] = {'binarySha256': study.sha(binary)}
        rows = [{'provider': 'tiny', 'target': target, 'pair': 0, 'variant': variant,
                 'phase': phase, 'status': 'pass', 'seconds': 1,
                 'peakSessionRSSBytes': 1, 'peakProcessRSSBytes': 1}
                for target in ('typescript', 'golang') for variant in study.REVISIONS
                for phase in ('generation', 'cold', 'warm')
                if not (missing and target == 'golang' and variant == 'baseline')]
        study.dump(root / 'provenance.json', provenance)
        study.dump(root / 'trials.json', rows)
        study.dump(root / 'summary.json', {'inventories': []})
        return provenance, rows

    def test_resource_policy_mismatch_stops_before_recovery(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            provenance, _ = self.fixture(root)
            provenance['resourcePolicySha256'] = 'different-policy'
            study.dump(root / 'provenance.json', provenance)
            with self.assertRaises(AssertionError):
                resume.resume(root, 'unused-cache')
            self.assertFalse((root / 'recovery').exists())

    def test_complete_cases_are_preserved_without_reexecuting_commands(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            _, rows = self.fixture(root)
            with mock.patch.object(resume.subprocess, 'check_output', return_value='fixture-go'), \
                 mock.patch.object(resume, 'owned_process', side_effect=AssertionError('reexecuted completed evidence')):
                self.assertEqual(resume.resume(root, 'current-run-cache'), 0)
            self.assertEqual(json.loads((root / 'trials.json').read_text()), rows)

    def test_missing_pair_reclaims_cache_on_success_cold_failure_and_warm_failure(self):
        for failed_phase in (None, 'cold', 'warm'):
            with self.subTest(failed_phase=failed_phase), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                _, before = self.fixture(root, missing=True)
                caches, phases = [], []
                @contextmanager
                def execute(invocation, cwd, env):
                    record = Path(invocation[invocation.index('--output') + 1])
                    record.mkdir()
                    index = invocation.index('_child')
                    metrics, command = Path(invocation[index + 1]), invocation[index + 2:]
                    phase = record.name
                    phases.append(phase)
                    if phase == 'generation':
                        sdk = Path(command[command.index('--output') + 1])
                        sdk.mkdir()
                        (sdk / 'value.go').write_text('package api\n')
                        study.dump(sdk / '.openapi-sdkgen-manifest.json', {'files': {'value.go': study.sha(sdk / 'value.go')}})
                    else:
                        cache = Path(env['GOCACHE'])
                        marker = cache / 'compiled'
                        if phase == 'cold':
                            self.assertEqual(list(cache.iterdir()), [])
                            marker.write_bytes(b'compiler cache')
                            caches.append(cache)
                        else:
                            self.assertEqual(cache, caches[0])
                            self.assertEqual(marker.read_bytes(), b'compiler cache')
                    status = 'fail' if phase == failed_phase else 'pass'
                    study.dump(record / 'result.json', {'status': status, 'peakSessionRSSBytes': 1})
                    study.dump(metrics, {'seconds': 1, 'peakProcessRSSBytes': 1, 'exitCode': 1 if status == 'fail' else 0})
                    yield mock.Mock(wait=lambda: 0)
                with mock.patch.object(resume.subprocess, 'check_output', return_value='fixture-go'), \
                     mock.patch.object(resume, 'owned_process', side_effect=execute):
                    self.assertEqual(resume.resume(root, 'run-cache'), 0 if failed_phase is None else 1)
                self.assertEqual(phases, ['generation', 'cold'] if failed_phase == 'cold' else ['generation', 'cold', 'warm'])
                self.assertTrue(caches)
                self.assertFalse(caches[0].exists())
                after = json.loads((root / 'trials.json').read_text())
                self.assertEqual(after[:len(before)], before)


if __name__ == '__main__':
    unittest.main()
