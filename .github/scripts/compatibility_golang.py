"""Validate independent Go generation/compilation shards from this Actions source."""
import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
from compatibility_matrix import CORPORA, typecheck_candidate
from compatibility_summary import table


def combine(root, shards, commit):
    documents = []
    for corpus in CORPORA:
        manifest = json.loads((root / f"test/compatibility/{corpus}.json").read_text())
        for entry in manifest["corpora"]:
            if not typecheck_candidate(entry["id"]):
                continue
            path = shards / f"document-shard-{corpus}-{entry['id']}" / "compatibility-ci/golang.json"
            report = json.loads(path.read_text())
            go = report.get("golang", {})
            if (report.get("sourceCommit") != commit or report.get("sourceDirty") is not False
                or report.get("id") != entry["id"] or report.get("input") != entry["input"]
                or report.get("inputSha256") != entry["sha256"] or report.get("mode") != "golang-generation"
                or report.get("addons") != ["server"] or report.get("verificationRequested") is not True
                or report.get("selection")):
                raise ValueError(f"Go generation/compilation proof mismatch: {corpus}/{entry['id']}")
            blocked = go.get("blocked")
            compilation = go.get("compilation", {}).get("status")
            success = blocked is False and compilation == "pass"
            if (not isinstance(blocked, bool) or report.get("pass") is not success
                or not success and not report.get("failure")
                or blocked and (compilation is not None or not (go.get("generationError") or any(
                    item.get("severity") == "error" for item in go.get("diagnostics", []))))
                or not blocked and compilation not in ("pass", "fail")
                or compilation == "fail" and not go["compilation"].get("detail")):
                raise ValueError("inconsistent Go verification outcome")
            expected = entry.get("preparation")
            evidence = report.get("inputPreparation")
            if expected:
                if (not evidence or evidence.get("kind") != expected["kind"] or evidence.get("sha256") != expected["sha256"]
                    or evidence.get("sourceSha256") != entry["sha256"] or evidence.get("operations") != expected["operations"]
                    or success and evidence.get("contractsVerified") is not True):
                    raise ValueError("Go prepared input proof mismatch")
            elif evidence is not None:
                raise ValueError("unexpected Go input preparation")
            documents.append(dict(report, corpus=corpus))
    return {"schemaVersion": 1, "measurement": {"sourceCommit": commit, "sourceDirty": False,
            "measuredAt": datetime.now(timezone.utc).isoformat()}, "documents": documents}


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--shards", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    report = combine(Path.cwd(), args.shards, os.environ["GITHUB_SHA"])
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, indent=2) + "\n")
    with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as output:
        output.write("## Go generation and compilation\n\nAll rows include the server addon; no provider API calls were made.\n\n")
        output.write(table(["Corpus", "Document", "Generated API calls", "Go compiler"],
                           [(item["corpus"], item["id"], len(item["golang"].get("routes", [])), item["golang"].get("compilation", {}).get("status", "blocked"))
                            for item in report["documents"]]))
