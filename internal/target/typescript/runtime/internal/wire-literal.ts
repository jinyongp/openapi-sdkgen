import type { WireSchema } from "./wire-types.js";
import { wireValueEquals } from "./wire-equality.js";
/** Checks const and enum values with JSON structural equality. */
export function validateLiteral(value: unknown, schema: WireSchema): void {
  if (schema.constValue !== undefined && !wireValueEquals(value, schema.constValue)) {
    throw new TypeError("value does not match const");
  }
  if (
    schema.enumValues !== undefined &&
    !schema.enumValues.some((item: unknown): boolean => wireValueEquals(value, item))
  ) {
    throw new TypeError("value is not in enum");
  }
}
