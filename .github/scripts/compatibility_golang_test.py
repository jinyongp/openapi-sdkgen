import copy
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
from compatibility_golang import combine, input_pin


class GoEvidenceTests(unittest.TestCase):
    def test_provider_http_contracts_require_the_actual_named_test(self):
        for provider, name in [('loket.nl', 'TestLoketHTTPContract'), ('stripe', 'TestStripeHTTPContract'),
                               ('cloudflare', 'TestCloudflareHTTPContract')]:
            with self.subTest(provider=provider), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                manifests = root / 'test/compatibility'
                manifests.mkdir(parents=True)
                entry = dict(id=provider, input='api.json', sha256='a' * 64)
                (manifests / 'test.json').write_text(json.dumps(dict(corpora=[entry])))
                report = dict(sourceCommit='b' * 40, sourceDirty=False, verificationRequested=True,
                              id=provider, input='api.json', inputSha256='a' * 64,
                              goCompilationPolicy=dict(timeoutSeconds=1200, buildParallelism=1, cpuConcurrency=4, memoryLimit='6GiB'),
                              mode='golang-generation', addons=['server'], **{'pass': True},
                              golang=dict(blocked=False, files=1, bytes=10, artifactSha256='e' * 64, compilation=dict(status='pass')))
                checkpoint = copy.deepcopy(report)
                checkpoint.update(generationCheckpoint=True, **{'pass': False})
                checkpoint['golang']['compilation'] = dict(status='not-run')
                directory = root / f'golang-shard-test-{provider}'
                directory.mkdir()
                checkpoint_directory = root / f'golang-generation-shard-test-{provider}'
                checkpoint_directory.mkdir()
                (checkpoint_directory / 'golang-generation.json').write_text(json.dumps(checkpoint))
                with patch('compatibility_golang.CORPORA', ['test']):
                    for completed in [[], ['TestUnrelatedHTTPContract'], [name]]:
                        report['golang']['compilation']['contractTests'] = completed
                        (directory / 'golang.json').write_text(json.dumps(report))
                        if completed == [name]:
                            self.assertTrue(combine(root, root, 'b' * 40)['documents'][0]['pass'])
                        else:
                            with self.assertRaisesRegex(ValueError, 'HTTP contract'):
                                combine(root, root, 'b' * 40)

    def test_git_blob_pins_bind_the_reported_sha256_to_original_bytes(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / 'test').mkdir()
            path = root / 'test/api.json'
            path.write_bytes(b'hello\n')
            entry = dict(input='api.json', gitBlob='ce013625030ba8dba906f756967f9e9ca394464a')
            pin = input_pin(entry, 'test', root)
            self.assertEqual(pin['kind'], 'git-blob')
            self.assertEqual(pin['sha256'], '5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03')
            path.write_bytes(b'changed\n')
            with self.assertRaises(ValueError):
                input_pin(entry, 'test', root)
            with self.assertRaises(ValueError):
                input_pin(entry, 'test', None)
            with self.assertRaises(ValueError):
                input_pin(dict(entry, input='../outside'), 'test', root)

    def test_real_compile_outcomes_and_proof_tampering(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            manifests = root / 'test/compatibility'
            manifests.mkdir(parents=True)
            entry = dict(id='provider', input='api.json', sha256='a' * 64)
            (manifests / 'test.json').write_text(json.dumps(dict(corpora=[entry])))
            directory = root / 'golang-shard-test-provider'
            directory.mkdir(parents=True)
            path = directory / 'golang.json'
            checkpoint_dir = root / 'golang-generation-shard-test-provider'
            checkpoint_dir.mkdir()
            checkpoint_path = checkpoint_dir / 'golang-generation.json'
            report = dict(sourceCommit='b' * 40, sourceDirty=False, verificationRequested=True,
                          id='provider', input='api.json', inputSha256='a' * 64,
                          goCompilationPolicy=dict(timeoutSeconds=1200, buildParallelism=1, cpuConcurrency=4, memoryLimit="6GiB"),
                          mode='golang-generation', addons=['server'], **{'pass': True},
                          golang=dict(blocked=False, files=1, bytes=10, artifactSha256='e' * 64, compilation=dict(status='pass')))
            with patch('compatibility_golang.CORPORA', ['test']):
                def check(value):
                    path.write_text(json.dumps(value))
                    checkpoint = copy.deepcopy(value)
                    checkpoint.update(generationCheckpoint=True, **{'pass': False})
                    if not checkpoint['golang'].get('blocked'):
                        checkpoint['golang']['compilation'] = dict(status='not-run')
                    checkpoint_path.write_text(json.dumps(checkpoint))
                    return combine(root, root, 'b' * 40)
                self.assertEqual(len(check(report)['documents']), 1)
                for mutation in [dict(sourceDirty=True), dict(sourceCommit='c' * 40),
                                 dict(verificationRequested=False), dict(goCompilationPolicy={}),
                                 dict(goCompilationPolicy=dict(timeoutSeconds=1200, buildParallelism=1, cpuConcurrency=2, memoryLimit='3GiB')),
                                 dict(selection=['GET /x']),
                                 dict(golang=dict(blocked=False)),
                                 dict(golang=dict(blocked=True, generationError='bad source'))]:
                    with self.assertRaises(ValueError):
                        check(dict(report, **mutation))
                blocked = dict(report, **{'pass': False}, failure='generation blocked',
                               golang=dict(blocked=True, generationError='invalid pattern'))
                self.assertFalse(check(blocked)['documents'][0]['pass'])
                failed = dict(report, **{'pass': False}, failure='compile failed',
                              golang=dict(blocked=False, files=1, bytes=10, artifactSha256='e' * 64, compilation=dict(status='fail', detail='syntax error')))
                self.assertFalse(check(failed)['documents'][0]['pass'])
                expected = dict(kind='operation-references-v1', sha256='d' * 64, operations=2)
                entry['preparation'] = expected
                (manifests / 'test.json').write_text(json.dumps(dict(corpora=[entry])))
                report['inputPreparation'] = dict(expected, sourceSha256='a' * 64, contractsVerified=True)
                self.assertTrue(check(report)['documents'][0]['pass'])
                for field, value in [('sha256', 'f' * 64), ('sourceSha256', 'f' * 64),
                                     ('operations', 1), ('contractsVerified', False)]:
                    changed = copy.deepcopy(report)
                    changed['inputPreparation'][field] = value
                    with self.assertRaises(ValueError):
                        check(changed)

                check(report)
                changed = json.loads(checkpoint_path.read_text())
                changed['golang']['artifactSha256'] = 'f' * 64
                checkpoint_path.write_text(json.dumps(changed))
                with self.assertRaises(ValueError):
                    combine(root, root, 'b' * 40)
                check(report)
                path.unlink()
                job = dict(id=123, name='Measure test / provider / full', head_sha='b' * 40, run_id=456,
                           status='completed', conclusion='failure', steps=[dict(name='Compile the independent Go SDK profile', started_at='2026-10-20T00:00:00Z', conclusion='success')])
                missing = combine(root, root, 'b' * 40, jobs=[job], run_id=456)['documents'][0]
                self.assertFalse(missing['pass'])
                self.assertEqual(missing['golang']['compilation']['status'], 'unavailable')
                self.assertEqual(missing['compilerEvidenceUnavailable']['jobId'], 123)
                for change in [dict(head_sha='f' * 40), dict(run_id=999), dict(conclusion='success'), dict(steps=[])]:
                    with self.assertRaises(ValueError):
                        combine(root, root, 'b' * 40, jobs=[dict(job, **change)], run_id=456)
                for kind, name in [('digitalocean-tags-v1', 'TestDigitalOceanHTTPContract'),
                                   ('spoonacular-parameter-bodies-v1', 'TestSpoonacularHTTPContract')]:
                    expected['kind'] = kind
                    report['inputPreparation']['kind'] = kind
                    (manifests / 'test.json').write_text(json.dumps(dict(corpora=[entry])))
                    report['golang']['compilation'] = dict(status='pass')
                    with self.assertRaisesRegex(ValueError, 'HTTP contract'):
                        check(report)
                    report['golang']['compilation']['contractTests'] = [name]
                    self.assertTrue(check(report)['documents'][0]['pass'])


if __name__ == '__main__':
    unittest.main()
