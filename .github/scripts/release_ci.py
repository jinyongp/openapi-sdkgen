"""Reuse the latest push CI for the exact release source; never dispatch CI."""
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import time
from urllib.parse import urlencode


def git(*args):
    return subprocess.check_output(["git", *args], text=True).strip()


def ci_source(commit):
    if not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise ValueError("Release source must be a full commit SHA")
    message = git("show", "-s", "--format=%B", commit)
    if not re.fullmatch(r"chore\(release\): v\S+\n\nReleaseway-Config: [0-9a-f]{64}", message):
        return commit
    # Releaseway's marker adds no source changes. Reuse its unchanged parent's
    # exact CI when retrying a release made by the earlier marker-based workflow.
    parents = git("show", "-s", "--format=%P", commit).split()
    if len(parents) != 1 or git("rev-parse", f"{commit}^{{tree}}") != git("rev-parse", f"{parents[0]}^{{tree}}"):
        raise ValueError("Release marker must have one parent with an identical source tree")
    return parents[0]


def latest_run(runs, repository, commit):
    candidates = [run for run in runs if (
        run.get("head_sha") == commit
        and run.get("event") == "push"
        and run.get("head_branch") == "main"
        and run.get("path") == ".github/workflows/ci.yml"
        and (run.get("head_repository") or {}).get("full_name") == repository
    )]
    if not candidates:
        raise ValueError(f"No main push CI exists for {commit}; push the source and run CI first")
    return max(candidates, key=lambda run: run["id"])


def fetch_runs(repository, commit):
    query = urlencode({"head_sha": commit, "event": "push", "branch": "main", "per_page": 100})
    # Resolve runs through the repository index. latest_run still checks the
    # exact workflow identity, alongside source, event, branch and repository.
    endpoint = f"repos/{repository}/actions/runs?{query}"
    result = subprocess.check_output(["gh", "api", "--paginate", "--slurp", endpoint], text=True)
    return [run for page in json.loads(result) for run in page["workflow_runs"]]


def require_ci(repository, commit, timeout=1800):
    deadline = time.monotonic() + timeout
    announced = None
    while True:
        run = latest_run(fetch_runs(repository, commit), repository, commit)
        if run["status"] == "completed":
            if run["conclusion"] != "success":
                raise ValueError(f"Latest CI is {run['conclusion']}: {run['html_url']}")
            return run
        if time.monotonic() >= deadline:
            raise TimeoutError(f"Timed out waiting for CI: {run['html_url']}")
        if announced != run["id"]:
            print(f"Waiting for CI: {run['html_url']}", file=sys.stderr, flush=True)
            announced = run["id"]
        time.sleep(10)


def main():
    repository = os.environ["GITHUB_REPOSITORY"]
    source = ci_source(os.environ["GITHUB_SHA"])
    run = require_ci(repository, source)
    print(f"Reusing successful CI for {source}: {run['html_url']}")
    summary = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary:
        with Path(summary).open("a") as output:
            output.write(f"### Reused product validation\n\nSource: `{source}`\n\n[Successful CI]({run['html_url']})\n")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, TimeoutError, subprocess.CalledProcessError) as error:
        print(f"Release CI validation failed: {error}", file=sys.stderr)
        sys.exit(1)
