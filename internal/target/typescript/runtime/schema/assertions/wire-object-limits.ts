import type { WireSchema, WireSchemas, WireTransformOptions, DynamicScope } from "../wire-types.js";
import type { ValidationContext, Evaluation } from "../wire-context.js";
import { isRecord } from "../../shared/runtime-support.js";
export function objectBefore(
  value: unknown,
  schema: WireSchema,
  _components: WireSchemas,
  _direction: "encode" | "decode",
  _options: WireTransformOptions,
  _scope: DynamicScope,
  _context: ValidationContext,
  _evaluation: Evaluation,
): void {
  if (!isRecord(value)) return;
  if (schema.minProperties !== undefined && Object.keys(value).length < schema.minProperties)
    throw new TypeError(`must contain at least ${schema.minProperties} properties`);
  if (schema.maxProperties !== undefined && Object.keys(value).length > schema.maxProperties)
    throw new TypeError(`must contain at most ${schema.maxProperties} properties`);
}
