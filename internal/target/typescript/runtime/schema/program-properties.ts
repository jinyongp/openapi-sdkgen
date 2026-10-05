import type {
  WireSchema,
  WireSchemas,
  WireTransformOptions,
  DynamicScope,
  ValidationContext,
  Evaluation,
} from "./wire-types.js";
import { defineOwnDataProperty } from "../shared/runtime-support.js";

/** Generated property plans retain exact wire names in both projections. */
export function validateProgramProperties(
  value: Record<string, unknown>,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  scope: DynamicScope,
  context: ValidationContext,
  evaluation?: Evaluation,
): void {
  for (const [name, property] of Object.entries(schema.properties!)) {
    const source: string = direction === "encode" ? property.property : name;
    if (!Object.hasOwn(value, source)) continue;
    try {
      context.execution.validate(
        value[source],
        property.schema,
        components,
        direction,
        options,
        scope,
        context,
      );
      evaluation?.properties.add(source);
    } catch (cause: unknown) {
      throw new TypeError(
        `property ${name}: ${cause instanceof Error ? cause.message : "invalid value"}`,
        { cause },
      );
    }
  }
}

/** Checks the owning contract's exact required property names. */
export function validateProgramRequired(
  value: Record<string, unknown>,
  schema: WireSchema,
  direction: "encode" | "decode",
): void {
  for (const name of schema.required!) {
    const required: string =
      direction === "encode" ? (schema.properties?.[name]?.property ?? name) : name;
    if (!Object.hasOwn(value, required) || value[required] === undefined) {
      throw new TypeError(`missing required property ${name}`);
    }
  }
}

/** Copies an object and transforms only its declared generated properties. */
export function transformProgramProperties(
  value: Record<string, unknown>,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  scope: DynamicScope,
  context: ValidationContext,
): Record<string, unknown> {
  const result: Record<string, unknown> = {};
  for (const [key, item] of Object.entries(value)) defineOwnDataProperty(result, key, item);
  for (const [name, property] of Object.entries(schema.properties!)) {
    if (Object.hasOwn(value, name)) {
      defineOwnDataProperty(
        result,
        name,
        context.execution.transform(
          value[name],
          property.schema,
          components,
          direction,
          options,
          scope,
          context,
        ),
      );
    }
  }
  return result;
}
