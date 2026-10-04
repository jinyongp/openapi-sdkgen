import type { SecuritySchemeDefinition, SecurityCredential } from "./security.js";
import type { SecurityCredentialHeader } from "./security-handler-types.js";
import type { SecurityCredentialHandler } from "./security-handler-types.js";
import { securityCredentialError } from "./security-diagnostics.js";
export const httpCredential: SecurityCredentialHandler = {
  validate(scheme: SecuritySchemeDefinition, credential: SecurityCredential): void {
    if (
      credential?.kind !== "http" ||
      typeof credential.value !== "string" ||
      credential.value === ""
    )
      throw securityCredentialError(scheme.name, "http credential");
  },
  header(
    scheme: SecuritySchemeDefinition,
    credential: SecurityCredential,
  ): SecurityCredentialHeader | undefined {
    return {
      name: "Authorization",
      value: `${scheme.scheme} ${(credential as import("./security.js").HTTPCredential).value}`,
    };
  },
};
