#!/usr/bin/env python3
"""Check the saved Go-target plan against its exact repository basis."""

from __future__ import annotations

import hashlib
import json
import re
from pathlib import Path
from record_evidence import validate_evidence


def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def local_links(source: str) -> list[str]:
    prose = re.sub(r"(?ms)^```[^\n]*\n.*?^```[ \t]*$", "", source)
    prose = re.sub(r"(`+)(.*?)\1", "", prose, flags=re.S)
    return [target for target in re.findall(r"\[[^\]]+\]\(([^)]+)\)", prose)
            if "://" not in target and not target.startswith("#")]


def main() -> None:
    folder = Path(__file__).resolve().parent
    root = folder.parent.parent
    manifest_path = root / "docs/openapi-feature-manifest.json"
    manifest = json.loads(manifest_path.read_text())
    mapping = json.loads((folder / "feature-parity.json").read_text())
    evidence = json.loads((folder / "evidence.json").read_text())
    assert mapping["baseline"]["manifestSha256"] == digest(manifest_path), "manifest changed"
    source_ids = [item["id"] for item in manifest["features"]]
    mapped_ids = [item["id"] for item in mapping["features"]]
    assert len(source_ids) == len(set(source_ids)) == 302
    assert len(mapped_ids) == len(set(mapped_ids)) == 302
    assert set(source_ids) == set(mapped_ids), "feature ID coverage mismatch"
    supplemental_ids = [item["id"] for item in mapping["supplementalContracts"]]
    assert len(supplemental_ids) == len(set(supplemental_ids)) == 18
    for feature in mapping["features"] + mapping["supplementalContracts"]:
        assert feature["workItems"], feature["id"]
        assert all(re.fullmatch(r"W(?:0[1-9]|1[01])", item) for item in feature["workItems"])
        assert feature["acceptance"]
    condition_count = sum(len(item.get("conditions", [])) for item in manifest["features"])
    assert mapping["baseline"]["conditionalBranches"] == condition_count == 5
    references = set()
    for feature in manifest["features"]:
        references.add(feature.get("evidence", ""))
        references.update(item.get("evidence", "") for item in feature.get("conditions", []))
        references.update(feature.get("proof", {}).values())
    references.discard("")
    for reference in references:
        relative, separator, symbol = reference.partition("::")
        path = root / relative
        assert path.exists(), f"missing baseline evidence file: {relative}"
        if separator:
            source = path.read_text()
            exists = bool(re.search(r"func\s+" + re.escape(symbol) + r"\s*\(", source)) if path.suffix == ".go" else symbol in source
            assert exists, f"missing baseline evidence symbol: {reference}"
    validate_evidence(folder, root, evidence)
    assert evidence["regex"]["selectedEngineDifferences"] == 0
    assert evidence["regex"]["cases"] == len(json.loads((folder / "probes/regex-cases.json").read_text()))
    assert evidence["dependencyInspection"]["cgoPackages"] == []
    checked_links = 0
    for document in [folder / "plan.md", folder / "research.md", folder / "review.md"]:
        text = document.read_text()
        assert not re.search(r"[\u3040-\u30ff]", text), f"unexpected Japanese prose: {document.name}"
        for target in local_links(text):
            path = (document.parent / target.split("#", 1)[0]).resolve()
            assert path.exists(), f"broken link: {document.name}: {target}"
            checked_links += 1
    print(f"pass: 302 feature IDs, 5 conditional branches, {len(mapping['supplementalContracts'])} supplemental contracts, {len(references)} baseline evidence references, {checked_links} local links, current probe evidence")


if __name__ == "__main__":
    main()
