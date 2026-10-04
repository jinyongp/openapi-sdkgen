import type {
  WireSchema,
  WireSchemas,
  DynamicScope,
  WireTransformOptions,
  SchemaProgram,
  SchemaViewKind,
  ValidationContext,
  Evaluation,
} from "./wire-types.js";
import type { Mutable } from "../shared/runtime-support.js";
import { emptyEvaluation, mergeEvaluation } from "./wire-state.js";
import { mergeWireRepresentations } from "./wire-object-mapping.js";

function evaluationFor(value: unknown): Evaluation {
  return typeof value === "object" && value !== null
    ? { properties: new Set<string>(), indexes: new Set<number>() }
    : emptyEvaluation;
}
const conjunctionProgram: SchemaProgram = {
  validate(
    value: unknown,
    schema: WireSchema,
    components: WireSchemas,
    direction: "encode" | "decode",
    options: WireTransformOptions,
    scope: DynamicScope,
    context: ValidationContext,
  ): Evaluation {
    if (schema.allOf === undefined) throw new TypeError("unprepared schema conjunction");
    const evaluation: Evaluation = evaluationFor(value);
    for (const branch of schema.allOf)
      mergeEvaluation(
        evaluation,
        context.execution.validate(value, branch, components, direction, options, scope, context),
      );
    return evaluation;
  },
  transform(
    value: unknown,
    schema: WireSchema,
    components: WireSchemas,
    direction: "encode" | "decode",
    options: WireTransformOptions,
    scope: DynamicScope,
    context: ValidationContext,
  ): unknown {
    if (schema.allOf === undefined) throw new TypeError("unprepared schema conjunction");
    return mergeWireRepresentations(
      value,
      schema.allOf.map((branch: WireSchema): unknown =>
        context.execution.transform(value, branch, components, direction, options, scope, context),
      ),
      context,
    );
  },
};
const negationProgram: SchemaProgram = {
  validate(
    value: unknown,
    schema: WireSchema,
    components: WireSchemas,
    direction: "encode" | "decode",
    options: WireTransformOptions,
    scope: DynamicScope,
    context: ValidationContext,
  ): Evaluation {
    if (schema.not === undefined) throw new TypeError("unprepared schema negation");
    if (
      context.execution.matches(value, schema.not, components, direction, options, scope, context)
    )
      throw new TypeError("must not match negated schema");
    return evaluationFor(value);
  },
};
const alternativesProgram: SchemaProgram = {
  validate(
    value: unknown,
    schema: WireSchema,
    components: WireSchemas,
    direction: "encode" | "decode",
    options: WireTransformOptions,
    scope: DynamicScope,
    context: ValidationContext,
  ): Evaluation {
    if (schema.anyOf === undefined) throw new TypeError("unprepared schema alternatives");
    const matches: readonly WireSchema[] = context.execution.matching(
      value,
      schema.anyOf,
      components,
      direction,
      options,
      scope,
      context,
    );
    if (matches.length === 0) throw new TypeError("must match a schema alternative");
    const evaluation: Evaluation = evaluationFor(value);
    for (const branch of matches)
      mergeEvaluation(
        evaluation,
        context.execution.validate(value, branch, components, direction, options, scope, context),
      );
    return evaluation;
  },
  transform(
    value: unknown,
    schema: WireSchema,
    components: WireSchemas,
    direction: "encode" | "decode",
    options: WireTransformOptions,
    scope: DynamicScope,
    context: ValidationContext,
  ): unknown {
    if (schema.anyOf === undefined) throw new TypeError("unprepared schema alternatives");
    return mergeWireRepresentations(
      value,
      context.execution
        .matching(value, schema.anyOf, components, direction, options, scope, context)
        .map((branch: WireSchema): unknown =>
          context.execution.transform(
            value,
            branch,
            components,
            direction,
            options,
            scope,
            context,
          ),
        ),
      context,
    );
  },
};
const emptyProgram: SchemaProgram = {
  validate(value: unknown): Evaluation {
    if (Array.isArray(value))
      for (let index: number = 0; index < value.length; index++) {
        if (!Object.hasOwn(value, index)) throw new TypeError("must not contain sparse items");
      }
    return evaluationFor(value);
  },
  transform(value: unknown): unknown {
    return Array.isArray(value) ? value.slice() : value;
  },
};
/** Prepared unconstrained contract used when a schema projection has no child. */
export const unconstrainedSchema: WireSchema = { program: emptyProgram };

/** A derived conjunction contains only compiler-owned child contracts. */
export function conjoinSchemas(schemas: readonly WireSchema[]): WireSchema {
  return {
    allOf: schemas,
    program: {
      ...conjunctionProgram,
      views: { local: emptyProgram, inherited: emptyProgram, alternatives: emptyProgram },
    },
  };
}
/** Builds the complementary branch of a prepared conditional contract. */
export function negateSchema(schema: WireSchema): WireSchema {
  return { not: schema, program: negationProgram };
}
/** Projects correlated branches onto one child without whole-value exclusivity. */
export function alternativeSchemas(schemas: readonly WireSchema[]): WireSchema {
  return {
    anyOf: schemas,
    program: { ...alternativesProgram, views: { local: emptyProgram, alternatives: emptyProgram } },
  };
}
/** Selects a compiler-prepared representation view and its matching program. */
export function schemaView(schema: WireSchema, kind: SchemaViewKind): WireSchema {
  const result: Mutable<WireSchema> = { ...schema };
  delete result.reference;
  delete result.allOf;
  if (kind !== "alternatives") delete result.dynamicReference;
  if (kind !== "inherited") {
    delete result.oneOf;
    delete result.anyOf;
  }
  if (kind === "local") {
    delete result.if;
    delete result.then;
    delete result.else;
  }
  if (schema.program !== undefined) result.program = schema.program.views?.[kind] ?? schema.program;
  return result;
}

/** Materialization changes query metadata while executing the original contract. */
export function bindSchemaView(
  view: WireSchema,
  source: WireSchema,
  inheritedScope: DynamicScope,
): WireSchema {
  if (source.program === undefined) return view;
  const program: SchemaProgram = {
    ...(source.program.views === undefined ? {} : { views: source.program.views }),
    validate(
      value: unknown,
      _schema: WireSchema,
      components: WireSchemas,
      direction: "encode" | "decode",
      options: WireTransformOptions,
      scope: DynamicScope,
      context: ValidationContext,
      ignore?: boolean,
    ): Evaluation {
      return context.execution.validate(
        value,
        source,
        components,
        direction,
        options,
        scope.length === 0 ? inheritedScope : scope,
        context,
        ignore,
      );
    },
    transform(
      value: unknown,
      _schema: WireSchema,
      components: WireSchemas,
      direction: "encode" | "decode",
      options: WireTransformOptions,
      scope: DynamicScope,
      context: ValidationContext,
      ignore?: boolean,
    ): unknown {
      return context.execution.transform(
        value,
        source,
        components,
        direction,
        options,
        scope.length === 0 ? inheritedScope : scope,
        context,
        ignore,
      );
    },
  };
  return { ...view, program };
}
