"""Require actual strict verification for the client or supported server profile."""

import argparse
import json
import os
from pathlib import Path

from compatibility_summary import read_resources, render_report, render_resources


def check_report(manifest, report, document=None, resources=None):
    expected = {item["id"] for item in manifest["corpora"]}
    if document is not None:
        if document not in expected:
            raise ValueError(f"unknown document {document}")
        expected = {document}
    observed = [item["id"] for item in report["documents"]]
    if len(observed) != len(set(observed)) or set(observed) != expected:
        raise ValueError("report has missing, duplicate or unexpected documents")
    failures = []
    for item in report["documents"]:
        profiles = [("client", item)] + [(p["name"], p) for p in item.get("supportProfiles", [])]
        passed = False
        for name, profile in profiles:
            success = (profile["discoveryComplete"] and profile["diagnostics"]["errors"] == 0
                       and profile["generation"]["status"] == "pass"
                       and profile["typecheck"]["status"] == "pass")
            declared = profile["documentSuccess"] if name == "client" else profile["success"]
            if success != declared:
                raise ValueError(f"inconsistent verification for {item['id']}/{name}")
            passed = passed or success
        if passed != item["capabilityAdjustedSuccess"]:
            raise ValueError(f"inconsistent supported-profile result for {item['id']}")
        if not passed:
            failures.append(item["id"])
    return render_report(manifest, report, resources), failures


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", required=True)
    parser.add_argument("--report", required=True)
    parser.add_argument("--document")
    parser.add_argument("--resources-directory", type=Path)
    parser.add_argument("--resource-shards", type=Path)
    args = parser.parse_args()
    manifest = json.loads(Path(args.manifest).read_text())
    if not Path(args.report).exists():
        metrics = read_resources(args.resources_directory) if args.resources_directory else {}
        resources = {args.document or Path(args.manifest).stem: metrics}
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as output:
            output.write("## Benchmark did not produce a verification report\n\n" + render_resources(resources))
        raise SystemExit("verification report unavailable; resource records preserved")
    report = json.loads(Path(args.report).read_text())
    # Validate membership/status before resolving any document-derived resource path.
    summary, failures = check_report(manifest, report, args.document)
    resources = {}
    for item in report["documents"]:
        directory = args.resources_directory
        if args.resource_shards:
            directory = args.resource_shards / f"document-shard-{Path(args.manifest).stem}-{item['id']}" / "compatibility-ci"
        if directory:
            metrics = read_resources(directory)
            resources[item["id"]] = metrics
    if resources:
        summary = render_report(manifest, report, resources)
    if args.resources_directory:
        (args.resources_directory / "resource-summary.json").write_text(json.dumps(resources, indent=2) + "\n")
    with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as output:
        output.write(f"## {Path(args.manifest).stem}\n\n" + summary + "\n")
    print(summary, flush=True)
    if failures:
        raise SystemExit("Strict verification incomplete: " + ", ".join(failures))
