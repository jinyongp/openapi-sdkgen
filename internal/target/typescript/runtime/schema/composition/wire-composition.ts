import type { WireSchema, WireSchemas, WireTransformOptions, DynamicScope } from "../wire-types.js";
import type { ValidationContext, Evaluation } from "../wire-context.js";
import { isRecord } from "../../shared/runtime-support.js";
import {
  validateWireValueWithContext,
  matchingSchemasForControlFlow,
  schemaMatchesForControlFlow,
  transformWireValueWithContext,
} from "../wire-execution.js";
import { mergeEvaluation } from "../wire-state.js";
export function validateComposition(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  scope: DynamicScope,
  context: ValidationContext,
  evaluation: Evaluation,
): void {
  if (schema.oneOf !== undefined) {
    const matches: readonly WireSchema[] = matchingSchemasForControlFlow(
      value,
      schema.oneOf,
      components,
      direction,
      options,
      scope,
      context,
    );
    if (matches.length !== 1)
      throw new TypeError(`oneOf requires exactly one matching schema, got ${matches.length}`);
    for (const branch of matches)
      mergeEvaluation(
        evaluation,
        validateWireValueWithContext(value, branch, components, direction, options, scope, context),
      );
  }
  if (schema.anyOf !== undefined) {
    const matches: readonly WireSchema[] = matchingSchemasForControlFlow(
      value,
      schema.anyOf,
      components,
      direction,
      options,
      scope,
      context,
    );
    if (matches.length === 0) throw new TypeError("anyOf requires at least one matching schema");
    for (const branch of matches)
      mergeEvaluation(
        evaluation,
        validateWireValueWithContext(value, branch, components, direction, options, scope, context),
      );
  }
  if (
    schema.not !== undefined &&
    schemaMatchesForControlFlow(value, schema.not, components, direction, options, scope, context)
  ) {
    throw new TypeError("must not match negated schema");
  }
  if (schema.if !== undefined) {
    const matches: boolean = schemaMatchesForControlFlow(
      value,
      schema.if,
      components,
      direction,
      options,
      scope,
      context,
    );
    if (matches)
      mergeEvaluation(
        evaluation,
        validateWireValueWithContext(
          value,
          schema.if,
          components,
          direction,
          options,
          scope,
          context,
        ),
      );
    const branch: WireSchema | undefined = matches ? schema.then : schema.else;
    if (branch !== undefined)
      mergeEvaluation(
        evaluation,
        validateWireValueWithContext(value, branch, components, direction, options, scope, context),
      );
  }
  for (const branch of schema.allOf ?? [])
    mergeEvaluation(
      evaluation,
      validateWireValueWithContext(value, branch, components, direction, options, scope, context),
    );
}
export function transformComposition(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  scope: DynamicScope,
  context: ValidationContext,
  representations: unknown[],
): void {
  for (const branch of schema.allOf ?? []) {
    representations?.push(
      transformWireValueWithContext(value, branch, components, direction, options, scope, context),
    );
  }
  if (schema.if !== undefined) {
    const branch: WireSchema | undefined = schemaMatchesForControlFlow(
      value,
      schema.if,
      components,
      direction,
      options,
      scope,
      context,
    )
      ? schema.then
      : schema.else;
    if (branch !== undefined)
      representations?.push(
        transformWireValueWithContext(
          value,
          branch,
          components,
          direction,
          options,
          scope,
          context,
        ),
      );
  }
  for (const keyword of ["oneOf", "anyOf"] as const) {
    const variants: readonly WireSchema[] | undefined = schema[keyword];
    if (variants === undefined) continue;
    const matches: readonly WireSchema[] = matchingSchemasForControlFlow(
      value,
      variants,
      components,
      direction,
      options,
      scope,
      context,
    );
    const preferred: WireSchema | undefined =
      schema.discriminator === undefined
        ? matches[0]
        : (discriminatorVariant(value, schema) ?? matches[0]);
    const selected: readonly WireSchema[] =
      keyword === "anyOf" && matches.length > 0
        ? matches
        : preferred === undefined
          ? []
          : [preferred];
    for (const branch of selected)
      representations?.push(
        transformWireValueWithContext(
          value,
          branch,
          components,
          direction,
          options,
          scope,
          context,
        ),
      );
  }
}
function discriminatorVariant(value: unknown, schema: WireSchema): WireSchema | undefined {
  if (!isRecord(value) || schema.discriminator === undefined) return undefined;
  const property: string = schema.discriminator.property;
  const candidate: unknown = value[property];
  if (typeof candidate !== "string") return schema.discriminator.defaultMapping;
  return schema.discriminator.mapping?.[candidate] ?? schema.discriminator.defaultMapping;
}
