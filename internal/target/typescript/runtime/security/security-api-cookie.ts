import type { SecuritySchemeDefinition, SecurityCredential } from "./security.js";
import type { Transport } from "../shared/transport.js";
import type { SecurityCredentialHandler } from "./security-handler-types.js";
import { securityCredentialError, securityCollision } from "./security-diagnostics.js";
import { TransportErrorCode } from "../shared/runtime-support.js";
import { transportError } from "../shared/runtime-support.js";
export const apiCookie: SecurityCredentialHandler = {
  validate(scheme: SecuritySchemeDefinition, credential: SecurityCredential): void {
    if (
      credential?.kind !== "api-key" ||
      typeof credential.value !== "string" ||
      credential.value === ""
    )
      throw securityCredentialError(scheme.name, "api-key value");
  },
  apply(
    transport: Transport | undefined,
    scheme: SecuritySchemeDefinition,
    credential: SecurityCredential,
    headers: Headers,
  ): void {
    if (!transport?.capabilities?.cookieJar)
      throw transportError(
        TransportErrorCode.TRANSPORT_CAPABILITY_REQUIRED,
        `Security scheme ${scheme.name} requires a cookie-jar transport`,
        undefined,
      );
    if (headers.has("Cookie")) throw securityCollision(scheme.name, "Cookie header");
    headers.set(
      "Cookie",
      `${encodeURIComponent(scheme.parameterName!)}=${encodeURIComponent((credential as import("./security.js").APIKeyCredential).value)}`,
    );
  },
};
