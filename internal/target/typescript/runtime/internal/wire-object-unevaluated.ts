import type { WireSchema, WireSchemas, WireTransformOptions, DynamicScope } from "./wire-types.js";
import type { ValidationContext, Evaluation } from "./wire-context.js";
import { isRecord } from "./runtime-support.js";
import { validateWireValueWithContext } from "./wire-core.js";
export function objectAfter(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  scope: DynamicScope,
  context: ValidationContext,
  evaluation: Evaluation,
): void {
  if (!isRecord(value)) return;
  if (schema.unevaluatedProperties !== undefined) {
    for (const [key, item] of Object.entries(value)) {
      if (evaluation.properties.has(key)) continue;
      if (schema.unevaluatedProperties === false) {
        if (options.unknownProperties === "reject")
          throw new TypeError(`unexpected unevaluated property ${key}`);
        continue;
      }
      validateWireValueWithContext(
        item,
        schema.unevaluatedProperties,
        components,
        direction,
        options,
        scope,
        context,
      );
      evaluation.properties.add(key);
    }
  }
}
