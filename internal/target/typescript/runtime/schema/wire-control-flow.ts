import type { WireSchema, WireSchemas, WireTransformOptions, DynamicScope } from "./wire-types.js";
import type { ValidationContext } from "./wire-context.js";
const strictWireTransformOptions: WireTransformOptions = { unknownProperties: "reject" };

/** Tests a branch with control-flow validation and preserved caller scope. */
export function schemaMatchesForControlFlow(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  dynamicScope: DynamicScope,
  context: ValidationContext,
): boolean {
  if (options.unknownProperties === "reject")
    return schemaMatches(value, schema, components, direction, options, dynamicScope, context);
  if (
    schemaMatches(
      value,
      schema,
      components,
      direction,
      strictWireTransformOptions,
      dynamicScope,
      context,
    )
  )
    return true;
  return schemaMatches(value, schema, components, direction, options, dynamicScope, context);
}

/** Finds correlated composition branches accepted by the current value. */
export function matchingSchemasForControlFlow(
  value: unknown,
  schemas: readonly WireSchema[],
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  dynamicScope: DynamicScope,
  context: ValidationContext,
): readonly WireSchema[] {
  const strictMatches: WireSchema[] = schemas.filter((schema: WireSchema): boolean =>
    schemaMatches(
      value,
      schema,
      components,
      direction,
      strictWireTransformOptions,
      dynamicScope,
      context,
    ),
  );
  if (strictMatches.length > 0 || options.unknownProperties === "reject") return strictMatches;
  return schemas.filter((schema: WireSchema): boolean =>
    schemaMatches(value, schema, components, direction, options, dynamicScope, context),
  );
}

function schemaMatches(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  dynamicScope: DynamicScope,
  context: ValidationContext,
): boolean {
  try {
    context.execution.validate(
      value,
      schema,
      components,
      direction,
      options,
      dynamicScope,
      context,
    );
    return true;
  } catch {
    return false;
  }
}
