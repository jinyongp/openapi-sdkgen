import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { lstatSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import { resolve, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { performance } from "node:perf_hooks";
import ts from "typescript-6";
import { containedPath, sha256 } from "./catalog.mjs";
import { createTypeReuseChecks } from "./type-reuse.mjs";

assert.equal(ts.version, "6.0.3", "declaration parser version changed; requalify syntax fixtures");
const returnKinds = new Set([
  ts.SyntaxKind.FunctionDeclaration,
  ts.SyntaxKind.FunctionExpression,
  ts.SyntaxKind.ArrowFunction,
  ts.SyntaxKind.MethodDeclaration,
  ts.SyntaxKind.GetAccessor,
  ts.SyntaxKind.MethodSignature,
  ts.SyntaxKind.FunctionType,
  ts.SyntaxKind.CallSignature,
  ts.SyntaxKind.ConstructSignature,
  ts.SyntaxKind.ConstructorType,
]);

/** Syntax only: never create a Program or request a TypeChecker. */
export function inspectSource(file, text, add, counters = {}, reuse) {
  const source = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);
  const diagnostic = (rule, node, message = rule, offset = node.getStart(source)) => {
    const position = source.getLineAndCharacterOfPosition(offset);
    add({
      file,
      rule,
      name:
        node.name?.getText(source) ??
        (ts.isIdentifier(node) || ts.isBindingPattern(node) ? node.getText(source) : "<anonymous>"),
      line: position.line + 1,
      column: position.character + 1,
      offset,
      message,
    });
  };
  if (source.parseDiagnostics.length) {
    for (const item of source.parseDiagnostics)
      diagnostic(
        "parse-error",
        source,
        ts.flattenDiagnosticMessageText(item.messageText, " "),
        item.start ?? 0,
      );
    return;
  }
  function visit(node) {
    if (node.kind !== ts.SyntaxKind.EndOfFileToken && ts.nodeIsMissing(node)) {
      diagnostic("parse-error", node, "missing syntax node");
      return;
    }
    if (ts.isVariableDeclaration(node) && !ts.isCatchClause(node.parent)) {
      const list = node.parent;
      const iteration =
        ts.isVariableDeclarationList(list) &&
        (ts.isForOfStatement(list.parent) || ts.isForInStatement(list.parent));
      if (iteration) counters.exceptions = (counters.exceptions ?? 0) + 1;
      else if (!node.type) diagnostic("variable-type", node.name);
    }
    if (ts.isParameter(node) && !node.type) diagnostic("parameter-type", node.name);
    if (returnKinds.has(node.kind) && !node.type) diagnostic("return-type", node);
    if (ts.isConstructorDeclaration(node) || ts.isSetAccessor(node))
      counters.exceptions = (counters.exceptions ?? 0) + 1;
    if (
      (ts.isPropertyDeclaration(node) ||
        ts.isPropertySignature(node) ||
        ts.isIndexSignatureDeclaration(node)) &&
      !node.type
    )
      diagnostic("property-type", node.name ?? node);
    if (
      ts.isCatchClause(node) &&
      node.variableDeclaration &&
      node.variableDeclaration.type?.kind !== ts.SyntaxKind.UnknownKeyword
    )
      diagnostic("catch-unknown", node.variableDeclaration.name);
    if (ts.isTypeLiteralNode(node)) {
      let definition = false;
      for (let parent = node.parent; parent; parent = parent.parent) {
        if (ts.isTypeAliasDeclaration(parent) || ts.isInterfaceDeclaration(parent)) {
          definition = true;
          break;
        }
      }
      if (!definition) diagnostic("named-object-type", node);
    }
    ts.forEachChild(node, visit);
  }
  visit(source);
  (reuse ?? createTypeReuseChecks([file]))(source, add);
}

/** Refuse symlinks and discover every source, including new runtime subdirectories. */
export function sourceFiles(root) {
  root = resolve(root);
  assert.ok(
    lstatSync(root).isDirectory() && !lstatSync(root).isSymbolicLink(),
    `invalid source root: ${root}`,
  );
  const files = [];
  function visit(directory) {
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const file = containedPath(root, relative(root, resolve(directory, entry.name)));
      if (entry.isDirectory()) visit(file);
      else if (entry.isFile() && entry.name.endsWith(".ts")) files.push(file);
      else assert.ok(!entry.name.endsWith(".ts"), `invalid TypeScript input: ${file}`);
    }
  }
  visit(root);
  assert.ok(files.length, `no TypeScript inputs: ${root}`);
  return files.sort();
}

export function generatedFiles(root) {
  root = resolve(root);
  const manifest = JSON.parse(
    readFileSync(containedPath(root, ".openapi-sdkgen-manifest.json"), "utf8"),
  );
  assert.ok([1, 2].includes(manifest.version), "unsupported output manifest version");
  assert.equal(
    manifest.version === 2,
    manifest.generation != null,
    "manifest generation identity/version mismatch",
  );
  assert.ok(
    manifest.files && typeof manifest.files === "object" && !Array.isArray(manifest.files),
    "invalid manifest files",
  );
  const files = [];
  for (const [name, digest] of Object.entries(manifest.files)) {
    const file = containedPath(root, name);
    assert.equal(
      relative(root, file).split(sep).join("/"),
      name,
      `noncanonical manifest path: ${name}`,
    );
    assert.match(digest, /^[a-f0-9]{64}$/);
    assert.equal(sha256(readFileSync(file)), digest, `generated artifact hash mismatch: ${name}`);
    if (name.endsWith(".ts")) files.push(file);
  }
  assert.ok(files.length, `no manifest-owned TypeScript inputs: ${root}`);
  assert.deepEqual(files.sort(), sourceFiles(root), "TypeScript files missing from manifest");
  return files;
}

export function checkFiles(inputs, diagnosticLimit = 200, roots = []) {
  assert.ok(Array.isArray(inputs) && inputs.length, "empty TypeScript input list");
  assert.ok(
    inputs.every((file) => typeof file === "string" && file.endsWith(".ts")),
    "invalid TypeScript input list",
  );
  const files = inputs.map((file) => resolve(file)).sort();
  assert.equal(new Set(files).size, files.length, "duplicate TypeScript input");
  const result = {
    version: 1,
    parser: `typescript-${ts.version}`,
    rulesVersion: 2,
    node: process.version,
    files: 0,
    bytes: 0,
    violations: {},
    errors: 0,
    exceptions: 0,
    diagnostics: [],
    truncated: 0,
  };
  const hash = createHash("sha256");
  const reuse = createTypeReuseChecks(files, roots);
  const started = performance.now();
  const add = (item) => {
    if (["parse-error", "input-error"].includes(item.rule)) result.errors++;
    else result.violations[item.rule] = (result.violations[item.rule] ?? 0) + 1;
    if (result.diagnostics.length < diagnosticLimit) result.diagnostics.push(item);
    else result.truncated++;
  };
  for (const file of files) {
    try {
      const info = lstatSync(file);
      assert.ok(info.isFile() && !info.isSymbolicLink(), `invalid TypeScript file: ${file}`);
      const bytes = readFileSync(file);
      const text = new TextDecoder("utf-8", { fatal: true }).decode(bytes);
      hash.update(file).update("\0").update(bytes).update("\0");
      result.files++;
      result.bytes += bytes.length;
      inspectSource(file, text, add, result, reuse);
    } catch (error) {
      add({
        file,
        rule: "input-error",
        name: "<input>",
        line: 0,
        column: 0,
        offset: 0,
        message: error.message,
      });
    }
  }
  result.scanMillis = performance.now() - started;
  result.inputSha256 = hash.digest("hex");
  result.diagnostics.sort(
    (a, b) =>
      (a.file > b.file) - (a.file < b.file) ||
      a.offset - b.offset ||
      (a.rule > b.rule) - (a.rule < b.rule),
  );
  result.ok = result.errors === 0 && Object.keys(result.violations).length === 0;
  return result;
}

export function main(args) {
  const files = [];
  const sources = [];
  let json;
  let input = "<input>";
  const started = performance.now();
  try {
    for (let index = 0; index < args.length; index += 2) {
      const [option, value] = args.slice(index, index + 2);
      assert.ok(value, `missing value: ${option}`);
      switch (option) {
        case "--runtime":
        case "--generated":
        case "--files": {
          sources.push([option, value]);
          break;
        }
        case "--json":
          assert.equal(json, undefined, "duplicate JSON output");
          json = value;
          break;
        default:
          throw new Error(`unknown declaration option: ${option}`);
      }
    }
    for (const [option, value] of sources) {
      input = value;
      if (option === "--runtime") files.push(...sourceFiles(value));
      else if (option === "--generated") files.push(...generatedFiles(value));
      else {
        const list = JSON.parse(readFileSync(value, "utf8"));
        assert.ok(Array.isArray(list), "invalid TypeScript input list");
        files.push(...list);
      }
    }
    const inventoryMillis = performance.now() - started;
    const roots = sources
      .filter(([option]) => option !== "--files")
      .map(([, value]) => resolve(value));
    const result = checkFiles(files, 200, roots);
    result.inventoryMillis = inventoryMillis;
    result.toolMillis = performance.now() - started;
    if (json) writeFileSync(json, JSON.stringify(result, null, 2) + "\n");
    for (const item of result.diagnostics)
      console.error(
        `${item.file}:${item.line}:${item.column} [${item.rule}] ${item.name}: ${item.message}`,
      );
    if (result.truncated) console.error(`${result.truncated} additional diagnostics omitted`);
    console.log(
      `${result.ok ? "ok" : "failed"} declarations: ${result.files} files, ${Object.values(result.violations).reduce((a, b) => a + b, 0)} violations, ${result.errors} errors (${result.parser})`,
    );
    return result.ok ? 0 : 1;
  } catch (error) {
    if (json) {
      const report = {
        version: 1,
        parser: `typescript-${ts.version}`,
        rulesVersion: 2,
        node: process.version,
        files: 0,
        bytes: 0,
        violations: {},
        errors: 1,
        exceptions: 0,
        truncated: 0,
        ok: false,
        diagnostics: [
          {
            file: input,
            rule: "input-error",
            name: "<input>",
            line: 0,
            column: 0,
            offset: 0,
            message: error.message,
          },
        ],
      };
      try {
        writeFileSync(json, JSON.stringify(report, null, 2) + "\n");
      } catch (outputError) {
        console.error(`declaration report error: ${outputError.message}`);
      }
    }
    console.error(`declaration input error: ${error.message}`);
    return 2;
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url))
  process.exitCode = main(process.argv.slice(2));
