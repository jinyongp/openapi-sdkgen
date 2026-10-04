import type { WireSchema, WireSchemas, WireTransformOptions, DynamicScope } from "../wire-types.js";
import type { ValidationContext, Evaluation } from "../wire-context.js";
import { isRecord } from "../../shared/runtime-support.js";
import type { WireProperty } from "../wire-types.js";
import { validateWireValueWithContext } from "../wire-execution.js";
import { mergeEvaluation } from "../wire-state.js";
export function dependencies(
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
  const properties: Readonly<Record<string, WireProperty>> = schema.properties ?? {};
  for (const [property, required] of Object.entries(schema.dependentRequired ?? {})) {
    const sourceProperty: string =
      direction === "encode" && properties[property] !== undefined
        ? properties[property].property
        : property;
    if (!Object.hasOwn(value, sourceProperty) || value[sourceProperty] === undefined) continue;
    for (const dependency of required) {
      const sourceDependency: string =
        direction === "encode" && properties[dependency] !== undefined
          ? properties[dependency].property
          : dependency;
      if (!Object.hasOwn(value, sourceDependency) || value[sourceDependency] === undefined) {
        throw new TypeError(`property ${property} requires property ${dependency}`);
      }
    }
  }
  for (const [property, dependency] of Object.entries(schema.dependentSchemas ?? {})) {
    const sourceProperty: string =
      direction === "encode" && properties[property] !== undefined
        ? properties[property].property
        : property;
    if (Object.hasOwn(value, sourceProperty) && value[sourceProperty] !== undefined)
      mergeEvaluation(
        evaluation,
        validateWireValueWithContext(
          value,
          dependency,
          components,
          direction,
          options,
          scope,
          context,
        ),
      );
  }
}
