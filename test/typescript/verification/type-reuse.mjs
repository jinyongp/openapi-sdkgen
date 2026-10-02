import { existsSync } from "node:fs";
import { createHash } from "node:crypto";
import { dirname, resolve } from "node:path";
import ts from "typescript-6";

/** Compare explicit object contracts; never infer types or retain syntax trees. */
export function createTypeReuseChecks(inputs, roots = []) {
  const files = new Set(inputs.map((file) => resolve(file)));
  const packages = new Map();
  const definitions = new Map();
  const reported = new Set();
  function packageRoot(file) {
    const explicit = roots.find((root) => file === root || file.startsWith(root + "/"));
    if (explicit) return explicit;
    function find(directory) {
      if (packages.has(directory)) return packages.get(directory);
      const parent = dirname(directory);
      const root =
        existsSync(resolve(directory, ".openapi-sdkgen-manifest.json")) ||
        existsSync(resolve(directory, "package.json"))
          ? directory
          : parent === directory
            ? undefined
            : find(parent);
      packages.set(directory, root);
      return root;
    }
    return find(dirname(file)) ?? dirname(file);
  }
  function modulePath(file, module) {
    if (!module.startsWith(".")) return module;
    const path = resolve(dirname(file), module.replace(/\.js$/, ".ts"));
    return [path, path + ".ts", resolve(path, "index.ts")].find((item) => files.has(item)) ?? path;
  }
  return function inspect(source, add) {
    const file = resolve(source.fileName);
    const imports = new Map();
    const localNames = new Set();
    const exported = new Set();
    const dependencies = new Map();
    const declarations = [];
    const scopes = new Map();
    function lexicalScope(node) {
      for (let parent = node.parent; parent; parent = parent.parent)
        if (ts.isSourceFile(parent) || ts.isBlock(parent) || ts.isModuleBlock(parent))
          return parent;
      return source;
    }
    function collect(node) {
      if (ts.isInterfaceDeclaration(node) || ts.isTypeAliasDeclaration(node)) {
        declarations.push(node);
      }
      if (
        (ts.isInterfaceDeclaration(node) ||
          ts.isTypeAliasDeclaration(node) ||
          ts.isClassDeclaration(node) ||
          ts.isEnumDeclaration(node) ||
          ts.isFunctionDeclaration(node) ||
          ts.isVariableDeclaration(node) ||
          ts.isParameter(node)) &&
        node.name &&
        ts.isIdentifier(node.name)
      ) {
        const scope = ts.isParameter(node) ? node.parent : lexicalScope(node);
        if (!scopes.has(scope)) scopes.set(scope, new Set());
        scopes.get(scope).add(node.name.text);
      }
      ts.forEachChild(node, collect);
    }
    collect(source);
    function references(nodes) {
      const names = new Set();
      function visit(node) {
        if (
          (ts.isTypeReferenceNode(node) && ts.isIdentifier(node.typeName)) ||
          (ts.isTypeQueryNode(node) && ts.isIdentifier(node.exprName))
        ) {
          const name = (node.typeName ?? node.exprName).text;
          let bound = false;
          if (ts.isTypeReferenceNode(node)) {
            for (let parent = node.parent; parent; parent = parent.parent)
              if (
                parent.typeParameters?.some((item) => item.name.text === name) ||
                (ts.isMappedTypeNode(parent) && parent.typeParameter.name.text === name)
              ) {
                bound = true;
                break;
              }
          }
          if (!bound) names.add(name);
        }
        ts.forEachChild(node, visit);
      }
      for (const node of nodes) if (node) visit(node);
      return names;
    }
    for (const statement of source.statements) {
      if (ts.isImportDeclaration(statement)) {
        const module = modulePath(file, statement.moduleSpecifier.text);
        if (statement.importClause?.name)
          imports.set(statement.importClause.name.text, `${module}#default`);
        const bindings = statement.importClause?.namedBindings;
        if (bindings && ts.isNamedImports(bindings)) {
          for (const item of bindings.elements)
            imports.set(item.name.text, `${module}#${(item.propertyName ?? item.name).text}`);
        } else if (bindings && ts.isNamespaceImport(bindings))
          imports.set(bindings.name.text, `${module}#`);
      }
      const isExport = !!statement.modifiers?.some(
        (item) => item.kind === ts.SyntaxKind.ExportKeyword,
      );
      if (statement.name && ts.isIdentifier(statement.name)) {
        const name = statement.name.text;
        localNames.add(name);
        const nodes =
          ts.isTypeAliasDeclaration(statement) || ts.isInterfaceDeclaration(statement)
            ? [statement]
            : ts.isFunctionDeclaration(statement)
              ? [statement.type, ...statement.parameters.map((item) => item.type)]
              : [];
        dependencies.set(name, references(nodes));
        if (isExport) exported.add(name);
      }
      if (ts.isVariableStatement(statement)) {
        for (const declaration of statement.declarationList.declarations) {
          if (!ts.isIdentifier(declaration.name)) continue;
          const name = declaration.name.text;
          localNames.add(name);
          dependencies.set(name, references([declaration.type]));
          if (isExport) exported.add(name);
        }
      }
      if (
        ts.isExportDeclaration(statement) &&
        !statement.moduleSpecifier &&
        statement.exportClause &&
        ts.isNamedExports(statement.exportClause)
      )
        for (const item of statement.exportClause.elements)
          exported.add((item.propertyName ?? item.name).text);
    }
    // Generated schema implementations remain public contracts through exported
    // projections. Follow only explicit signatures, never function bodies.
    const pending = [...exported];
    while (pending.length) {
      for (const name of dependencies.get(pending.pop()) ?? []) {
        if (!exported.has(name)) {
          exported.add(name);
          pending.push(name);
        }
      }
    }
    for (const statement of declarations) {
      const members =
        ts.isInterfaceDeclaration(statement) && !statement.heritageClauses
          ? statement.members
          : ts.isTypeAliasDeclaration(statement) && ts.isTypeLiteralNode(statement.type)
            ? statement.type.members
            : undefined;
      const name = statement.name.text;
      const parameters = new Map(
        (statement.typeParameters ?? []).map((item, index) => [
          item.name.text,
          `parameter:${index}`,
        ]),
      );
      for (let parent = statement.parent; parent; parent = parent.parent) {
        for (const parameter of parent.typeParameters ?? [])
          if (!parameters.has(parameter.name.text))
            parameters.set(
              parameter.name.text,
              `${file}#outer:${parent.pos}:${parameter.name.text}`,
            );
      }
      function localIdentity(text) {
        for (let parent = statement.parent; parent; parent = parent.parent) {
          if (scopes.get(parent)?.has(text))
            return ts.isSourceFile(parent)
              ? `${file}#${text}`
              : `${file}#scope:${parent.pos}:${text}`;
        }
        return localNames.has(text) ? `${file}#${text}` : undefined;
      }
      function entityIdentity(node) {
        if (ts.isQualifiedName(node)) {
          const left = entityIdentity(node.left);
          return `${left}${left.endsWith("#") ? "" : "."}${node.right.text}`;
        }
        const text = node.text;
        for (let parent = node.parent; parent && parent !== statement; parent = parent.parent)
          if (
            parent.typeParameters?.some((item) => item.name.text === text) ||
            (ts.isMappedTypeNode(parent) && parent.typeParameter.name.text === text)
          )
            return `${file}#generic:${parent.pos}:${text}`;
        return (
          parameters.get(text) ??
          (text === name ? "self" : (localIdentity(text) ?? imports.get(text) ?? text))
        );
      }
      function fingerprint(node) {
        if (ts.isParenthesizedTypeNode(node)) return fingerprint(node.type);
        if (ts.isTypeReferenceNode(node) || ts.isTypeQueryNode(node))
          return [
            node.kind,
            entityIdentity(node.typeName ?? node.exprName),
            (node.typeArguments ?? []).map(fingerprint),
          ];
        if (ts.isPropertySignature(node))
          return [
            node.kind,
            node.modifiers?.map((item) => item.kind) ?? [],
            ts.isIdentifier(node.name) || ts.isStringLiteral(node.name)
              ? node.name.text
              : fingerprint(node.name),
            !!node.questionToken,
            node.type && fingerprint(node.type),
          ];
        if (ts.isTypeLiteralNode(node)) return object(node.members);
        const children = [];
        ts.forEachChild(node, (child) => {
          children.push(fingerprint(child));
        });
        return [
          node.kind,
          children.length ? children : ts.isStringLiteral(node) ? node.text : node.getText(source),
        ];
      }
      function object(items) {
        const shapes = items.map((item) => JSON.stringify(fingerprint(item)));
        if (items.every(ts.isPropertySignature)) shapes.sort();
        return shapes;
      }
      // Repeated anonymous fields inside one named contract should reference
      // that contract's existing field, even when the outer contract is public.
      const nested = new Map();
      function inspectNested(node) {
        if (ts.isTypeLiteralNode(node) && node !== statement.type && node.members.length) {
          const key = JSON.stringify(object(node.members));
          const offset = node.getStart(source);
          const position = source.getLineAndCharacterOfPosition(offset);
          const previous = nested.get(key);
          if (previous)
            add({
              file,
              name,
              offset,
              line: position.line + 1,
              column: position.character + 1,
              rule: "duplicate-object-type",
              message: `reuse the existing object contract in ${name} at ${file}:${previous.line}:${previous.column}`,
            });
          else nested.set(key, { line: position.line + 1, column: position.character + 1 });
        }
        ts.forEachChild(node, inspectNested);
      }
      inspectNested(statement);
      if (!members?.length) continue;
      const key = createHash("sha256")
        .update(
          JSON.stringify([
            packageRoot(file),
            (statement.typeParameters ?? []).map((item) => [
              item.constraint && fingerprint(item.constraint),
              item.default && fingerprint(item.default),
            ]),
            object(members),
          ]),
        )
        .digest("hex");
      const offset = statement.name.getStart(source);
      const position = source.getLineAndCharacterOfPosition(offset);
      const current = {
        file,
        name,
        offset,
        line: position.line + 1,
        column: position.character + 1,
        public:
          (lexicalScope(statement) === source && exported.has(name)) ||
          !!statement.modifiers?.some((item) => item.kind === ts.SyntaxKind.ExportKeyword),
      };
      const previous = definitions.get(key);
      const report = (duplicate, canonical) => {
        const identity = `${duplicate.file}:${duplicate.offset}`;
        if (reported.has(identity)) return;
        reported.add(identity);
        const { public: visibility, ...location } = duplicate;
        add({
          ...location,
          rule: "duplicate-object-type",
          message: `reuse ${canonical.name} from ${canonical.file}:${canonical.line} instead of copying its object contract`,
        });
      };
      if (previous) {
        if (!current.public) report(current, previous);
        else if (!previous.public) report(previous, current);
        if (current.public) definitions.set(key, current);
      } else definitions.set(key, current);
    }
  };
}
