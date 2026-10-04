import { runInNewContext } from "node:vm";
import { describe, expect, it } from "vitest";
import { collectSelectionReferences as template } from "../../../internal/target/typescript/runtime/client/selection.js";
import { collectSelectionReferences as emitted } from "../fixtures/generated/lifecycle/internal/runtime/client/selection.js";

for (const [label, collect] of [
  ["template", template],
  ["emitted", emitted],
] as const) {
  describe(`${label} selection snapshot`, () => {
    const a: Readonly<{ route: string }> = Object.freeze({ route: "GET /a" });
    const b: Readonly<{ route: string }> = Object.freeze({ route: "POST /b" });
    const identities = new WeakMap<object, object>([
      [a, a],
      [b, b],
    ]);
    const read = (value: object) => identities.get(value);
    const select = (value: unknown) => collect(value, read);

    it("composes arrays, default groups and nested objects in value order", () => {
      expect(select([{ default: [a], nested: { write: b } }, [a, b]])).toEqual([a, b]);
      expect(select([])).toEqual([]);
      expect(select({})).toEqual([]);
    });

    it("reads a changing getter once even when the same feature is repeated", () => {
      let reads = 0;
      const feature = {
        get chosen() {
          return reads++ === 0 ? a : 12;
        },
      };
      expect(select([feature, feature])).toEqual([a]);
      expect(reads).toBe(1);
    });

    it("allows user effects and preserves the original thrown value", () => {
      let effects = 0;
      expect(
        select({
          get value() {
            effects++;
            return a;
          },
        }),
      ).toEqual([a]);
      const error = { userError: true };
      expect(() =>
        select({
          get value() {
            throw error;
          },
        }),
      ).toThrow();
      try {
        select({
          get value() {
            throw error;
          },
        });
      } catch (actual) {
        expect(actual).toBe(error);
      }
      expect(effects).toBe(1);
    });

    it("takes a fresh snapshot on a later preparation", () => {
      let current = a;
      const feature = {
        get value() {
          return current;
        },
      };
      expect(select(feature)).toEqual([a]);
      current = b;
      expect(select(feature)).toEqual([b]);
    });

    it("fixes array length and reads index accessors once", () => {
      let reads = 0;
      const values = [a];
      Object.defineProperty(values, 0, {
        get() {
          reads++;
          values.push(b);
          return a;
        },
      });
      expect(select([values, values])).toEqual([a]);
      expect(reads).toBe(1);
      expect(values.length).toBe(2);
    });

    it("uses fixed own keys while preserving ordinary getter-induced mutations", () => {
      const group = {
        get first() {
          group.second = b;
          return a;
        },
        second: a,
      };
      expect(select(group)).toEqual([a, b]);
      const feature: Record<string, unknown> = {};
      Object.defineProperty(feature, "hidden", {
        get() {
          feature.later = b;
          return a;
        },
      });
      expect(select(feature)).toEqual([a]);
    });

    it("accepts own selections without prototype or realm screening", () => {
      class Feature {
        readonly value = a;
      }
      const inherited = Object.assign(Object.create({ outside: b }), { own: a });
      const foreign: object = runInNewContext("({})");
      Object.defineProperty(foreign, "value", { value: a });
      expect(select([new Feature(), inherited, foreign])).toEqual([a]);
    });

    it("accepts Proxy getters without asking for their prototype", () => {
      const reads: PropertyKey[] = [];
      const feature = new Proxy(
        { chosen: a },
        {
          get(target, key, receiver) {
            reads.push(key);
            return Reflect.get(target, key, receiver);
          },
          getPrototypeOf() {
            throw new Error("prototype is irrelevant");
          },
        },
      );
      expect(select(feature)).toEqual([a]);
      expect(reads).toEqual(["chosen", "then"]);
    });

    it("does not collect inherited or symbol members", () => {
      const group = Object.assign(Object.create({ outside: b }), {
        own: a,
        [Symbol("outside")]: b,
      });
      expect(select(group)).toEqual([a]);
    });

    it("keeps a selection-valued then property and reads its getter once", () => {
      let reads = 0;
      const feature = {
        get then() {
          reads++;
          return [a, b];
        },
      };
      expect(select(feature)).toEqual([a, b]);
      expect(reads).toBe(1);
    });

    it("rejects asynchronous selections without invoking then", () => {
      let called = 0;
      let reads = 0;
      const asyncValue = {
        get then() {
          reads++;
          return () => {
            called++;
          };
        },
      };
      expect(() => select(asyncValue)).toThrow(/Await/);
      expect(reads).toBe(1);
      expect(called).toBe(0);
      expect(() => select(Promise.resolve(a))).toThrow(/Await/);
      expect(() => select(runInNewContext("Promise.resolve(1)"))).toThrow(/Await/);
    });

    it("rejects invalid leaves, functions and array holes without partial results", () => {
      let called = 0;
      for (const value of [
        undefined,
        null,
        1,
        "name",
        true,
        Symbol("x"),
        () => {
          called++;
          return a;
        },
      ]) {
        expect(() => select([a, value])).toThrow(TypeError);
      }
      expect(() => select({ present: undefined })).toThrow(TypeError);
      expect(() => select(new Array(1))).toThrow(TypeError);
      expect(called).toBe(0);
    });

    it("rejects invalid array lengths supplied by a Proxy", () => {
      const value = new Proxy([a], {
        get(target, key, receiver) {
          return key === "length" ? Infinity : Reflect.get(target, key, receiver);
        },
      });
      expect(() => select(value)).toThrow(/length/);
    });

    it("rejects active cycles but allows shared subgraphs", () => {
      const array: unknown[] = [];
      array.push(array);
      const object: { self?: object } = {};
      object.self = object;
      expect(() => select(array)).toThrow(/Cyclic/);
      expect(() => select(object)).toThrow(/Cyclic/);
      const shared = { values: [a, b] };
      expect(select([shared, shared])).toEqual([a, b]);
    });

    it("processes a shared DAG once per unique container", () => {
      let graph: unknown = a;
      for (let index = 0; index < 60; index++) graph = [graph, graph];
      let visits = 0;
      expect(
        collect(graph, (value) => {
          visits++;
          return read(value);
        }),
      ).toEqual([a]);
      expect(visits).toBe(61);
    });

    it("traverses deeply nested finite inputs without recursive call-stack growth", () => {
      let graph: unknown = b;
      for (let index = 0; index < 10_000; index++) graph = { child: graph };
      expect(select(graph)).toEqual([b]);
    });

    it("reuses decoder canonical identity and preserves decoder failures", () => {
      const alias = Object.freeze({ alias: true });
      expect(collect([a, alias], (value) => (value === alias ? a : read(value)))).toEqual([a]);
      const error = new Error("incompatible reference");
      expect(() =>
        collect([a], () => {
          throw error;
        }),
      ).toThrow(error);
    });

    it("does not consume an iterable to infer operation selection", () => {
      let iterations = 0;
      const feature = {
        own: a,
        *[Symbol.iterator]() {
          iterations++;
          yield b;
        },
      };
      expect(select(feature)).toEqual([a]);
      expect(iterations).toBe(0);
    });
  });
}
