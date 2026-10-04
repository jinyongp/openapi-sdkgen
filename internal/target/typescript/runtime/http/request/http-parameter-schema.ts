import type { WireCodec } from "../../schema/wire-types.js";
import type { OperationDefinition, ParameterDefinition } from "../operation.js";
import type { ParameterWireEncoder } from "./http-request-parameter-types.js";
/** Encodes prepared schema values without structured sort mapping. */
export function createSchemaParameterEncoder(wire: WireCodec): ParameterWireEncoder {
  const { transformWireValue }: WireCodec = wire;
  return (
    operation: OperationDefinition,
    parameter: ParameterDefinition | undefined,
    value: unknown,
  ): unknown =>
    parameter?.schema === undefined
      ? value
      : transformWireValue(value, parameter.schema, operation.inputSchemas ?? {}, "encode");
}
