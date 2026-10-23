"""Run disjoint Go CI groups and preserve execution timings, without skipping tests."""
import argparse
import hashlib
import heapq
import json
import math
from pathlib import Path
import re
import statistics
import subprocess
import sys
import time


GOLANG = "openapi-sdkgen/internal/target/golang"
TYPESCRIPT = "openapi-sdkgen/internal/target/typescript"
PACKAGES = {"golang": GOLANG, "typescript": TYPESCRIPT}
MATRICES = "^TestRuntimeFeature(Native|Declaration)MatrixRegression$"
TIMINGS = Path(__file__).with_name("go_ci_timings.json")


def partition(names, shard, count, timings=None):
    if count < 1 or not 1 <= shard <= count:
        raise ValueError("shard must be between 1 and the positive shard count")
    timings = timings or {}
    buckets = [[] for _ in range(count)]
    loads = [(0, 0, index) for index in range(count)]
    # Longest tests first; stable ties and integer costs keep every runner's
    # independently computed plan identical. New tests receive one second.
    costs = {name: max(1, round(timings.get(name, 1) * 1_000_000)) for name in set(names)}
    for name in sorted(costs, key=lambda name: (-costs[name], name)):
        cost, size, index = heapq.heappop(loads)
        buckets[index].append(name)
        heapq.heappush(loads, (cost + costs[name], size + 1, index))
    return sorted(buckets[shard - 1])


def load_timings(path=TIMINGS):
    raw = path.read_bytes()
    profile = json.loads(raw)
    return validate_timings(profile), hashlib.sha256(raw).hexdigest()


def validate_timings(profile):
    groups = profile.get("groups") if isinstance(profile, dict) else None
    if (not isinstance(groups, dict) or type(profile.get("schema")) is not int
            or profile["schema"] != 1 or set(groups) != set(PACKAGES)):
        raise ValueError("CI timings require schema 1 and both generator groups")
    for values in groups.values():
        if not isinstance(values, dict) or not values or any(
            not isinstance(name, str) or not re.fullmatch(r"(?:Test|Fuzz|Example)\w*", name)
            or isinstance(seconds, bool) or not isinstance(seconds, (int, float))
            or not math.isfinite(seconds) or seconds < 0
            for name, seconds in values.items()
        ):
            raise ValueError("CI timings must contain runnable test names and finite nonnegative seconds")
    return groups


def refresh_timings(directory, destination=TIMINGS):
    samples = {group: {} for group in PACKAGES}
    sources = []
    for path in sorted(directory.glob("*/run.json")):
        run = json.loads(path.read_text())
        if (run.get("status") != "completed" or run.get("conclusion") != "success"
                or run.get("event") != "push" or run.get("head_branch") != "main"
                or run.get("path") != ".github/workflows/ci.yml"):
            raise ValueError("CI timing sources must be successful completed main push CI runs")
        if any(source["id"] == run["id"] for source in sources):
            raise ValueError("CI timing source runs must be unique")
        sources.append({"id": run["id"], "commit": run["head_sha"], "url": run["html_url"]})
        reports = [json.loads(report.read_text()) for report in (path.parent / "reports").rglob("*.json")]
        for group, package in PACKAGES.items():
            selected = [report for report in reports if report.get("group") == group]
            verify_group_reports(selected, group)
            for report in selected:
                for name, outcome in report["tests"].items():
                    samples[group].setdefault(name.removeprefix(package + "/"), []).append(outcome["seconds"])
    if not sources:
        raise ValueError("CI timing source runs are missing")
    profile = {"schema": 1, "sources": sources, "groups": {
        group: {name: round(statistics.median(values), 6) for name, values in tests.items()}
        for group, tests in samples.items()
    }}
    validate_timings(profile)
    destination.write_text(json.dumps(profile, indent=2, sort_keys=True) + "\n")
    print(f"Updated CI timings for {sum(map(len, profile['groups'].values()))} tests from {len(sources)} runs")


def discover(package):
    output = subprocess.check_output(["go", "test", "-list", ".", package], text=True)
    names = [line for line in output.splitlines() if re.fullmatch(r"(?:Test|Fuzz|Example)\w*", line)]
    if not names:
        raise ValueError("Go test discovery returned no runnable tests")
    return names


def group_tests(group, names):
    return [name for name in names if group != "typescript" or not re.fullmatch(MATRICES, name)]


def verify_reports(directory, group="golang"):
    reports = [json.loads(path.read_text()) for path in sorted(directory.glob(f"{group}-*.json"))]
    verify_group_reports(reports, group)
    print(f"Verified {sum(len(report['selected']) for report in reports)} {group} generator tests in {len(reports)} disjoint shards")


def verify_group_reports(reports, group):
    if not reports:
        raise ValueError(f"{group} generator shard reports are missing")
    first = reports[0]
    count = first["count"]
    if count < 1 or len(reports) != count or sorted(report["shard"] for report in reports) != list(range(1, count + 1)):
        raise ValueError("Go consumer shard reports are incomplete or duplicated")
    selected = []
    for report in reports:
        if (report["group"] != group or report["count"] != count or report["exitCode"] != 0
                or report["inventoryCount"] != first["inventoryCount"]
                or report["inventorySha256"] != first["inventorySha256"]
                or report.get("timingsSha256") != first.get("timingsSha256")
                or not report["selected"]
                or set(report["tests"]) != {PACKAGES[group] + "/" + name for name in report["selected"]}):
            raise ValueError("Go consumer shard execution differs from its inventory")
        selected.extend(report["selected"])
    digest = hashlib.sha256("\n".join(sorted(selected)).encode()).hexdigest()
    if len(selected) != len(set(selected)) or len(selected) != first["inventoryCount"] or digest != first["inventorySha256"]:
        raise ValueError("Go consumer shards overlap or omit tests")


def run(group, shard=1, count=1):
    selected = None
    timing_record = {}
    if group in PACKAGES:
        timings, digest = load_timings()
        inventory = group_tests(group, discover(PACKAGES[group]))
        selected = partition(inventory, shard, count, timings[group])
        timing_record = {"timingsSha256": digest,
                         "estimatedSeconds": round(sum(timings[group].get(name, 1) for name in selected), 6)}
        if not selected:
            raise ValueError("empty Go consumer shard")
        pattern = "^(?:" + "|".join(re.escape(name) for name in selected) + ")$"
        args = ["-run", pattern, PACKAGES[group]]
    elif group == "native":
        args = ["-run", "^TestRuntimeFeatureNativeMatrixRegression$", TYPESCRIPT]
    else:
        packages = subprocess.check_output(["go", "list", "./..."], text=True).splitlines()
        args = [package for package in packages if package not in (GOLANG, TYPESCRIPT)]
        if not args:
            raise ValueError("Go core package discovery returned no packages")
    started = time.monotonic()
    outcomes = {}
    completed = set()
    command = ["go", "test", "-json", "-count=1", "-timeout=20m", *args]
    with subprocess.Popen(command, stdout=subprocess.PIPE, text=True) as process:
        for line in process.stdout:
            # Keep the ordinary redacted script logs, including build failures.
            sys.stdout.write(line)
            try:
                event = json.loads(line)
            except ValueError:
                continue
            name = event.get("Test", "")
            if name and "/" not in name and event.get("Action") in ("pass", "fail", "skip"):
                outcomes[event["Package"] + "/" + name] = {"status": event["Action"], "seconds": event.get("Elapsed", 0)}
                if event["Package"] == PACKAGES.get(group):
                    completed.add(name)
        status = process.wait()
    report = {"group": group, "shard": shard, "count": count,
              "seconds": time.monotonic() - started, "exitCode": status,
              "tests": outcomes}
    if selected is not None:
        report.update(**timing_record, selected=selected, inventoryCount=len(inventory),
                      inventorySha256=hashlib.sha256("\n".join(sorted(inventory)).encode()).hexdigest())
        if status == 0 and completed != set(selected):
            status = 1
            report["exitCode"] = status
            report["failure"] = "selected Go tests did not all finish"
    destination = Path(".tmp/ci-tests")
    destination.mkdir(parents=True, exist_ok=True)
    (destination / f"{group}-{shard}.json").write_text(json.dumps(report, indent=2) + "\n")
    print(f"Go {group} {shard}/{count}: {len(outcomes)} tests, {report['seconds']:.1f}s", file=sys.stderr)
    return status


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("group", choices=("core", "golang", "typescript", "native", "verify", "refresh"))
    parser.add_argument("shard", type=int, nargs="?", default=1)
    parser.add_argument("count", type=int, nargs="?", default=1)
    parser.add_argument("--reports", type=Path, default=Path(".tmp/ci-tests"))
    args = parser.parse_args()
    if args.group not in PACKAGES and (args.shard != 1 or args.count != 1):
        parser.error("only generator test groups accept shards")
    try:
        if args.group == "refresh":
            refresh_timings(args.reports)
        elif args.group == "verify":
            for group in PACKAGES:
                verify_reports(args.reports, group)
        else:
            sys.exit(run(args.group, args.shard, args.count))
    except (ValueError, subprocess.CalledProcessError) as error:
        sys.exit(str(error))
