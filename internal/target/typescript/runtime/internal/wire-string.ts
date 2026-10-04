import type { WireSchema } from "./wire-types.js";
export function validateString(value: string, schema: WireSchema): void {
  if (schema.minLength !== undefined && [...value].length < schema.minLength)
    throw new TypeError(`must have length >= ${schema.minLength}`);
  if (schema.maxLength !== undefined && [...value].length > schema.maxLength)
    throw new TypeError(`must have length <= ${schema.maxLength}`);
}
