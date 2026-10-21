"""Run disjoint Go CI groups and preserve execution timings, without skipping tests."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys
import time


GOLANG = "openapi-sdkgen/internal/target/golang"
TYPESCRIPT = "openapi-sdkgen/internal/target/typescript"
MATRICES = "^TestRuntimeFeature(Native|Declaration)MatrixRegression$"


def partition(names, shard, count):
    if count < 1 or not 1 <= shard <= count:
        raise ValueError("shard must be between 1 and the positive shard count")
    return sorted(set(names))[shard - 1::count]


def discover(package):
    output = subprocess.check_output(["go", "test", "-list", ".", package], text=True)
    names = [line for line in output.splitlines() if re.fullmatch(r"(?:Test|Fuzz|Example)\w*", line)]
    if not names:
        raise ValueError("Go test discovery returned no runnable tests")
    return names


def verify_reports(directory):
    reports = [json.loads(path.read_text()) for path in sorted(directory.glob("golang-*.json"))]
    if not reports:
        raise ValueError("Go consumer shard reports are missing")
    first = reports[0]
    count = first["count"]
    if count < 1 or len(reports) != count or sorted(report["shard"] for report in reports) != list(range(1, count + 1)):
        raise ValueError("Go consumer shard reports are incomplete or duplicated")
    selected = []
    for report in reports:
        if (report["group"] != "golang" or report["count"] != count or report["exitCode"] != 0
                or report["inventoryCount"] != first["inventoryCount"]
                or report["inventorySha256"] != first["inventorySha256"]
                or not report["selected"]
                or set(report["tests"]) != {GOLANG + "/" + name for name in report["selected"]}):
            raise ValueError("Go consumer shard execution differs from its inventory")
        selected.extend(report["selected"])
    digest = hashlib.sha256("\n".join(sorted(selected)).encode()).hexdigest()
    if len(selected) != len(set(selected)) or len(selected) != first["inventoryCount"] or digest != first["inventorySha256"]:
        raise ValueError("Go consumer shards overlap or omit tests")
    print(f"Verified {len(selected)} Go consumer tests in {count} disjoint shards")


def run(group, shard=1, count=1):
    selected = None
    if group == "golang":
        inventory = discover(GOLANG)
        selected = partition(inventory, shard, count)
        if not selected:
            raise ValueError("empty Go consumer shard")
        pattern = "^(?:" + "|".join(re.escape(name) for name in selected) + ")$"
        args = ["-run", pattern, GOLANG]
    elif group == "typescript":
        args = ["-skip", MATRICES, TYPESCRIPT]
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
                if event["Package"] == GOLANG:
                    completed.add(name)
        status = process.wait()
    report = {"group": group, "shard": shard, "count": count,
              "seconds": time.monotonic() - started, "exitCode": status,
              "tests": outcomes}
    if selected is not None:
        report.update(selected=selected, inventoryCount=len(inventory),
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
    parser.add_argument("group", choices=("core", "golang", "typescript", "native", "verify"))
    parser.add_argument("shard", type=int, nargs="?", default=1)
    parser.add_argument("count", type=int, nargs="?", default=1)
    parser.add_argument("--reports", type=Path, default=Path(".tmp/ci-tests"))
    args = parser.parse_args()
    if args.group != "golang" and (args.shard != 1 or args.count != 1):
        parser.error("only the Go consumer group accepts shards")
    try:
        if args.group == "verify":
            verify_reports(args.reports)
        else:
            sys.exit(run(args.group, args.shard, args.count))
    except (ValueError, subprocess.CalledProcessError) as error:
        sys.exit(str(error))
