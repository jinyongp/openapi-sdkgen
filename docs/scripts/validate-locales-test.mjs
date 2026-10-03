import { appendFile, cp, mkdtemp, mkdir, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { validateLocaleStructure } from "./validate-locales.mjs";

const scriptsDirectory = dirname(fileURLToPath(import.meta.url));
const docsRoot = resolve(scriptsDirectory, "..");

await expectFailure(
  "missing Korean counterpart",
  async (fixture) => {
    await rm(resolve(fixture, "ko/guide/client.md"));
  },
  "Korean pages is missing /guide/client",
);

await expectFailure(
  "undeclared paired page",
  async (fixture) => {
    await writeFile(resolve(fixture, "guide/extra.md"), "# Extra\n");
    await writeFile(resolve(fixture, "ko/guide/extra.md"), "# 추가\n");
  },
  "English pages has undeclared route /guide/extra",
);

await expectFailure(
  "broken internal Markdown link",
  async (fixture) => {
    await appendFile(resolve(fixture, "guide/client.md"), "\n[Broken](./missing.md)\n");
  },
  "guide/client.md links to missing public documentation page ./missing.md",
);

console.log("ok documentation locale validator self-tests");

async function expectFailure(name, mutate, expected) {
  const fixture = await createFixture();
  try {
    await mutate(fixture);
    let error;
    try {
      await validateLocaleStructure({ docsRoot: fixture });
    } catch (cause) {
      error = cause;
    }
    if (!(error instanceof Error) || !error.message.includes(expected)) {
      throw new Error(
        `${name}: expected validation failure containing ${JSON.stringify(expected)}, got ${error instanceof Error ? error.message : String(error)}`,
      );
    }
  } finally {
    await rm(fixture, { recursive: true, force: true });
  }
}

async function createFixture() {
  const fixture = await mkdtemp(resolve(tmpdir(), "openapi-sdkgen-docs-locale-"));
  await mkdir(resolve(fixture, "ko"), { recursive: true });
  for (const name of ["index.md", "playground.md"]) {
    await cp(resolve(docsRoot, name), resolve(fixture, name));
    await cp(resolve(docsRoot, "ko", name), resolve(fixture, "ko", name));
  }
  for (const directory of ["guide", "examples", "reference"]) {
    await cp(resolve(docsRoot, directory), resolve(fixture, directory), { recursive: true });
    await cp(resolve(docsRoot, "ko", directory), resolve(fixture, "ko", directory), {
      recursive: true,
    });
  }
  return fixture;
}
