import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
const ts = createRequire(import.meta.url)("typescript-6");

export function generatedModuleGraph(root, entry, { allowGenericLoader = false } = {}) {
  const seen = new Set();
  function walk(file) {
    file = path.resolve(file);
    if (seen.has(file)) return;
    assert.ok(fs.existsSync(file), `Missing generated dependency ${file}`);
    seen.add(file);
    const source = ts.createSourceFile(
      file,
      fs.readFileSync(file, "utf8"),
      ts.ScriptTarget.Latest,
      true,
      ts.ScriptKind.JS,
    );
    function visit(node) {
      let specifier;
      if (ts.isImportDeclaration(node) || ts.isExportDeclaration(node))
        specifier = node.moduleSpecifier;
      else if (ts.isCallExpression(node) && node.expression.kind === ts.SyntaxKind.ImportKeyword)
        specifier = node.arguments[0];
      if (specifier !== undefined) {
        // The established generic loader's import(url) has no fixed dependency.
        // Named-entry callers opt in only while also exercising their real calls.
        if (
          !ts.isStringLiteral(specifier) &&
          allowGenericLoader &&
          path.basename(file) === "operation-loader.js" &&
          ts.isIdentifier(specifier) &&
          specifier.text === "url"
        )
          return;
        assert.ok(ts.isStringLiteral(specifier), `Nonliteral generated import ${file}`);
        if (specifier.text.startsWith(".")) walk(path.resolve(path.dirname(file), specifier.text));
      }
      ts.forEachChild(node, visit);
    }
    visit(source);
  }
  walk(path.join(root, entry));
  return [...seen].sort();
}
