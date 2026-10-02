import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

export const strictCompilerOptions = Object.freeze(
  JSON.parse(
    readFileSync(new URL("../../../internal/tscheck/strict-options.json", import.meta.url), "utf8"),
  ).compilerOptions,
);

export function assertStrictCompilerOptions(options) {
  for (const [flag, value] of Object.entries(strictCompilerOptions))
    assert.equal(options[flag], value, `verification compiler option ${flag} must be ${value}`);
}

export function assertCheckedSources(files) {
  for (const file of files)
    assert(
      !/^\s*\/\/\s*@ts-nocheck\b/m.test(readFileSync(file, "utf8")),
      `unchecked verification source: ${file}`,
    );
}
