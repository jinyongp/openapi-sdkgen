import type { WireCodec, WireTransformOptions } from "../../schema/wire-types.js";
import type { OperationDefinition } from "../operation.js";
import type { WireResponseDefinition } from "./http-response-types.js";
/** Preserves response transformation while allowing undeclared and missing fields. */
export const tolerantResponseTransformOptions: WireTransformOptions = {
  unknownProperties: "preserve",
};
/** Returns the empty decoded header view for contracts without declared headers. */
export async function emptyResponseHeaders(): Promise<Readonly<Record<string, unknown>>> {
  return Object.create(null) as Readonly<Record<string, unknown>>;
}
/** Applies a selected prepared contract without changing undeclared response values. */
export function transformPreparedResponse(
  wire: WireCodec,
  definition: WireResponseDefinition | undefined,
  operation: OperationDefinition,
  value: unknown,
): unknown {
  const { transformWireValue }: WireCodec = wire;
  return definition === undefined
    ? value
    : transformWireValue(
        value,
        definition.schema,
        operation.outputSchemas ?? {},
        "decode",
        tolerantResponseTransformOptions,
      );
}
