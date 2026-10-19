import assert from "node:assert/strict";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { resolve } from "node:path";
import test from "node:test";
import { unzipSync } from "fflate";
import {
  buildExampleDownloads,
  exampleDownloads,
} from "./build-example-downloads.mjs";

const root = resolve(import.meta.dirname, "../..");
test("download archives contain exactly their complete source and license", async () => {
  const directory = await mkdtemp(
    resolve(tmpdir(), "sdkgen-example-downloads-"),
  );
  try {
    await buildExampleDownloads(directory);
    for (const example of exampleDownloads) {
      const path = resolve(directory, `${example.name}.zip`);
      const archive = await readFile(path);
      const files = unzipSync(archive);
      assert.deepEqual(
        Object.keys(files).sort(),
        [...example.files, "LICENSE"]
          .map((file) => `${example.name}/${file}`)
          .sort(),
      );
      for (const file of example.files)
        assert.deepEqual(
          Buffer.from(files[`${example.name}/${file}`]),
          await readFile(resolve(root, "examples", example.name, file)),
        );
      assert.deepEqual(
        Buffer.from(files[`${example.name}/LICENSE`]),
        await readFile(resolve(root, "LICENSE")),
      );
      await buildExampleDownloads(directory);
      assert.deepEqual(
        await readFile(path),
        archive,
        "Identical source produces identical downloads",
      );
    }
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});
