import type { WireSchema, WireSchemas, WireTransformOptions, DynamicScope } from "../wire-types.js";
import type { ValidationContext, Evaluation } from "../wire-context.js";
import { schemaMatchesForControlFlow } from "../wire-execution.js";
export function arrayContains(
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
  if (schema.contains !== undefined) {
    const matches: number = value.filter((item: unknown, index: number): boolean => {
      const matches: boolean = schemaMatchesForControlFlow(
        item,
        schema.contains!,
        components,
        direction,
        options,
        scope,
        context,
      );
      if (matches) evaluation.indexes.add(index);
      return matches;
    }).length;
    const minimum: number = schema.minContains ?? 1;
    if (matches < minimum) throw new TypeError(`must contain at least ${minimum} matching items`);
    if (schema.maxContains !== undefined && matches > schema.maxContains)
      throw new TypeError(`must contain at most ${schema.maxContains} matching items`);
  }
}
