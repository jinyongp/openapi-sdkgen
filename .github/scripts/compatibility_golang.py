"""Validate independent Go generation/compilation shards from this Actions source."""
import argparse
import hashlib
from datetime import datetime, timezone
import json
import os
import re
import subprocess
from pathlib import Path
from compatibility_matrix import CORPORA, typecheck_candidate
from compatibility_summary import table


def actions_jobs(repository, run_id):
    result = subprocess.run(["gh", "api", f"repos/{repository}/actions/runs/{run_id}/jobs?per_page=100"],
                            capture_output=True, text=True, check=True)
    response = json.loads(result.stdout)
    if response["total_count"] > 100:
        raise ValueError("Go evidence job lookup requires pagination")
    return response["jobs"]


def recover_compiler_evidence(checkpoint, jobs, corpus, entry, commit, run_id):
    go = checkpoint.get("golang", {})
    if (checkpoint.get("generationCheckpoint") is not True or checkpoint.get("pass") is not False
        or go.get("compilation", {}).get("status") != "not-run" or go.get("blocked") is not False
        or not go.get("artifactSha256") or go.get("files", 0) <= 0):
        raise ValueError("missing completed Go generation checkpoint")
    matches = [job for job in jobs if job["name"] == f"Measure {corpus} / {entry['id']} / full"]
    if len(matches) != 1:
        raise ValueError("missing or duplicate compiler job identity")
    job = matches[0]
    if (job["head_sha"] != commit or job["run_id"] != run_id or job["status"] != "completed"
        or job["conclusion"] not in ("failure", "cancelled", "timed_out")
        or not any(step["name"] == "Compile the independent Go SDK profile" and step.get("started_at")
                   and step.get("conclusion") != "skipped" for step in job["steps"])):
        raise ValueError("unavailable compiler evidence lacks an independently verified failed job")
    checkpoint["compilerEvidenceUnavailable"] = dict(runId=run_id, jobId=job["id"],
                                                     jobConclusion=job["conclusion"], sourceCommit=commit)
    checkpoint["failure"] = "compiler evidence unavailable"
    go["compilation"] = dict(status="unavailable", detail="No completed compiler result was recovered from the Actions job.")
    return checkpoint


def input_pin(entry, corpus, corpus_root):
    if entry.get("sha256"):
        return dict(kind="sha256", value=entry["sha256"], sha256=entry["sha256"])
    if not entry.get("gitBlob") or corpus_root is None:
        raise ValueError("Git blob input verification requires the immutable corpus")
    base = (corpus_root / corpus).resolve()
    source = (base / entry["input"]).resolve()
    if not source.is_relative_to(base):
        raise ValueError("input pin path escapes the corpus")
    data = source.read_bytes()
    blob = hashlib.sha1(f"blob {len(data)}\0".encode() + data).hexdigest()
    if blob != entry["gitBlob"]:
        raise ValueError("input Git blob pin mismatch")
    return dict(kind="git-blob", value=blob, sha256=hashlib.sha256(data).hexdigest())


def combine(root, shards, commit, jobs=None, run_id=None, corpus_root=None, repository=None):
    documents = []
    for corpus in CORPORA:
        manifest = json.loads((root / f"test/compatibility/{corpus}.json").read_text())
        for entry in manifest["corpora"]:
            if not typecheck_candidate(entry["id"]):
                continue
            pin = input_pin(entry, corpus, corpus_root)
            path = shards / f"golang-shard-{corpus}-{entry['id']}" / "golang.json"
            checkpoint_path = shards / f"golang-generation-shard-{corpus}-{entry['id']}" / "golang-generation.json"
            checkpoint = json.loads(checkpoint_path.read_text())
            if (checkpoint.get("generationCheckpoint") is not True or checkpoint.get("pass") is not False
                or checkpoint.get("compilerEvidenceUnavailable")):
                raise ValueError("invalid Go generation checkpoint")
            if path.exists():
                report = json.loads(path.read_text())
                identity_fields = ("sourceCommit", "sourceDirty", "id", "input", "inputSha256", "mode", "addons", "inputPreparation", "goCompilationPolicy", "verificationRequested", "selection", "goVersion")
                if (checkpoint.get("generationCheckpoint") is not True or checkpoint.get("pass") is not False
                    or (checkpoint.get("golang", {}).get("blocked") is False and checkpoint.get("golang", {}).get("compilation", {}).get("status") != "not-run")
                    or report.get("generationCheckpoint") or report.get("compilerEvidenceUnavailable")
                    or any(report.get(key) != checkpoint.get(key) for key in identity_fields)
                    or any(report.get("golang", {}).get(key) != checkpoint.get("golang", {}).get(key)
                           for key in ("blocked", "artifactSha256", "files", "bytes", "routes", "inbound"))):
                    raise ValueError("compiler report differs from its generation checkpoint")
            else:
                if checkpoint.get("golang", {}).get("blocked") is True:
                    report = checkpoint
                else:
                    if jobs is None:
                        if run_id is None:
                            run_id = int(os.environ["GITHUB_RUN_ID"])
                        jobs = actions_jobs(repository or os.environ["GITHUB_REPOSITORY"], run_id)
                    if run_id is None:
                        run_id = int(os.environ["GITHUB_RUN_ID"])
                    report = recover_compiler_evidence(checkpoint, jobs, corpus, entry, commit, run_id)
            go = report.get("golang", {})
            if (report.get("sourceCommit") != commit or report.get("sourceDirty") is not False
                or report.get("id") != entry["id"] or report.get("input") != entry["input"]
                or report.get("inputSha256") != pin["sha256"] or report.get("mode") != "golang-generation"
                or report.get("addons") != ["server"] or report.get("verificationRequested") is not True
                or report.get("selection")
                or report.get("goCompilationPolicy") != dict(timeoutSeconds=1200, buildParallelism=1, cpuConcurrency=2, memoryLimit="3GiB")):
                raise ValueError(f"Go generation/compilation proof mismatch: {corpus}/{entry['id']}")
            blocked = go.get("blocked")
            compilation = go.get("compilation", {}).get("status")
            success = blocked is False and compilation == "pass"
            if (not isinstance(blocked, bool) or report.get("pass") is not success
                or not success and not report.get("failure")
                or blocked and (compilation is not None or not (go.get("generationError") or any(
                    item.get("severity") == "error" for item in go.get("diagnostics", []))))
                or not blocked and compilation not in ("pass", "fail", "unavailable")
                or not blocked and (type(go.get("files")) is not int or go["files"] <= 0
                                    or type(go.get("bytes")) is not int or go["bytes"] <= 0
                                    or not re.fullmatch(r"[a-f0-9]{64}", go.get("artifactSha256", "")))
                or compilation == "unavailable" and not report.get("compilerEvidenceUnavailable")
                or compilation == "fail" and not go["compilation"].get("detail")):
                raise ValueError("inconsistent Go verification outcome")
            expected = entry.get("preparation")
            evidence = report.get("inputPreparation")
            if expected:
                if (not evidence or evidence.get("kind") != expected["kind"] or evidence.get("sha256") != expected["sha256"]
                    or evidence.get("sourceSha256") != pin["sha256"] or evidence.get("operations") != expected["operations"]
                    or success and evidence.get("contractsVerified") is not True):
                    raise ValueError("Go prepared input proof mismatch")
            elif evidence is not None:
                raise ValueError("unexpected Go input preparation")
            required_test = {"digitalocean-tags-v1": "TestDigitalOceanHTTPContract",
                             "spoonacular-parameter-bodies-v1": "TestSpoonacularHTTPContract"}.get((expected or {}).get("kind"))
            required_test = {"loket.nl": "TestLoketHTTPContract", "stripe": "TestStripeHTTPContract",
                             "cloudflare": "TestCloudflareHTTPContract"}.get(entry["id"], required_test)
            if success and required_test and go["compilation"].get("contractTests") != [required_test]:
                raise ValueError("Go provider HTTP contract proof is missing")
            documents.append(dict(report, corpus=corpus, inputPin=pin))
    return {"schemaVersion": 1, "measurement": {"sourceCommit": commit, "sourceDirty": False,
            "measuredAt": datetime.now(timezone.utc).isoformat()}, "documents": documents}


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--shards", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--corpus-root", type=Path)
    args = parser.parse_args()
    report = combine(Path.cwd(), args.shards, os.environ["GITHUB_SHA"], corpus_root=args.corpus_root)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, indent=2) + "\n")
    with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as output:
        output.write("## Go generation and compilation\n\nAll rows include the server addon; no provider API calls were made.\n\n")
        output.write(table(["Corpus", "Document", "Generated API calls", "Go compiler"],
                           [(item["corpus"], item["id"], ("—" if item["golang"]["blocked"] else len(item["golang"].get("routes", []))), item["golang"].get("compilation", {}).get("status", "blocked"))
                            for item in report["documents"]]))
