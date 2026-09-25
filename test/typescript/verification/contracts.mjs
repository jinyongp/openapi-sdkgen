import assert from "node:assert/strict";
import { createHash } from "node:crypto";

/** Capture own descriptors and graph sharing without invoking getters or function bodies. */
export function graphFingerprint(root) {
  const objects = new Map();
  const symbols = new Map();
  const nodes = [];
  function symbolKey(value) {
    if (!symbols.has(value)) symbols.set(value, symbols.size);
    return ["symbol", symbols.get(value), Symbol.keyFor(value) ?? null, value.description ?? null];
  }
  function visit(value) {
    if (typeof value === "symbol") return symbolKey(value);
    if (value === null || (typeof value !== "object" && typeof value !== "function")) {
      if (typeof value === "number" && !Number.isFinite(value)) return ["number", String(value)];
      if (Object.is(value, -0)) return ["number", "-0"];
      return [typeof value, typeof value === "bigint" ? String(value) : value];
    }
    if (objects.has(value)) return ["ref", objects.get(value)];
    assert.ok(nodes.length < 200000, "contract graph unexpectedly large");
    const id = nodes.length;
    objects.set(value, id);
    const proto = Object.getPrototypeOf(value);
    const constructor = proto && Object.getOwnPropertyDescriptor(proto, "constructor")?.value;
    const constructorName =
      typeof constructor === "function"
        ? Object.getOwnPropertyDescriptor(constructor, "name")?.value
        : undefined;
    const prototype =
      proto === null
        ? "null"
        : proto === Object.prototype
          ? "Object"
          : proto === Function.prototype
            ? "Function"
            : proto === Array.prototype
              ? "Array"
              : typeof constructorName === "string"
                ? constructorName
                : "other";
    const node = {
      id,
      kind: typeof value,
      prototype,
      extensible: Object.isExtensible(value),
      properties: [],
    };
    nodes.push(node);
    for (const key of Reflect.ownKeys(value)) {
      const d = Object.getOwnPropertyDescriptor(value, key);
      const property = {
        key: typeof key === "symbol" ? symbolKey(key) : key,
        enumerable: d.enumerable,
        configurable: d.configurable,
      };
      if ("value" in d) {
        property.writable = d.writable;
        property.value = visit(d.value);
      } else {
        property.get = visit(d.get);
        property.set = visit(d.set);
      }
      node.properties.push(property);
    }
    return ["ref", id];
  }
  const rootValue = visit(root);
  return {
    version: 2,
    digest: createHash("sha256")
      .update(JSON.stringify({ root: rootValue, nodes }))
      .digest("hex"),
    nodes: nodes.length,
  };
}

export function assertSameContract(baseline, candidate) {
  assert.deepEqual(candidate, baseline, "public contract, descriptor, or alias graph changed");
}

export function assertSameSdkContract(baseline, candidate) {
  const normalized = structuredClone(candidate);
  const additions = [];
  const helper = "internal/runtime/wire-properties.js";
  const facade = "internal/runtime/callables.js";
  if (!Object.hasOwn(baseline.exports, helper) && Object.hasOwn(candidate.exports, helper)) {
    assert.deepEqual(
      candidate.exports[helper],
      ["wireProperties"],
      "unexpected new helper exports",
    );
    delete normalized.exports[helper];
    additions.push({ module: helper, export: "wireProperties" });
  }
  if (
    !baseline.exports[facade]?.includes("createWireProperties") &&
    candidate.exports[facade]?.includes("createWireProperties")
  ) {
    normalized.exports[facade] = candidate.exports[facade].filter(
      (name) => name !== "createWireProperties",
    );
    additions.push({ module: facade, export: "createWireProperties" });
  }
  assertSameContract(baseline, normalized);
  return additions;
}

export function median(values) {
  assert.ok(values.length > 0 && values.every(Number.isFinite));
  const sorted = [...values].sort((a, b) => a - b);
  const middle = Math.floor(sorted.length / 2);
  return sorted.length % 2 ? sorted[middle] : (sorted[middle - 1] + sorted[middle]) / 2;
}

export function pairedSummary(pairs, field) {
  const left = pairs.map((p) => p.baseline[field]);
  const right = pairs.map((p) => p.candidate[field]);
  const deltas = right.map((n, i) => n - left[i]);
  return {
    baseline: median(left),
    candidate: median(right),
    pairedDelta: median(deltas),
    pairedPercent: median(right.map((n, i) => (left[i] === 0 ? 0 : (n / left[i] - 1) * 100))),
    minDelta: Math.min(...deltas),
    maxDelta: Math.max(...deltas),
    pairs: pairs.length,
  };
}
