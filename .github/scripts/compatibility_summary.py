"""Render SDK measurements and measured runner resources in Actions summaries."""

import html
import json
from pathlib import Path


def cell(value):
    if isinstance(value, int):
        return f"{value:,}"
    return html.escape(str(value)).replace("|", "\\|").replace("\n", " ") if value is not None else "—"


def size(value):
    if value is None:
        return "—"
    for unit in ("B", "KiB", "MiB", "GiB"):
        if value < 1024 or unit == "GiB":
            return f"{value:,.2f} {unit}" if unit != "B" else f"{value:,} B"
        value /= 1024


def seconds(value):
    return f"{value:,.2f} s" if value is not None else "—"


def table(headers, rows):
    return "| " + " | ".join(headers) + " |\n| " + " | ".join("---" for _ in headers) + " |\n" + "\n".join("| " + " | ".join(cell(value) for value in row) + " |" for row in rows) + "\n"


def read_resources(directory):
    result = {}
    path = directory / "resources.txt"
    fields = {
        "User time (seconds)": ("cpuUserSeconds", float),
        "System time (seconds)": ("cpuSystemSeconds", float),
        "Percent of CPU this job got": ("cpuPercent", lambda value: float(value.rstrip("%"))),
        "Elapsed (wall clock) time (h:mm:ss or m:ss)": ("wallSeconds", lambda value: sum(float(part) * 60 ** index for index, part in enumerate(reversed(value.split(":"))))),
        "Maximum resident set size (kbytes)": ("peakRssBytes", lambda value: int(value) * 1024),
        "Major (requiring I/O) page faults": ("majorPageFaults", int),
        "File system inputs": ("filesystemInputBlocks", int),
        "File system outputs": ("filesystemOutputBlocks", int),
        "Exit status": ("exitCode", int),
    }
    if path.exists():
        for line in path.read_text().splitlines():
            for label, (key, parse) in fields.items():
                if line.strip().startswith(label + ":"):
                    value = line.strip()[len(label) + 1:].strip()
                    try:
                        result[key] = parse(value)
                    except ValueError:
                        pass
    path = directory / "progress.jsonl"
    samples = []
    if path.exists():
        for line in path.read_text().splitlines():
            try:
                samples.append(json.loads(line))
            except json.JSONDecodeError:
                result["incompleteSamples"] = result.get("incompleteSamples", 0) + 1
    memory = [sample["memory"] for sample in samples if "memory" in sample]
    if memory:
        result["sampleCount"] = len(memory)
        result["ramTotalBytes"] = max(sample["MemTotalKiB"] for sample in memory) * 1024
        result["minimumAvailableRamBytes"] = min(sample["MemAvailableKiB"] for sample in memory) * 1024
        result["swapTotalBytes"] = max(sample["SwapTotalKiB"] for sample in memory) * 1024
        result["peakSwapUsedBytes"] = max(sample["SwapTotalKiB"] - sample["SwapFreeKiB"] for sample in memory) * 1024
    protections = [sample for sample in samples if "terminatedChecker" in sample]
    if protections:
        result["protection"] = [dict(at=sample["at"], reason=sample["reason"], rssBytes=sample["terminatedChecker"]["rssKiB"] * 1024) for sample in protections]
    return result


def render_resources(resources):
    result = "\n### Benchmark resources\n\n"
    result += "CPU time and peak RSS cover the full benchmark command. Available RAM and swap use are runner snapshots sampled once per minute.\n\n"
    result += table(["Document", "Elapsed", "CPU average", "Peak RSS", "Min available RAM", "Peak swap used"],
        [[identifier, seconds(resource.get("wallSeconds")),
          f"{resource['cpuPercent']:g}%" if "cpuPercent" in resource else "—",
          size(resource.get("peakRssBytes")), size(resource.get("minimumAvailableRamBytes")),
          size(resource.get("peakSwapUsedBytes"))] for identifier, resource in resources.items()])
    result += "\n<details><summary>CPU time, runner capacity and I/O</summary>\n\n"
    result += table(["Document", "CPU user / system", "RAM / swap capacity", "Major page faults", "FS input / output blocks", "Samples", "Exit"],
        [[identifier, f"{seconds(resource.get('cpuUserSeconds'))} / {seconds(resource.get('cpuSystemSeconds'))}",
          f"{size(resource.get('ramTotalBytes'))} / {size(resource.get('swapTotalBytes'))}",
          resource.get("majorPageFaults"),
          f"{cell(resource.get('filesystemInputBlocks'))} / {cell(resource.get('filesystemOutputBlocks'))}",
          resource.get("sampleCount"), resource.get("exitCode")] for identifier, resource in resources.items()])
    result += "\n</details>\n"
    events = [f"**{cell(identifier)}: resource protection** · {cell(event['at'])} · {cell(event['reason'])} · checker RSS {size(event['rssBytes'])}\n"
              for identifier, resource in resources.items() for event in resource.get("protection", [])]
    if events:
        result += "\n<details><summary>Resource protection events</summary>\n\n" + "\n".join(events) + "\n</details>\n"
    return result


def render_report(manifest, report, resources=None):
    resources = resources or {}
    documents = report["documents"]
    client = sum(item["documentSuccess"] for item in documents)
    supported = sum(item["capabilityAdjustedSuccess"] for item in documents)
    result = f"**{supported}/{len(documents)} verified with a supported SDK profile · {client}/{len(documents)} client SDKs verified**\n\n"
    environment = report.get("measurement") or {}
    cpus = {value["cpu"] for value in report.get("documentMeasurements", {}).values() if value and value.get("cpu")}
    cpu = environment.get("cpu") or ("Varies by runner" if len(cpus) > 1 else next(iter(cpus), None))
    result += table(["Measured at (UTC)", "OS / architecture", "Go", "CPU"], [[environment.get("measuredAt"),
        f"{environment.get('os', '—')} / {environment.get('architecture', '—')}", environment.get("goVersion"), cpu]])
    profiles = [(item, name, profile) for item in documents for name, profile in
                [("client", item)] + [(p["name"], p) for p in item.get("supportProfiles", [])]]
    result += "\n### SDK verification\n\n"
    result += table(["Document", "Profile", "Generation", "Strict typecheck", "API calls", "Generation time", "Check time"],
        [[item["id"], name, profile["generation"]["status"], profile["typecheck"]["status"],
          (profile.get("operationEmission") or {}).get("count") if (profile.get("operationEmission") or {}).get("available") else None,
          seconds(profile["generation"].get("durationMillis") / 1000) if profile["generation"].get("durationMillis") is not None else "—",
          seconds(profile["typecheck"].get("durationMillis") / 1000) if profile["typecheck"].get("durationMillis") is not None else "—"] for item, name, profile in profiles])
    result += "\n<details><summary>Generated files and diagnostics</summary>\n\n"
    result += table(["Document", "OpenAPI", "Profile", "Files", "SDK size", "API omissions", "Helper omissions", "Errors / warnings"],
        [[item["id"], item.get("openapiVersion"), name, profile["generation"].get("artifactCount"), size(profile["generation"].get("artifactBytes")),
          (profile.get("operationEmission") or {}).get("operationOmissions"), (profile.get("operationEmission") or {}).get("helperOmissions"),
          f"{profile['diagnostics']['errors']} / {profile['diagnostics']['warnings']}"] for item, name, profile in profiles])
    result += "\n</details>\n"
    if resources:
        result += render_resources(resources)
    details = []
    for item, name, profile in profiles:
        for phase in ("generation", "typecheck"):
            if profile[phase].get("detail"):
                details.append(f"**{cell(item['id'])} / {cell(name)} / {phase}**\n\n<pre>{html.escape(profile[phase]['detail'])}</pre>\n")
    if details:
        result += "\n<details><summary>Failure details</summary>\n\n" + "\n".join(details) + "\n</details>\n"
    return result


def render_preparation(manifest, root, mode, elapsed=None):
    inputs = manifest["corpora"] + manifest.get("files", [])
    total = sum((root / item["input"]).stat().st_size for item in inputs)
    return table(["Input mode", "Documents", "Reference files", "Pinned input size", "Preparation elapsed"],
                 [[mode, len(manifest["corpora"]), len(manifest.get("files", [])), size(total), seconds(elapsed)]])
