import type { OperationDefinition, ParameterDefinition } from "../operation.js";
import type {
  ParameterWireEncoder,
  HeaderParameterEncoder,
} from "./http-request-parameter-types.js";
import { findParameterByProperty, rejectUndefinedArrayValues } from "./http-request-input.js";
import { serializeSimpleValue } from "./http-simple-value.js";

/** Encodes schema-based header parameters with the prepared parameter validator. */
export function createSchemaHeaderEncoder(
  encodeParameter: ParameterWireEncoder,
): HeaderParameterEncoder {
  return (
    headers: Headers,
    values: Record<string, unknown>,
    operation: OperationDefinition,
  ): void => {
    rejectUndefinedArrayValues(values);
    for (const [property, value] of Object.entries(values)) {
      if (value === undefined) continue;
      const parameter: ParameterDefinition | undefined = findParameterByProperty(
        operation,
        "header",
        property,
      );
      headers.set(
        parameter?.name ?? property,
        serializeSimpleValue(
          encodeParameter(operation, parameter, value),
          parameter?.explode ?? false,
        ),
      );
    }
    for (const parameter of operation.parameters ?? []) {
      if (
        parameter.location === "header" &&
        parameter.required &&
        (!Object.hasOwn(values, parameter.property) || values[parameter.property] === undefined)
      ) {
        throw new TypeError(`Missing required header parameter ${parameter.name}`);
      }
    }
  };
}
