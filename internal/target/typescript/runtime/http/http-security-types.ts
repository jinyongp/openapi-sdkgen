import type { ClientOptions } from "./configuration.js";
import type { OperationDefinition } from "./operation.js";
import type { RequestOptions } from "./request.js";
import type { EncodedRequest, SDKSecuritySource } from "./http-types.js";
import type {
  SecurityRequirementDefinition,
  SecuritySchemeDefinition,
} from "../security/security.js";
import type { SecurityCredentialHandler } from "../security/security-handler-types.js";
/** Resolves configured SDK credentials and detects a conflicting source. */
export type SecuritySourceResolver = (
  options: ClientOptions,
  requestOptions: RequestOptions,
  encoded: EncodedRequest,
  credentials: RequestCredentials | undefined,
  scheme: SecuritySchemeDefinition,
  allowMutualTLS: boolean,
) => SDKSecuritySource;
/** Finds the credential handler for one normalized security scheme. */
export type SecurityCredentialResolver = (
  scheme: SecuritySchemeDefinition,
) => SecurityCredentialHandler;
/** Selects one normalized operation security requirement. */
export type SecurityRequirementSelector = (
  operation: OperationDefinition,
  declared: readonly SecurityRequirementDefinition[],
  requestedID: string | undefined,
) => SecurityRequirementDefinition;
/** Canonical ports for source matching, credential dispatch and requirement selection. */
export interface OperationSecurityPolicy {
  readonly source: SecuritySourceResolver;
  readonly credential: SecurityCredentialResolver;
  readonly select: SecurityRequirementSelector;
}
