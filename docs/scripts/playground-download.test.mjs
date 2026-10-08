import assert from "node:assert/strict";
import { test } from "node:test";
import { unzipSync, strFromU8 } from "fflate";
import { archiveArtifacts } from "../.vitepress/theme/playground/download.ts";

test("ZIP export preserves nested paths and exact UTF-8 content", async () => {
  const artifacts = [
    { path: "index.ts", content: 'export { createClient } from "./internal/client";\n' },
    { path: "internal/client/index.ts", content: '// Unicode: \uD55C\uAE00\nexport const createClient = () => ({});\n' },
    { path: "internal/empty.ts", content: "" },
  ];
  const archive = await archiveArtifacts(artifacts);
  const files = unzipSync(archive);
  assert.deepEqual(Object.keys(files), artifacts.map((artifact) => artifact.path));
  for (const artifact of artifacts) {
    assert.equal(strFromU8(files[artifact.path]), artifact.content);
  }
});
