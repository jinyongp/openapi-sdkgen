import type { WireSchema } from "../wire-types.js";
/** Checks numeric bounds without loading decimal multipleOf handling. */
export function validateNumber(value: number, schema: WireSchema): void {
  if (schema.maximum !== undefined && value > schema.maximum)
    throw new TypeError(`must be <= ${schema.maximum}`);
  if (schema.exclusiveMaximum !== undefined && value >= schema.exclusiveMaximum)
    throw new TypeError(`must be < ${schema.exclusiveMaximum}`);
  if (schema.minimum !== undefined && value < schema.minimum)
    throw new TypeError(`must be >= ${schema.minimum}`);
  if (schema.exclusiveMinimum !== undefined && value <= schema.exclusiveMinimum)
    throw new TypeError(`must be > ${schema.exclusiveMinimum}`);
}
