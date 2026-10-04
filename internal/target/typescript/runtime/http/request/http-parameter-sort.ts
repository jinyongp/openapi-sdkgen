import type { WireCodec } from "../../schema/wire-types.js";
import type { OperationDefinition, ParameterDefinition } from "../operation.js";
import type { ParameterWireEncoder } from "./http-request-parameter-types.js";
import { isRecord } from "../../shared/runtime-support.js";
import { createSchemaParameterEncoder } from "./http-parameter-schema.js";
/** Validates structured sort entries, then applies the canonical schema encoder. */
export function createParameterEncoder(wire: WireCodec): ParameterWireEncoder {
  const encodeSchema: ParameterWireEncoder = createSchemaParameterEncoder(wire);
  function encodeParameterWireValue(
    operation: OperationDefinition,
    parameter: ParameterDefinition | undefined,
    value: unknown,
  ): unknown {
    if (parameter?.sort !== undefined && Array.isArray(value)) {
      value = value.map((entry: unknown): string => {
        if (
          !isRecord(entry) ||
          typeof entry["field"] !== "string" ||
          typeof entry["direction"] !== "string"
        ) {
          throw new TypeError(`Invalid structured sort value for ${parameter.name}`);
        }
        const wire: string | undefined =
          parameter.sort?.[`${entry["field"]}\u0000${entry["direction"]}`];
        if (wire === undefined)
          throw new TypeError(`Invalid structured sort value for ${parameter.name}`);
        return wire;
      });
    }
    return encodeSchema(operation, parameter, value);
  }

  return encodeParameterWireValue;
}
