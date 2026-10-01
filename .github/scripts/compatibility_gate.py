"""Require actual strict verification for the client or supported server profile."""

import argparse
import json
import os
from pathlib import Path


def check_report(manifest, report, document=None):
    expected = {item["id"] for item in manifest["corpora"]}
    if document is not None:
        if document not in expected:
            raise ValueError(f"unknown document {document}")
        expected = {document}
    observed = [item["id"] for item in report["documents"]]
    if len(observed) != len(set(observed)) or set(observed) != expected:
        raise ValueError("report has missing, duplicate or unexpected documents")
    rows = []
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
            count = (profile.get("operationEmission") or {}).get("count", 0)
            rows.append(f"| {item['id']} | {name} | {profile['generation']['status']} | {profile['typecheck']['status']} | {count} |")
        if passed != item["capabilityAdjustedSuccess"]:
            raise ValueError(f"inconsistent supported-profile result for {item['id']}")
        if not passed:
            failures.append(item["id"])
    summary = "| Document | Profile | Generation | Strict typecheck | API calls |\n| --- | --- | --- | --- | ---: |\n" + "\n".join(rows) + "\n"
    return summary, failures


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", required=True)
    parser.add_argument("--report", required=True)
    parser.add_argument("--document")
    args = parser.parse_args()
    summary, failures = check_report(json.loads(Path(args.manifest).read_text()),
                                     json.loads(Path(args.report).read_text()), args.document)
    with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as output:
        output.write(f"## {Path(args.manifest).stem}\n\n" + summary + "\n")
    print(summary, flush=True)
    if failures:
        raise SystemExit("Strict verification incomplete: " + ", ".join(failures))
