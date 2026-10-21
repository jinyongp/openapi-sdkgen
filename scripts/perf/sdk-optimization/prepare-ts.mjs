// Enumerate every generated .ts explicitly; no source exclusions or no-check flags.
import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
const [rootArg] = process.argv.slice(2);
if (!rootArg) throw Error("usage: prepare-ts.mjs SDK_ROOT");
const root = path.resolve(rootArg);
const packagePath = path.join(root, "package.json");
if (fs.existsSync(packagePath)) {
  if (JSON.parse(fs.readFileSync(packagePath, "utf8")).type !== "module") throw Error("SDK must be an ESM consumer");
} else {
  fs.writeFileSync(packagePath, JSON.stringify({ type: "module", private: true }) + "\n", { flag: "wx" });
}
function walk(dir) {
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    if (entry.name === "node_modules") throw Error("unexpected node_modules in generated SDK");
    const file = path.join(dir, entry.name);
    return entry.isDirectory() ? walk(file) : entry.isFile() && entry.name.endsWith(".ts") ? [file] : [];
  });
}
const files = walk(root).map((file) => path.relative(root, file).split(path.sep).join("/")).sort();
if (!files.length) throw Error("empty SDK");
for (const file of files) if (fs.readFileSync(path.join(root, file), "utf8").includes("@ts-nocheck")) throw Error("no-check source: " + file);
const strict = JSON.parse(fs.readFileSync(new URL("../../../internal/tscheck/strict-options.json", import.meta.url), "utf8"));
const compilerOptions = { ...strict.compilerOptions, target: "ES2022", module: "NodeNext", moduleResolution: "NodeNext", lib: ["ES2022", "DOM", "DOM.Iterable"], types: [], noEmit: true, incremental: false };
const config = JSON.stringify({ compilerOptions, files }, null, 2) + "\n";
const output = path.join(root, "tsconfig.optimization.json");
if (fs.existsSync(output)) {
  if (fs.readFileSync(output, "utf8") !== config) throw Error("existing configuration differs");
} else fs.writeFileSync(output, config, { flag: "wx" });
console.log(JSON.stringify({ config: output, files: files.length, sha256: crypto.createHash("sha256").update(config).digest("hex") }));
