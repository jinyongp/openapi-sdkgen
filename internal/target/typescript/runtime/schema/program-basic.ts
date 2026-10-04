import type { WireCodec, WireValidationHandlers, WireExecution } from "./wire-types.js";
import { validateProgram, transformProgram } from "./program-execution.js";
import { bindProgramCodec } from "./program-binding.js";
function unsupportedBranch(): never {
  throw new TypeError("unprepared schema branch execution");
}
const execution: WireExecution = {
  validate: validateProgram,
  transform: transformProgram,
  matches: unsupportedBranch,
  matching: unsupportedBranch,
};
/** Binds prepared contracts that do not require branch selection. */
export function createBasicProgramCodec(handlers: WireValidationHandlers): WireCodec {
  return bindProgramCodec(handlers, execution);
}
