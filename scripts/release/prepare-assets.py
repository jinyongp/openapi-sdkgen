#!/usr/bin/env python3
from __future__ import annotations

import argparse
import hashlib
import json
import shutil
from pathlib import Path

PROJECT = "openapi-sdkgen"
PLATFORMS = (
    ("darwin", "amd64"),
    ("darwin", "arm64"),
    ("linux", "amd64"),
    ("linux", "arm64"),
    ("windows", "amd64"),
    ("windows", "arm64"),
)


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def load_binary_artifacts(path: Path) -> dict[tuple[str, str], Path]:
    artifacts = json.loads(path.read_text())
    selected: dict[tuple[str, str], Path] = {}

    for artifact in artifacts:
        if artifact.get("type") != "Binary":
            continue
        extra = artifact.get("extra") or {}
        if extra.get("ID") != PROJECT:
            continue

        platform = (artifact.get("goos"), artifact.get("goarch"))
        if platform not in PLATFORMS:
            continue
        if platform in selected:
            raise SystemExit(f"duplicate GoReleaser binary artifact for {platform[0]}/{platform[1]}")

        artifact_path = Path(artifact["path"])
        selected[platform] = artifact_path

    missing = [f"{goos}/{goarch}" for goos, goarch in PLATFORMS if (goos, goarch) not in selected]
    if missing:
        raise SystemExit(f"missing GoReleaser binary artifacts: {', '.join(missing)}")
    return selected


def release_name(version: str, goos: str, goarch: str) -> str:
    suffix = ".exe" if goos == "windows" else ""
    return f"{PROJECT}_{version}_{goos}_{goarch}{suffix}"


def archive_name(version: str, goos: str, goarch: str) -> str:
    suffix = ".zip" if goos == "windows" else ".tar.gz"
    return f"{PROJECT}_{version}_{goos}_{goarch}{suffix}"


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dist", type=Path, required=True)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()

    dist = args.dist
    artifacts_path = dist / "artifacts.json"
    if not artifacts_path.is_file():
        raise SystemExit(f"GoReleaser artifacts manifest not found: {artifacts_path}")

    binaries = load_binary_artifacts(artifacts_path)
    release_files: list[Path] = []

    for goos, goarch in PLATFORMS:
        source = binaries[(goos, goarch)]
        if not source.is_file():
            raise SystemExit(f"GoReleaser binary artifact does not exist: {source}")
        target = dist / release_name(args.version, goos, goarch)
        shutil.copyfile(source, target)
        release_files.append(target)

        archive = dist / archive_name(args.version, goos, goarch)
        if not archive.is_file():
            raise SystemExit(f"GoReleaser archive does not exist: {archive}")
        release_files.append(archive)

    checksums = dist / "checksums.txt"
    lines = [f"{sha256(path)}  {path.name}" for path in sorted(release_files, key=lambda value: value.name)]
    checksums.write_text("\n".join(lines) + "\n")


if __name__ == "__main__":
    main()
