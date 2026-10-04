import type { SecuritySchemeDefinition, SecurityCredential } from "./security.js";
import type { Transport } from "../shared/transport.js";
import type { SecurityCredentialHandler } from "./security-handler-types.js";
import { securityCredentialError, securityCollision } from "./security-diagnostics.js";
export const apiQuery: SecurityCredentialHandler = {
  validate(scheme: SecuritySchemeDefinition, credential: SecurityCredential): void {
    if (
      credential?.kind !== "api-key" ||
      typeof credential.value !== "string" ||
      credential.value === ""
    )
      throw securityCredentialError(scheme.name, "api-key value");
  },
  apply(
    _transport: Transport | undefined,
    scheme: SecuritySchemeDefinition,
    credential: SecurityCredential,
    _headers: Headers,
    url: URL,
  ): void {
    if (url.searchParams.has(scheme.parameterName!))
      throw securityCollision(scheme.name, `query parameter ${scheme.parameterName}`);
    url.searchParams.set(
      scheme.parameterName!,
      (credential as import("./security.js").APIKeyCredential).value,
    );
  },
};
