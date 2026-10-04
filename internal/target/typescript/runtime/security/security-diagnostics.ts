import { TransportErrorCode } from "../shared/runtime-support.js";
import type { TransportError } from "../shared/runtime-support.js";
import { transportError } from "../shared/runtime-support.js";
export function securityCredentialError(scheme: string, expected: string): TransportError {
  return transportError(
    TransportErrorCode.SECURITY_CREDENTIALS_INVALID,
    `Security scheme ${scheme} requires ${expected}`,
    undefined,
  );
}

export function securityCollision(scheme: string, location: string): TransportError {
  return transportError(
    TransportErrorCode.SECURITY_CREDENTIALS_INVALID,
    `Security scheme ${scheme} conflicts with caller-supplied ${location}`,
    undefined,
  );
}
