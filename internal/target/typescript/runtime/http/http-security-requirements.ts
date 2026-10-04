import type { OperationDefinition } from "./operation.js";
import type { SecurityRequirementDefinition } from "../security/security.js";
import { operationDiagnosticName } from "./operation.js";
import {
  defineOwnDataProperty,
  transportError,
  TransportErrorCode,
} from "../shared/runtime-support.js";
import { securityRequirementInvalid } from "./http-security-errors.js";
import { selectSingleSecurityRequirement } from "./http-security-single.js";
/** Selects and validates an explicit requirement when an operation declares alternatives. */
export function selectSecurityRequirement(
  operation: OperationDefinition,
  declared: readonly SecurityRequirementDefinition[],
  requestedID: string | undefined,
): SecurityRequirementDefinition {
  const requirements: Record<string, SecurityRequirementDefinition> = Object.create(null) as Record<
    string,
    SecurityRequirementDefinition
  >;
  for (const requirement of declared)
    defineOwnDataProperty(requirements, requirement.id, requirement);
  let selected: SecurityRequirementDefinition;
  if (declared.length === 1) {
    selected = selectSingleSecurityRequirement(operation, declared, requestedID);
  } else {
    if (requestedID === undefined) {
      throw transportError(
        TransportErrorCode.SECURITY_REQUIREMENT_REQUIRED,
        `Operation ${operationDiagnosticName(operation)} requires an explicit OpenAPI security requirement`,
        undefined,
      );
    }
    const requested: SecurityRequirementDefinition | undefined = requirements[requestedID];
    if (requested === undefined) {
      throw securityRequirementInvalid(
        `Operation ${operationDiagnosticName(operation)} does not declare security requirement ${requestedID}`,
      );
    }
    selected = requested;
  }
  return selected;
}
