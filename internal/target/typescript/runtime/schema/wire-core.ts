import { schemaMatchesForControlFlow, matchingSchemasForControlFlow } from "./wire-control-flow.js";
import { assertFiniteJSONNumbers } from "../shared/json-values.js";
import {
  createValidationContext,
  cachedValidation,
  cacheValidation,
  mergeEvaluation,
} from "./wire-state.js";
import { classifyWireProperties, mergeWireRepresentations } from "./wire-object-mapping.js";
import type { ClassifiedWireProperty } from "./wire-object-mapping.js";
import type { WireExecution } from "./wire-context.js";
import type {
  WireSchema,
  WireSchemas,
  WireProperty,
  WireCodec,
  WireTransformOptions,
  DynamicScope,
} from "./wire-types.js";
import type { ValidationContext, Evaluation, WireValidationHandlers } from "./wire-context.js";
import { defineOwnDataProperty, isRecord } from "../shared/runtime-support.js";

/** Creates a wire API sharing one validation/transform implementation with call-local state. */
export function createWireCodec(handlers: WireValidationHandlers): WireCodec {
  /** Recursively maps a value between generated property names and wire names. */
  function transformWireValue(
    value: unknown,
    schema: WireSchema,
    components: WireSchemas,
    direction: "encode" | "decode",
    options: WireTransformOptions = strictWireTransformOptions,
    dynamicScope: DynamicScope = [],
  ): unknown {
    return transformWireValueWithContext(
      value,
      schema,
      components,
      direction,
      options,
      dynamicScope,
      createValidationContext(handlers, genericWireExecution),
    );
  }

  /** Converts a validated JSON wire value into generated TypeScript property names. */
  function decodeWireValue(value: unknown, schema: WireSchema, components: WireSchemas): unknown {
    return transformWireValue(value, schema, components, "decode");
  }

  /** Converts generated TypeScript property names into validated JSON wire names. */
  function encodeWireValue(value: unknown, schema: WireSchema, components: WireSchemas): unknown {
    return transformWireValue(value, schema, components, "encode");
  }

  /** Validates a transformed wire value against its generated schema. */
  function validateWireValue(
    value: unknown,
    schema: WireSchema,
    components: WireSchemas,
    direction: "encode" | "decode",
    options: WireTransformOptions = strictWireTransformOptions,
    dynamicScope: DynamicScope = [],
  ): void {
    validateWireValueWithContext(
      value,
      schema,
      components,
      direction,
      options,
      dynamicScope,
      createValidationContext(handlers, genericWireExecution),
    );
  }
  return { transformWireValue, decodeWireValue, encodeWireValue, validateWireValue };
}

const strictWireTransformOptions: WireTransformOptions = { unknownProperties: "reject" };

// Primitive instances cannot contribute property or item annotations.
const emptyEvaluation: Evaluation = {
  properties: /* @__PURE__ */ new Set(),
  indexes: /* @__PURE__ */ new Set(),
};

/** Recursively transforms a value while reusing the current validation context. */
export function transformWireValueWithContext(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  dynamicScope: DynamicScope,
  context: ValidationContext,
  ignoreContentMediaType: boolean = schema.ignoreContentMediaType === true,
): unknown {
  const scope: DynamicScope =
    context.handlers.dynamic?.extend(dynamicScope, schema) ?? dynamicScope;
  validateWireValueWithContext(
    value,
    schema,
    components,
    direction,
    options,
    dynamicScope,
    context,
    ignoreContentMediaType,
  );
  if (value === null || value === undefined) return value;
  const dynamicTarget: WireSchema | undefined = context.handlers.dynamic?.resolve(schema, scope);
  const representations: unknown[] | undefined =
    dynamicTarget !== undefined ||
    schema.reference !== undefined ||
    context.handlers.transformComposition !== undefined
      ? []
      : undefined;
  let transformed: unknown = value;
  if (dynamicTarget !== undefined)
    representations?.push(
      transformWireValueWithContext(
        value,
        dynamicTarget,
        components,
        direction,
        options,
        scope,
        context,
        ignoreContentMediaType,
      ),
    );
  if (schema.reference !== undefined) {
    const referenced: WireSchema | undefined = components[schema.reference];
    if (referenced !== undefined)
      representations?.push(
        transformWireValueWithContext(
          value,
          referenced,
          components,
          direction,
          options,
          scope,
          context,
          ignoreContentMediaType,
        ),
      );
  }
  if (Array.isArray(transformed)) {
    transformed = transformed.map((item: unknown, index: number): unknown => {
      const itemSchema: WireSchema | undefined = schema.prefixItems?.[index] ?? schema.items;
      return itemSchema === undefined
        ? item
        : transformWireValueWithContext(
            item,
            itemSchema,
            components,
            direction,
            options,
            scope,
            context,
          );
    });
  }
  if (
    isRecord(transformed) &&
    (schema.properties !== undefined ||
      schema.patternProperties !== undefined ||
      schema.additionalProperties !== undefined)
  ) {
    const source: Record<string, unknown> = transformed;
    const result: Record<string, unknown> = {};
    let targets: Set<string> | undefined;
    for (const [key, item] of Object.entries(source)) defineOwnDataProperty(result, key, item);
    const properties: ClassifiedWireProperty[] = classifyWireProperties(
      source,
      schema,
      direction,
      context,
    );
    for (const { sourceName, targetName } of properties) {
      if (sourceName === targetName) continue;
      delete result[sourceName];
      targets ??= new Set<string>();
      targets.add(targetName);
    }
    for (const classified of properties) {
      const { sourceName, targetName }: ClassifiedWireProperty = classified;
      const item: unknown = mergeWireRepresentations(
        source[sourceName],
        classified.schemas.map((child: WireSchema): unknown =>
          transformWireValueWithContext(
            source[sourceName],
            child,
            components,
            direction,
            options,
            scope,
            context,
          ),
        ),
        context,
      );
      if (sourceName !== targetName || !targets?.has(targetName))
        defineOwnDataProperty(result, targetName, item);
    }
    if (targets !== undefined) {
      context.mappedProperties ??= new WeakMap<object, ReadonlySet<string>>();
      context.mappedProperties.set(result, targets);
    }
    transformed = result;
  }
  representations?.push(transformed);
  if (representations !== undefined)
    context.handlers.transformComposition?.(
      value,
      schema,
      components,
      direction,
      options,
      scope,
      context,
      representations,
    );
  return representations === undefined
    ? transformed
    : mergeWireRepresentations(value, representations, context);
}

/** Validates one schema node and returns its evaluated properties and indexes. */
export function validateWireValueWithContext(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  dynamicScope: DynamicScope,
  context: ValidationContext,
  ignoreContentMediaType: boolean = schema.ignoreContentMediaType === true,
): Evaluation {
  const cached: Evaluation | undefined = cachedValidation(
    context,
    value,
    schema,
    direction,
    options,
    dynamicScope,
    ignoreContentMediaType,
  );
  if (cached !== undefined) return cached;
  const evaluation: Evaluation =
    typeof value !== "object" || value === null
      ? emptyEvaluation
      : { properties: new Set(), indexes: new Set() };
  assertFiniteJSONNumbers(value, context.finiteSeen);
  const scope: DynamicScope =
    context.handlers.dynamic?.extend(dynamicScope, schema) ?? dynamicScope;
  if (schema.boolean === false) throw new TypeError("schema is false");
  if (value === undefined) return evaluation;
  const dynamicTarget: WireSchema | undefined = context.handlers.dynamic?.resolve(schema, scope);
  if (dynamicTarget !== undefined) {
    mergeEvaluation(
      evaluation,
      validateWireValueWithContext(
        value,
        dynamicTarget,
        components,
        direction,
        options,
        scope,
        context,
        ignoreContentMediaType,
      ),
    );
  }
  if (schema.reference !== undefined) {
    const referenced: WireSchema | undefined = components[schema.reference];
    if (referenced !== undefined)
      mergeEvaluation(
        evaluation,
        validateWireValueWithContext(
          value,
          referenced,
          components,
          direction,
          options,
          scope,
          context,
          ignoreContentMediaType,
        ),
      );
  }
  if (
    schema.types !== undefined &&
    !schema.types.some((type: string): boolean => valueMatchesType(value, type))
  ) {
    throw new TypeError(`expected ${schema.types.join(" | ")}`);
  }
  context.handlers.literal?.(value, schema);
  if (typeof value === "number") {
    context.handlers.multipleOf?.(value, schema);
    context.handlers.number?.(value, schema);
  }
  if (typeof value === "string") {
    context.handlers.string?.(value, schema);
    context.handlers.stringPattern?.(value, schema);
    if (
      schema.formatAssertion &&
      schema.format !== undefined &&
      context.handlers.format !== undefined &&
      !context.handlers.format(value, schema.format)
    )
      throw new TypeError(`must match format ${schema.format}`);
  }
  context.handlers.composition?.(
    value,
    schema,
    components,
    direction,
    options,
    scope,
    context,
    evaluation,
  );
  if (
    !ignoreContentMediaType &&
    context.handlers.decodeContent !== undefined &&
    schema.contentSchema !== undefined &&
    typeof value === "string"
  ) {
    validateWireValueWithContext(
      context.handlers.decodeContent(value, schema, components, ignoreContentMediaType),
      schema.contentSchema,
      components,
      direction,
      options,
      scope,
      context,
    );
  }
  if (Array.isArray(value)) {
    for (let index: number = 0; index < value.length; index++) {
      if (!Object.hasOwn(value, index)) throw new TypeError("must not contain sparse items");
    }
    context.handlers.arrayBefore?.(
      value,
      schema,
      components,
      direction,
      options,
      scope,
      context,
      evaluation,
    );
    context.handlers.arrayUnique?.(
      value,
      schema,
      components,
      direction,
      options,
      scope,
      context,
      evaluation,
    );
    context.handlers.arrayContains?.(
      value,
      schema,
      components,
      direction,
      options,
      scope,
      context,
      evaluation,
    );
    for (const [index, item] of value.entries()) {
      const itemSchema: WireSchema | undefined = schema.prefixItems?.[index] ?? schema.items;
      if (itemSchema !== undefined) {
        validateWireValueWithContext(
          item,
          itemSchema,
          components,
          direction,
          options,
          scope,
          context,
        );
        evaluation.indexes.add(index);
      }
    }
    context.handlers.arrayAfter?.(
      value,
      schema,
      components,
      direction,
      options,
      scope,
      context,
      evaluation,
    );
    cacheValidation(
      context,
      value,
      schema,
      direction,
      options,
      dynamicScope,
      ignoreContentMediaType,
      evaluation,
    );
    return evaluation;
  }
  if (!isRecord(value)) {
    cacheValidation(
      context,
      value,
      schema,
      direction,
      options,
      dynamicScope,
      ignoreContentMediaType,
      evaluation,
    );
    return evaluation;
  }
  context.handlers.objectBefore?.(
    value,
    schema,
    components,
    direction,
    options,
    scope,
    context,
    evaluation,
  );
  const properties: Readonly<Record<string, WireProperty>> = schema.properties ?? {};
  const allowed: Set<string> = new Set<string>();
  for (const [wireName, definition] of Object.entries(properties)) {
    const sourceName: string = direction === "encode" ? definition.property : wireName;
    allowed.add(sourceName);
    if (Object.hasOwn(value, sourceName)) {
      try {
        validateWireValueWithContext(
          value[sourceName],
          definition.schema,
          components,
          direction,
          options,
          scope,
          context,
        );
        evaluation.properties.add(sourceName);
      } catch (cause: unknown) {
        throw new TypeError(
          `property ${wireName}: ${cause instanceof Error ? cause.message : "invalid value"}`,
          { cause },
        );
      }
    }
  }
  for (const required of schema.required ?? []) {
    const definition: WireProperty | undefined = properties[required];
    const sourceName: string =
      direction === "encode" && definition !== undefined ? definition.property : required;
    if (!Object.hasOwn(value, sourceName) || value[sourceName] === undefined) {
      throw new TypeError(`missing required property ${required}`);
    }
  }
  context.handlers.dependencies?.(
    value,
    schema,
    components,
    direction,
    options,
    scope,
    context,
    evaluation,
  );
  const classified: ClassifiedWireProperty[] = classifyWireProperties(
    value,
    schema,
    direction,
    context,
  );
  for (const property of classified) {
    if (!property.additional) allowed.add(property.sourceName);
    for (const child of property.schemas)
      validateWireValueWithContext(
        value[property.sourceName],
        child,
        components,
        direction,
        options,
        scope,
        context,
      );
    if (property.schemas.length !== 0) evaluation.properties.add(property.sourceName);
  }
  context.handlers.propertyNames?.(
    value,
    schema,
    components,
    direction,
    options,
    scope,
    context,
    evaluation,
  );
  if (schema.additionalProperties === false && options.unknownProperties === "reject") {
    for (const key of Object.keys(value)) {
      if (!allowed.has(key)) throw new TypeError(`unexpected property ${key}`);
    }
  }
  context.handlers.objectAfter?.(
    value,
    schema,
    components,
    direction,
    options,
    scope,
    context,
    evaluation,
  );
  cacheValidation(
    context,
    value,
    schema,
    direction,
    options,
    dynamicScope,
    ignoreContentMediaType,
    evaluation,
  );
  return evaluation;
}

function valueMatchesType(value: unknown, type: string): boolean {
  switch (type) {
    case "null":
      return value === null;
    case "boolean":
      return typeof value === "boolean";
    case "string":
      return typeof value === "string";
    case "number":
      return typeof value === "number" && Number.isFinite(value);
    case "integer":
      return typeof value === "number" && Number.isInteger(value);
    case "array":
      return Array.isArray(value);
    case "object":
      return isRecord(value);
    default:
      return true;
  }
}

const genericWireExecution: WireExecution = {
  validate: validateWireValueWithContext,
  transform: transformWireValueWithContext,
  matches: schemaMatchesForControlFlow,
  matching: matchingSchemasForControlFlow,
};
export { mergeEvaluation } from "./wire-state.js";
export { classifyWireProperties } from "./wire-object-mapping.js";
export { schemaMatchesForControlFlow, matchingSchemasForControlFlow } from "./wire-control-flow.js";
