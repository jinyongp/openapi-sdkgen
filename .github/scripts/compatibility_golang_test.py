import copy
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
from compatibility_golang import combine


class GoEvidenceTests(unittest.TestCase):
    def test_real_compile_outcomes_and_proof_tampering(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            manifests = root / 'test/compatibility'
            manifests.mkdir(parents=True)
            entry = dict(id='provider', input='api.json', sha256='a' * 64)
            (manifests / 'test.json').write_text(json.dumps(dict(corpora=[entry])))
            directory = root / 'document-shard-test-provider/compatibility-ci'
            directory.mkdir(parents=True)
            path = directory / 'golang.json'
            report = dict(sourceCommit='b' * 40, sourceDirty=False, verificationRequested=True,
                          id='provider', input='api.json', inputSha256='a' * 64,
                          goCompilationPolicy=dict(timeoutSeconds=1200, buildParallelism=1, cpuConcurrency=2, memoryLimit="3GiB"),
                          mode='golang-generation', addons=['server'], **{'pass': True},
                          golang=dict(blocked=False, compilation=dict(status='pass')))
            with patch('compatibility_golang.CORPORA', ['test']):
                def check(value):
                    path.write_text(json.dumps(value))
                    return combine(root, root, 'b' * 40)
                self.assertEqual(len(check(report)['documents']), 1)
                for mutation in [dict(sourceDirty=True), dict(sourceCommit='c' * 40),
                                 dict(verificationRequested=False), dict(goCompilationPolicy={}), dict(selection=['GET /x']),
                                 dict(golang=dict(blocked=False)),
                                 dict(golang=dict(blocked=True, generationError='bad source'))]:
                    with self.assertRaises(ValueError):
                        check(dict(report, **mutation))
                blocked = dict(report, **{'pass': False}, failure='generation blocked',
                               golang=dict(blocked=True, generationError='invalid pattern'))
                self.assertFalse(check(blocked)['documents'][0]['pass'])
                failed = dict(report, **{'pass': False}, failure='compile failed',
                              golang=dict(blocked=False, compilation=dict(status='fail', detail='syntax error')))
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


if __name__ == '__main__':
    unittest.main()
