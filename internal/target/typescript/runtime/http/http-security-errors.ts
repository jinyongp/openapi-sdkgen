import { transportError, TransportErrorCode } from "../shared/runtime-support.js";
import type { TransportError } from "../shared/runtime-support.js";
/** Creates the shared invalid-security-requirement transport error. */
export function securityRequirementInvalid(message: string): TransportError {
  return transportError(TransportErrorCode.SECURITY_REQUIREMENT_INVALID, message, undefined);
}
