"""Archive local measurement evidence without generated binaries/build caches.

Usage: python3 record_measurements.py DEST NAME=RUN_DIRECTORY ...
Large repeated hash maps are losslessly interned. Originals remain untouched.
"""
import gzip
import hashlib
import io
import json
from pathlib import Path
import sys
import tarfile


def sha(data):
    return hashlib.sha256(data).hexdigest()


def encoded(value):
    return (json.dumps(value, indent=2, ensure_ascii=True) + "\n").encode()


def compact(value, trees):
    if isinstance(value, dict):
        if len(value) > 30 and all(isinstance(v, str) and len(v) == 64
                                   and all(c in "0123456789abcdef" for c in v)
                                   for v in value.values()):
            key = sha(encoded(value))
            trees[key] = value
            return {"$measurementHashMap": key}
        return {k: compact(v, trees) for k, v in value.items()}
    if isinstance(value, list):
        return [compact(v, trees) for v in value]
    return value


def expand(value, trees):
    if isinstance(value, dict):
        if set(value) == {"$measurementHashMap"}:
            return trees[value["$measurementHashMap"]]
        return {k: expand(v, trees) for k, v in value.items()}
    if isinstance(value, list):
        return [expand(v, trees) for v in value]
    return value


def record(destination, name, root):
    target = destination / (name + ".tar.gz")
    if target.exists():
        raise SystemExit(f"Refusing to overwrite {target}")
    trees, files = {}, []
    # Top-level harness outputs contain exact commands, results, metrics, raw
    # stdout/stderr, provider reports and profiles. SDK trees are represented by
    # their complete hash maps, not copied a second time.
    paths = [p for p in sorted(root.iterdir()) if p.is_file() and
             (p.suffix in {".json", ".stdout", ".stderr", ".pprof", ".txt", ".log",
                           ".go", ".py", ".tmpl", ".sh"}
              or p.name.endswith(".tar.gz"))]
    payloads = []
    for path in paths:
        original = path.read_bytes()
        data, transformed, semantic_hash = original, False, None
        if path.suffix == ".json":
            try:
                value = json.loads(original)
            except json.JSONDecodeError:
                pass  # Failed commands can leave incomplete output; preserve it.
            else:
                reduced = compact(value, trees)
                assert expand(reduced, trees) == value
                data, transformed = encoded(reduced), True
                semantic_hash = sha(encoded(value))
        files.append({"path": path.name, "originalBytes": len(original),
                      "originalSha256": sha(original), "archivedSha256": sha(data),
                      "jsonReencoded": transformed, "expandedJSONSha256": semantic_hash})
        payloads.append((path.name, data))
    payloads += [("hash-maps.json", encoded(trees)), ("files.json", encoded(files))]
    # Deterministic gzip/tar metadata; do not preserve host usernames or mtimes.
    with target.open("wb") as output:
        with gzip.GzipFile(fileobj=output, mode="wb", mtime=0, filename="") as zipped:
            with tarfile.open(fileobj=zipped, mode="w") as bundle:
                for path, data in payloads:
                    info = tarfile.TarInfo(path)
                    info.size, info.mode = len(data), 0o644
                    bundle.addfile(info, io.BytesIO(data))
    verify(target)
    return {"name": name, "sourceDirectory": str(root.resolve()),
            "archive": target.name, "sha256": sha(target.read_bytes()),
            "files": len(files), "hashMaps": len(trees)}


def verify(path):
    with tarfile.open(path, "r:gz") as bundle:
        def read(name):
            return bundle.extractfile(name).read()
        trees = json.loads(read("hash-maps.json"))
        files = json.loads(read("files.json"))
        for key, value in trees.items():
            assert sha(encoded(value)) == key
        for entry in files:
            data = read(entry["path"])
            assert sha(data) == entry["archivedSha256"], entry["path"]
            if entry["jsonReencoded"]:
                # Preserve parsed JSON exactly; whitespace of original JSON is
                # intentionally represented by its original byte hash/size.
                assert sha(encoded(expand(json.loads(data), trees))) == entry["expandedJSONSha256"]


if __name__ == "__main__":
    if sys.argv[1] == "--verify":
        for name in sys.argv[2:]:
            verify(Path(name))
        print(f"Verified {len(sys.argv) - 2} archives")
    else:
        destination = Path(sys.argv[1])
        destination.mkdir(parents=True, exist_ok=True)
        entries = [record(destination, name, Path(root))
                   for name, root in (argument.split("=", 1) for argument in sys.argv[2:])]
        print(json.dumps(entries, indent=2))
