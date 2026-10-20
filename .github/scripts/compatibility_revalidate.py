"""Validate archived Actions measurements after reporting-only corrections."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess

from compatibility_gate import check_report
from compatibility_generation_gate import check_generation_report
from compatibility_golang import combine
from compatibility_matrix import CORPORA


def validate(root, source_run, reports, shards, corpora, graphs, output):
    repository = os.environ["GITHUB_REPOSITORY"]
    result = subprocess.run(["gh", "api", f"repos/{repository}/actions/runs/{source_run}"],
                            capture_output=True, text=True, check=True)
    run = json.loads(result.stdout)
    if (run["repository"]["full_name"] != repository or run["status"] != "completed"
        or run["path"].split("@")[0] != ".github/workflows/compatibility.yml"):
        raise ValueError("measurement run identity mismatch")
    commit = run["head_sha"]
    # Revalidation cannot make measurements of older SDK behavior current.
    protected = ["internal", "cmd", "go.mod", "go.sum", ".node-version", "scripts/golang-compat",
                 "scripts/compat-benchmark", "scripts/compatibility", "test/compatibility/selections"]
    unchanged = subprocess.run(["git", "diff", "--quiet", commit, "--", *protected], cwd=root)
    if unchanged.returncode != 0:
        raise ValueError("SDK or measurement producer changed; run new measurements")
    output.mkdir(parents=True, exist_ok=True)
    for corpus in CORPORA:
        manifest_bytes = (root / f"test/compatibility/{corpus}.json").read_bytes()
        report_path = reports / f"{corpus}-results.json"
        report = json.loads(report_path.read_text())
        _, failures = check_report(json.loads(manifest_bytes), report,
                                   selection_directory=root / "test/compatibility/selections",
                                   manifest_sha=hashlib.sha256(manifest_bytes).hexdigest())
        if failures:
            raise ValueError(f"strict TypeScript verification incomplete: {failures}")
        shutil.copyfile(report_path, output / report_path.name)
    go = combine(root, shards, commit, run_id=source_run, corpus_root=corpora, repository=repository)
    # This timestamp belongs to the archived Actions attempt, not local processing.
    go["measurement"]["measuredAt"] = run["updated_at"]
    (output / "golang-results.json").write_text(json.dumps(go, indent=2) + "\n")
    manifest_bytes = (root / "test/compatibility/regression.json").read_bytes()
    for scope in ("full", "selected"):
        path = graphs / f"graph-shard-{scope}/compatibility-ci/report.json"
        report = json.loads(path.read_text())
        check_generation_report(manifest_bytes, report, scope, root / "test/compatibility/selections")
        shutil.copyfile(path, output / f"graph-{scope}-results.json")
    return commit, run["html_url"]


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--source-run", type=int, required=True)
    parser.add_argument("--reports", type=Path, required=True)
    parser.add_argument("--shards", type=Path, required=True)
    parser.add_argument("--corpus-root", type=Path, required=True)
    parser.add_argument("--graphs", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    commit, url = validate(Path.cwd(), args.source_run, args.reports, args.shards,
                           args.corpus_root, args.graphs, args.output)
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        output.write(f"source-commit={commit}\nsource-run-url={url}\n")
