"""Verify archived measurements without relying on original temporary paths."""
import hashlib
import json
from pathlib import Path
import tarfile

ROOT = Path(__file__).resolve().parent

def digest(stream):
    value = hashlib.sha256()
    for chunk in iter(lambda: stream.read(1024 * 1024), b""):
        value.update(chunk)
    return value.hexdigest()

def main():
    index = json.loads((ROOT / "index.json").read_text())
    count = 0
    for archive in index["archives"]:
        path = ROOT / archive["path"]
        with path.open("rb") as stream:
            assert digest(stream) == archive["sha256"], archive["path"]
        expected = {entry["archiveMember"]: entry for entry in index["files"] if entry["archive"] == archive["path"]}
        with tarfile.open(path, "r:gz") as bundle:
            members = bundle.getmembers()
            assert len(members) == len(expected), archive["path"]
            for member in members:
                entry = expected.pop(member.name)
                assert member.isfile() and member.size == entry["bytes"], member.name
                with bundle.extractfile(member) as stream:
                    assert digest(stream) == entry["sha256"], member.name
                count += 1
            assert not expected, archive["path"]
    assert count == len(index["files"])
    print(f"Verified {len(index['archives'])} archives and {count} original files.")

if __name__ == "__main__":
    main()
