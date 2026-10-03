import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync, mkdtempSync, mkdirSync, writeFileSync, rmSync, symlinkSync } from "node:fs";
import { resolve } from "node:path";
import {
  loadCatalog,
  validateCatalog,
  fixtureRoot,
  repositoryRoot,
  containedPath,
  inspectGenerated,
  sha256,
} from "./catalog.mjs";
import {
  graphFingerprint,
  assertSameContract,
  assertSameSdkContract,
  median,
  pairedSummary,
} from "./contracts.mjs";

function surface() {
  const one = () => 1;
  const two = () => 2;
  const leaf = (response) => response;
  return {
    $routes: { "GET /one": one, "GET /two": two },
    $operations: { one, two },
    $links: { next: { byStatus: { status200: leaf } } },
  };
}

test("catalog rejects duplicate identities, unsafe output paths", () => {
  for (const mutate of [
    (c) => c.local.push(c.local[0]),
    (c) => {
      c.local[1].output = c.local[0].output;
    },
    (c) => {
      c.local[1].output = `./${c.local[0].output}`;
    },
    (c) => {
      c.local[1].output = `${c.local[0].output}/child`;
    },
    (c) => {
      c.local[0].output = "../outside";
    },
    (c) => {
      c.local[0].input = "/absolute.json";
    },
  ]) {
    const copy = structuredClone(loadCatalog());
    mutate(copy);
    assert.throws(() => validateCatalog(copy));
  }
  assert.throws(() => containedPath(fixtureRoot, "../outside"), { code: "ERR_ASSERTION" });
  assert.throws(() => containedPath(fixtureRoot, "."), { code: "ERR_ASSERTION" });
});

test("A/A contract fingerprints match without executing getters", () => {
  assertSameContract(graphFingerprint(surface()), graphFingerprint(surface()));
  const getter = () => {
    throw new Error("must not execute");
  };
  const value = Object.defineProperty({}, "field", { get: getter });
  assert.ok(graphFingerprint(value).nodes > 0);
});

for (const [name, mutate] of [
  [
    "removed exact key",
    (x) => {
      delete x.$operations.one;
    },
  ],
  ["prototype changed", (x) => Object.setPrototypeOf(x.$routes, null)],
  [
    "operation references wrong route",
    (x) => {
      x.$operations.one = x.$routes["GET /two"];
    },
  ],
  [
    "callable name changed",
    (x) => Object.defineProperty(x.$routes["GET /one"], "name", { value: "wrong" }),
  ],
  [
    "Link byStatus name changed",
    (x) => Object.defineProperty(x.$links.next.byStatus.status200, "name", { value: "wrong" }),
  ],
  [
    "property descriptor changed",
    (x) => Object.defineProperty(x.$operations, "one", { enumerable: false }),
  ],
]) {
  test(`negative control detects ${name}`, () => {
    const a = surface();
    const b = surface();
    mutate(b);
    assert.throws(() => assertSameContract(graphFingerprint(a), graphFingerprint(b)));
  });
}

test("own __proto__ keys, explicit undefined and alias identity are visible", () => {
  const a = Object.fromEntries([["__proto__", undefined]]);
  assert.notEqual(graphFingerprint(a).digest, graphFingerprint({}).digest);
  assert.notEqual(graphFingerprint({ a: undefined }).digest, graphFingerprint({ a: null }).digest);
  const child = {};
  assert.notEqual(
    graphFingerprint({ a: child, b: child }).digest,
    graphFingerprint({ a: {}, b: {} }).digest,
  );
});

test("manifest audit rejects missing and edited artifacts", () => {
  const parent = resolve(repositoryRoot, ".tmp/verification-self-tests");
  mkdirSync(parent, { recursive: true });
  const dir = mkdtempSync(resolve(parent, "case-"));
  try {
    writeFileSync(resolve(dir, "index.ts"), "export {};\n");
    writeFileSync(
      resolve(dir, ".openapi-sdkgen-manifest.json"),
      JSON.stringify({
        version: 1,
        files: { "index.ts": sha256(readFileSync(resolve(dir, "index.ts"))) },
      }),
    );
    assert.equal(inspectGenerated(dir).fileCount, 1);
    writeFileSync(resolve(dir, "index.ts"), "modified\n");
    assert.throws(() => inspectGenerated(dir), /hash mismatch/);
    rmSync(resolve(dir, "index.ts"));
    assert.throws(() => inspectGenerated(dir));
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("fixture containment rejects live, dangling and root symlinks", () => {
  const parent = resolve(repositoryRoot, ".tmp/verification-self-tests");
  mkdirSync(parent, { recursive: true });
  const dir = mkdtempSync(resolve(parent, "paths-"));
  try {
    const real = resolve(dir, "real");
    mkdirSync(real);
    symlinkSync(real, resolve(dir, "live"), "dir");
    symlinkSync(resolve(dir, "missing"), resolve(dir, "dangling"), "dir");
    assert.throws(() => containedPath(dir, "live/output.ts"), /symlink/);
    assert.throws(() => containedPath(dir, "dangling/output.ts"), /symlink/);
    assert.throws(() => containedPath(resolve(dir, "live"), "output.ts"), /symlink/);
    assert.equal(containedPath(real, "new/output.ts"), resolve(real, "new/output.ts"));
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("fingerprinting does not execute a prototype constructor name getter", () => {
  const constructor = function Example() {};
  Object.defineProperty(constructor, "name", {
    get() {
      throw new Error("name getter must not run");
    },
  });
  const value = Object.create({ constructor });
  assert.ok(graphFingerprint(value).digest);
});

test("manifest audit accepts supported v2 and rejects invalid version identity pairs", () => {
  const parent = resolve(repositoryRoot, ".tmp/verification-self-tests");
  mkdirSync(parent, { recursive: true });
  const dir = mkdtempSync(resolve(parent, "manifest-"));
  try {
    const file = resolve(dir, ".openapi-sdkgen-manifest.json");
    writeFileSync(file, JSON.stringify({ version: 2, generation: { probe: true }, files: {} }));
    assert.equal(inspectGenerated(dir).fileCount, 0);
    for (const manifest of [
      { version: 3, files: {} },
      { version: 2, files: {} },
      { version: 1, generation: {}, files: {} },
    ]) {
      writeFileSync(file, JSON.stringify(manifest));
      assert.throws(() => inspectGenerated(dir));
    }
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("paired summary separates direct observations, medians and noise", () => {
  assert.equal(median([4, 1, 3, 2]), 2.5);
  const values = [
    { baseline: { wall: 1 }, candidate: { wall: 1 } },
    { baseline: { wall: 10 }, candidate: { wall: 10 } },
  ];
  assert.equal(pairedSummary(values, "wall").pairedPercent, 0);
  assert.throws(() => median([NaN]));
});

for (const [left, right] of [
  ["expected", "wrong"],
  ["ok", undefined],
  [0, -0],
  [null, undefined],
  [false, true],
  [NaN, Infinity],
]) {
  test(`negative control detects primitive root mismatch: ${String(left)} / ${String(right)}`, () => {
    assert.throws(() => assertSameContract(graphFingerprint(left), graphFingerprint(right)));
    assertSameContract(graphFingerprint(left), graphFingerprint(left));
  });
}

test("shared helper-contract vectors accept the baseline and reject wrong property mappings", async () => {
  const { explicitProperties, assertWirePropertiesContract } =
    await import("./helper-contracts.mjs");
  assert.equal(assertWirePropertiesContract(explicitProperties).status, "pass");
  assert.throws(() =>
    assertWirePropertiesContract((entries) =>
      Object.fromEntries(entries.map(([key, schema]) => [key, { property: "wrong", schema }])),
    ),
  );
});

function moduleContract() {
  return {
    exports: { "internal/runtime/callables.js": ["bindOperation"] },
    rootExports: ["createClient"],
  };
}
test("only the precise additive internal helper and facade exports are allowed", () => {
  const a = moduleContract(),
    b = structuredClone(a);
  b.exports["internal/runtime/wire-properties.js"] = ["wireProperties"];
  b.exports["internal/runtime/callables.js"].push("createWireProperties");
  assert.equal(assertSameSdkContract(a, b).length, 2);
});
test("removing an existing helper export is not hidden by the addition allowance", () => {
  const a = moduleContract();
  a.exports["internal/runtime/wire-properties.js"] = ["wireProperties"];
  a.exports["internal/runtime/callables.js"].push("createWireProperties");
  const b = structuredClone(a);
  b.exports["internal/runtime/callables.js"] = ["bindOperation"];
  assert.throws(() => assertSameSdkContract(a, b));
});
test("unrelated export changes remain failures", () => {
  const a = moduleContract(),
    b = structuredClone(a);
  b.exports["internal/runtime/wire-properties.js"] = ["wireProperties", "hiddenExtra"];
  assert.throws(() => assertSameSdkContract(a, b));
  b.exports["internal/runtime/wire-properties.js"] = ["wireProperties"];
  b.exports["internal/runtime/callables.js"] = ["createWireProperties"];
  assert.throws(() => assertSameSdkContract(a, b));
});

test("declaration accounting excludes auxiliary consumers and requires every managed declaration", async () => {
  const { managedDeclarationStats } = await import("./catalog.mjs");
  const parent = resolve(repositoryRoot, ".tmp/verification-self-tests");
  mkdirSync(parent, { recursive: true });
  const dir = mkdtempSync(resolve(parent, "declarations-"));
  try {
    writeFileSync(resolve(dir, "index.d.ts"), "export {};\n");
    writeFileSync(resolve(dir, "consumer.d.ts"), "unmanaged probe must not count\n");
    const files = { "index.ts": "unused-by-byte-accounting" };
    assert.deepEqual(managedDeclarationStats(dir, files), { files: 1, bytes: 11 });
    rmSync(resolve(dir, "index.d.ts"));
    assert.throws(() => managedDeclarationStats(dir, files));
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
