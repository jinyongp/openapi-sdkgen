#!/usr/bin/env bash
# Invoked by check.sh after one input snapshot and generation.
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
checked="$1"
find "$checked" -type f -name '*.ts' -exec sed -i.bak '/^\/\/ @ts-nocheck$/d' {} \;
find "$checked" -type f -name '*.bak' -delete

cat >"$checked/generate-check.consumer.ts" <<'EOF'
import { createClient, type Components, type Operations } from "./index.js"

type Equal<Left, Right> =
  (<Value>() => Value extends Left ? 1 : 2) extends
  (<Value>() => Value extends Right ? 1 : 2) ? true : false
type Assert<Value extends true> = Value

type OperationID = keyof Operations
type Operation = Operations[OperationID]
type _Input = Operation["input"]
type _Output = Operation["output"]
type _Error = Operation["error"]
type _Options = Operation["options"]
type _Call = Operation["call"]
type _Raw = Operation["call"]["raw"]
type _RawResponse = Operation["rawResponse"]
type _Pagination = Operation["pagination"]

type ComponentName = keyof Components
type Component = Components[ComponentName]
type _ComponentInput = Component["input"]
type _ComponentOutput = Component["output"]

type Client = ReturnType<typeof createClient>
type _ExactOperations = Assert<Equal<keyof Client["$operations"], OperationID>>

export type GenerateCheckConsumerProbe = {
  readonly operation: [_Input, _Output, _Error, _Options, _Call, _Raw, _RawResponse, _Pagination]
  readonly component: [_ComponentInput, _ComponentOutput]
  readonly operations: _ExactOperations
}
EOF

cat >"$checked/package.json" <<'EOF'
{"type":"module","private":true}
EOF

cat >"$checked/tsconfig.json" <<'EOF'
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "skipLibCheck": false,
    "noEmit": true
  },
  "include": ["**/*.ts"]
}
EOF

typecheck_status=0
ts_node node --input-type=module - "$ROOT/internal/tscheck/strict-options.json" "$checked/tsconfig.json" <<'EOF'
import fs from "node:fs";
import path from "node:path";
const [profile, configPath] = process.argv.slice(2);
const config = JSON.parse(fs.readFileSync(configPath, "utf8"));
Object.assign(config.compilerOptions, JSON.parse(fs.readFileSync(profile, "utf8")).compilerOptions);
const moduleProfile = process.env.SDKGEN_GENERATE_CHECK_MODULE_PROFILE ?? "nodenext";
if (moduleProfile === "bundler") Object.assign(config.compilerOptions, { module: "ESNext", moduleResolution: "Bundler" });
else if (moduleProfile === "commonjs") {
  config.compilerOptions.verbatimModuleSyntax = false;
  fs.writeFileSync(path.join(path.dirname(configPath), "package.json"), JSON.stringify({ type: "commonjs", private: true }));
}
else if (moduleProfile !== "nodenext") throw new Error(`unknown verification module profile ${moduleProfile}`);
const checkingProfile = process.env.SDKGEN_GENERATE_CHECK_CHECKING_PROFILE ?? "strict";
if (!["strict", "relaxed", "isolated", "relaxed-isolated"].includes(checkingProfile))
  throw new Error(`unknown verification checking profile ${checkingProfile}`);
if (checkingProfile.startsWith("relaxed")) Object.assign(config.compilerOptions, { strict: false, exactOptionalPropertyTypes: false });
if (checkingProfile.endsWith("isolated")) {
  Object.assign(config.compilerOptions, { isolatedDeclarations: true, declaration: true, emitDeclarationOnly: true, noEmit: false, outDir: "./declarations" });
  config.exclude = ["declarations"];
}
if (process.env.SDKGEN_GENERATE_CHECK_RUNTIME_PROBE === "selection") {
  if (checkingProfile.endsWith("isolated")) throw new Error("runtime probing requires JavaScript output");
  Object.assign(config.compilerOptions, { noEmit: false, noEmitOnError: true, outDir: "./javascript" });
  config.exclude = ["javascript"];
}
fs.writeFileSync(configPath, JSON.stringify(config));
EOF
set +e
if [[ -n "${SDKGEN_GENERATE_CHECK_TSC_BIN:-}" ]]; then
  run_step "typecheck generated SDK" "$SDKGEN_GENERATE_CHECK_TSC_BIN" --project "$checked/tsconfig.json"
else
  run_step "typecheck generated SDK" ts_node node "$TYPESCRIPT_ROOT/node_modules/typescript/lib/tsc.js" --project "$checked/tsconfig.json"
fi
typecheck_status=$?
set -e
if ((typecheck_status == 0)) && [[ "${SDKGEN_GENERATE_CHECK_CHECKING_PROFILE:-}" == *isolated ]]; then
  ts_node node --input-type=module - "$checked/tsconfig.json" <<'EOF'
import fs from "node:fs";
import path from "node:path";
const configPath = process.argv[2];
const config = JSON.parse(fs.readFileSync(configPath, "utf8"));
Object.assign(config.compilerOptions, { isolatedDeclarations: false, emitDeclarationOnly: false, noEmit: true });
delete config.compilerOptions.outDir;
config.include = ["**/*.d.ts"];
delete config.exclude;
fs.writeFileSync(path.join(path.dirname(configPath), "declarations", "tsconfig.json"), JSON.stringify(config));
EOF
  set +e
  if [[ -n "${SDKGEN_GENERATE_CHECK_TSC_BIN:-}" ]]; then
    run_step "typecheck generated declarations" "$SDKGEN_GENERATE_CHECK_TSC_BIN" --project "$checked/declarations/tsconfig.json"
  else
    run_step "typecheck generated declarations" ts_node node "$TYPESCRIPT_ROOT/node_modules/typescript/lib/tsc.js" --project "$checked/declarations/tsconfig.json"
  fi
  typecheck_status=$?
  set -e
fi
script_note "typecheck exit: $typecheck_status"
if ((typecheck_status == 0)) && [[ "${SDKGEN_GENERATE_CHECK_RUNTIME_PROBE:-}" == "selection" ]]; then
  run_step "generated selection runtime" ts_node node "$ROOT/scripts/generate-check/selection-runtime.mjs" "$checked/javascript"
fi
exit "$typecheck_status"
