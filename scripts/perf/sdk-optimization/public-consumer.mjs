// Freeze the existing public keys, then check that a candidate exposes exactly
// that independently captured surface through its public root.
import fs from "node:fs";
import path from "node:path";
const [mode, sdkArg, referenceArg, outputArg] = process.argv.slice(2);
if (!["freeze", "prepare"].includes(mode) || !sdkArg || !referenceArg) throw Error("usage: public-consumer.mjs freeze SDK REFERENCE | prepare SDK REFERENCE FRESH_OUTPUT");
const sdk = path.resolve(sdkArg), reference = path.resolve(referenceArg);
if (mode === "freeze") {
  // The frozen generator writes each Client map at a known indentation. Parse
  // JSON string tokens, never split keys on ':' or unescape them by substitution.
  const source = fs.readFileSync(path.join(sdk, "internal/client/types.ts"), "utf8");
  const maps = { routes: [], operations: [], links: [] };
  let linksPresent = false;
  let section;
  for (const line of source.split("\n")) {
    const start = /^  readonly \$(routes|operations|links): \{$/.exec(line);
    if (start) { section = start[1]; if (section === "links") linksPresent = true; continue; }
    if (line === "  }") { section = undefined; continue; }
    const key = /^    readonly ("(?:[^"\\]|\\.)*"):/.exec(line);
    if (section && key) maps[section].push(JSON.parse(key[1]));
  }
  if (!maps.routes.length || new Set(maps.routes).size !== maps.routes.length) throw Error("unrecognized or duplicate frozen route keys");
  if (!linksPresent) delete maps.links;
  fs.writeFileSync(reference, JSON.stringify(maps, null, 2) + "\n", { flag: "wx" });
  console.log(JSON.stringify(Object.fromEntries(Object.entries(maps).map(([key, value]) => [key, value.length]))));
} else {
  if (!outputArg) throw Error("fresh consumer output required");
  const output = path.resolve(outputArg);
  fs.mkdirSync(output, { mode: 0o700 });
  const keys = JSON.parse(fs.readFileSync(reference, "utf8"));
  let entry = path.relative(output, path.join(sdk, "index.js")).split(path.sep).join("/");
  if (!entry.startsWith(".")) entry = "./" + entry;
  const union = (names) => names.length ? names.map((name) => JSON.stringify(name)).join(" | ") : "never";
  const code = `import type { Client } from ${JSON.stringify(entry)};\n` +
    `type Equal<A, B> = [A] extends [B] ? [B] extends [A] ? true : false : false;\ntype Assert<T extends true> = T;\n` +
    Object.entries(keys).map(([key, names]) => `type Expected_${key} = ${union(names)};\nexport type Check_${key} = Assert<Equal<keyof Client["$${key}"], Expected_${key}>>;\n`).join("") +
    (!("links" in keys) ? `export type CheckAbsentLinks = Assert<Equal<Extract<"$links", keyof Client>, never>>;\n` : "") +
    `declare const client: Client;\n// @ts-expect-error unknown operation must stay rejected\nclient.$operations.__sdkgen_missing_operation_contract__;\n// @ts-expect-error unknown exact route must stay rejected\nclient.$routes["GET /__sdkgen_missing_route_contract__"];\n`;
  fs.writeFileSync(path.join(output, "public-contract.mts"), code, { flag: "wx" });
  const base = JSON.parse(fs.readFileSync(path.join(sdk, "tsconfig.optimization.json"), "utf8"));
  const files = [...base.files.map((file) => path.resolve(sdk, file)), path.join(output, "public-contract.mts")];
  fs.writeFileSync(path.join(output, "tsconfig.json"), JSON.stringify({ compilerOptions: base.compilerOptions, files }, null, 2) + "\n");
  console.log(JSON.stringify({ files: files.length, consumer: path.join(output, "public-contract.mts") }));
}
