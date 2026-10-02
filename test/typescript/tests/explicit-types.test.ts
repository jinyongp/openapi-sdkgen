import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript-6";
import { it, expect } from "vitest";

it("uses explicit variable and return types at the selective execution boundary", (): void => {
  const root: string = fileURLToPath(new URL("../fixtures/generated/client/", import.meta.url));
  const files: string[] = [
    "internal/runtime/operation-loader.ts",
    "selective/index.ts",
    "selective/all.ts",
  ];
  for (const directory of ["internal/executions", "selective/operations", "selective/lookup"]) {
    const entries: string[] = readdirSync(join(root, directory), {
      recursive: true,
      encoding: "utf8",
    });
    files.push(
      ...entries
        .filter((name: string): boolean => name.endsWith(".ts"))
        .map((name: string): string => join(directory, name)),
    );
  }
  for (const file of files) {
    const source: ts.SourceFile = ts.createSourceFile(
      file,
      readFileSync(join(root, file), "utf8"),
      ts.ScriptTarget.Latest,
      true,
    );
    function inspect(node: ts.Node): void {
      if (ts.isVariableDeclaration(node)) {
        // TypeScript disallows annotations on for...of/in bindings.
        const list: ts.Node = node.parent;
        const iteration: boolean =
          ts.isVariableDeclarationList(list) &&
          (ts.isForOfStatement(list.parent) || ts.isForInStatement(list.parent));
        if (!iteration)
          expect(node.type, `${file}: variable ${node.name.getText(source)}`).toBeDefined();
      }
      if (
        ts.isFunctionDeclaration(node) ||
        ts.isFunctionExpression(node) ||
        ts.isArrowFunction(node) ||
        ts.isMethodDeclaration(node)
      ) {
        expect(node.type, `${file}: function ${node.getText(source).slice(0, 80)}`).toBeDefined();
      }
      ts.forEachChild(node, inspect);
    }
    inspect(source);
  }
});
