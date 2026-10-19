import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../..", import.meta.url));
const manifest = JSON.parse(await readFile(resolve(root, "docs/openapi-feature-manifest.json"), "utf8"));
const states = new Set(manifest.states);
const ids = new Set();
const sources = new Map();
async function checkEvidence(evidence, label) {
  const entries = Array.isArray(evidence) ? evidence : [evidence];
  if (!entries.length) throw new Error(`${label}: missing evidence`);
  for (const entry of entries) {
    if (typeof entry !== "string") throw new Error(`${label}: evidence must reference source`);
    const [path, name] = entry.split("::");
    if (!path || !name || path.startsWith("/") || path.split("/").includes("..")) throw new Error(`${label}: invalid evidence ${entry}`);
    if (!sources.has(path)) sources.set(path, await readFile(resolve(root, path), "utf8"));
    if (path.endsWith("_test.go")) {
      const test = name.split("/")[0];
      if (!/^Test[A-Za-z0-9_]+$/.test(test) || !sources.get(path).includes(`func ${test}(`)) throw new Error(`${label}: missing executable evidence ${entry}`);
    } else if (!/\.(ts|mjs)$/.test(path) || !sources.get(path).includes(name)) throw new Error(`${label}: missing executable evidence ${entry}`);
  }
}
for (const feature of manifest.features) {
  if (ids.has(feature.id)) throw new Error(`duplicate feature ID: ${feature.id}`);
  ids.add(feature.id);
  if (!states.has(feature.state)) throw new Error(`${feature.id}: invalid default TypeScript state`);
  await checkEvidence(feature.evidence, feature.id);
  const profile = feature.targetStates?.golang;
  if (!profile || !states.has(profile.state)) throw new Error(`${feature.id}: missing Go state`);
  await checkEvidence(profile.evidence, `${feature.id}/golang`);
  if (["error", "omitted"].includes(profile.state) && (!profile.failureScope || !profile.generationEffect)) throw new Error(`${feature.id}: Go failure scope/effect missing`);
  const conditions = feature.conditions ?? [];
  if (conditions.length !== (profile.conditions ?? []).length) throw new Error(`${feature.id}: Go conditional branch missing`);
  for (const [index, condition] of (profile.conditions ?? []).entries()) {
    if (condition.target !== "golang" || !states.has(condition.state)) throw new Error(`${feature.id}: invalid Go condition`);
    if (JSON.stringify(condition.with) !== JSON.stringify(conditions[index].with)) throw new Error(`${feature.id}: addon condition changed`);
    await checkEvidence(condition.evidence, `${feature.id}/golang/condition`);
  }
}
if (!manifest.targets.includes("typescript") || !manifest.targets.includes("golang")) throw new Error("target declaration missing");
console.error(`ok feature evidence: ${ids.size} IDs, ${manifest.features.reduce((count, feature) => count + (feature.conditions?.length ?? 0), 0)} conditional branches, TypeScript and Go`);
