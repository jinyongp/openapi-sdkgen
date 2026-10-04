import type { SecurityCredentialHandler } from "../security/security-handler-types.js";
import type { OperationSecurity } from "./http-types.js";
import { composeOperationSecurity } from "./http-security-core.js";
import { securitySourceForScheme } from "./http-security-source.js";
import { createSecurityCredentialResolver } from "./http-security-credentials.js";
import { selectSecurityRequirement } from "./http-security-requirements.js";
/** Generic security facade composes the canonical orchestration and scheme policies. */
export function createOperationSecurity(
  handlers: Readonly<Record<string, SecurityCredentialHandler>>,
): OperationSecurity {
  return composeOperationSecurity({
    source: securitySourceForScheme,
    credential: createSecurityCredentialResolver(handlers),
    select: selectSecurityRequirement,
  });
}
