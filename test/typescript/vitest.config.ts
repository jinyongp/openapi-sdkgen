import { resolve } from "node:path";
import { defineConfig } from "vitest/config";

const runtimeRoot = resolve(
  import.meta.dirname,
  "../../internal/target/typescript/runtime/internal",
);

export default defineConfig({
  test: {
    include: ["tests/**/*.test.ts"],
    coverage: {
      // Measure the handwritten client runtime once. Per-operation generated
      // files scale with the OpenAPI input and are covered by conformance tests.
      provider: "v8",
      allowExternal: true,
      include: [
        `${runtimeRoot}/**/*.ts`,
        "fixtures/generated/client/internal/client/factory.ts",
        "fixtures/generated/client/internal/client/registry.ts",
        "fixtures/generated/client/selective/index.ts",
        "fixtures/generated/client/selective/all.ts",
      ],
      exclude: [
        `${runtimeRoot}/configuration.ts`,
        `${runtimeRoot}/contract-types.ts`,
        `${runtimeRoot}/errors.ts`,
        `${runtimeRoot}/http-json-stream.ts`,
        `${runtimeRoot}/http-types.ts`,
        `${runtimeRoot}/identity.ts`,
        `${runtimeRoot}/media-type.ts`,
        `${runtimeRoot}/objects.ts`,
        `${runtimeRoot}/operation.ts`,
        `${runtimeRoot}/request.ts`,
        `${runtimeRoot}/security.ts`,
        `${runtimeRoot}/selection-types.ts`,
        `${runtimeRoot}/transport.ts`,
        `${runtimeRoot}/wire-xml.ts`,
      ],
      reporter: ["text", "json-summary", "lcov"],
      reportsDirectory: "../../.tmp/coverage/typescript",
      thresholds: {
        statements: 80,
        branches: 70,
        functions: 80,
        lines: 80,
      },
    },
  },
});
