import { TransportErrorCode } from "./runtime-support.js";
import type { TransportError } from "./runtime-support.js";
import { transportError } from "./http-execution-support.js";
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
