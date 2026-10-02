import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import { expect, it } from "vitest";
import { build } from "vite";

const fixtureRoot = fileURLToPath(new URL("../fixtures/generated/client/", import.meta.url));

it("builds and runs one static selective operation under Vite production", async () => {
  const selectiveSource = readFileSync(join(fixtureRoot, "selective/index.ts"), "utf8");
  expect(selectiveSource).toContain("import(/* @vite-ignore */ filename)");
  expect(selectiveSource).not.toContain("import.meta");

  const entryDirectory = mkdtempSync(join(fixtureRoot, ".vite-production-"));
  const outputDirectory = mkdtempSync(join(tmpdir(), "openapi-sdkgen-vite-"));
  try {
    const entry = join(entryDirectory, "entry.ts");
    writeFileSync(
      entry,
      [
        'import { createClient, loadOperations } from "../selective/index.js";',
        'import { operation } from "../selective/operations/health/get.js";',
        "",
        "export async function exercise() {",
        "  const prepared = await loadOperations([operation]);",
        "  const requests: string[] = [];",
        "  const api = createClient({",
        "    operations: prepared,",
        '    baseURL: "https://api.test",',
        "    fetch: async (input) => {",
        "      requests.push(String(input));",
        "      return new Response(null, { status: 204 });",
        "    },",
        "  });",
        "  await api.health.get();",
        "  return { routes: Object.keys(api.$routes), requests };",
        "}",
        "",
      ].join("\n"),
    );

    const result = await build({
      root: fixtureRoot,
      configFile: false,
      logLevel: "silent",
      build: {
        target: "es2022",
        write: false,
        rollupOptions: {
          input: entry,
          preserveEntrySignatures: "strict",
          output: {
            format: "es",
            entryFileNames: "index.mjs",
            chunkFileNames: "chunks/[name]-[hash].mjs",
            assetFileNames: "assets/[name]-[hash][extname]",
          },
        },
      },
    });
    if (!Array.isArray(result) && !("output" in result)) {
      await result.close();
      throw new Error("Vite build unexpectedly entered watch mode");
    }
    const results = Array.isArray(result) ? result : [result];
    const outputs = results.flatMap((value) => value.output);
    const sourceAssets = outputs
      .filter((item) => item.type === "asset")
      .map((item) => item.fileName)
      .filter((name) => name.endsWith(".ts"));
    expect(sourceAssets).toEqual([]);

    const entryChunk = outputs.find((item) => item.type === "chunk" && item.isEntry);
    expect(entryChunk?.type).toBe("chunk");
    if (entryChunk?.type !== "chunk") throw new Error("Vite emitted no entry chunk");

    for (const item of outputs) {
      const destination = join(outputDirectory, item.fileName);
      mkdirSync(dirname(destination), { recursive: true });
      if (item.type === "chunk") writeFileSync(destination, item.code);
      else writeFileSync(destination, item.source);
    }

    const documentDescriptor = Object.getOwnPropertyDescriptor(globalThis, "document");
    const windowDescriptor = Object.getOwnPropertyDescriptor(globalThis, "window");
    Object.defineProperty(globalThis, "document", {
      configurable: true,
      value: {
        createElement() {
          return {
            relList: { supports: () => true },
            rel: "",
            as: "",
            crossOrigin: "",
            href: "",
            setAttribute() {},
            addEventListener() {},
          };
        },
        getElementsByTagName() {
          return [];
        },
        querySelector() {
          return null;
        },
        head: { appendChild() {} },
      },
    });
    Object.defineProperty(globalThis, "window", {
      configurable: true,
      value: { dispatchEvent: () => true },
    });
    try {
      const bundled = (await import(
        pathToFileURL(join(outputDirectory, entryChunk.fileName)).href
      )) as {
        exercise(): Promise<{ routes: string[]; requests: string[] }>;
      };
      await expect(bundled.exercise()).resolves.toEqual({
        routes: ["GET /health"],
        requests: ["https://api.test/health"],
      });
    } finally {
      if (documentDescriptor !== undefined)
        Object.defineProperty(globalThis, "document", documentDescriptor);
      else Reflect.deleteProperty(globalThis, "document");
      if (windowDescriptor !== undefined)
        Object.defineProperty(globalThis, "window", windowDescriptor);
      else Reflect.deleteProperty(globalThis, "window");
    }
  } finally {
    rmSync(entryDirectory, { recursive: true, force: true });
    rmSync(outputDirectory, { recursive: true, force: true });
  }
});
