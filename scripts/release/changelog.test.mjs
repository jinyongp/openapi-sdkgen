import assert from "node:assert/strict";
import { test } from "node:test";
import { prepareChangelog } from "./changelog.mjs";

const original = "# Changelog\n\n## Unreleased\n\n- Fixed a bug.\n\n## v1.0.0 — 2026-09-01\n\n- Initial release.\n";

test("moves notes under a dated release, keeping history and an empty Unreleased section", () => {
  assert.equal(prepareChangelog(original, "v1.0.1", "2026-10-01"),
    "# Changelog\n\n## Unreleased\n\n## v1.0.1 — 2026-10-01\n\n- Fixed a bug.\n\n## v1.0.0 — 2026-09-01\n\n- Initial release.\n");
});

test("retries preserve the preparation date and notes", () => {
  const prepared = prepareChangelog(original, "v1.0.1-rc.1", "2026-10-01");
  assert.equal(prepareChangelog(prepared, "v1.0.1-rc.1", "2026-10-02"), prepared);
});

test("empty, duplicate, misplaced, and conflicting entries fail before release", () => {
  for (const invalid of [
    "# Changelog\n",
    "# Changelog\n\n## Unreleased\n",
    original.replace("## v1.0.0", "## Unreleased\n\n## v1.0.0"),
    original.replace("## Unreleased", "## v0.9.0 — 2026-08-01\n\n## Unreleased"),
    original.replace("## v1.0.0", "## v1.0.1"),
    "# Changelog\n\n## Unreleased\n\n## v1.0.1 — 2026-10-01\n",
  ]) {
    assert.throws(() => prepareChangelog(invalid, "v1.0.1", "2026-10-01"));
  }
});
