"""Build the complete document matrix from the existing corpus manifests."""

import json
import hashlib
import os
import sys
from pathlib import Path

CORPORA = ("holdout", "modern", "production32", "regression")


def typecheck_candidate(identifier):
    """Graph contributes generation measurements outside the verification matrix."""
    return identifier != "microsoft-graph-beta"


def corpus_matrix(root):
    jobs = []
    for name in CORPORA:
        data = (root / "test/compatibility" / f"{name}.json").read_bytes()
        key = hashlib.sha256(data).hexdigest()
        jobs.append(dict(corpus=name, artifact=f"pinned-corpus-{name}-{key}", fetch=name in ("holdout", "regression")))
    return {"include": jobs}


def document_matrix(root):
    jobs = []
    artifacts = {job["corpus"]: job["artifact"] for job in corpus_matrix(root)["include"]}
    for corpus in CORPORA:
        manifest = json.loads((root / "test/compatibility" / f"{corpus}.json").read_text())
        identifiers = [document["id"] for document in manifest["corpora"]]
        if not identifiers or len(identifiers) != len(set(identifiers)):
            raise ValueError(f"empty or duplicate document IDs in {corpus}")
        jobs.extend(dict(corpus=corpus, document=identifier, artifact=artifacts[corpus]) for identifier in identifiers if typecheck_candidate(identifier))
    if len(jobs) > 256:
        raise ValueError("document matrix exceeds GitHub's 256-job limit")
    return {"include": jobs}


if __name__ == "__main__":
    matrix = document_matrix(Path.cwd())
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        output.write("documents=" + json.dumps(matrix) + "\n")
        output.write("corpora=" + json.dumps(corpus_matrix(Path.cwd())) + "\n")
    print(f"Selected {len(matrix['include'])} documents across {len(CORPORA)} corpora", file=sys.stderr)
