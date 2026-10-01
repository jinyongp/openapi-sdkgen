"""Build the complete document matrix from the existing corpus manifests."""

import json
import os
from pathlib import Path

CORPORA = ("holdout", "modern", "production32", "regression")


def document_matrix(root):
    jobs = []
    for corpus in CORPORA:
        manifest = json.loads((root / "test/compatibility" / f"{corpus}.json").read_text())
        identifiers = [document["id"] for document in manifest["corpora"]]
        if not identifiers or len(identifiers) != len(set(identifiers)):
            raise ValueError(f"empty or duplicate document IDs in {corpus}")
        jobs.extend(dict(corpus=corpus, document=identifier) for identifier in identifiers)
    if len(jobs) > 256:
        raise ValueError("document matrix exceeds GitHub's 256-job limit")
    return {"include": jobs}


if __name__ == "__main__":
    matrix = document_matrix(Path.cwd())
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        output.write("documents=" + json.dumps(matrix) + "\n")
    print(f"Selected {len(matrix['include'])} documents across {len(CORPORA)} corpora")
