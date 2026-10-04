import { gzipSync } from "node:zlib";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { beforeAll, describe, expect, it } from "vitest";
import { build } from "vite";

const fixtureRoot = fileURLToPath(
  new URL("../fixtures/generated/bundle-isolation/", import.meta.url),
);
const publicEntry = join(fixtureRoot, "index.ts");
const metadataEntry = join(fixtureRoot, "metadata.ts");
const requestStreamRoot: string = fileURLToPath(
  new URL("../fixtures/generated/request-stream-bundle-isolation/", import.meta.url),
);
const internalEntry = (module: string) => join(fixtureRoot, "internal", `${module}.ts`);

type BundleResult = {
  readonly code: string;
  readonly gzipBytes: number;
  readonly modules: readonly string[];
};

const bundleCases = new Map<string, Promise<BundleResult>>();

function bundle(name: string, source: string): Promise<BundleResult> {
  const existing = bundleCases.get(name);
  if (existing !== undefined) return existing;

  const pending = build({
    configFile: false,
    logLevel: "silent",
    plugins: [
      {
        name: "sdkgen-bundle-entry",
        resolveId(id) {
          if (id === "virtual:sdkgen-bundle-entry") return "\0virtual:sdkgen-bundle-entry.ts";
          if (id === "sdkgen-fixture:index") return publicEntry;
          if (id === "sdkgen-request-stream:index") return join(requestStreamRoot, "index.ts");
          if (id === "sdkgen-fixture:metadata") return metadataEntry;
          if (id.startsWith("sdkgen-fixture:"))
            return internalEntry(id.slice("sdkgen-fixture:".length));
          return null;
        },
        load(id) {
          return id === "\0virtual:sdkgen-bundle-entry.ts" ? source : null;
        },
      },
    ],
    build: {
      target: "es2022",
      minify: "oxc",
      write: false,
      rollupOptions: {
        input: "virtual:sdkgen-bundle-entry",
        preserveEntrySignatures: "strict",
        output: { format: "es", codeSplitting: false },
      },
    },
  }).then(async (result) => {
    if (!Array.isArray(result) && !("output" in result)) {
      await result.close();
      throw new Error("bundle build unexpectedly entered watch mode");
    }
    const outputs = Array.isArray(result) ? result : [result];
    const chunks = outputs
      .flatMap((output) => output.output)
      .filter((item) => item.type === "chunk");
    if (chunks.length !== 1) throw new Error(`bundle emitted ${chunks.length} chunks, want 1`);
    const chunk = chunks[0];
    if (chunk === undefined) throw new Error("bundle emitted no JavaScript chunk");
    return {
      code: chunk.code,
      gzipBytes: gzipSync(chunk.code, { level: 9 }).byteLength,
      // Parsed/re-exported modules may remain in moduleIds with zero emitted
      // bytes. Ownership isolation concerns code that survived tree shaking.
      modules: Object.entries(chunk.modules)
        .filter(([, module]) => module.renderedLength > 0)
        .map(([id]) => id),
    };
  });
  bundleCases.set(name, pending);
  return pending;
}

function rootValue(name: string): string {
  return `export { ${name} } from "sdkgen-fixture:index"`;
}

function directValue(name: string, module: string): string {
  return `export { ${name} } from "sdkgen-fixture:${module}"`;
}

function internalModules(result: BundleResult, root: string = fixtureRoot): string[] {
  return result.modules
    .filter((id) => id.startsWith(root))
    .map((id) => id.slice(root.length))
    .filter((id) => id.startsWith("internal/"))
    .sort();
}

function bundleEvidence(result: BundleResult): string {
  return `${result.modules.join("\n")}\nCODE\n${result.code}`;
}

const results: Record<string, BundleResult> = {};

beforeAll(async () => {
  const cases = {
    rootError: rootValue("isAPIError"),
    requestStreamError: `export { isAPIError } from "sdkgen-request-stream:index"`,
    requestStreamClient: `export { createClient } from "sdkgen-request-stream:index"`,
    directError: directValue("isAPIError", "runtime/client/errors"),
    executionProvider: directValue("provider", "executions/bundle-isolation-sentinel/get"),
    rootClient: rootValue("createClient"),
    directClient: directValue("createClient", "client/index"),
    rootSort: rootValue("SortDirection"),
    directSort: directValue("SortDirection", "runtime/shared/constants"),
    rootEnums: rootValue("Enums"),
    directEnums: directValue("Enums", "enums"),
    rootType: `import type { Client } from "sdkgen-fixture:index"; export type BundledClient = Client`,
    metadata: `export { openapi } from "sdkgen-fixture:metadata"`,
  };
  await Promise.all(
    Object.entries(cases).map(async ([name, source]) => {
      results[name] = await bundle(name, source);
    }),
  );
});

describe("generated public entry bundle isolation", () => {
  it.each([
    ["rootError", "directError"],
    ["rootClient", "directClient"],
    ["rootSort", "directSort"],
    ["rootEnums", "directEnums"],
  ] as const)("keeps %s within the owning-module value graph", (rootName, directName) => {
    expect(internalModules(results[rootName]!)).toEqual(internalModules(results[directName]!));
  });

  it("keeps the runtime error guard independent", () => {
    const result = results.rootError;
    expect(result).toBeDefined();
    expect(internalModules(result!), bundleEvidence(result!)).toEqual([
      "internal/runtime/shared/runtime-support.ts",
    ]);
    expect(result!.code).not.toContain("XML schema");
    expect(result!.code).not.toContain("baseURL");
    expect(result!.code).not.toContain("application/json");
    expect(result!.code).not.toContain("bundle-enum-sentinel-01");
    expect(result!.code).not.toContain("bundle-error-category-sentinel");
    expect(result!.code).not.toContain("bundle-isolation-sentinel");
  });

  it("keeps the client independent from public enum and error-category runtime", () => {
    const result = results.rootClient;
    expect(result).toBeDefined();
    const modules = internalModules(result!);
    for (const required of [
      "internal/client/factory.ts",
      "internal/client/registry.ts",
      "internal/runtime/http/request/http-request-core.ts",
      "internal/runtime/schema/program-execution.ts",
    ])
      expect(modules, bundleEvidence(result!)).toContain(required);
    for (const excluded of [
      "internal/enums.ts",
      "internal/errors.ts",
      "internal/runtime/http/request/http-services.ts",
      "internal/runtime/http/response/http-response-headers.ts",
      "internal/runtime/compatibility/http-codecs.ts",
      "internal/runtime/compatibility/codecs.ts",
      "internal/runtime/media/xml/xml-codec.ts",
      "internal/runtime/stream/stream-sse.ts",
    ])
      expect(modules, bundleEvidence(result!)).not.toContain(excluded);
    expect(result!.code).not.toContain("Symbol.iterator");
    expect(result!.code).not.toContain("bundle-error-category-sentinel");
  });

  it("removes request stream encoders when only the public error guard is used", () => {
    const guard: BundleResult = results.requestStreamError!;
    const client: BundleResult = results.requestStreamClient!;
    expect(internalModules(guard, requestStreamRoot), bundleEvidence(guard)).toEqual([
      "internal/runtime/shared/runtime-support.ts",
    ]);
    const modules: string[] = internalModules(client, requestStreamRoot);
    expect(modules, bundleEvidence(client)).toContain(
      "internal/runtime/http/request/http-request-text-stream.ts",
    );
    expect(modules, bundleEvidence(client)).toContain(
      "internal/runtime/http/request/http-request-json-frame.ts",
    );
  });

  it("emits a JSON execution provider without the full runtime or wire registry", () => {
    const result = results.executionProvider;
    expect(result).toBeDefined();
    const modules = internalModules(result!);
    expect(modules).toContain("internal/executions/bundle-isolation-sentinel/get.ts");
    expect(modules).toContain("internal/runtime/http/request/http-request-core.ts");
    expect(modules).toContain("internal/runtime/schema/program-execution.ts");
    expect(modules).not.toContain("internal/runtime/schema/wire-core.ts");
    for (const excluded of [
      "internal/runtime/compatibility/http.ts",
      "internal/runtime/compatibility/http-codecs.ts",
      "internal/runtime/compatibility/http-advanced.ts",
      "internal/runtime/compatibility/http-stream.ts",
      "internal/runtime/compatibility/streaming.ts",
      "internal/runtime/compatibility/codecs.ts",
      "internal/runtime/compatibility/wire-xml.ts",
      "internal/schemas/wire.ts",
      "internal/client/registry.ts",
      "internal/client/factory.ts",
      "internal/enums.ts",
    ])
      expect(modules, bundleEvidence(result!)).not.toContain(excluded);
    expect(result!.code).not.toContain("multipart response");
    expect(result!.code).not.toContain("XML document");
    expect(result!.gzipBytes).toBeLessThan(results.rootClient!.gzipBytes);
  });

  it("keeps the sort constant independent", () => {
    const result = results.rootSort;
    expect(result).toBeDefined();
    expect(internalModules(result!), bundleEvidence(result!)).toEqual([
      "internal/runtime/shared/constants.ts",
    ]);
    expect(result!.code).not.toContain("bundle-enum-sentinel-01");
    expect(result!.code).not.toContain("bundle-error-category-sentinel");
    expect(result!.code).not.toContain("bundle-isolation-sentinel");
  });

  it("keeps public enum runtime behind its owning concern", () => {
    const result = results.rootEnums;
    expect(result).toBeDefined();
    expect(internalModules(result!), bundleEvidence(result!)).toEqual(["internal/enums.ts"]);
    expect(result!.code).toContain("bundle-enum-sentinel-01");
    expect(result!.code).not.toContain("bundle-error-category-sentinel");
    expect(result!.code).not.toContain("bundle-isolation-sentinel");
  });

  it("emits no runtime for a type-only root import", () => {
    const result = results.rootType;
    expect(result).toBeDefined();
    expect(internalModules(result!)).toEqual([]);
    expect(result!.code.trim()).toBe("");
  });

  it("keeps metadata at its explicit public subpath", () => {
    const result = results.metadata;
    expect(result).toBeDefined();
    expect(internalModules(result!)).toEqual([]);
    expect(result!.modules.filter((id) => id.startsWith(fixtureRoot))).toEqual([metadataEntry]);
    expect(result!.code).toContain("Bundle Isolation API");
  });
});
