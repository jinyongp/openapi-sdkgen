import assert from "node:assert/strict";

/** Baseline construction contract, not a second production implementation. */
export function explicitProperties(entries) {
  return Object.fromEntries(entries.map(([property, schema]) => [property, { property, schema }]));
}

/** Exercise the actual emitted helper; keep these vectors reusable across candidates. */
export function assertWirePropertiesContract(wireProperties, codecs) {
  const checks = [];
  const check = (name, run) => {
    run();
    checks.push(name);
  };
  const shared = Object.freeze({ types: Object.freeze(["string"]) });
  const keys = [
    "__proto__",
    "constructor",
    "prototype",
    "toString",
    "",
    "1",
    "01",
    "é",
    "e\u0301",
    "a\0b",
    "line\nbreak",
    'quote"key',
    "schema",
    "property",
  ];
  check("exact own keys, descriptors, order and ordinary prototype", () => {
    const entries = keys.map((key) => [key, shared]);
    const result = wireProperties(entries);
    assert.equal(Object.getPrototypeOf(result), Object.prototype);
    assert.deepEqual(Reflect.ownKeys(result), Reflect.ownKeys(explicitProperties(entries)));
    assert.deepEqual(
      Object.getOwnPropertyDescriptors(result),
      Object.getOwnPropertyDescriptors(explicitProperties(entries)),
    );
  });
  check("empty maps and duplicate last-write preserve insertion order", () => {
    assert.deepEqual(wireProperties([]), {});
    const last = { boolean: false };
    const entries = [
      ["b", shared],
      ["a", shared],
      ["1", shared],
      ["b", last],
    ];
    assert.deepEqual(wireProperties(entries), explicitProperties(entries));
    assert.deepEqual(Object.keys(wireProperties(entries)), ["1", "b", "a"]);
    assert.equal(wireProperties(entries).b.schema, last);
  });
  check("child identities shared, wrappers fresh within and across calls", () => {
    const entries = [
      ["a", shared],
      ["b", shared],
    ];
    const a = wireProperties(entries),
      b = wireProperties(entries);
    assert.equal(a.a.schema, shared);
    assert.equal(a.a.schema, a.b.schema);
    assert.notEqual(a.a, a.b);
    assert.notEqual(a, b);
    assert.notEqual(a.a, b.a);
  });
  check("frozen arrays and child schemas remain unchanged", () => {
    const entries = Object.freeze([Object.freeze(["x", shared])]);
    assert.deepEqual(wireProperties(entries), explicitProperties(entries));
    assert.equal(wireProperties(entries).x.schema, shared);
  });
  check("opaque values are passed by identity and are not compacted", () => {
    const data = JSON.parse(
      '{"__proto__":null,"property":"schema","schema":"__sdkgen_Input","p":false,"s":0}',
    );
    const schema = Object.freeze({ constValue: data, enumValues: [data] });
    const value = wireProperties([["x", schema]]).x.schema;
    assert.equal(value.constValue, data);
    assert.equal(value.enumValues[0], data);
    assert.equal(Object.hasOwn(value.constValue, "__proto__"), true);
  });
  check("absent, undefined, false, null, zero and empty constraints survive", () => {
    const schema = Object.freeze({
      boolean: false,
      minimum: 0,
      constValue: null,
      enumValues: [],
      additionalProperties: false,
      nullable: undefined,
    });
    const before = Object.getOwnPropertyDescriptors(schema);
    assert.equal(wireProperties([["x", schema]]).x.schema, schema);
    assert.deepEqual(Object.getOwnPropertyDescriptors(schema), before);
    assert.equal(Object.hasOwn(schema, "nullable"), true);
    assert.equal(Object.hasOwn(schema, "maximum"), false);
  });
  check("an inherited setter is never invoked", () => {
    const key = "__sdkgenVerificationSetter";
    assert.equal(Object.hasOwn(Object.prototype, key), false);
    Object.defineProperty(Object.prototype, key, {
      configurable: true,
      set() {
        throw new Error("inherited setter called");
      },
    });
    try {
      const value = wireProperties([[key, shared]]);
      assert.equal(Object.hasOwn(value, key), true);
      assert.equal(value[key].schema, shared);
    } finally {
      delete Object.prototype[key];
    }
  });
  check("deterministic adversarial map vectors preserve exact identity", () => {
    let state = 0x51d3a7;
    const random = () => {
      state = (Math.imul(state, 1664525) + 1013904223) >>> 0;
      return state;
    };
    for (let trial = 0; trial < 128; trial++) {
      const entries = Array.from({ length: 1 + (random() % 50) }, () => [
        keys[random() % keys.length],
        { minimum: random() % 3 },
      ]);
      assert.deepEqual(
        Object.getOwnPropertyDescriptors(wireProperties(entries)),
        Object.getOwnPropertyDescriptors(explicitProperties(entries)),
      );
    }
  });
  if (codecs) {
    const { encodeWireValue, decodeWireValue, validateWireValue } = codecs;
    check("required and closed-object validation matches the original runtime", () => {
      const schema = {
        types: ["object"],
        properties: wireProperties([
          ["x", { types: ["number"], minimum: 0 }],
          ["forbidden", { boolean: false }],
        ]),
        required: ["x"],
        additionalProperties: false,
      };
      validateWireValue({ x: 0 }, schema, {}, "encode");
      for (const invalid of [{}, { x: -1 }, { x: 0, forbidden: null }, { x: 0, extra: 1 }])
        assert.throws(() => validateWireValue(invalid, schema, {}, "encode"));
    });
    check("manual nonidentity WireProperty mappings still encode and decode", () => {
      const schema = {
        types: ["object"],
        properties: { wire_name: { property: "sdkName", schema: shared } },
        required: ["wire_name"],
      };
      assert.deepEqual({ ...encodeWireValue({ sdkName: "ok" }, schema, {}) }, { wire_name: "ok" });
      assert.deepEqual({ ...decodeWireValue({ wire_name: "ok" }, schema, {}) }, { sdkName: "ok" });
    });
    check("recursive references retain baseline graph semantics", () => {
      const entries = [
        ["value", shared],
        ["next", { reference: "Node" }],
      ];
      const a = { types: ["object"], properties: explicitProperties(entries) };
      const b = { types: ["object"], properties: wireProperties(entries) };
      const data = { value: "a", next: { value: "b" } };
      assert.deepEqual(
        encodeWireValue(data, b, { Node: b }),
        encodeWireValue(data, a, { Node: a }),
      );
    });
  }
  return { status: "pass", checks, adversarialVectors: 128 };
}
