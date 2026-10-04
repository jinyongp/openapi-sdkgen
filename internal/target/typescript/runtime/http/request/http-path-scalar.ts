import type { ParameterDefinition } from "../operation.js";
/** Prepared simple scalar contracts have no array, object, label or matrix serialization. */
export function serializeSimpleScalarPathParameter(
  _parameter: ParameterDefinition | undefined,
  _name: string,
  value: unknown,
): string {
  return encodeURIComponent(String(value));
}
