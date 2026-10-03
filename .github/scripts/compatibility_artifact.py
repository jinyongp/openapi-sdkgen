"""Resolve reusable pinned inputs and publish only verified corpus directories."""

import argparse
import json
import os
import sys
from pathlib import Path
import shutil
import subprocess
import time
from urllib.parse import quote

from compatibility_matrix import CORPORA
from compatibility_summary import render_preparation


def select_artifact(artifacts, name):
    matching = [item for item in artifacts if item["name"] == name and not item["expired"]]
    return max(matching, key=lambda item: item["created_at"], default=None)


def find_artifact(name, required=False):
    repository = os.environ["GITHUB_REPOSITORY"]
    result = subprocess.run(["gh", "api", f"repos/{repository}/actions/artifacts?name={quote(name)}&per_page=100"],
                            capture_output=True, text=True)
    artifact = select_artifact(json.loads(result.stdout)["artifacts"], name) if result.returncode == 0 else None
    if required and artifact is None:
        raise RuntimeError("verified corpus artifact unavailable")
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        output.write(f"artifact-id={artifact['id'] if artifact else ''}\n")
        output.write(f"source-run={artifact['workflow_run']['id'] if artifact else ''}\n")
    print("Pinned corpus artifact found" if artifact else "No reusable artifact; prepare fresh inputs", file=sys.stderr)


def restore_corpus(corpus, root):
    source = root / ".tmp/compatibility-restore"
    destination = root / ".tmp/compatibility-corpora" / corpus
    if destination.exists():
        raise RuntimeError("corpus destination already exists")
    # Both absolute paths are owned children of this checkout's .tmp directory.
    temporary = (root / ".tmp").resolve()
    if not source.resolve().is_relative_to(temporary) or not destination.resolve().is_relative_to(temporary):
        raise RuntimeError("corpus paths must stay inside workspace .tmp")
    result = subprocess.run(["bash", "scripts/compatibility/fetch.sh", "offline",
                             f"test/compatibility/{corpus}.json", str(source)], cwd=root)
    if result.returncode:
        print("Artifact verification failed; prepare fresh inputs", file=sys.stderr)
        return False
    destination.parent.mkdir(parents=True, exist_ok=True)
    source.rename(destination)
    return True


def prepare_local(corpus, root):
    manifest = json.loads((root / "test/compatibility" / f"{corpus}.json").read_text())
    destination = root / ".tmp/compatibility-corpora" / corpus
    for entry in manifest["corpora"] + manifest.get("files", []):
        target = destination / entry["input"]
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(root / "test" / entry["input"], target)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("mode", choices=("find", "restore", "local", "summary"))
    parser.add_argument("value")
    parser.add_argument("--required", action="store_true")
    args = parser.parse_args()
    if args.mode == "find":
        find_artifact(args.value, args.required)
    else:
        if args.value not in CORPORA:
            raise ValueError("unknown corpus")
        if args.mode == "summary":
            corpus = args.value
            manifest = json.loads(Path(f"test/compatibility/{corpus}.json").read_text())
            reused = os.environ.get("COMPATIBILITY_REUSED") == "true"
            mode = "Artifact reused" if reused else ("Fetched" if corpus in ("holdout", "regression") else "Checked-in snapshot")
            started = os.environ.get("COMPATIBILITY_PREPARE_STARTED")
            elapsed = time.time() - float(started) if started else None
            summary = render_preparation(manifest, Path(f".tmp/compatibility-corpora/{corpus}"), mode, elapsed)
            with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as output:
                output.write(f"## Prepare {corpus}\n\n" + summary)
        elif args.mode == "local":
            prepare_local(args.value, Path.cwd())
        else:
            reused = restore_corpus(args.value, Path.cwd())
            with open(os.environ["GITHUB_OUTPUT"], "a") as output:
                output.write(f"reused={str(reused).lower()}\n")
