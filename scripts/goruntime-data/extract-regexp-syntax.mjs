/** Prepare constructor-pattern fixtures without executing upstream JavaScript. */
import { readFileSync, writeFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { resolve } from 'node:path';
const root = resolve(import.meta.dirname, '../..');
const ts = (await import(resolve(root, 'test/typescript/node_modules/typescript-6/lib/typescript.js'))).default;
const cache = resolve(root, '.tmp/goruntime-corpora');
const inventory = JSON.parse(readFileSync(resolve(root, 'testdata/goruntime/corpora.json'), 'utf8'));
const cases = new Map(), files = [];
function literal(node) {
 if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) return node.text;
 if (ts.isParenthesizedExpression(node)) return literal(node.expression);
 if (ts.isBinaryExpression(node) && node.operatorToken.kind === ts.SyntaxKind.PlusToken) {
  const left = literal(node.left), right = literal(node.right);
  return left === null || right === null ? null : left + right;
 }
 if (ts.isRegularExpressionLiteral(node)) return node.text.slice(1, node.text.lastIndexOf('/'));
 return null;
}
function record(pattern, file, position, origin) {
 const key = JSON.stringify(pattern);
 let entry = cases.get(key);
 if (!entry) {
  let valid = true;
  try { new RegExp(pattern, 'u'); } catch { valid = false; }
  entry = { pattern, units: Array.from({ length: pattern.length }, (_, i) => pattern.charCodeAt(i)), valid, sources: [] };
  cases.set(key, entry);
 }
 entry.sources.push({ file, position, origin });
}
for (const entry of inventory.files) {
 if (entry.suite !== 'test262' || !entry.path.startsWith('test/')) continue;
 const bytes = readFileSync(resolve(cache, 'test262', entry.path));
 if (createHash('sha256').update(bytes).digest('hex') !== entry.sha256) throw new Error(`Source hash mismatch: ${entry.path}`);
 const text = bytes.toString('utf8');
 const source = ts.createSourceFile(entry.path, text, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
 let extracted = 0, otherFlags = 0, dynamic = 0;
 function visit(node) {
  if (ts.isRegularExpressionLiteral(node)) {
   const slash = node.text.lastIndexOf('/'), flags = node.text.slice(slash + 1);
   if (flags === 'u') { record(node.text.slice(1, slash), entry.path, node.pos, 'literal pattern recompiled with u'); extracted++; }
   else otherFlags++;
  }
  if ((ts.isNewExpression(node) || ts.isCallExpression(node)) && ts.isIdentifier(node.expression) && node.expression.text === 'RegExp') {
   const args = node.arguments ?? [], flags = args[1] ? literal(args[1]) : '';
   if (flags === 'u') {
    const pattern = args[0] ? literal(args[0]) : '';
    if (pattern !== null) { record(pattern, entry.path, node.pos, 'static constructor'); extracted++; }
    else dynamic++;
   } else if (flags === null) dynamic++;
   else otherFlags++;
  }
  ts.forEachChild(node, visit);
 }
 visit(source);
 files.push({ path: entry.path, sourceSha256: entry.sha256, staticUPatterns: extracted, otherFlagExpressions: otherFlags, dynamicExpressions: dynamic, status: extracted ? 'static syntax extracted; runtime semantics pending' : 'runtime adapter or explicit API-scope classification pending' });
}
const output = resolve(root, 'testdata/goruntime/regexp-syntax.json');
writeFileSync(output, JSON.stringify({ source: inventory.sources.test262, oracle: { node: process.version, unicode: process.versions.unicode }, scope: 'Static constructor-pattern validity with fixed u mode; this does not execute or pass Test262 JS tests.', cases: [...cases.values()] }, null, 2) + '\n');
writeFileSync(resolve(root, 'testdata/goruntime/regexp-coverage.json'), JSON.stringify({ source: inventory.sources.test262, files }, null, 2) + '\n');
console.log(JSON.stringify({ sourceFiles: files.length, uniquePatterns: cases.size, valid: [...cases.values()].filter(c => c.valid).length, invalid: [...cases.values()].filter(c => !c.valid).length }));
