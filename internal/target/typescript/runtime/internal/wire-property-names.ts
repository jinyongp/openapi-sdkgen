import type { WireSchema, WireSchemas, WireTransformOptions, DynamicScope } from "./wire-types.js";
import type { ValidationContext, Evaluation } from "./wire-context.js";
import { isRecord } from "./runtime-support.js";
import { validateWireValueWithContext, classifyWireProperties } from "./wire-core.js";
export function propertyNames(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  scope: DynamicScope,
  context: ValidationContext,
  _evaluation: Evaluation,
): void {
  if (!isRecord(value)) return;
  if (schema.propertyNames !== undefined) {
    for (const property of classifyWireProperties(value, schema, direction, context))
      validateWireValueWithContext(
        property.wireName,
        schema.propertyNames,
        components,
        direction,
        options,
        scope,
        context,
      );
  }
}
