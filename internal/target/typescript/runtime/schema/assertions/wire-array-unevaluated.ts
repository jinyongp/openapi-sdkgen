import type { WireSchema, WireSchemas, WireTransformOptions, DynamicScope } from "../wire-types.js";
import type { ValidationContext, Evaluation } from "../wire-context.js";
import { validateWireValueWithContext } from "../wire-execution.js";
export function arrayAfter(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  scope: DynamicScope,
  context: ValidationContext,
  evaluation: Evaluation,
): void {
  if (!Array.isArray(value)) return;
  if (schema.unevaluatedItems !== undefined) {
    for (const [index, item] of value.entries()) {
      if (evaluation.indexes.has(index)) continue;
      if (schema.unevaluatedItems === false)
        throw new TypeError(`unexpected unevaluated item ${index}`);
      validateWireValueWithContext(
        item,
        schema.unevaluatedItems,
        components,
        direction,
        options,
        scope,
        context,
      );
      evaluation.indexes.add(index);
    }
  }
}
