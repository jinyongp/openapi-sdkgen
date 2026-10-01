"""Record runner resources and protect the runner from this job's checker."""

import argparse
import json
import os
from pathlib import Path
import signal
import subprocess
import time
from datetime import datetime, timezone


def process_rows():
    output = subprocess.check_output(
        ["ps", "-eo", "pid=,ppid=,rss=,pcpu=,comm="], text=True
    )
    rows = []
    for line in output.splitlines():
        pid, ppid, rss, cpu, name = line.split(maxsplit=4)
        rows.append(dict(pid=int(pid), ppid=int(ppid), rssKiB=int(rss), cpu=cpu, name=name))
    return rows


def memory_values():
    values = {}
    for line in Path("/proc/meminfo").read_text().splitlines():
        name, value = line.split(":", 1)
        if name in {"MemTotal", "MemAvailable", "SwapTotal", "SwapFree"}:
            values[name + "KiB"] = int(value.split()[0])
    return values


def checker_candidates(rows, root_pid, modules, executable):
    parents = {row["pid"]: row["ppid"] for row in rows}
    candidates = []
    for row in rows:
        if row["name"] not in {"tsc", "tsgo"} or row["rssKiB"] < 256 * 1024:
            continue
        pid = row["pid"]
        visited = set()
        while pid in parents and pid not in visited and pid != root_pid:
            visited.add(pid)
            pid = parents[pid]
        if pid != root_pid:
            continue
        try:
            path = executable(row["pid"]).resolve()
        except (OSError, RuntimeError):
            continue
        if path.is_relative_to(modules) and path.name in {"tsc", "tsgo"}:
            candidates.append(row)
    return sorted(candidates, key=lambda row: row["rssKiB"], reverse=True)


def observe(root_pid, modules, output, reserve_kib):
    next_sample = 0
    while True:
        rows = process_rows()
        memory = memory_values()
        row = {
            "at": datetime.now(timezone.utc).isoformat(),
            "memory": memory,
            "processes": sorted(rows, key=lambda item: item["rssKiB"], reverse=True)[:5],
        }
        remaining = memory["MemAvailableKiB"] + memory["SwapFreeKiB"]
        if remaining < reserve_kib:
            candidates = checker_candidates(
                rows, root_pid, modules, lambda pid: Path(f"/proc/{pid}/exe")
            )
            if candidates:
                checker = candidates[0]
                try:
                    os.kill(checker["pid"], signal.SIGKILL)
                except ProcessLookupError:
                    pass
                else:
                    row["terminatedChecker"] = checker
                    row["reason"] = "preserve runner responsiveness at configured memory reserve"
        if time.monotonic() >= next_sample or "terminatedChecker" in row:
            line = json.dumps(row)
            with output.open("a") as destination:
                destination.write(line + "\n")
            print(line, flush=True)
            next_sample = time.monotonic() + 60
        time.sleep(2)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--root-pid", type=int, required=True)
    parser.add_argument("--typescript-root", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--reserve-mib", type=int, default=3072)
    options = parser.parse_args()
    observe(
        options.root_pid,
        (options.typescript_root / "node_modules").resolve(),
        options.output,
        options.reserve_mib * 1024,
    )
