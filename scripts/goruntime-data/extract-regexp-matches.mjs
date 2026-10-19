/** Extract Boolean matching contracts from pinned assertions, never execute source. */
import { readFileSync, writeFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { resolve } from 'node:path';
const root = resolve(import.meta.dirname, '../..');
const ts = (await import(resolve(root, 'test/typescript/node_modules/typescript-6/lib/typescript.js'))).default;
const inventory = JSON.parse(readFileSync(resolve(root, 'testdata/goruntime/corpora.json'), 'utf8'));
const cases = new Map(), files = [];
function unwrap(node) {
 while (node && ts.isParenthesizedExpression(node)) node = node.expression;
 return node;
}
function string(node) {
 node = unwrap(node);
 if (!node) return null;
 if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) return node.text;
 if (ts.isBinaryExpression(node) && node.operatorToken.kind === ts.SyntaxKind.PlusToken) {
  const a = string(node.left), b = string(node.right);
  return a === null || b === null ? null : a + b;
 }
 return null;
}
function pattern(node) {
 node = unwrap(node);
 if (!node) return null;
 if (ts.isRegularExpressionLiteral(node)) {
  const slash = node.text.lastIndexOf('/');
  return node.text.slice(slash + 1) === 'u' ? node.text.slice(1, slash) : null;
 }
 if ((ts.isNewExpression(node) || ts.isCallExpression(node)) && ts.isIdentifier(node.expression) && node.expression.text === 'RegExp') {
  return node.arguments?.[1] && string(node.arguments[1]) === 'u' ? string(node.arguments[0]) : null;
 }
 return null;
}
function match(node) {
 node = unwrap(node);
 if (!node || !ts.isCallExpression(node) || !ts.isPropertyAccessExpression(node.expression)) return null;
 const receiver = node.expression.expression, method = node.expression.name.text;
 let source, input;
 if (method === 'test' || method === 'exec') { source = pattern(receiver); input = string(node.arguments[0]); }
 else if (method === 'match') { input = string(receiver); source = pattern(node.arguments[0]); }
 else return null;
 return source === null || input === null ? null : { pattern: source, input, boolean: method === 'test' };
}
function units(value) { return Array.from({ length: value.length }, (_, i) => value.charCodeAt(i)); }
function record(sample, want, file, node) {
 const key = JSON.stringify([sample.pattern, sample.input]);
 const previous = cases.get(key);
 if (previous && previous.matches !== want) throw new Error(`Conflicting assertion: ${file}`);
 const c = previous ?? { pattern: sample.pattern, input: sample.input, patternUnits: units(sample.pattern), inputUnits: units(sample.input), matches: want, sources: [] };
 c.sources.push({ file, position: node.getStart(), assertion: node.getText().slice(0, 240) });
 cases.set(key, c);
}
for (const entry of inventory.files) {
 if (entry.suite !== 'test262' || !entry.path.startsWith('test/')) continue;
 const bytes = readFileSync(resolve(root, '.tmp/goruntime-corpora/test262', entry.path));
 if (createHash('sha256').update(bytes).digest('hex') !== entry.sha256) throw new Error(`Source hash mismatch: ${entry.path}`);
 const source = ts.createSourceFile(entry.path, bytes.toString('utf8'), ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
 let extracted = 0;
 function take(node, expected, mode) {
  const sample = match(node);
  if (!sample) return;
  expected = unwrap(expected);
  let want;
  if (mode === 'compareArray' && !sample.boolean && expected && ts.isArrayLiteralExpression(expected)) want = true;
  else if (mode === 'sameValue') {
   if (sample.boolean && expected?.kind === ts.SyntaxKind.TrueKeyword) want = true;
   else if (sample.boolean && expected?.kind === ts.SyntaxKind.FalseKeyword) want = false;
   else if (!sample.boolean && expected?.kind === ts.SyntaxKind.NullKeyword) want = false;
  } else if (mode === 'truthy') want = true;
  else if (mode === 'falsy') want = false;
  if (want !== undefined) { record(sample, want, entry.path, node); extracted++; }
 }
 function visit(node) {
  if (ts.isCallExpression(node)) {
   if (ts.isPropertyAccessExpression(node.expression) && ts.isIdentifier(node.expression.expression) && node.expression.expression.text === 'assert') {
    const mode = node.expression.name.text;
    if (mode === 'sameValue') { take(node.arguments[0], node.arguments[1], mode); take(node.arguments[1], node.arguments[0], mode); }
    else if (mode === 'compareArray') { take(node.arguments[0], node.arguments[1], mode); take(node.arguments[1], node.arguments[0], mode); }
   } else if (ts.isIdentifier(node.expression) && node.expression.text === 'assert') {
    const argument = unwrap(node.arguments[0]);
    if (argument && ts.isPrefixUnaryExpression(argument) && argument.operator === ts.SyntaxKind.ExclamationToken) take(argument.operand, null, 'falsy');
    else take(argument, null, 'truthy');
   }
  }
  ts.forEachChild(node, visit);
 }
 visit(source);
 files.push({ path: entry.path, assertions: extracted });
}
const output = { source: inventory.sources.test262, scope: 'Boolean existence derived from static u-mode assert/sameValue/compareArray assertions. No upstream JS execution, capture-array or JS-object conformance claim.', cases: [...cases.values()], files };
writeFileSync(resolve(root, 'testdata/goruntime/regexp-matches.json'), JSON.stringify(output, null, 2) + '\n');
console.log(JSON.stringify({ cases: cases.size, files: files.filter(f => f.assertions).length, assertions: files.reduce((sum, f) => sum + f.assertions, 0) }));
