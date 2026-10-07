"""Validate Graph generation evidence without claiming TypeScript verification."""

import argparse
import hashlib
import json
import math
import os
from pathlib import Path

from compatibility_gate import check_selection
from compatibility_summary import read_resources, render_resources, size, table


def check_generation_report(manifest_bytes, report, scope, selection_directory):
    manifest = json.loads(manifest_bytes)
    entry = next(item for item in manifest["corpora"] if item["id"] == "microsoft-graph-beta")
    if report.get("schemaVersion") != 2 or report.get("manifestSha256") != hashlib.sha256(manifest_bytes).hexdigest():
        raise ValueError("generation report manifest mismatch")
    if len(report["documents"]) != 1:
        raise ValueError("generation report must contain exactly one Graph document")
    item = report["documents"][0]
    if (item["id"] != entry["id"] or item["input"] != entry["input"] or item["cohort"] != entry["cohort"]
        or item["inputSha256"] != entry["sha256"] or item["openapiVersion"] != entry["openapiVersion"]
        or item.get("generationScope", "full") != scope
        or item["documentSuccess"] is not False or item["capabilityAdjustedSuccess"] is not False):
        raise ValueError("generation document provenance or verification status mismatch")
    if scope == "selected":
        check_selection(item, selection_directory)
        if item["generationSelection"]["runtime"]["status"] != "not-run":
            raise ValueError("Graph runtime compilation must be skipped")
    elif item.get("generationSelection") is not None:
        raise ValueError("full Graph generation contains a selection")
    profiles = [item] + item.get("supportProfiles", [])
    for profile in profiles:
        generation = profile["generation"]
        emission = profile.get("operationEmission") or {}
        if (not profile["discoveryComplete"] or profile["diagnostics"]["errors"] != 0
            or generation["status"] != "pass" or profile["typecheck"]["status"] != "not-run"
            or profile.get("success", False) is not False or not emission.get("available")
            or not isinstance(emission.get("count"), int) or emission["count"] <= 0
            or any(not isinstance(generation.get(key), int) or generation[key] <= 0 for key in ("artifactCount", "artifactBytes"))
            or not isinstance(generation.get("durationMillis"), (int, float))
            or not math.isfinite(generation["durationMillis"]) or generation["durationMillis"] <= 0):
            raise ValueError("incomplete or incorrectly verified Graph generation measurement")
    if scope == "full" and (not item["operationRetention"]["available"]
        or item["operationRetention"]["omitted"] != 0
        or item["operationEmission"]["count"] != item["operationRetention"]["total"]):
        raise ValueError("full Graph generation omitted operations")
    if item.get("generationAddons") != []:
        raise ValueError("Graph measurement requires the default client")
    return item


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", type=Path, required=True)
    parser.add_argument("--report", type=Path, required=True)
    parser.add_argument("--scope", choices=("full", "selected"), required=True)
    parser.add_argument("--resources-directory", type=Path)
    args = parser.parse_args()
    report = json.loads(args.report.read_text())
    item = check_generation_report(args.manifest.read_bytes(), report, args.scope, args.manifest.parent / "selections")
    generation = item["generation"]
    summary = f"## Microsoft Graph / {args.scope}\n\nGeneration only; TypeScript and runtime compilation were skipped.\n\n"
    summary += table(["API calls", "Files", "Size", "Generation time"], [[item["operationEmission"]["count"],
        generation["artifactCount"], size(generation["artifactBytes"]), f"{generation['durationMillis'] / 1000:.2f} s"]])
    if args.resources_directory:
        summary += render_resources({item["id"]: read_resources(args.resources_directory)})
    with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as output:
        output.write(summary)
    print(f"ok Graph {args.scope}: generation measured; typecheck not run")
