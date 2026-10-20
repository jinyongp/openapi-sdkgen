import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { sourceTagPlan } from "./release_tag_plan.mjs";

test("source tags retain release identity across retries and channels", () => {
  const cwd = mkdtempSync(join(tmpdir(), "sdkgen-tag-plan-"));
  const git = (...args) => execFileSync("git", args, { cwd, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] }).trim();
  try {
    git("init", "-b", "main");
    git("remote", "add", "origin", cwd);
    git("config", "user.name", "Release test");
    git("config", "user.email", "release@example.test");
    git("config", "commit.gpgsign", "false");
    git("config", "tag.gpgsign", "false");
    git("commit", "--allow-empty", "-m", "chore: historical release");
    git("tag", "v11.0.4");
    const boundary = git("rev-parse", "HEAD");
    writeFileSync(join(cwd, "source"), "release source\n");
    git("add", "source");
    git("commit", "-m", "feat: new release series");
    const source = git("rev-parse", "HEAD");
    const plan = {
      source,
      commit: "0".repeat(40), // Releaseway's unpushed marker must never be tagged.
      config: { tagPrefix: "v", series: { baseTag: "v11.0.4" } },
      tag: "v1.2.0",
      prereleaseId: "",
    };
    assert.deepEqual(sourceTagPlan(plan, cwd), { source, tag: "v1.2.0", version: "1.2.0" });
    git("tag", "v1.2.0-rc.0");
    assert.equal(sourceTagPlan(plan, cwd).tag, "v1.2.0");
    assert.equal(sourceTagPlan({ ...plan, tag: "v1.2.0-rc.1", prereleaseId: "rc" }, cwd).tag, "v1.2.0-rc.0");
    git("tag", "-a", "v1.2.0", "-m", "Stable release");
    assert.equal(sourceTagPlan({ ...plan, tag: "v1.2.1" }, cwd).tag, "v1.2.0");
    assert.equal(sourceTagPlan({ ...plan, tag: "v1.2.1-beta.0", prereleaseId: "beta" }, cwd).tag, "v1.2.1-beta.0");
    assert.throws(() => sourceTagPlan({ ...plan, source: boundary }, cwd), /checked-out source/);
    git("tag", "v1.2.1");
    assert.throws(() => sourceTagPlan(plan, cwd), /Multiple release tags/);
    git("checkout", "--detach", boundary);
    assert.throws(() => sourceTagPlan({ ...plan, source: boundary }, cwd), /historical series boundary/);
  } finally {
    rmSync(cwd, { recursive: true, force: true });
  }
});
