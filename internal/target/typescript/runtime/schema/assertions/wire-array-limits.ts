import type { WireSchema } from "../wire-types.js";
export function arrayBefore(value: unknown, schema: WireSchema): void {
  if (!Array.isArray(value)) return;
  if (schema.minItems !== undefined && value.length < schema.minItems)
    throw new TypeError(`must contain at least ${schema.minItems} items`);
  if (schema.maxItems !== undefined && value.length > schema.maxItems)
    throw new TypeError(`must contain at most ${schema.maxItems} items`);
}
