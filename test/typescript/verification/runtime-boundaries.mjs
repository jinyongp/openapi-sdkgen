import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";

const ts = createRequire(import.meta.url)("typescript-6");
const allowed = {
  shared: ["shared"],
  schema: ["schema", "shared"],
  stream: ["stream", "shared"],
  media: ["media", "schema", "stream", "shared"],
  security: ["security", "shared"],
  http: ["http", "schema", "media", "stream", "security", "shared"],
  client: ["client", "composition", "http", "schema", "media", "stream", "security", "shared"],
  composition: ["http", "schema", "media", "stream", "security", "shared"],
  server: ["server", "schema", "media", "stream", "security", "shared"],
  compatibility: [
    "compatibility",
    "client",
    "http",
    "server",
    "schema",
    "media",
    "stream",
    "security",
    "shared",
  ],
};

/** Includes erased type imports, re-exports, import types and dynamic imports. */
export function runtimeBoundaryViolations(root, { generated = false } = {}) {
  const graph = new Map();
  const violations = [];
  function layerOf(relative) {
    if (!generated) return relative.split("/")[0];
    if (relative.startsWith("internal/runtime/")) return relative.split("/")[2];
    if (relative.startsWith("internal/execution-compositions/")) return "composition";
    if (
      relative.startsWith("internal/schema-programs/") ||
      relative === "internal/types.ts" ||
      relative.startsWith("internal/schemas/") ||
      relative.startsWith("internal/projections/")
    )
      return "schema";
    if (relative.startsWith("server/")) return "server";
    return "client";
  }
  function inspect(file) {
    const relative = path.relative(root, file).split(path.sep).join("/");
    const layer = layerOf(relative);
    const edges = new Set();
    graph.set(relative, edges);
    const tree = ts.createSourceFile(
      file,
      fs.readFileSync(file, "utf8"),
      ts.ScriptTarget.Latest,
      true,
    );
    if (layer === "composition") {
      for (const statement of tree.statements) {
        if (
          !ts.isImportDeclaration(statement) &&
          !ts.isExportDeclaration(statement) &&
          !ts.isFunctionDeclaration(statement) &&
          !ts.isInterfaceDeclaration(statement) &&
          !ts.isTypeAliasDeclaration(statement)
        )
          violations.push(`${relative}: composition initializes module state`);
      }
    }
    function dependency(specifier, dynamic = false) {
      if (!ts.isStringLiteralLike(specifier)) {
        if (
          dynamic &&
          relative ===
            (generated
              ? "internal/runtime/client/operation-loader.ts"
              : "client/operation-loader.ts") &&
          ts.isIdentifier(specifier) &&
          specifier.text === "url"
        )
          return;
        violations.push(`${relative}: nonliteral dependency`);
        return;
      }
      if (!specifier.text.startsWith(".")) {
        violations.push(`${relative}: external dependency ${specifier.text}`);
        return;
      }
      const target = path.resolve(path.dirname(file), specifier.text.replace(/\.js$/, ".ts"));
      const targetRelative = path.relative(root, target).split(path.sep).join("/");
      const targetLayer = layerOf(targetRelative);
      if (!fs.existsSync(target)) violations.push(`${relative}: missing ${targetRelative}`);
      // The public arbitrary-schema server facade deliberately composes all codecs.
      const compatibilityRoot = relative === "server/runtime.ts" && targetLayer === "compatibility";
      if (!compatibilityRoot && !allowed[layer]?.includes(targetLayer))
        violations.push(`${relative}: forbidden ${layer} -> ${targetLayer} (${targetRelative})`);
      if (layer === "composition" && !targetRelative.startsWith("internal/runtime/"))
        violations.push(`${relative}: composition captures generated dependency ${targetRelative}`);
      edges.add(targetRelative);
    }
    function visit(node) {
      if ((ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) && node.moduleSpecifier)
        dependency(node.moduleSpecifier);
      if (ts.isImportTypeNode(node) && ts.isLiteralTypeNode(node.argument))
        dependency(node.argument.literal);
      if (ts.isCallExpression(node) && node.expression.kind === ts.SyntaxKind.ImportKeyword)
        dependency(node.arguments[0], true);
      ts.forEachChild(node, visit);
    }
    visit(tree);
  }
  function walk(directory) {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const file = path.join(directory, entry.name);
      if (entry.isDirectory()) walk(file);
      else if (entry.name.endsWith(".ts")) inspect(file);
    }
  }
  if (generated) {
    for (const directory of [
      "internal/runtime",
      "internal/schema-programs",
      "internal/execution-compositions",
      "server",
    ]) {
      const location = path.join(root, directory);
      if (fs.existsSync(location)) walk(location);
    }
  } else walk(root);
  const visited = new Set();
  const active = [];
  function checkCycle(file) {
    const index = active.indexOf(file);
    if (index !== -1) {
      violations.push(`cycle: ${[...active.slice(index), file].join(" -> ")}`);
      return;
    }
    if (visited.has(file)) return;
    active.push(file);
    for (const target of graph.get(file) ?? []) checkCycle(target);
    active.pop();
    visited.add(file);
  }
  for (const file of graph.keys()) checkCycle(file);
  return violations;
}
