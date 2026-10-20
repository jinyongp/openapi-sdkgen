import { execFileSync } from "node:child_process";
import { appendFileSync, readFileSync, writeFileSync } from "node:fs";
import { pathToFileURL } from "node:url";

// Releaseway owns version policy. Its plan includes a marker commit; our tag
// binds directly to the source instead. Reuse a tag after a partial dispatch.
export function sourceTagPlan(plan, cwd = process.cwd()) {
  const git = (...args) => execFileSync("git", args, { cwd, encoding: "utf8" }).trim();
  if (!/^[0-9a-f]{40}$/.test(plan.source) || git("rev-parse", "HEAD") !== plan.source) {
    throw new Error("Release plan must match the checked-out source");
  }
  const prefix = plan.config.tagPrefix;
  const channel = plan.prereleaseId;
  // Read origin rather than local tags: a previous tag push may have succeeded
  // even when its client never received the result or updated local refs.
  const refs = git("ls-remote", "origin", "refs/tags/*").split("\n");
  const sourceTags = new Set(refs.flatMap((line) => {
    const [commit, ref] = line.split(/\s+/);
    return commit === plan.source ? [ref.replace(/^refs\/tags\//, "").replace(/\^\{\}$/, "")] : [];
  }));
  const tags = [...sourceTags].filter((tag) => {
    if (!tag.startsWith(prefix)) return false;
    const match = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([A-Za-z][A-Za-z0-9-]*)\.(0|[1-9]\d*))?$/.exec(tag.slice(prefix.length));
    return match && (match[4] || "") === channel;
  });
  if (tags.length > 1) {
    throw new Error(`Multiple release tags match this source; use --resume TAG: ${tags.join(", ")}`);
  }
  // The series boundary itself is historical, never a release in this series.
  if (plan.config.series && sourceTags.has(plan.config.series.baseTag)) {
    throw new Error("Release source must follow the historical series boundary");
  }
  const tag = tags[0] || plan.tag;
  return { source: plan.source, tag, version: tag.slice(prefix.length) };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const [input, output] = process.argv.slice(2);
  const plan = sourceTagPlan(JSON.parse(readFileSync(input, "utf8")));
  writeFileSync(output, `${JSON.stringify(plan, null, 2)}\n`);
  const values = `tag=${plan.tag}\nversion=${plan.version}\ncommit=${plan.source}\n`;
  if (process.env.GITHUB_OUTPUT) appendFileSync(process.env.GITHUB_OUTPUT, values);
  else process.stdout.write(values);
}
