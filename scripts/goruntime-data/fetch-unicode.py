"""Prepare hash-pinned Unicode data explicitly; ordinary builds stay offline."""
import argparse
import hashlib
import json
from pathlib import Path
import urllib.request

ROOT = Path(__file__).resolve().parents[2]
PIN = ROOT / 'plans/golang-zero-dependencies/unicode-inputs.json'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=True)
    manifest = json.loads(PIN.read_text())
    for row in manifest['inputs']:
        relative = row['url'].split('/17.0.0/', 1)[1]
        destination = args.output / relative
        data = destination.read_bytes() if destination.exists() else urllib.request.urlopen(row['url'], timeout=60).read()
        if len(data) != row['bytes'] or hashlib.sha256(data).hexdigest() != row['sha256']:
            raise SystemExit(f'Unicode input hash mismatch: {relative}')
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_bytes(data)
    print(f'Prepared {len(manifest["inputs"])} hash-verified Unicode {manifest["unicode"]} inputs')


if __name__ == '__main__':
    main()
