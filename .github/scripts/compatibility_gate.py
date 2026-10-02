"""Require actual strict verification for the client or supported server profile."""

import argparse
import hashlib
import json
import os
import tomllib
from pathlib import Path

from compatibility_summary import read_resources, render_report, render_resources


def check_selection(item, directory):
    path = directory / f"{item['id']}.toml"
    scope = item.get("generationScope", "full")
    if not path.exists():
        if scope != "full" or item.get("generationSelection") is not None:
            raise ValueError(f"unexpected selection for {item['id']}")
        return
    raw = path.read_bytes()
    policy = tomllib.loads(raw.decode())
    evidence = item.get("generationSelection") or {}
    requested = {key: sorted(set(values)) for key, values in policy["selection"].items() if values}
    if (scope != "selected" or policy["document"] != item["id"]
        or policy["input_sha256"] != item["inputSha256"]
        or evidence.get("requested") != requested
        or evidence.get("fixtureSha256") != hashlib.sha256(raw).hexdigest()
        or evidence.get("runtimeProbeSha256") != hashlib.sha256((directory / policy["runtime_probe"]).read_bytes()).hexdigest()):
        raise ValueError(f"selection provenance mismatch for {item['id']}")
    if item["generation"]["status"] == "pass":
        routes, dependencies = evidence.get("routes", []), evidence.get("dependencyRoutes", [])
        if (not routes or routes != sorted(set(routes)) or dependencies != sorted(set(dependencies))
            or set(routes) & set(dependencies)
            or (not requested.get("operations") and routes != requested.get("routes"))
            or evidence.get("excludedOperations", -1) < 0
            or item["operationRetention"]["total"] != len(routes) + len(dependencies) + evidence["excludedOperations"]
            or item["operationEmission"]["count"] != len(routes)):
            raise ValueError(f"selection membership mismatch for {item['id']}")


def check_report(manifest, report, document=None, resources=None, selection_directory=None, manifest_sha=None):
    if manifest_sha is not None and report.get("manifestSha256") != manifest_sha:
        raise ValueError("report manifest hash mismatch")
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
        if "generationAddons" in item:
            addons = item["generationAddons"]
            if addons not in ([], ["metadata"]):
                raise ValueError(f"invalid generation add-ons for {item['id']}")
            for profile in item.get("supportProfiles", []):
                if profile["name"] == "server-addon" and profile.get("generationAddons") != addons + ["server"]:
                    raise ValueError(f"server add-on settings differ for {item['id']}")
        if selection_directory is not None:
            check_selection(item, selection_directory)
        scope = item.get("generationScope", "full")
        if scope not in ("full", "selected"):
            raise ValueError(f"unknown generation scope for {item['id']}")
        runtime_pass = scope == "full" or (item.get("generationSelection") or {}).get("runtime", {}).get("status") == "pass"
        profiles = [("client", item)] + [(p["name"], p) for p in item.get("supportProfiles", [])]
        passed = False
        for name, profile in profiles:
            success = (profile["discoveryComplete"] and profile["diagnostics"]["errors"] == 0
                       and profile["generation"]["status"] == "pass"
                       and profile["typecheck"]["status"] == "pass" and runtime_pass)
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
    parser.add_argument("--selection-directory", type=Path)
    args = parser.parse_args()
    manifest_bytes = Path(args.manifest).read_bytes()
    manifest = json.loads(manifest_bytes)
    if not Path(args.report).exists():
        metrics = read_resources(args.resources_directory) if args.resources_directory else {}
        resources = {args.document or Path(args.manifest).stem: metrics}
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as output:
            output.write("## Benchmark did not produce a verification report\n\n" + render_resources(resources))
        raise SystemExit("verification report unavailable; resource records preserved")
    report = json.loads(Path(args.report).read_text())
    # Validate membership/status before resolving any document-derived resource path.
    summary, failures = check_report(manifest, report, args.document,
                                    selection_directory=args.selection_directory,
                                    manifest_sha=hashlib.sha256(manifest_bytes).hexdigest())
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
