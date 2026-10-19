"""Reproduce paired summaries from unpacked original measurement directories.

Usage: python3 compare_measurements.py BEFORE AFTER OUTPUT.json
Uses successful rows only; failed rows remain explicitly listed, never samples.
Benchmark and command scopes are separate. No significance/threshold assertion.
"""
import json
from pathlib import Path
import re
import statistics
import sys


def summary(values):
    return {"n": len(values), "median": statistics.median(values),
            "min": min(values), "max": max(values)}


def benchmarks(text):
    groups = {}
    for line in text.splitlines():
        fields = line.split()
        if not fields or not fields[0].startswith("Benchmark"):
            continue
        name = re.sub(r"-\d+$", "", fields[0])
        for index in range(2, len(fields) - 1, 2):
            groups.setdefault(name, {}).setdefault(fields[index + 1], []).append(float(fields[index]))
    return groups


def measurements(root):
    rows = json.loads((root / "results.json").read_text())
    groups, failures, inputs = {}, [], {}

    def add(name, unit, value):
        groups.setdefault(name, {}).setdefault(unit, []).append(value)

    for row in rows:
        if row["exitCode"] != 0:
            failures.append({"label": row["label"], "exitCode": row["exitCode"],
                             "stoppedReason": row.get("stoppedReason")})
            continue
        label = row["label"]
        group = re.sub(r"(?:-sample|generation|cold|warm)\d+$",
                       lambda m: re.sub(r"\d+$", "", m[0]).removesuffix("-sample"), label)
        if "inputSha256" in row:
            inputs.setdefault(group, set()).add(row["inputSha256"])
        for key, value in row.get("metrics", {}).items():
            add(group, key, value)
        for key in ["binaryBytes", "generatedBytes", "generatedFiles"]:
            if key in row:
                add(group, key, row[key])
        for key in ["files", "bytes"]:
            if key in row.get("output", {}):
                add(group, "source" + key.capitalize(), row["output"][key])
        if label == "runtime-benchmark":
            # Archived raw stdout retains its basename even when the original
            # absolute temporary directory is unavailable.
            raw = root / Path(row["stdout"]).name
            for name, units in benchmarks(raw.read_text()).items():
                for unit, values in units.items():
                    for value in values:
                        add(name, unit, value)
    return {"groups": groups, "failures": failures,
            "inputs": {k: sorted(v) for k, v in inputs.items()}, "commands": len(rows)}


def compare(before, after):
    pairs = {}
    for group in sorted(before["groups"].keys() & after["groups"].keys()):
        if group in before["inputs"] and group in after["inputs"]:
            assert before["inputs"][group] == after["inputs"][group], group
        units = {}
        for unit in sorted(before["groups"][group].keys() & after["groups"][group].keys()):
            left = summary(before["groups"][group][unit])
            right = summary(after["groups"][group][unit])
            units[unit] = {"before": left, "after": right,
                           "changePercent": ((right["median"] / left["median"] - 1) * 100
                                             if left["median"] else None)}
        pairs[group] = units
    return {"scope": "Local sequential diagnostic samples; no statistical significance or public benchmark claim.",
            "changeFormula": "100 * (after median / before median - 1); null when before is zero",
            "beforeCommands": before["commands"], "afterCommands": after["commands"],
            "beforeFailures": before["failures"], "afterFailures": after["failures"],
            "beforeOnly": sorted(before["groups"].keys() - after["groups"].keys()),
            "afterOnly": sorted(after["groups"].keys() - before["groups"].keys()),
            "comparisons": pairs}


if __name__ == "__main__":
    result = compare(measurements(Path(sys.argv[1])), measurements(Path(sys.argv[2])))
    Path(sys.argv[3]).write_text(json.dumps(result, indent=2) + "\n")
    print(f"Compared {len(result['comparisons'])} groups; failures retained separately")
