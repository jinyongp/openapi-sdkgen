#!/usr/bin/env python3
"""Execute probes against a captured basis; never certify pre-existing logs."""
from __future__ import annotations

from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile


def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def snapshot(folder: Path, root: Path) -> dict[str, str]:
    paths = list((folder / "probes").glob("*"))
    paths += [folder / name for name in ["record_evidence.py", "verify_plan.py", "test_evidence.py",
                                       "plan.md", "research.md", "review.md", "feature-parity.json"]]
    manifest = root / "docs/openapi-feature-manifest.json"
    paths.append(manifest)
    for item in json.loads(manifest.read_text())["features"]:
        refs = [item.get("evidence", ""), *item.get("proof", {}).values()]
        refs += [condition.get("evidence", "") for condition in item.get("conditions", [])]
        paths += [root / ref.partition("::")[0] for ref in refs if ref]
    paths += [root / "scripts/lib" / name for name in ["commands.sh", "redact.py"]]
    paths += [root / "internal/target/typescript" / name for name in [
        "enums.go", "links.go", "server.go", "runtime/client/links.ts",
        "runtime/client/links-types.ts", "runtime/media/media-codec-types.ts", "runtime/server/runtime-types.ts"]]
    paths += [root / name for name in ["internal/generator/generator.go", "internal/generator/pipeline.go",
        "internal/target/typescript/emit.go", "internal/target/typescript/schema/plan/node.go", "docs/architecture.md"]]
    return {str(path.relative_to(root)): digest(path) for path in sorted(set(paths)) if path.is_file()}


def commands() -> dict[str, list[str]]:
    return {
        "list": ["go", "test", "-list", "^Test", "."],
        "tests": ["go", "test", "-count=1", "-timeout", "120s", "-json", "."],
        "race": ["go", "test", "-race", "-count=1", "-timeout", "120s", "-json", "."],
        "purego": ["env", "CGO_ENABLED=0", "go", "test", "-count=1", "-timeout", "120s", "-json", "."],
        "benchmark": ["go", "test", "-run", "^$", "-bench", "BenchmarkRegexEngines", "-benchmem", "-benchtime=100ms", "-count=3"],
        "deps": ["env", "CGO_ENABLED=0", "go", "list", "-deps", "-json", "github.com/go-quickjs/go-quickjs/jsregexp",
                 "github.com/andybalholm/brotli", "github.com/klauspost/compress/zstd"],
        "vet": ["go", "vet", "./..."],
        "go": ["go", "env", "-json", "GOVERSION", "GOOS", "GOARCH", "CGO_ENABLED"],
        "node": ["node", "-p", "JSON.stringify(process.versions)"],
        "evidence-tests": [sys.executable, "-B", "-m", "unittest", "discover", "-s", "..", "-p", "test_evidence.py"],
    }


def json_stream(value: str) -> list[dict]:
    result = []
    decoder = json.JSONDecoder()
    while value.strip():
        item, index = decoder.raw_decode(value.lstrip())
        result.append(item)
        value = value.lstrip()[index:]
    return result


def summarize(run: Path, current: dict[str, str]) -> dict:
    basis = json.loads((run / "basis.json").read_text())
    assert basis["sourceSha256"] == current, "source changed since run started; rerun probes"
    results = json.loads((run / "results.json").read_text())
    assert set(results) == set(commands()), "missing or unexpected check"
    checks = {}
    output = {}
    for name, command in commands().items():
        item = results[name]
        assert item["command"] == command and item["exitCode"] == 0, f"failed/replaced check: {name}"
        for stream in ["stdout", "stderr"]:
            path = run / f"{name}.{stream}"
            assert digest(path) == item[stream + "Sha256"], f"changed log: {name}.{stream}"
        output[name] = (run / f"{name}.stdout").read_text()
        checks[name] = {"status": "pass", **item}
    expected = {line for line in output["list"].splitlines() if re.fullmatch(r"Test\w+", line)}
    assert expected, "no discovered tests"
    for name in ["tests", "race", "purego"]:
        events = json_stream(output[name])
        assert not any(item["Action"] in ["fail", "skip"] for item in events), f"failed/skipped test: {name}"
        terminals = [item for item in events if "Test" not in item and item["Action"] in ["pass", "fail"]]
        assert len(terminals) == 1 and terminals[0]["Action"] == "pass", f"incomplete package: {name}"
        passed = {item["Test"] for item in events if item["Action"] == "pass" and "Test" in item and "/" not in item["Test"]}
        assert passed == expected, f"missing top-level tests: {name}"
        checks[name].update(topLevelTests=len(passed), packageElapsedSeconds=terminals[0]["Elapsed"])
    observations = [item["Output"].strip() for item in json_stream(output["tests"])
                    if item.get("Action") == "output" and "OBSERVATION " in item.get("Output", "")]
    regex_lines = [item["Output"] for item in json_stream(output["tests"])
                   if item.get("Action") == "output" and "ECMAScript Unicode samples;" in item.get("Output", "")]
    assert len(regex_lines) == 1
    match = re.search(r"(\d+) ECMAScript Unicode samples; standard regexp differences: (\d+); raw regexp2 differences: (\d+); goja differences: (\d+); selected differences: (\d+)", regex_lines[0])
    assert match
    cases, standard, regexp2, goja, selected = map(int, match.groups())
    assert selected == 0, "selected regex engine differs"
    packages = json_stream(output["deps"])
    assert all(not item.get("Error") and not item.get("DepsErrors") for item in packages)
    cgo = sorted({item["ImportPath"] for item in packages if item.get("CgoFiles")})
    assert not cgo, "CGO dependency appeared"
    samples = []
    for line in output["benchmark"].splitlines():
        match = re.match(r"BenchmarkRegexEngines/(\S+)\s+\d+\s+([\d.]+) ns/op\s+(\d+) B/op\s+(\d+) allocs/op", line)
        if match:
            samples.append({"variant": match[1].rsplit("-", 1)[0], "nsPerOp": float(match[2]),
                            "bytesPerOp": int(match[3]), "allocsPerOp": int(match[4])})
    assert len(samples) == 9 and all(sum(item["variant"] == variant for item in samples) == 3
                                    for variant in ["standard", "ecmascript", "ecmascript-utf8"])
    assert "PASS" in output["benchmark"], "incomplete benchmark"
    return {
        "schemaVersion": 2,
        "scope": "Design probes only; generated Go SDK parity and performance remain unimplemented",
        "runID": basis["runID"], "startedAt": basis["startedAt"],
        "sourceSha256": current, "checks": checks,
        "environment": {"go": json.loads(output["go"]), "node": json.loads(output["node"])},
        "regex": {"cases": cases, "standardEngineDifferences": standard, "regexp2Differences": regexp2,
                  "gojaDifferences": goja, "selectedEngineDifferences": selected,
                  "basis": "All discovered comparison tests passed; finite samples only"},
        "dependencyInspection": {"nonStandardPackages": sorted({item["ImportPath"] for item in packages if not item.get("Standard")}),
                                 "cgoPackages": cgo},
        "observations": observations, "microbenchmark": {"scope": "Local ASCII regex probe only", "samples": samples},
    }


def validate_evidence(folder: Path, root: Path, evidence: dict) -> None:
    run = (root / evidence["runDirectory"]).resolve()
    assert run.is_relative_to(root / ".tmp/golang-target-plan"), "unexpected run directory"
    expected = summarize(run, snapshot(folder, root))
    expected["runDirectory"] = evidence["runDirectory"]
    assert evidence == expected, "saved evidence differs from actual run"


def main() -> None:
    folder = Path(__file__).resolve().parent
    root = folder.parent.parent
    parent = root / ".tmp/golang-target-plan"
    parent.mkdir(parents=True, exist_ok=True)
    run = Path(tempfile.mkdtemp(prefix="run-", dir=parent))
    current = snapshot(folder, root)
    basis = {"runID": run.name, "startedAt": datetime.now(timezone.utc).isoformat(), "sourceSha256": current}
    (run / "basis.json").write_text(json.dumps(basis, indent=2) + "\n")
    results = {}
    for name, command in commands().items():
        with (run / f"{name}.stdout").open("wb") as stdout, (run / f"{name}.stderr").open("wb") as stderr:
            result = subprocess.run(command, cwd=folder / "probes", stdout=stdout, stderr=stderr, timeout=300)
        results[name] = {"command": command, "exitCode": result.returncode,
                         "stdoutSha256": digest(run / f"{name}.stdout"), "stderrSha256": digest(run / f"{name}.stderr")}
        (run / "results.json").write_text(json.dumps(results, indent=2) + "\n")
        if result.returncode:
            print((run / f"{name}.stdout").read_text()[-5000:], file=sys.stderr)
            print((run / f"{name}.stderr").read_text()[-5000:], file=sys.stderr)
            raise SystemExit(f"failed: {name}; logs: {run}")
        print(f"pass: {name}", file=sys.stderr, flush=True)
    evidence = summarize(run, snapshot(folder, root))
    evidence["runDirectory"] = str(run.relative_to(root))
    descriptor, temporary = tempfile.mkstemp(prefix=".evidence-", dir=folder)
    try:
        with os.fdopen(descriptor, "w") as target:
            target.write(json.dumps(evidence, ensure_ascii=False, indent=2) + "\n")
        os.replace(temporary, folder / "evidence.json")
    finally:
        Path(temporary).unlink(missing_ok=True)
    print(f"saved executed evidence: {run}", file=sys.stderr)


if __name__ == "__main__":
    main()
