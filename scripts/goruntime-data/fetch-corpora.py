"""Fetch pinned specification fixtures; ordinary builds/tests never fetch data."""
import argparse
import hashlib
import io
import json
from pathlib import Path
import tarfile
import urllib.request

ROOT = Path(__file__).resolve().parents[2]
SOURCES = {
    'test262': ('tc39/test262', 'd53ebc8b1931d9f1541e43d59650776e4b766248'),
    'wpt': ('web-platform-tests/wpt', '6cd3f1c39e2aec3d58f140aa0ff2991d79eb9991'),
}


def fetch(url):
    request = urllib.request.Request(url, headers={'User-Agent': 'openapi-sdkgen-fixture-preparation'})
    with urllib.request.urlopen(request, timeout=120) as response:
        return response.read()


def write_checked(destination, data):
    if destination.exists() and destination.read_bytes() != data:
        raise SystemExit(f'Refusing to overwrite different fixture: {destination}')
    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_bytes(data)
    return hashlib.sha256(data).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    entries = []
    # A scoped extraction avoids depending on GitHub's truncated whole-tree API.
    repo, pin = SOURCES['test262']
    archive = fetch(f'https://codeload.github.com/{repo}/tar.gz/{pin}')
    with tarfile.open(fileobj=io.BytesIO(archive), mode='r:gz') as bundle:
        for member in bundle:
            path = member.name.partition('/')[2]
            selected = path.startswith(('test/built-ins/RegExp/', 'test/language/literals/regexp/', 'harness/')) or path == 'LICENSE'
            if not member.isfile() or not selected:
                continue
            if '..' in Path(path).parts or Path(path).is_absolute():
                raise SystemExit(f'Unsafe archive path: {path}')
            data = bundle.extractfile(member).read()
            sha = write_checked(output / 'test262' / path, data)
            entries.append({'suite': 'test262', 'path': path, 'sha256': sha, 'bytes': len(data)})
    # WPT is much larger; retrieve the URL directory tree and its data/JS tests only.
    repo, pin = SOURCES['wpt']
    listing = json.loads(fetch(f'https://api.github.com/repos/{repo}/contents/url?ref={pin}'))
    directory = next(item for item in listing if item['name'] == 'resources')
    tree = json.loads(fetch(directory['git_url'] + '?recursive=1'))
    if tree.get('truncated'):
        raise SystemExit('WPT URL resource tree is incomplete')
    paths = ['LICENSE.md', 'url/README.md']
    paths += ['url/resources/' + item['path'] for item in tree['tree'] if item['type'] == 'blob' and item['path'].endswith(('.json', '.js'))]
    for path in paths:
        data = fetch(f'https://raw.githubusercontent.com/{repo}/{pin}/{path}')
        sha = write_checked(output / 'wpt' / path, data)
        entries.append({'suite': 'wpt', 'path': path, 'sha256': sha, 'bytes': len(data)})
    manifest = {'sources': {name: {'repository': repo, 'commit': pin} for name, (repo, pin) in SOURCES.items()}, 'files': sorted(entries, key=lambda row: (row['suite'], row['path']))}
    write_checked(output / 'manifest.json', (json.dumps(manifest, indent=2) + '\n').encode())
    print(json.dumps({'files': len(entries), 'manifest': str(output / 'manifest.json')}))


if __name__ == '__main__':
    main()
