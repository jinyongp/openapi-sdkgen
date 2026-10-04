import type { SecuritySchemeDefinition, SecurityCredential } from "./security.js";
import type { Transport } from "./transport.js";
import type { SecurityCredentialHandler } from "./security-handler-types.js";
import { securityCredentialError } from "./security-diagnostics.js";
import { TransportErrorCode } from "./runtime-support.js";
import { transportError } from "./http-execution-support.js";
export const mutualTLS: SecurityCredentialHandler = {
  validate(scheme: SecuritySchemeDefinition, credential: SecurityCredential): void {
    if (credential?.kind !== "mutual-tls")
      throw securityCredentialError(scheme.name, "mutual-tls credential");
  },
  apply(transport: Transport | undefined, scheme: SecuritySchemeDefinition): void {
    if (!transport?.capabilities?.mutualTLS)
      throw transportError(
        TransportErrorCode.TRANSPORT_CAPABILITY_REQUIRED,
        `Security scheme ${scheme.name} requires a mutual-TLS transport`,
        undefined,
      );
  },
};
