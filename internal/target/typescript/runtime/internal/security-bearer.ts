import type { SecuritySchemeDefinition, SecurityCredential } from "./security.js";
import type { SecurityCredentialHeader } from "./security-handler-types.js";
import type { SecurityCredentialHandler } from "./security-handler-types.js";
import { securityCredentialError } from "./security-diagnostics.js";
export const httpBearer: SecurityCredentialHandler = {
  validate(scheme: SecuritySchemeDefinition, credential: SecurityCredential): void {
    if (
      credential?.kind !== "http-bearer" ||
      typeof credential.token !== "string" ||
      credential.token === ""
    )
      throw securityCredentialError(scheme.name, "http-bearer token");
  },
  header(
    _scheme: SecuritySchemeDefinition,
    credential: SecurityCredential,
  ): SecurityCredentialHeader | undefined {
    return {
      name: "Authorization",
      value: `Bearer ${(credential as import("./security.js").HTTPBearerCredential).token}`,
    };
  },
};
