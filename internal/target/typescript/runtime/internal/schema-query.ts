import type { WireSchema, WireSchemas } from "./wire-types.js";
import { defineOwnDataProperty } from "./runtime-support.js";
import type { Mutable } from "./runtime-support.js";

function combineWireSchemas(schemas: readonly WireSchema[]): WireSchema | undefined {
  if (schemas.length === 0) return undefined;
  return schemas.length === 1 ? schemas[0] : { allOf: schemas };
}

/** Queries the current node's accepted types without eagerly expanding recursive properties. */
export function wireSchemaTypes(
  schema: WireSchema,
  schemas: WireSchemas,
  seen: ReadonlySet<WireSchema> = new Set(),
): readonly string[] {
  if (seen.has(schema)) return [];
  const nestedSeen: Set<WireSchema> = new Set(seen);
  nestedSeen.add(schema);
  const result: Set<string> = new Set(schema.types ?? []);
  if (schema.constValue !== undefined) result.add(wireValueType(schema.constValue));
  for (const value of schema.enumValues ?? []) result.add(wireValueType(value));
  if (
    schema.prefixItems !== undefined ||
    schema.items !== undefined ||
    schema.contains !== undefined ||
    schema.minItems !== undefined ||
    schema.maxItems !== undefined ||
    schema.uniqueItems !== undefined
  )
    result.add("array");
  if (
    schema.properties !== undefined ||
    schema.patternProperties !== undefined ||
    schema.additionalProperties !== undefined ||
    schema.required !== undefined ||
    schema.minProperties !== undefined ||
    schema.maxProperties !== undefined
  )
    result.add("object");
  if (schema.dynamicReference !== undefined) {
    for (const value of wireSchemaTypes(schema.dynamicReference.fallback, schemas, nestedSeen))
      result.add(value);
  }
  if (schema.reference !== undefined && schemas[schema.reference] !== undefined) {
    for (const value of wireSchemaTypes(schemas[schema.reference]!, schemas, nestedSeen))
      result.add(value);
  }
  for (const branches of [schema.allOf, schema.oneOf, schema.anyOf, [schema.then, schema.else]]) {
    for (const branch of branches ?? []) {
      if (branch === undefined) continue;
      for (const value of wireSchemaTypes(branch, schemas, nestedSeen)) result.add(value);
    }
  }
  return [...result];
}

function wireValueType(value: unknown): string {
  if (value === null) return "null";
  if (Array.isArray(value)) return "array";
  if (typeof value === "number") return Number.isInteger(value) ? "integer" : "number";
  if (typeof value === "object") return "object";
  return typeof value;
}

/** Resolves the correlated tuple or repeated-item contract at one array index. */
export function wireArrayItemSchema(
  schema: WireSchema,
  index: number,
  schemas: WireSchemas,
  seen: ReadonlySet<WireSchema> = new Set(),
): WireSchema | undefined {
  if (seen.has(schema)) return undefined;
  const nestedSeen: Set<WireSchema> = new Set(seen);
  nestedSeen.add(schema);
  const result: WireSchema[] = [];
  const direct: WireSchema | undefined = schema.prefixItems?.[index] ?? schema.items;
  if (direct !== undefined) result.push(direct);
  if (schema.dynamicReference !== undefined) {
    const dynamic: WireSchema | undefined = wireArrayItemSchema(
      schema.dynamicReference.fallback,
      index,
      schemas,
      nestedSeen,
    );
    if (dynamic !== undefined) result.push(dynamic);
  }
  if (schema.reference !== undefined && schemas[schema.reference] !== undefined) {
    const referenced: WireSchema | undefined = wireArrayItemSchema(
      schemas[schema.reference]!,
      index,
      schemas,
      nestedSeen,
    );
    if (referenced !== undefined) result.push(referenced);
  }
  for (const branch of schema.allOf ?? []) {
    const nested: WireSchema | undefined = wireArrayItemSchema(branch, index, schemas, nestedSeen);
    if (nested !== undefined) result.push(nested);
  }
  for (const keyword of ["oneOf", "anyOf"] as const) {
    const branches: readonly WireSchema[] | undefined = schema[keyword];
    if (branches === undefined) continue;
    const nested: WireSchema[] = branches.map(
      (branch: WireSchema): WireSchema =>
        wireArrayItemSchema(branch, index, schemas, nestedSeen) ?? {},
    );
    // Exclusivity belongs to the whole object, not to this projected item.
    result.push({ anyOf: nested });
  }
  if (schema.if !== undefined) {
    result.push({
      anyOf: [
        schema.then === undefined
          ? {}
          : (wireArrayItemSchema(schema.then, index, schemas, nestedSeen) ?? {}),
        schema.else === undefined
          ? {}
          : (wireArrayItemSchema(schema.else, index, schemas, nestedSeen) ?? {}),
      ],
    });
  }
  return combineWireSchemas(result);
}

/** Combines explicit, pattern and additional-property contracts for one key. */
export function wirePropertySchema(
  schema: WireSchema,
  name: string,
  schemas: WireSchemas,
  seen: ReadonlySet<WireSchema> = new Set(),
): WireSchema | undefined {
  if (seen.has(schema)) return undefined;
  const nestedSeen: Set<WireSchema> = new Set(seen);
  nestedSeen.add(schema);
  const result: WireSchema[] = [];
  let matched: boolean = false;
  const direct: WireSchema | undefined = schema.properties?.[name]?.schema;
  if (direct !== undefined) {
    result.push(direct);
    matched = true;
  }
  for (const [pattern, candidate] of Object.entries(schema.patternProperties ?? {})) {
    if (!new RegExp(pattern, "u").test(name)) continue;
    result.push(candidate);
    matched = true;
  }
  if (
    !matched &&
    schema.additionalProperties !== undefined &&
    schema.additionalProperties !== false
  )
    result.push(schema.additionalProperties);
  if (schema.dynamicReference !== undefined) {
    const dynamic: WireSchema | undefined = wirePropertySchema(
      schema.dynamicReference.fallback,
      name,
      schemas,
      nestedSeen,
    );
    if (dynamic !== undefined) result.push(dynamic);
  }
  if (schema.reference !== undefined && schemas[schema.reference] !== undefined) {
    const referenced: WireSchema | undefined = wirePropertySchema(
      schemas[schema.reference]!,
      name,
      schemas,
      nestedSeen,
    );
    if (referenced !== undefined) result.push(referenced);
  }
  for (const branch of schema.allOf ?? []) {
    const nested: WireSchema | undefined = wirePropertySchema(branch, name, schemas, nestedSeen);
    if (nested !== undefined) result.push(nested);
  }
  for (const keyword of ["oneOf", "anyOf"] as const) {
    const branches: readonly WireSchema[] | undefined = schema[keyword];
    if (branches === undefined) continue;
    const nested: WireSchema[] = branches.map(
      (branch: WireSchema): WireSchema =>
        wirePropertySchema(branch, name, schemas, nestedSeen) ?? {},
    );
    // Whole-value validation preserves the correlation between branch fields.
    result.push({ anyOf: nested });
  }
  if (schema.if !== undefined) {
    result.push({
      anyOf: [
        schema.then === undefined
          ? {}
          : (wirePropertySchema(schema.then, name, schemas, nestedSeen) ?? {}),
        schema.else === undefined
          ? {}
          : (wirePropertySchema(schema.else, name, schemas, nestedSeen) ?? {}),
      ],
    });
  }
  return combineWireSchemas(result);
}

/** Lists declared property names across the current node's lazy schema branches. */
export function wirePropertyNames(
  schema: WireSchema,
  schemas: WireSchemas,
  seen: ReadonlySet<WireSchema> = new Set(),
): readonly string[] {
  if (seen.has(schema)) return [];
  const nestedSeen: Set<WireSchema> = new Set(seen);
  nestedSeen.add(schema);
  const result: Set<string> = new Set(Object.keys(schema.properties ?? {}));
  if (schema.dynamicReference !== undefined) {
    for (const name of wirePropertyNames(schema.dynamicReference.fallback, schemas, nestedSeen))
      result.add(name);
  }
  if (schema.reference !== undefined && schemas[schema.reference] !== undefined) {
    for (const name of wirePropertyNames(schemas[schema.reference]!, schemas, nestedSeen))
      result.add(name);
  }
  for (const keyword of ["allOf", "oneOf", "anyOf"] as const) {
    for (const branch of schema[keyword] ?? []) {
      for (const name of wirePropertyNames(branch, schemas, nestedSeen)) result.add(name);
    }
  }
  for (const branch of [schema.if, schema.then, schema.else]) {
    if (branch === undefined) continue;
    for (const name of wirePropertyNames(branch, schemas, nestedSeen)) result.add(name);
  }
  return [...result];
}

/** Validates a candidate against the complete correlated header contract. */
export type WireCandidateValidator = (value: unknown, schema: WireSchema) => void;

/** Expands only the current node; candidates are consumed lazily, children stay references. */
function* wireSchemaAlternatives(
  schema: WireSchema,
  schemas: WireSchemas,
  seen: ReadonlySet<WireSchema> = new Set(),
): Generator<WireSchema> {
  if (seen.has(schema)) {
    yield {};
    return;
  }
  const path: Set<WireSchema> = new Set(seen).add(schema);
  const own: Mutable<WireSchema> = { ...schema };
  delete own.reference;
  delete own.dynamicReference;
  delete own.allOf;
  delete own.oneOf;
  delete own.anyOf;
  delete own.if;
  delete own.then;
  delete own.else;
  const groups: (readonly WireSchema[])[] = [];
  if (schema.dynamicReference !== undefined) groups.push([schema.dynamicReference.fallback]);
  if (schema.reference !== undefined && schemas[schema.reference] !== undefined)
    groups.push([schemas[schema.reference]!]);
  for (const branch of schema.allOf ?? []) groups.push([branch]);
  for (const branches of [schema.oneOf, schema.anyOf])
    if (branches !== undefined) groups.push(branches);
  if (schema.if !== undefined)
    groups.push([
      { allOf: [schema.if, schema.then ?? {}] },
      { allOf: [{ not: schema.if }, schema.else ?? {}] },
    ]);
  function* combine(index: number, conjunction: readonly WireSchema[]): Generator<WireSchema> {
    const choices: readonly WireSchema[] | undefined = groups[index];
    if (choices === undefined) {
      yield { allOf: conjunction };
      return;
    }
    for (const choice of choices)
      for (const alternative of wireSchemaAlternatives(choice, schemas, path))
        yield* combine(index + 1, [...conjunction, alternative]);
  }
  yield* combine(0, [own]);
}

/** Coerces one complete simple-style header and validates its correlated contract. */
export function decodeSimpleWireHeader(
  value: string,
  schema: WireSchema,
  schemas: WireSchemas,
  explode: boolean,
  validate: WireCandidateValidator,
  preference: "converted" | "string" = "converted",
  diagnosticName: string = "header",
): unknown {
  const types: readonly string[] = wireSchemaTypes(schema, schemas);
  function scalar(token: string, contract: WireSchema): unknown {
    try {
      return coerceWireScalar(token, contract, schemas, validate, preference);
    } catch (cause: unknown) {
      const candidates: readonly string[] = wireSchemaTypes(contract, schemas);
      if (candidates.length === 1) {
        if (candidates[0] === "integer" && !Number.isInteger(Number(token)))
          throw new TypeError(`${diagnosticName} is not an integer`, { cause });
        if (candidates[0] === "number" && !Number.isFinite(Number(token)))
          throw new TypeError(`${diagnosticName} is not a number`, { cause });
        if (candidates[0] === "boolean" && token !== "true" && token !== "false")
          throw new TypeError(`${diagnosticName} is not a boolean`, { cause });
      }
      throw cause;
    }
  }
  if (!types.includes("object") && !types.includes("array")) return scalar(value, schema);
  const tokens: readonly string[] = value.split(",");
  function objectTokens(): Record<string, string> {
    const raw: Record<string, string> = Object.create(null) as Record<string, string>;
    if (explode) {
      for (const token of tokens) {
        const separator: number = token.indexOf("=");
        if (separator < 0) throw new TypeError("simple object header requires name=value pairs");
        defineOwnDataProperty(raw, token.slice(0, separator), token.slice(separator + 1));
      }
    } else {
      if (tokens.length % 2 !== 0)
        throw new TypeError("simple object header requires name,value pairs");
      for (let index: number = 0; index < tokens.length; index += 2)
        defineOwnDataProperty(raw, tokens[index]!, tokens[index + 1]!);
    }
    return raw;
  }
  function candidate(representation: WireSchema, array: boolean): unknown {
    if (array)
      return tokens.map((token: string, index: number): unknown =>
        scalar(token, wireArrayItemSchema(representation, index, schemas) ?? {}),
      );
    const result: Record<string, unknown> = Object.create(null) as Record<string, unknown>;
    for (const [name, token] of Object.entries(objectTokens()))
      defineOwnDataProperty(
        result,
        name,
        scalar(token, wirePropertySchema(representation, name, schemas) ?? {}),
      );
    return result;
  }
  let failure: unknown;
  function tryCandidate(representation: WireSchema): unknown {
    for (const type of ["array", "object"] as const) {
      if (!types.includes(type) || !wireSchemaTypes(representation, schemas).includes(type))
        continue;
      try {
        const result: unknown = candidate(representation, type === "array");
        validate(result, schema);
        return result;
      } catch (error: unknown) {
        failure = error;
      }
    }
    return undefined;
  }
  const direct: unknown = tryCandidate(schema);
  if (direct !== undefined) return direct;
  for (const alternative of wireSchemaAlternatives(schema, schemas)) {
    const result: unknown = tryCandidate(alternative);
    if (result !== undefined) return result;
  }
  if (
    !types.some((type: string): boolean =>
      ["string", "number", "integer", "boolean", "null"].includes(type),
    )
  )
    throw failure ?? new TypeError("header does not satisfy its schema");
  try {
    return scalar(value, schema);
  } catch (error: unknown) {
    throw failure ?? error;
  }
}

/** Preserves the caller's conversion preference; validates the whole contract. */
export function coerceWireScalar(
  value: string,
  schema: WireSchema,
  schemas: WireSchemas,
  validate: WireCandidateValidator,
  preference: "converted" | "string" = "converted",
): unknown {
  const types: readonly string[] = wireSchemaTypes(schema, schemas);
  const converted: unknown[] = [];
  if (types.includes("integer")) {
    const candidate: number = Number(value);
    if (Number.isInteger(candidate)) converted.push(candidate);
  }
  if (types.includes("number")) {
    const candidate: number = Number(value);
    if (Number.isFinite(candidate)) converted.push(candidate);
  }
  if (types.includes("boolean") && (value === "true" || value === "false"))
    converted.push(value === "true");
  const candidates: unknown[] =
    preference === "string" ? [value, ...converted] : [...converted, value];
  let failure: unknown;
  for (const candidate of candidates) {
    try {
      validate(candidate, schema);
      return candidate;
    } catch (error: unknown) {
      failure = error;
    }
  }
  throw failure;
}
