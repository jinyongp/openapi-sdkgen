import type {
  WireSchema,
  WireSchemas,
  WireTransformOptions,
  DynamicScope,
  SchemaProgram,
  ValidationContext,
  Evaluation,
} from "./wire-types.js";
import { cachedValidation, cacheValidation } from "./wire-state.js";
import { assertFiniteJSONNumbers } from "../shared/json-values.js";

function requiredProgram(schema: WireSchema): SchemaProgram {
  if (schema.program === undefined) throw new TypeError("missing generated schema program");
  return schema.program;
}

/** Validates a prepared contract using call-local caches and finite JSON checks. */
export function validateProgram(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  scope: DynamicScope,
  context: ValidationContext,
  ignoreContentMediaType: boolean = schema.ignoreContentMediaType === true,
): Evaluation {
  const program: SchemaProgram = requiredProgram(schema);
  const cached: Evaluation | undefined = cachedValidation(
    context,
    value,
    schema,
    direction,
    options,
    scope,
    ignoreContentMediaType,
  );
  if (cached !== undefined) return cached;
  assertFiniteJSONNumbers(value, context.finiteSeen);
  const evaluation: Evaluation = program.validate(
    value,
    schema,
    components,
    direction,
    options,
    scope,
    context,
    ignoreContentMediaType,
  );
  cacheValidation(
    context,
    value,
    schema,
    direction,
    options,
    scope,
    ignoreContentMediaType,
    evaluation,
  );
  return evaluation;
}

/** Validates before invoking the contract's optional DTO transformation. */
export function transformProgram(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  scope: DynamicScope,
  context: ValidationContext,
  ignoreContentMediaType: boolean = schema.ignoreContentMediaType === true,
): unknown {
  const program: SchemaProgram = requiredProgram(schema);
  validateProgram(
    value,
    schema,
    components,
    direction,
    options,
    scope,
    context,
    ignoreContentMediaType,
  );
  if (value === null || value === undefined) return value;
  return program.transform === undefined
    ? value
    : program.transform(
        value,
        schema,
        components,
        direction,
        options,
        scope,
        context,
        ignoreContentMediaType,
      );
}
