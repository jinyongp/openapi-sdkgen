import type { SecurityCredentialHandler } from "../security/security-handler-types.js";
import type { SecuritySchemeDefinition } from "../security/security.js";
import type { OperationSecurity } from "./http-types.js";
import type { SecuritySourceResolver } from "./http-security-types.js";
import type { ClientOptions } from "./configuration.js";
import type { RequestOptions } from "./request.js";
import type { EncodedRequest } from "./http-types.js";
import { composeOperationSecurity } from "./http-security-core.js";
import {
  authorizationProtocol,
  authorizationSecuritySource,
} from "./http-security-authorization.js";
import { selectSingleSecurityRequirement } from "./http-security-single.js";
const bearerSource: SecuritySourceResolver = (
  options: ClientOptions,
  requestOptions: RequestOptions,
  encoded: EncodedRequest,
): ReturnType<SecuritySourceResolver> =>
  authorizationSecuritySource(
    options,
    requestOptions,
    encoded,
    (value: string): boolean => authorizationProtocol(value) === "bearer",
  );
/** Prepared Bearer-only contracts preserve providers, collisions and credential validation. */
export function createBearerOperationSecurity(
  handler: SecurityCredentialHandler,
): OperationSecurity {
  return composeOperationSecurity({
    source: bearerSource,
    credential: (_scheme: SecuritySchemeDefinition): SecurityCredentialHandler => handler,
    select: selectSingleSecurityRequirement,
  });
}
