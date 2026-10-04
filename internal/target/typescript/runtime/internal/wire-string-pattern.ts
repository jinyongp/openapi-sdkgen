import type { WireSchema } from "./wire-types.js";
export function validateStringPattern(value: string, schema: WireSchema): void {
  if (schema.pattern !== undefined && !new RegExp(schema.pattern, "u").test(value))
    throw new TypeError(`must match pattern ${schema.pattern}`);
}
