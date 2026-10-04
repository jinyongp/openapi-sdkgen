import type { SecuritySchemeDefinition, SecurityCredential } from "./security.js";
import type { SecurityCredentialHeader } from "./security-handler-types.js";
import type { SecurityCredentialHandler } from "./security-handler-types.js";
import { securityCredentialError } from "./security-diagnostics.js";
export const apiHeader: SecurityCredentialHandler = {
  validate(scheme: SecuritySchemeDefinition, credential: SecurityCredential): void {
    if (
      credential?.kind !== "api-key" ||
      typeof credential.value !== "string" ||
      credential.value === ""
    )
      throw securityCredentialError(scheme.name, "api-key value");
  },
  header(
    scheme: SecuritySchemeDefinition,
    credential: SecurityCredential,
  ): SecurityCredentialHeader | undefined {
    return {
      name: scheme.parameterName!,
      value: (credential as import("./security.js").APIKeyCredential).value,
    };
  },
};
