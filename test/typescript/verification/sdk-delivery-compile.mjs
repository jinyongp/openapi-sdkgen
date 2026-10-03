// The pinned native compiler has no JS compiler-host API. Keep one generated
// tree, temporarily remove only @ts-nocheck, then restore every source by hash.
import assert from "node:assert/strict";
import { assertCheckedSources } from "./strict-options.mjs";
import fs from "node:fs";
import path from "node:path";
import { createHash, randomUUID } from "node:crypto";
import { createRequire } from "node:module";
import { spawnSync } from "node:child_process";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const require = createRequire(new URL("../package.json", import.meta.url));
const packageFile = require.resolve("typescript/package.json");
const compiler = path.resolve(
  path.dirname(packageFile),
  JSON.parse(fs.readFileSync(packageFile, "utf8")).bin.tsc,
);
// Whole-client roots need more headroom than isolated operation leaves. Keep
// one bounded compiler with room for the measured 10k-route graph; these test
// process settings do not relax checking flags or change consumer requirements.
const directive = "// @ts-nocheck\n";
const hash = (text) => createHash("sha256").update(text).digest("hex");

/** Fail before consuming the space needed for source recovery and failure evidence. */
export function requireVerificationSpace(directory, minimumBytes, inspect = fs.statfsSync) {
  assert(Number.isSafeInteger(minimumBytes) && minimumBytes > 0);
  const stats = inspect(directory);
  const availableBytes = Number(stats.bavail) * Number(stats.bsize);
  assert(Number.isSafeInteger(availableBytes) && availableBytes >= 0);
  assert(
    availableBytes >= minimumBytes,
    `SDK verification needs ${minimumBytes} available bytes before this phase; found ${availableBytes}. No validation was waived.`,
  );
  return { availableBytes, minimumBytes, freeInodes: Number(stats.ffree) };
}

function sources(directory) {
  const result = [];
  const pending = [directory];
  while (pending.length) {
    const current = pending.pop();
    for (const entry of fs.readdirSync(current, { withFileTypes: true })) {
      assert(!entry.isSymbolicLink(), "Generated verification tree must not contain symlinks");
      const filename = path.join(current, entry.name);
      if (entry.isDirectory()) pending.push(filename);
      else if (entry.isFile() && filename.endsWith(".ts")) result.push(filename);
    }
  }
  return result.sort();
}
function replaceAtomic(filename, text, id) {
  const temporary = filename + `.sdk-strict-${id}.tmp`;
  fs.writeFileSync(temporary, text, { flag: "wx" });
  fs.renameSync(temporary, filename);
}

// Recovery is explicit after an interrupted run, never an automatic takeover
// of another live compiler. Only known original/stripped bytes may be restored.
export function restoreGeneratedSources(journalFile) {
  const journal = JSON.parse(fs.readFileSync(journalFile, "utf8"));
  assert(path.resolve(journal.sourceRoot).startsWith(path.join(root, ".tmp") + path.sep));
  // Reject a stale recovery request before touching another compiler's inputs.
  const lock = path.join(journal.sourceRoot, ".sdk-delivery-strict-lock");
  if (fs.existsSync(lock)) {
    assert.equal(
      fs.readFileSync(lock, "utf8"),
      journalFile,
      "Another compiler owns the generated tree",
    );
  }
  for (const entry of journal.entries) {
    const filename = path.resolve(journal.sourceRoot, entry.name);
    assert(filename.startsWith(journal.sourceRoot + path.sep));
    assert.equal(fs.realpathSync(filename), filename);
    let text = fs.readFileSync(filename, "utf8");
    const current = hash(text);
    const temporary = filename + `.sdk-strict-${journal.id}.tmp`;
    if (fs.existsSync(temporary)) {
      const saved = hash(fs.readFileSync(temporary));
      assert(
        saved === entry.original || saved === entry.stripped,
        "Unexpected recovery temporary file",
      );
      fs.unlinkSync(temporary);
    }
    if (current === entry.original) continue;
    assert.equal(
      current,
      entry.stripped,
      `Generated source changed during verification: ${entry.name}`,
    );
    for (const offset of [...entry.offsets].reverse())
      text = text.slice(0, offset) + directive + text.slice(offset);
    assert.equal(hash(text), entry.original, "Recovery reconstruction differs from original");
    replaceAtomic(filename, text, journal.id);
  }
  if (fs.existsSync(lock)) {
    assert.equal(
      fs.readFileSync(lock, "utf8"),
      journalFile,
      "Another compiler owns the generated tree",
    );
    fs.unlinkSync(lock);
  }
  fs.unlinkSync(journalFile);
}

export function compileGenerated(configFile, maxRoots = 1000) {
  assert(Number.isInteger(maxRoots) && maxRoots > 0);
  const absolute = path.resolve(configFile);
  const config = JSON.parse(fs.readFileSync(absolute, "utf8"));
  const options = config.compilerOptions;
  for (const flag of ["strict", "noEmitOnError", "exactOptionalPropertyTypes", "declaration"])
    assert.equal(options[flag], true);
  assert.equal(options.skipLibCheck, false);
  assert.notEqual(
    options.noCheck,
    true,
    "Unchecked compilation cannot produce verification evidence",
  );
  const sourceRoot = fs.realpathSync(path.resolve(path.dirname(absolute), options.rootDir));
  assert(
    sourceRoot.startsWith(path.join(root, ".tmp") + path.sep),
    "Only disposable generated trees can be checked in place",
  );
  requireVerificationSpace(sourceRoot, 512 * 1024 * 1024);
  const files = sources(sourceRoot);
  const journalFile = absolute + ".source-journal.json";
  const lock = path.join(sourceRoot, ".sdk-delivery-strict-lock");
  assert(!fs.existsSync(journalFile), `Recover the interrupted compiler first: ${journalFile}`);
  fs.writeFileSync(lock, journalFile, { flag: "wx" });
  const journal = { id: randomUUID(), sourceRoot, entries: [] };
  let result;
  try {
    for (const filename of files) {
      const original = fs.readFileSync(filename, "utf8");
      const offsets = [];
      let cursor = 0,
        removed = 0,
        offset;
      while ((offset = original.indexOf(directive, cursor)) !== -1) {
        offsets.push(offset - removed);
        removed += directive.length;
        cursor = offset + directive.length;
      }
      if (offsets.length)
        journal.entries.push({
          name: path.relative(sourceRoot, filename),
          offsets,
          original: hash(original),
          stripped: hash(original.replaceAll(directive, "")),
        });
    }
    // The small recovery journal is durable before the first source change.
    const journalFD = fs.openSync(journalFile, "wx");
    fs.writeFileSync(journalFD, JSON.stringify(journal));
    fs.fsyncSync(journalFD);
    fs.closeSync(journalFD);
    for (const entry of journal.entries) {
      const filename = path.join(sourceRoot, entry.name);
      const original = fs.readFileSync(filename, "utf8");
      assert.equal(hash(original), entry.original, "Generated source changed before compilation");
      replaceAtomic(filename, original.replaceAll(directive, ""), journal.id);
    }
    assertCheckedSources(files);
    // A successful strict program checks its resolved imports as well as roots.
    // Record the compiler's actual file list and schedule only uncovered sources.
    // This avoids repeatedly checking the whole SDK through resource imports.
    const batches = [];
    const fileSet = new Set(files);
    const checked = new Set();
    let checkedRoots = 0;
    result = {
      status: "pass",
      diagnostics: "",
      files: files.length,
      strippedFiles: journal.entries.length,
      batches,
      checkedRoots,
      checkedFiles: 0,
      checkingPolicy: "strict-roots-and-resolved-imports",
      rootInventorySHA256: hash(files.map((name) => path.relative(sourceRoot, name)).join("\n")),
    };
    // Keep global entry points separate from thousands of lookup/provider
    // roots; mixing them needlessly retains both dependency graphs at once.
    const groups = new Map();
    for (const filename of files) {
      const relative = path.relative(sourceRoot, filename);
      const parts = relative.split(path.sep);
      const group = parts.length > 2 ? parts.slice(0, 2).join("/") : path.dirname(relative);
      if (!groups.has(group)) groups.set(group, []);
      groups.get(group).push(filename);
    }
    const programs = [...groups].flatMap(([group, entries]) => {
      const programs = [];
      for (let start = 0; start < entries.length; start += maxRoots)
        programs.push({ group, roots: entries.slice(start, start + maxRoots) });
      return programs;
    });
    for (const { group, roots: plannedRoots } of programs) {
      const roots = plannedRoots.filter((file) => !checked.has(file));
      if (roots.length === 0) continue;
      requireVerificationSpace(sourceRoot, 512 * 1024 * 1024);
      const index = batches.length;
      const batchConfig = absolute + `.batch-${index}.json`;
      const { include: _include, exclude: _exclude, ...baseConfig } = config;
      fs.writeFileSync(batchConfig, JSON.stringify({ ...baseConfig, files: roots }));
      fs.writeFileSync(
        absolute + ".stage.json",
        JSON.stringify({
          stage: "compiler-start",
          batch: index,
          group,
          roots: roots.length,
          checkedRoots,
          checkedFiles: checked.size,
          files: files.length,
          singleThreaded: true,
          goMemoryLimit: "4GiB",
        }),
      );
      const started = performance.now();
      const executed = spawnSync(
        process.execPath,
        [
          compiler,
          "--project",
          batchConfig,
          "--singleThreaded",
          "--extendedDiagnostics",
          "--listFiles",
        ],
        {
          encoding: "utf8",
          timeout: 180000,
          killSignal: "SIGKILL",
          maxBuffer: 16 * 1024 * 1024,
          env: { ...process.env, GOMEMLIMIT: "4GiB" },
        },
      );
      const diagnostics = (executed.stdout ?? "") + (executed.stderr ?? "");
      fs.writeFileSync(batchConfig + ".log", diagnostics);
      const resolved = new Set(
        (executed.stdout ?? "")
          .split(/\r?\n/)
          .map((name) => path.normalize(name))
          .filter((name) => fileSet.has(name)),
      );
      const before = checked.size;
      if (executed.status === 0) {
        for (const file of roots)
          assert(resolved.has(file), "Compiler omitted an explicit root from its file list");
        for (const file of resolved) checked.add(file);
        checkedRoots += roots.length;
      }
      const resolvedNames = [...resolved].map((file) => path.relative(sourceRoot, file)).sort();
      fs.writeFileSync(batchConfig + ".files.json", JSON.stringify(resolvedNames));
      batches.push({
        index,
        group,
        roots: roots.length,
        resolvedFiles: resolved.size,
        newlyCheckedFiles: checked.size - before,
        resolvedInventorySHA256: hash(resolvedNames.join("\n")),
        exitCode: executed.status,
        signal: executed.signal,
        error: executed.error?.message,
        elapsedMS: performance.now() - started,
      });
      result.checkedRoots = checkedRoots;
      result.checkedFiles = checked.size;
      fs.writeFileSync(
        absolute + ".stage.json",
        JSON.stringify({
          stage: "compiler-exit",
          checkedRoots,
          checkedFiles: checked.size,
          files: files.length,
          batches,
        }),
      );
      if (executed.status !== 0) {
        result.status = "fail";
        result.diagnostics = diagnostics;
        result.error = executed.error?.message;
        break;
      }
    }
    const checkedNames = [...checked].map((file) => path.relative(sourceRoot, file)).sort();
    result.checkedInventorySHA256 = hash(checkedNames.join("\n"));
    fs.writeFileSync(absolute + ".checked-files.json", JSON.stringify(checkedNames));
    if (result.status === "pass") {
      assert.equal(checked.size, files.length, "A generated source was not checked");
      assert.equal(
        result.checkedInventorySHA256,
        result.rootInventorySHA256,
        "Checked source inventory differs from generated sources",
      );
    }
  } finally {
    if (fs.existsSync(journalFile)) restoreGeneratedSources(journalFile);
    if (fs.existsSync(lock)) fs.unlinkSync(lock);
  }
  return result;
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  assert(process.argv[2], "Pass a generated SDK tsconfig path");
  const result = compileGenerated(process.argv[2]);
  if (result.diagnostics) console.error(result.diagnostics);
  if (result.error) console.error(result.error);
  console.log(
    JSON.stringify({
      status: result.status,
      files: result.files,
      checkedRoots: result.checkedRoots,
      checkedFiles: result.checkedFiles,
      checkingPolicy: result.checkingPolicy,
      checkedInventorySHA256: result.checkedInventorySHA256,
      rootInventorySHA256: result.rootInventorySHA256,
      batches: result.batches,
      strippedFiles: result.strippedFiles,
      physicalStrictCopy: false,
      restoredSources: true,
      singleThreaded: true,
      goMemoryLimit: "4GiB",
    }),
  );
  if (result.status !== "pass") process.exitCode = 1;
}
