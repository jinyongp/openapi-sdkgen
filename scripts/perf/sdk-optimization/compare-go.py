"""Compare frozen final Go contracts independently of artifact filenames.

Reads audit-go JSONL, validates coverage against publisher manifests, and compares
defined type ownership and exported AST contracts as multisets. Supplemental
unlimited audits must replace precisely the matching capped source records.
"""
import argparse
import hashlib
import json
from pathlib import Path
import sqlite3
import tempfile


def sha(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def rows(path):
    with path.open() as stream:
        for line in stream:
            row = json.loads(line)
            if not row.get('summary'):
                yield row


def load(connection, side, audit, manifest_path, supplements):
    manifest = json.loads(manifest_path.read_text())['files']
    replacements = {}
    for supplement in supplements:
        for row in rows(supplement):
            if row['path'] in replacements:
                raise ValueError('duplicate supplemental source: ' + row['path'])
            replacements[row['path']] = row
    seen = set()
    count = 0
    for row in rows(audit):
        name = row['path']
        if name in seen or manifest.get(name) != row['sha256']:
            raise ValueError('duplicate/stale audit source: ' + name)
        seen.add(name)
        if any(reason.startswith('parse_cap_') for reason in row['reasons']):
            replacement = replacements.pop(name, None)
            if replacement is None or replacement['sha256'] != row['sha256'] or replacement['bytes'] != row['bytes']:
                raise ValueError('missing/stale unlimited source audit: ' + name)
            row = replacement
        if any(reason.startswith('parse_cap_') for reason in row['reasons']):
            raise ValueError('supplement still has parse cap: ' + name)
        if not row['package'] and row['reasons'] != ['go_ignored_filename']:
            raise ValueError('source package absent: ' + name)
        package = str(Path(name).parent) + ':' + row['package']
        # A facade can import hundreds of packages and export thousands of
        # methods. Never retain importKey + signature once per method: that
        # Cartesian-sized temporary can exceed the SDK compiler's own budget.
        prefix = hashlib.sha256()
        prefix.update(row['importKey'].encode())
        prefix.update(b'\x00')

        def entries():
            for name in row['definedTypes'] or []:
                yield side, package, 'type:' + name, hashlib.sha256(b'').hexdigest(), 1
            for name, signature in row['publicSignatures'].items():
                digest = prefix.copy()
                digest.update(signature.encode())
                yield side, package, 'public:' + name, digest.hexdigest(), 1

        connection.executemany('INSERT INTO contracts VALUES (?, ?, ?, ?, ?) ON CONFLICT(side, package, name, value) DO UPDATE SET occurrences=occurrences+excluded.occurrences', entries())
        count += len(row['definedTypes'] or []) + len(row['publicSignatures'])
    if replacements:
        raise ValueError('supplement contains uncapped/unknown source')
    if seen != {name for name in manifest if name.endswith('.go')}:
        raise ValueError('incomplete Go manifest coverage')
    connection.commit()
    return {'sources': len(seen), 'contracts': count, 'manifestSha256': sha(manifest_path),
            'auditSha256': sha(audit), 'supplementSha256': [sha(path) for path in supplements]}


def compare(baseline, candidate, baseline_manifest, candidate_manifest, supplements):
    with tempfile.TemporaryDirectory(prefix='sdk-go-contracts-') as directory:
        connection = sqlite3.connect(Path(directory) / 'contracts.db')
        connection.execute('PRAGMA cache_size=-32768')
        connection.execute('CREATE TABLE contracts(side INTEGER, package TEXT, name TEXT, value TEXT, occurrences INTEGER, PRIMARY KEY(side, package, name, value)) WITHOUT ROWID')
        first = load(connection, 0, baseline, baseline_manifest, supplements)
        second = load(connection, 1, candidate, candidate_manifest, [])
        differences = connection.execute('SELECT package, name, value, SUM(CASE side WHEN 0 THEN occurrences ELSE -occurrences END) AS delta FROM contracts GROUP BY package, name, value HAVING delta != 0 ORDER BY package, name, value')
        samples, total = [], 0
        for package, name, value, delta in differences:
            total += 1
            if len(samples) < 8:
                samples.append({'package': package, 'name': name, 'contractSha256': value, 'baselineMinusCandidate': delta})
        return {'status': 'pass' if total == 0 else 'fail', 'baseline': first, 'candidate': second,
                'differentContracts': total, 'samples': samples,
                'scope': 'all defined type package/name and public AST signatures; filenames ignored; not compiler/runtime proof'}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('baseline', 'candidate', 'baseline-manifest', 'candidate-manifest', 'output'):
        parser.add_argument('--' + name, type=Path, required=True)
    parser.add_argument('--baseline-supplement', type=Path, action='append', default=[])
    args = parser.parse_args()
    result = compare(args.baseline, args.candidate, args.baseline_manifest, args.candidate_manifest, args.baseline_supplement)
    with args.output.open('x') as stream:
        json.dump(result, stream, indent=2)
        stream.write('\n')
    print(json.dumps(result))
    return 0 if result['status'] == 'pass' else 1


if __name__ == '__main__':
    raise SystemExit(main())
