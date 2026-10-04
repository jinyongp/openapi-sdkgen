import type { WireCodec, WireValidationHandlers, WireExecution } from "./wire-types.js";
import { validateProgram, transformProgram } from "./program-execution.js";
import { schemaMatchesForControlFlow, matchingSchemasForControlFlow } from "./wire-control-flow.js";
import { bindProgramCodec } from "./program-binding.js";
const execution: WireExecution = {
  validate: validateProgram,
  transform: transformProgram,
  matches: schemaMatchesForControlFlow,
  matching: matchingSchemasForControlFlow,
};
export function createProgramCodec(handlers: WireValidationHandlers): WireCodec {
  return bindProgramCodec(handlers, execution);
}
