import assert from "node:assert/strict";
import test from "node:test";
import { formatCodeBlocks } from "./format-code-blocks.mjs";

test("formats supported snippets while preserving surrounding Markdown and fence metadata", async () => {
  const source =
    '# Keep  prose\n\n```ts {2} title="example.ts"\nconst value={count:1}\n```\n\n~~~json\n{"ok":true}\n~~~~\n\n```yaml\nitems: [one,two]\n```\n';
  const result = await formatCodeBlocks(source);
  assert.equal(result.blocks, 3);
  assert.equal(result.changes.length, 3);
  assert.equal(
    result.source,
    '# Keep  prose\n\n```ts {2} title="example.ts"\nconst value = { count: 1 };\n```\n\n~~~json\n{ "ok": true }\n~~~~\n\n```yaml\nitems: [one, two]\n```\n',
  );
  assert.equal((await formatCodeBlocks(result.source)).changes.length, 0);
});

test("gofmt handles complete Go files and statement fragments", async () => {
  const result = await formatCodeBlocks(
    '```go\npackage main\nfunc main(){println("hello")}\n```\n\n```go\nvalue:=1\nprintln(value)\n```\n',
  );
  assert.equal(
    result.source,
    '```go\npackage main\n\nfunc main() { println("hello") }\n```\n\n```go\nvalue := 1\nprintln(value)\n```\n',
  );
  assert.equal(result.blocks, 2);
});

test("preserves unformatted literal output, shell and TOML, including nested fences", async () => {
  const source =
    '````text\n```ts\nconst x={unformatted:true}\n```\n````\n\n```sh\necho   "example"\n```\n\n```toml\nkey="value"\n```\n';
  const result = await formatCodeBlocks(source);
  assert.equal(result.source, source);
  assert.equal(result.blocks, 0);
});

test("preserves fence indentation", async () => {
  const result = await formatCodeBlocks("  ```js\n  const value=1\n  ```\n");
  assert.equal(result.source, "  ```js\n  const value = 1;\n  ```\n");
});

test("reports invalid examples with filename, line and language", async () => {
  await assert.rejects(
    formatCodeBlocks("# Example\n\n```ts\nconst =\n```\n", "guide.md"),
    /guide\.md:4 \(ts\)/,
  );
  await assert.rejects(
    formatCodeBlocks("```go\nfunc (\n```\n", "go.md"),
    /go\.md:2 \(go\)/,
  );
});
