"""Select public releases whose installer URLs must survive every site deployment."""

import json
import re
import subprocess
import sys


TAG = re.compile(r"v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?")
INSTALLERS = {"install.sh", "install.ps1"}


def select_tags(releases, required_tag):
    tags = []
    for release in releases:
        if release.get("draft"):
            continue
        assets = [asset for asset in release["assets"] if asset["name"] in INSTALLERS]
        if not assets:
            continue  # Historical releases before standalone installers existed.
        tag = release["tag_name"]
        if not TAG.fullmatch(tag):
            raise ValueError(f"Invalid installer release tag: {tag}")
        if len(assets) != 2 or {asset["name"] for asset in assets} != INSTALLERS:
            raise ValueError(f"Incomplete installer pair: {tag}")
        if not release.get("immutable"):
            raise ValueError(f"Installer release must be immutable: {tag}")
        if any(not re.fullmatch(r"sha256:[0-9a-f]{64}", asset.get("digest") or "") for asset in assets):
            raise ValueError(f"Missing installer digest: {tag}")
        tags.append(tag)
    if len(tags) != len(set(tags)):
        raise ValueError("Duplicate installer release tag")
    if required_tag not in tags:
        raise ValueError(f"Current release has no public installer pair: {required_tag}")
    return sorted(tags)


if __name__ == "__main__":
    repository, required_tag = sys.argv[1:]
    pages = json.loads(subprocess.check_output([
        "gh", "api", "--paginate", "--slurp",
        f"repos/{repository}/releases?per_page=100",
    ], text=True))
    print(json.dumps(select_tags([release for page in pages for release in page], required_tag)))
