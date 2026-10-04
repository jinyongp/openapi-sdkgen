import type { OperationDefinition } from "./operation.js";
import type { SecurityRequirementDefinition } from "../security/security.js";
import { operationDiagnosticName } from "./operation.js";
import { securityRequirementInvalid } from "./http-security-errors.js";
/** Selects the sole prepared requirement and rejects explicit selection. */
export function selectSingleSecurityRequirement(
  operation: OperationDefinition,
  declared: readonly SecurityRequirementDefinition[],
  requestedID: string | undefined,
): SecurityRequirementDefinition {
  if (requestedID !== undefined)
    throw securityRequirementInvalid(
      `Operation ${operationDiagnosticName(operation)} has one SDK-selected security requirement and does not accept an explicit selection`,
    );
  return declared[0]!;
}
