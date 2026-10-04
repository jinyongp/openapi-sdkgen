import type { WireSchema, WireTransformOptions, DynamicScope } from "./wire-types.js";
import type {
  ValidationContext,
  Evaluation,
  WireValidationHandlers,
  WireExecution,
} from "./wire-context.js";

/** Primitive and assertion-only programs cannot contribute annotations. */
export const emptyEvaluation: Evaluation = {
  properties: /* @__PURE__ */ new Set<string>(),
  indexes: /* @__PURE__ */ new Set<number>(),
};

/** Merges branch annotations without changing the source evaluation. */
export function mergeEvaluation(target: Evaluation, source: Evaluation): void {
  for (const name of source.properties) target.properties.add(name);
  for (const index of source.indexes) target.indexes.add(index);
}

/** Owns caches and schema identities for one complete codec call. */
export function createValidationContext(
  handlers: WireValidationHandlers,
  execution: WireExecution,
): ValidationContext {
  return {
    handlers,
    execution,
    finiteSeen: new WeakSet<object>(),
    validatedObjects: new WeakMap<object, WeakMap<WireSchema, Map<string, Evaluation>>>(),
    nextSchemaID: 1,
  };
}

function schemaIdentity(context: ValidationContext, schema: WireSchema): number {
  const identities: WeakMap<WireSchema, number> = (context.schemaIDs ??= new WeakMap<
    WireSchema,
    number
  >());
  const existing: number | undefined = identities.get(schema);
  if (existing !== undefined) return existing;
  const identity: number = context.nextSchemaID++;
  identities.set(schema, identity);
  return identity;
}

function validationCacheKey(
  context: ValidationContext,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  dynamicScope: DynamicScope,
  ignoreContentMediaType: boolean,
): string {
  const scope: string = dynamicScope
    .map((schema: WireSchema): number => schemaIdentity(context, schema))
    .join(",");
  return `${direction}:${options.unknownProperties}:${ignoreContentMediaType ? "ignore-content-media" : "content-media"}:${scope}`;
}

/** Reuses an accepted object only within its exact execution and scope policy. */
export function cachedValidation(
  context: ValidationContext,
  value: unknown,
  schema: WireSchema,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  dynamicScope: DynamicScope,
  ignoreContentMediaType: boolean,
): Evaluation | undefined {
  if (typeof value !== "object" || value === null) return undefined;
  return context.validatedObjects
    .get(value)
    ?.get(schema)
    ?.get(validationCacheKey(context, direction, options, dynamicScope, ignoreContentMediaType));
}

/** Records accepted object annotations in the current codec call. */
export function cacheValidation(
  context: ValidationContext,
  value: unknown,
  schema: WireSchema,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  dynamicScope: DynamicScope,
  ignoreContentMediaType: boolean,
  evaluation: Evaluation,
): void {
  if (typeof value !== "object" || value === null) return;
  let schemas: WeakMap<WireSchema, Map<string, Evaluation>> | undefined =
    context.validatedObjects.get(value);
  if (schemas === undefined) {
    schemas = new WeakMap<WireSchema, Map<string, Evaluation>>();
    context.validatedObjects.set(value, schemas);
  }
  let keys: Map<string, Evaluation> | undefined = schemas.get(schema);
  if (keys === undefined) {
    keys = new Map<string, Evaluation>();
    schemas.set(schema, keys);
  }
  keys.set(
    validationCacheKey(context, direction, options, dynamicScope, ignoreContentMediaType),
    evaluation,
  );
}
