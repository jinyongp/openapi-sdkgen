import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('compare_go', Path(__file__).with_name('compare-go.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class FinalContractTests(unittest.TestCase):
    def test_packing_preserves_aliases_receivers_tags_and_defined_package(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)

            def write(name, records):
                file = root / name
                file.write_text(''.join(json.dumps(record) + '\n' for record in records))
                return file

            def row(path, types, public):
                return dict(path=path, package='p', bytes=10, sha256='source', reasons=[], importKey='',
                            definedTypes=types, publicSignatures=public)

            original = [row('internal/p/a.go', ['Node'], {'Node': 'Node struct { Value string `json:"value"` }', '*Node.Render': 'func (*Node) Render() string'}),
                        row('internal/p/b.go', ['B'], {'B': 'B int'}),
                        row('models/alias.go', [], {'Node': 'Node = p.Node'})]
            manifest = write('original-manifest.json', [{'files': {r['path']: r['sha256'] for r in original}}])
            baseline = write('baseline.jsonl', original)
            public = original[0]['publicSignatures'] | original[1]['publicSignatures']
            packed = [row('internal/p/packed.go', ['Node', 'B'], public), original[2]]
            target_manifest = write('target-manifest.json', [{'files': {r['path']: r['sha256'] for r in packed}}])

            def check():
                target = write('candidate.jsonl', packed)
                return module.compare(baseline, target, manifest, target_manifest, [])

            self.assertEqual(check()['status'], 'pass')
            packed[0]['importKey'] = 'p "example.test/changed"'
            self.assertEqual(check()['status'], 'fail')
            packed[0]['importKey'] = ''
            # Same public alias with the actual definition moved is incompatible.
            packed[0]['path'] = 'internal/other/packed.go'
            target_manifest.write_text(json.dumps({'files': {r['path']: r['sha256'] for r in packed}}))
            self.assertEqual(check()['status'], 'fail')
            packed[0]['path'] = 'internal/p/packed.go'
            target_manifest.write_text(json.dumps({'files': {r['path']: r['sha256'] for r in packed}}))
            packed[0]['publicSignatures'] = dict(public, **{'*Node.Render': 'func (*Node) Render() int'})
            self.assertEqual(check()['status'], 'fail')
            packed[0]['publicSignatures'] = dict(public, Node='Node struct { Value string `json:"other"` }')
            self.assertEqual(check()['status'], 'fail')

    def test_incomplete_stale_or_capped_audits_cannot_pass(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            row = dict(path='types.go', package='p', bytes=10, sha256='source', reasons=[], importKey='', definedTypes=['A'], publicSignatures={'A': 'A int'})
            manifest = root / 'manifest.json'
            manifest.write_text(json.dumps({'files': {'types.go': 'source'}}))
            audit, candidate, supplement = [root / name for name in ['audit.jsonl', 'candidate.jsonl', 'supplement.jsonl']]
            candidate.write_text(json.dumps(row) + '\n')
            audit.write_text(json.dumps(dict(row, package='', reasons=['parse_cap_8MiB'], definedTypes=None, publicSignatures={})) + '\n')
            with self.assertRaisesRegex(ValueError, 'missing/stale unlimited'):
                module.compare(audit, candidate, manifest, manifest, [])
            supplement.write_text(json.dumps(row) + '\n')
            self.assertEqual(module.compare(audit, candidate, manifest, manifest, [supplement])['status'], 'pass')
            supplement.write_text(json.dumps(dict(row, sha256='changed')) + '\n')
            with self.assertRaisesRegex(ValueError, 'missing/stale unlimited'):
                module.compare(audit, candidate, manifest, manifest, [supplement])
            audit.write_text('')
            with self.assertRaisesRegex(ValueError, 'incomplete Go manifest'):
                module.compare(audit, candidate, manifest, manifest, [])
            audit.write_text(json.dumps(row) + '\n' + json.dumps(row) + '\n')
            with self.assertRaisesRegex(ValueError, 'duplicate/stale audit'):
                module.compare(audit, candidate, manifest, manifest, [])


if __name__ == '__main__':
    unittest.main()
