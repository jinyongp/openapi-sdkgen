import { isRecord } from "../../shared/runtime-support.js";
import type { OperationDefinition, ParameterDefinition } from "../operation.js";
/** Rejects undefined items throughout an input array or object. */
export function rejectUndefinedArrayValues(value: unknown): void {
  if (Array.isArray(value)) {
    for (const item of value) {
      if (item === undefined) {
        throw new TypeError("Request arrays cannot contain undefined");
      }
      rejectUndefinedArrayValues(item);
    }
    return;
  }
  if (isRecord(value)) {
    for (const item of Object.values(value)) rejectUndefinedArrayValues(item);
  }
}

/** Finds the normalized parameter owning one public input property. */
export function findParameterByProperty(
  operation: OperationDefinition,
  location: ParameterDefinition["location"],
  property: string,
): ParameterDefinition | undefined {
  return operation.parameters?.find(
    (parameter: ParameterDefinition): boolean =>
      parameter.location === location && parameter.property === property,
  );
}
