import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";

const contacts = [
  { name: "Ada Lovelace", email: "ada@example.test" },
  { name: "Grace Hopper", email: "grace@example.test" },
];
export async function verifyConversionClient(directory, output) {
  const file = await readFile(resolve(directory, "contacts.ndjson"), "utf8");
  assert.equal(output, `Saved: contacts.ndjson\n${file.trimEnd()}`);
  assert.deepEqual(file.trimEnd().split("\n").map(JSON.parse), contacts);
}
export async function verifyConversionServer(baseURL) {
  const convert = (body) =>
    fetch(`${baseURL}/convert/contacts`, {
      method: "POST",
      headers: { "content-type": "application/octet-stream" },
      body,
      signal: AbortSignal.timeout(5_000),
    });
  for (const body of [
    "{",
    "null",
    "{}",
    '[{"name":"Ada"}]',
    "[null]",
    "[]{}",
  ]) {
    const response = await convert(body);
    assert.equal(response.status, 400);
    await response.text();
  }
  const large = await convert(" ".repeat(65 * 1024));
  assert.equal(large.status, 413);
  await large.text();
  for (const data of [
    [],
    [{ name: "김민수\nDeveloper", email: "minsu@example.test" }],
  ]) {
    const response = await convert(JSON.stringify(data));
    assert.equal(response.status, 200);
    assert.equal(
      response.headers.get("content-type"),
      "application/octet-stream",
    );
    assert.equal(
      response.headers.get("content-disposition"),
      'attachment; filename="contacts.ndjson"',
    );
    const text = await response.text();
    assert.deepEqual(
      text ? text.trimEnd().split("\n").map(JSON.parse) : [],
      data,
    );
  }
}
