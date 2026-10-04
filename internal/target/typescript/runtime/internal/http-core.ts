export * from "./http-execution.js";
export { applyOperationSecurity } from "./security-handlers.js";
import { createHTTPServices as createCoreHTTPServices } from "./http-execution.js";
import { applyOperationSecurity } from "./security-handlers.js";
import type { WireCodec } from "./wire-types.js";
import type { HTTPCodecExtensions, RequestExecutionServices } from "./http-types.js";
/** Compatibility factory retains credential support for arbitrary operation definitions. */
export function createHTTPServices(
  wire: WireCodec,
  extensions: HTTPCodecExtensions,
): RequestExecutionServices {
  return { ...createCoreHTTPServices(wire, extensions), applyOperationSecurity };
}
