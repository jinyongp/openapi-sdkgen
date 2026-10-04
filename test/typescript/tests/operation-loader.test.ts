import { createHash } from "node:crypto";
import { describe, expect, it } from "vitest";
import {
  createOperationLoader,
  operationLookupFilename,
  staticOperationReference,
} from "../../../internal/target/typescript/runtime/client/operation-loader.js";
import type {
  OperationExecutionProvider,
  OperationLookupEntry,
} from "../../../internal/target/typescript/runtime/client/operation-loader.js";
import {
  createOperationLoader as emittedLoader,
  operationLookupFilename as emittedFilename,
  staticOperationReference as emittedStatic,
} from "../fixtures/generated/lifecycle/internal/runtime/client/operation-loader.js";

const generation = "fixture-generation";
const provider = (route: string, operationID?: string): OperationExecutionProvider => ({
  abi: 1,
  generation,
  route,
  ...(operationID === undefined ? {} : { operationID }),
  bind: () => () => Promise.resolve(route),
});
const a = provider("GET /a", "readA");
const b = provider("POST /b", "writeB");

for (const [label, create, filename, staticRef] of [
  ["template", createOperationLoader, operationLookupFilename, staticOperationReference],
  // This runtime matrix bridges module-local nominal types; d.ts consumers check them separately.
  [
    "emitted",
    emittedLoader as unknown as typeof createOperationLoader,
    emittedFilename,
    emittedStatic as unknown as typeof staticOperationReference,
  ],
] as const) {
  function setup(
    extra: { importModule?: (url: string) => Promise<unknown>; generation?: string } = {},
  ) {
    const requests: string[] = [];
    const entries = new Map<string, OperationLookupEntry>();
    for (const value of [a, b]) {
      for (const [kind, key] of [
        ["route", value.route],
        ["operation", value.operationID],
      ] as const) {
        if (key === undefined) continue;
        const hash = createHash("sha256").update(`${kind}\0${key}`).digest("hex");
        entries.set(`${kind === "route" ? "r" : "o"}-${hash.slice(0, 16)}/${hash.slice(16)}.js`, {
          abi: 1,
          generation,
          kind,
          key,
          provider: value,
        });
      }
    }
    let clientLoads = 0;
    const loader = create({
      generation: extra.generation ?? generation,
      baseURL: new URL("https://assets.test/release/api/selective/"),
      importModule: async (url) => {
        requests.push(url);
        if (extra.importModule) return extra.importModule(url);
        const entry = entries.get(new URL(url).pathname.split("/lookup/")[1]!);
        if (!entry) throw new Error("not found");
        return { entry };
      },
      loadClient: async () => {
        clientLoads++;
        return { createSelectedClient: (options, providers) => ({ options, providers }) };
      },
    });
    return { loader, requests, entries, clientLoads: () => clientLoads };
  }
  describe(`${label} operation code loader`, () => {
    it("does not import execution code merely by constructing references", () => {
      const test = setup();
      expect(test.loader.operations.readA).toBe(test.loader.operations.readA);
      expect(test.loader.routes["GET /a"]).toBeDefined();
      expect(test.requests).toEqual([]);
      expect(test.clientLoads()).toBe(0);
    });
    it("loads both identities but registers one canonical operation", async () => {
      const test = setup();
      const prepared = await test.loader.loadOperations([
        test.loader.operations.readA!,
        test.loader.routes["GET /a"]!,
      ]);
      const client = test.loader.createClient({ operations: prepared }) as { providers: object[] };
      expect(client.providers).toEqual([a]);
      expect(test.requests).toHaveLength(2);
      expect(
        test.requests.every((url) =>
          url.startsWith("https://assets.test/release/api/selective/lookup/"),
        ),
      ).toBe(true);
      expect(Object.getPrototypeOf(prepared)).toBe(null);
      expect(Reflect.get(prepared, "then")).toBeUndefined();
      expect(await Promise.resolve(prepared)).toBe(prepared);
    });
    it("reuses code across simultaneous preparation and isolates client options", async () => {
      const test = setup();
      const selection = [test.loader.operations.readA!] as const;
      const [one, two] = await Promise.all([
        test.loader.loadOperations(selection),
        test.loader.loadOperations(selection),
      ]);
      const first = test.loader.createClient({ operations: one, authorization: "one" }) as {
        options: { authorization: string };
      };
      const second = test.loader.createClient({ operations: two, authorization: "two" }) as {
        options: { authorization: string };
      };
      expect(test.requests).toHaveLength(1);
      expect(test.clientLoads()).toBe(1);
      expect(first).not.toBe(second);
      expect(first.options.authorization).toBe("one");
      expect(second.options.authorization).toBe("two");
    });
    it("collects nested getter selections once before beginning any module request", async () => {
      const test = setup();
      let reads = 0;
      const feature = {
        get list() {
          reads++;
          expect(test.requests).toHaveLength(0);
          return [test.loader.operations.readA!];
        },
      };
      await test.loader.loadOperations([feature, feature]);
      expect(reads).toBe(1);
      expect(test.requests).toHaveLength(1);
    });
    it("preserves getter exceptions and rejects invalid leaves before imports", async () => {
      const test = setup();
      const cause = new Error("application getter");
      await expect(
        test.loader.loadOperations({
          get a(): never {
            throw cause;
          },
        }),
      ).rejects.toBe(cause);
      await expect(
        // @ts-expect-error Invalid selection is rejected at runtime too.
        test.loader.loadOperations([test.loader.operations.readA!, 4]),
      ).rejects.toBeInstanceOf(TypeError);
      expect(test.requests).toEqual([]);
    });
    it("rejects foreign-generation references before module work", async () => {
      const test = setup();
      const foreign = setup({ generation: "other-generation" });
      await expect(
        test.loader.loadOperations(foreign.loader.operations.readA!),
      ).rejects.toMatchObject({ stage: "IDENTITY" });
      expect(test.requests).toEqual([]);
    });
    it("keeps empty selection valid without importing operations", async () => {
      const test = setup();
      const prepared = await test.loader.loadOperations([]);
      expect(
        (test.loader.createClient({ operations: prepared }) as { providers: object[] }).providers,
      ).toEqual([]);
      expect(test.requests).toEqual([]);
      expect(test.clientLoads()).toBe(1);
    });
    it("preserves module failure causes without permanently caching rejected work", async () => {
      const cause = new Error("module unavailable");
      const test = setup({
        importModule: async () => {
          throw cause;
        },
      });
      for (let attempt = 0; attempt < 2; attempt++) {
        await expect(
          test.loader.loadOperations(test.loader.operations.readA!),
        ).rejects.toMatchObject({ stage: "MODULE_LOAD", cause });
      }
      expect(test.requests).toHaveLength(2);
      expect(test.clientLoads()).toBe(0);
    });
    it.each(["generation", "kind", "key", "abi", "route", "provider-generation", "bind"])(
      "rejects a mismatched %s before binding",
      async (field) => {
        const test = setup({
          importModule: async () => {
            const entry: Record<string, unknown> = {
              abi: 1,
              generation,
              kind: "operation",
              key: "readA",
              provider: a,
            };
            if (field === "route") entry.provider = { ...a, operationID: "wrong" };
            else if (field === "provider-generation")
              entry.provider = { ...a, generation: "wrong" };
            else if (field === "bind") entry.provider = { ...a, bind: 1 };
            else entry[field] = "wrong";
            return { entry };
          },
        });
        await expect(
          test.loader.loadOperations(test.loader.operations.readA!),
        ).rejects.toMatchObject({ stage: "IDENTITY" });
        expect(test.clientLoads()).toBe(0);
      },
    );
    it("uses direct static providers without lookup downloads", async () => {
      const test = setup();
      const prepared = await test.loader.loadOperations([staticRef(a), staticRef(a), staticRef(b)]);
      expect(
        (test.loader.createClient({ operations: prepared }) as { providers: object[] }).providers,
      ).toEqual([a, b]);
      expect(test.requests).toEqual([]);
    });
    it("does not let a prepared handle from another loader bind silently", async () => {
      const test = setup();
      const other = setup();
      const prepared = await other.loader.loadOperations([]);
      expect(() => test.loader.createClient({ operations: prepared })).toThrow(
        /this generated SDK/,
      );
    });
    it("allows then and prototype-like operation IDs without creating thenable references", async () => {
      const test = setup();
      for (const key of ["then", "__proto__", "constructor", "toString"]) {
        const value = test.loader.operations[key]!;
        expect(await Promise.resolve(value)).toBe(value);
        expect(Object.getPrototypeOf(value)).toBe(null);
      }
      expect(() => Object.keys(test.loader.operations)).toThrow(/all.js/);
      expect(() => "readA" in test.loader.operations).toThrow(/all.js/);
      expect(() => Reflect.set(test.loader.operations, "readA", null)).toThrow(/readonly/);
    });
    it("uses exact UTF-8 key framing and refuses isolated surrogate aliases", async () => {
      for (const key of [
        "GET /안녕/{id}",
        "__proto__",
        "a\0b",
        "../outside",
        "e\u0301",
        "é",
        "😀",
      ]) {
        const expected = createHash("sha256").update(`route\0${key}`).digest("hex");
        expect(await filename("route", key)).toBe(
          `r-${expected.slice(0, 16)}/${expected.slice(16)}.js`,
        );
      }
      for (const key of ["\ud800", "\udfff", "bad\ud800x"]) {
        await expect(filename("route", key)).rejects.toMatchObject({ stage: "INPUT" });
      }
      expect(await filename("route", "é")).not.toBe(await filename("route", "e\u0301"));
    });
  });
}
