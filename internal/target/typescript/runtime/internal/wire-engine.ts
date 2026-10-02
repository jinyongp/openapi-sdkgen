import { isJSONMediaType } from "./runtime-support.js";
import { defineOwnDataProperty, isRecord } from "./runtime-support.js";

/** Host-owned encoder/decoder for one complete declared media value. */
export interface MediaCodec<Value> {
  readonly encode?: (
    value: Value,
    context: { readonly contentType: string },
  ) => BodyInit | Promise<BodyInit>;
  readonly decode?: (
    response: Response,
    context: { readonly contentType: string },
  ) => Value | Promise<Value>;
  /** Serializes one Parameter Object `content` value into its required string representation. */
  readonly encodeParameter?: (
    value: Value,
    context: { readonly contentType: string },
  ) => string | Promise<string>;
  /** Decodes a Parameter Object or response Header Object `content` string. */
  readonly decodeParameter?: (
    value: string,
    context: { readonly contentType: string },
  ) => Value | Promise<Value>;
  /** Decodes one non-streaming inbound server request for a declared custom media type. */
  readonly decodeInbound?: (
    request: Request,
    context: { readonly contentType: string },
  ) => Value | Promise<Value>;
}

/** Bounded, cancellable byte reader supplied to custom stream protocols. */
export interface StreamReader {
  /** Reads at most `maxBytes`, which must not exceed the configured frame limit. */
  read(maxBytes: number): Promise<Uint8Array | null>;
  /** Cancels the source body and releases its reader lock. */
  cancel(reason?: unknown): Promise<void>;
}

/** Context shared by stream protocols and adapters. */
export interface StreamContext {
  readonly contentType: string;
  readonly maxFrameBytes: number;
  readonly signal?: AbortSignal | undefined;
}

/** Byte framing for one sequential media protocol. */
export interface StreamProtocol<Frame> {
  decode(reader: StreamReader, context: StreamContext): AsyncIterable<Frame>;
  encode(
    frames: AsyncIterable<Frame>,
    context: StreamContext,
  ): ReadableStream<Uint8Array> | Promise<ReadableStream<Uint8Array>>;
}

/** Application-level transform layered over a stream protocol. */
export interface StreamAdapter<Frame, Item> {
  decode(frames: AsyncIterable<Frame>, context: StreamContext): AsyncIterable<Item>;
  encode(items: AsyncIterable<Item>, context: StreamContext): AsyncIterable<Frame>;
}

/** Optional protocol and application adapter for one sequential media type. */
export interface StreamCodec<Frame = unknown, Item = unknown> {
  readonly protocol?: StreamProtocol<Frame>;
  readonly adapter?: StreamAdapter<Frame, Item>;
}

/** Explicit capabilities a host transport grants to generated SDK code. */
/** Minimal recursive schema used for runtime wire-name transformation. */
export interface WireSchema {
  /** Boolean-schema acceptance. `false` rejects every value. */
  readonly boolean?: boolean;
  /** Referenced component name. */
  readonly reference?: string;
  /** Name this schema contributes to JSON Schema's dynamic scope. */
  readonly dynamicAnchor?: string;
  /** A dynamic reference plus its static fallback target. */
  readonly dynamicReference?: WireDynamicReference;
  /** Allowed JSON Schema primitive or composite types. */
  readonly types?: readonly string[];
  /** Exact permitted literal value. */
  readonly constValue?: unknown;
  /** Permitted literal values. */
  readonly enumValues?: readonly unknown[];
  readonly multipleOf?: number;
  readonly maximum?: number;
  readonly exclusiveMaximum?: number;
  readonly minimum?: number;
  readonly exclusiveMinimum?: number;
  readonly minLength?: number;
  readonly maxLength?: number;
  readonly pattern?: string;
  /** JSON Schema format annotation; asserted only when formatAssertion is true. */
  readonly format?: string;
  /** The active schema dialect requires the standard format-assertion vocabulary. */
  readonly formatAssertion?: boolean;
  readonly minItems?: number;
  readonly maxItems?: number;
  readonly uniqueItems?: boolean;
  readonly contains?: WireSchema;
  readonly minContains?: number;
  readonly maxContains?: number;
  readonly minProperties?: number;
  readonly maxProperties?: number;
  /** Object properties keyed by their JSON wire names. */
  readonly properties?: Readonly<Record<string, WireProperty>>;
  readonly patternProperties?: Readonly<Record<string, WireSchema>>;
  readonly propertyNames?: WireSchema;
  readonly dependentRequired?: Readonly<Record<string, readonly string[]>>;
  readonly dependentSchemas?: Readonly<Record<string, WireSchema>>;
  /** Homogeneous array item schema. */
  readonly items?: WireSchema;
  /** Tuple item schemas in positional order. */
  readonly prefixItems?: readonly WireSchema[];
  /** Schema for additional object properties, or false for a closed object. */
  readonly additionalProperties?: WireSchema | false;
  /** Schema for object properties left unevaluated by sibling applicators, or false to reject them. */
  readonly unevaluatedProperties?: WireSchema | false;
  /** Schema for array items left unevaluated by sibling applicators, or false to reject them. */
  readonly unevaluatedItems?: WireSchema | false;
  /** Required JSON wire property names. */
  readonly required?: readonly string[];
  /** Schemas whose transformations are applied cumulatively. */
  readonly allOf?: readonly WireSchema[];
  /** Alternative schemas considered when transforming a value. */
  readonly oneOf?: readonly WireSchema[];
  /** Alternative schemas considered when transforming a value. */
  readonly anyOf?: readonly WireSchema[];
  /** Schema that must not match. */
  readonly not?: WireSchema;
  readonly if?: WireSchema;
  readonly then?: WireSchema;
  readonly else?: WireSchema;
  /** OpenAPI discriminator dispatch metadata for polymorphic schemas. */
  readonly discriminator?: WireDiscriminator;
  /** OpenAPI XML Object serialization metadata. */
  readonly xml?: WireXML;
  /** JSON Schema content encoding applied before validating contentSchema. */
  readonly contentEncoding?: string;
  /** Media type of string content validated by contentSchema. */
  readonly contentMediaType?: string;
  /** Ignore contentMediaType for this media-root occurrence while preserving nested schema semantics. */
  readonly ignoreContentMediaType?: true;
  /** Schema applied to decoded string content without changing the outer value. */
  readonly contentSchema?: WireSchema;
}

/** Runtime representation of one lowered JSON Schema `$dynamicRef`. */
export interface WireDynamicReference {
  readonly anchor: string;
  readonly fallback: WireSchema;
}

/** OpenAPI XML Object metadata attached to a Schema Object. */
export interface WireXML {
  readonly name?: string;
  readonly namespace?: string;
  readonly prefix?: string;
  readonly attribute?: boolean;
  readonly wrapped?: boolean;
  readonly nodeType?: "element" | "attribute" | "text" | "cdata" | "none";
}

/** Maps an OpenAPI discriminator property and values to concrete schema branches. */
export interface WireDiscriminator {
  readonly property: string;
  readonly mapping?: Readonly<Record<string, WireSchema>>;
  readonly defaultMapping?: WireSchema;
}

/** Generated component schema registry keyed by OpenAPI component name. */
export type WireSchemas = Readonly<Record<string, WireSchema>>;

/** Mapping between one JSON wire name and its generated TypeScript property. */
export interface WireProperty {
  /** Generated TypeScript property name. */
  readonly property: string;
  /** Nested transformation schema for the property value. */
  readonly schema: WireSchema;
}

/** Compiler-owned sequential framing identity carried into generated runtime metadata. */
export type StreamFraming =
  | "line-delimited-json"
  | "json-sequence"
  | "sse"
  | "multipart"
  | "custom";

/** Request or response body representation understood by the runtime. */
export interface WireBodyDefinition {
  /** Exact media type, excluding parameters such as charset. */
  readonly contentType: string;
  /** Wire transformation schema for this representation. */
  readonly schema: WireSchema;
  /** Whether the Media Type Object explicitly declares a complete-content schema. */
  readonly schemaDeclared?: true;
  /** Compiler-selected raw binary representation, independent of MIME heuristics. */
  readonly binary?: true;
  /** Compiler-selected sequential framing; omitted for ordinary media. */
  readonly streamFraming?: StreamFraming;
  /** OpenAPI 3.2 schema for one streamed response item. */
  readonly itemSchema?: WireSchema;
  /** Per-property Encoding Object declarations for form request bodies. */
  readonly encoding?: readonly WireEncodingDefinition[];
  /** Positional Encoding Objects for the first parts of a multipart body. */
  readonly prefixEncoding?: readonly WireEncodingDefinition[];
  /** Positional Encoding Object applied to the remaining multipart parts. */
  readonly itemEncoding?: WireEncodingDefinition;
}

/** One OpenAPI Encoding Object declaration for a form request-body property. */
export interface WireEncodingDefinition {
  /** Form property name. Omitted for positional multipart encodings. */
  readonly name?: string;
  readonly contentType?: string;
  readonly style?: string;
  readonly explode?: boolean;
  readonly allowReserved?: boolean;
  readonly headers?: readonly WireMultipartHeaderDefinition[];
  /** Nested Encoding Objects for an embedded form or multipart representation. */
  readonly encoding?: readonly WireEncodingDefinition[];
  /** Nested positional Encoding Objects for an embedded multipart representation. */
  readonly prefixEncoding?: readonly WireEncodingDefinition[];
  /** Nested streaming positional Encoding Object for an embedded multipart representation. */
  readonly itemEncoding?: WireEncodingDefinition;
}

/** A Header Object attached to one multipart part by an Encoding Object. */
export interface WireMultipartHeaderDefinition {
  readonly name: string;
  readonly required?: boolean;
  readonly style?: string;
  readonly explode?: boolean;
  readonly contentType?: string;
  readonly schema: WireSchema;
}

/** Successful response representation understood by the runtime. */
export interface WireResponseDefinition extends WireBodyDefinition {
  /** Exact status code, `default`, or wildcard status such as `2XX`. */
  readonly status: string;
  readonly headers?: readonly WireHeaderDefinition[];
}

/** Generated response-header decoding metadata. */
export interface WireHeaderDefinition {
  readonly name: string;
  readonly property: string;
  readonly required?: boolean;
  /** Header serialization style. OpenAPI defaults this to `simple`. */
  readonly style?: string;
  /** Whether an object uses `name=value` entries instead of alternating tokens. */
  readonly explode?: boolean;
  /** The sole Header Object content media type, when content is used instead of schema. */
  readonly contentType?: string;
  readonly schema: WireSchema;
}

/** The active schema resources used to resolve dynamic references during one traversal. */
export type DynamicScope = readonly WireSchema[];

/** Controls compatibility behavior while transforming and validating wire values. */
export interface WireTransformOptions {
  /** Whether object properties outside a closed response schema are rejected or preserved. */
  readonly unknownProperties: "reject" | "preserve";
}

/** Per-invocation schema content decoding; never stored in a global mutable context. */
export type SchemaContentDecoder = (
  value: string,
  schema: WireSchema,
  components: WireSchemas,
  ignoreContentMediaType?: boolean,
) => unknown;

/** Adds a schema resource to the current dynamic anchor scope when needed. */
export function extendDynamicScope(scope: DynamicScope, schema: WireSchema): DynamicScope {
  return schema.dynamicAnchor === undefined ? scope : [...scope, schema];
}

/** Resolves a dynamic reference against its active scope and declared fallback. */
export function resolveDynamicReference(
  schema: WireSchema,
  scope: DynamicScope,
): WireSchema | undefined {
  const reference = schema.dynamicReference;
  if (reference === undefined) return undefined;
  // The outer resource is searched first. This lets a resource that overrides
  // an anchor constrain a base schema reached through a normal `$ref`.
  return (
    scope.find((candidate) => candidate.dynamicAnchor === reference.anchor) ?? reference.fallback
  );
}

/** Decodes common schema content, delegating extended media only when explicitly connected. */
export function decodeSchemaContent(
  value: string,
  schema: WireSchema,
  components: WireSchemas,
  ignoreContentMediaType = false,
  decodeExtended?: (
    value: string,
    mediaType: string,
    schema: WireSchema,
    components: WireSchemas,
  ) => unknown,
): unknown {
  let decoded = value;
  const encoding = schema.contentEncoding?.toLowerCase();
  if (encoding === "base64" || encoding === "base64url") {
    try {
      const normalized =
        encoding === "base64url" ? value.replaceAll("-", "+").replaceAll("_", "/") : value;
      decoded = new TextDecoder().decode(
        Uint8Array.from(atob(normalized), (character) => character.charCodeAt(0)),
      );
    } catch (cause) {
      throw new TypeError(`contentEncoding ${schema.contentEncoding} cannot decode the value`, {
        cause,
      });
    }
  } else if (
    encoding !== undefined &&
    encoding !== "7bit" &&
    encoding !== "8bit" &&
    encoding !== "binary"
  ) {
    throw new TypeError(`unsupported contentEncoding ${schema.contentEncoding}`);
  }
  const mediaType = ignoreContentMediaType ? undefined : schema.contentMediaType;
  if (mediaType === undefined || mediaType === "" || mediaType.toLowerCase().startsWith("text/"))
    return decoded;
  if (isJSONMediaType(mediaType)) {
    try {
      return JSON.parse(decoded);
    } catch (cause) {
      throw new TypeError(`contentMediaType ${mediaType} cannot decode JSON`, { cause });
    }
  }
  if (decodeExtended !== undefined) return decodeExtended(decoded, mediaType, schema, components);
  throw new TypeError(`unsupported contentMediaType ${mediaType}`);
}

/** Creates a wire API sharing one validation/transform implementation with call-local state. */
export function createWireCodec(decodeContent: SchemaContentDecoder) {
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
      createValidationContext(decodeContent),
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
      createValidationContext(decodeContent),
    );
  }
  return { transformWireValue, decodeWireValue, encodeWireValue, validateWireValue };
}

/** The typed wire operations required by HTTP serialization and decoding. */
export type WireCodec = ReturnType<typeof createWireCodec>;

const strictWireTransformOptions: WireTransformOptions = { unknownProperties: "reject" };

interface ValidationContext {
  readonly decodeContent: SchemaContentDecoder;
  readonly finiteSeen: WeakSet<object>;
  readonly validatedObjects: WeakMap<object, WeakMap<WireSchema, Map<string, Evaluation>>>;
  readonly schemaIDs: WeakMap<WireSchema, number>;
  nextSchemaID: number;
}

interface Evaluation {
  readonly properties: Set<string>;
  readonly indexes: Set<number>;
}

function mergeEvaluation(target: Evaluation, source: Evaluation): void {
  for (const name of source.properties) target.properties.add(name);
  for (const index of source.indexes) target.indexes.add(index);
}

function createValidationContext(decodeContent: SchemaContentDecoder): ValidationContext {
  return {
    decodeContent,
    finiteSeen: new WeakSet<object>(),
    validatedObjects: new WeakMap<object, WeakMap<WireSchema, Map<string, Evaluation>>>(),
    schemaIDs: new WeakMap<WireSchema, number>(),
    nextSchemaID: 1,
  };
}

function schemaIdentity(context: ValidationContext, schema: WireSchema): number {
  const existing = context.schemaIDs.get(schema);
  if (existing !== undefined) return existing;
  const identity = context.nextSchemaID++;
  context.schemaIDs.set(schema, identity);
  return identity;
}

function validationCacheKey(
  context: ValidationContext,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  dynamicScope: DynamicScope,
  ignoreContentMediaType: boolean,
): string {
  const scope = dynamicScope.map((schema) => schemaIdentity(context, schema)).join(",");
  return `${direction}:${options.unknownProperties}:${ignoreContentMediaType ? "ignore-content-media" : "content-media"}:${scope}`;
}

function cachedValidation(
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

function cacheValidation(
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
  let schemas = context.validatedObjects.get(value);
  if (schemas === undefined) {
    schemas = new WeakMap<WireSchema, Map<string, Evaluation>>();
    context.validatedObjects.set(value, schemas);
  }
  let keys = schemas.get(schema);
  if (keys === undefined) {
    keys = new Map<string, Evaluation>();
    schemas.set(schema, keys);
  }
  keys.set(
    validationCacheKey(context, direction, options, dynamicScope, ignoreContentMediaType),
    evaluation,
  );
}

function transformWireValueWithContext(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  dynamicScope: DynamicScope,
  context: ValidationContext,
  ignoreContentMediaType: boolean = schema.ignoreContentMediaType === true,
): unknown {
  const scope = extendDynamicScope(dynamicScope, schema);
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
  const dynamicTarget = resolveDynamicReference(schema, scope);
  let transformed: unknown = value;
  if (dynamicTarget !== undefined)
    transformed = transformWireValueWithContext(
      transformed,
      dynamicTarget,
      components,
      direction,
      options,
      scope,
      context,
      ignoreContentMediaType,
    );
  if (schema.reference !== undefined) {
    const referenced = components[schema.reference];
    if (referenced !== undefined)
      transformed = transformWireValueWithContext(
        transformed,
        referenced,
        components,
        direction,
        options,
        scope,
        context,
        ignoreContentMediaType,
      );
  }
  if (Array.isArray(transformed)) {
    return transformed.map((item, index) => {
      const itemSchema = schema.prefixItems?.[index] ?? schema.items;
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
    const source = transformed;
    const result = Object.create(null) as Record<string, unknown>;
    for (const [key, item] of Object.entries(source)) defineOwnDataProperty(result, key, item);
    for (const classified of classifyWireProperties(source, schema, direction)) {
      const { sourceName, targetName } = classified;
      let item = source[sourceName];
      for (const child of classified.schemas)
        item = transformWireValueWithContext(
          item,
          child,
          components,
          direction,
          options,
          scope,
          context,
        );
      if (sourceName !== targetName) delete result[sourceName];
      defineOwnDataProperty(result, targetName, item);
    }
    transformed = result;
  }
  for (const branch of schema.allOf ?? []) {
    transformed = transformWireValueWithContext(
      transformed,
      branch,
      components,
      direction,
      options,
      scope,
      context,
    );
  }
  if (schema.if !== undefined) {
    const branch = schemaMatchesForControlFlow(
      transformed,
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
      transformed = transformWireValueWithContext(
        transformed,
        branch,
        components,
        direction,
        options,
        scope,
        context,
      );
  }
  for (const variants of [schema.oneOf, schema.anyOf]) {
    if (variants === undefined) continue;
    const matches = matchingSchemasForControlFlow(
      transformed,
      variants,
      components,
      direction,
      options,
      scope,
      context,
    );
    const selected =
      schema.discriminator !== undefined
        ? (discriminatorVariant(transformed, schema, components, direction) ?? matches[0])
        : matches[0];
    if (selected !== undefined)
      transformed = transformWireValueWithContext(
        transformed,
        selected,
        components,
        direction,
        options,
        scope,
        context,
      );
  }
  return transformed;
}

interface ClassifiedWireProperty {
  readonly sourceName: string;
  readonly targetName: string;
  readonly wireName: string;
  readonly schemas: readonly WireSchema[];
  readonly additional: boolean;
}

/** Classifies an instance's keys within this schema object, before name mapping. */
function classifyWireProperties(
  value: Readonly<Record<string, unknown>>,
  schema: WireSchema,
  direction: "encode" | "decode",
): ClassifiedWireProperty[] {
  const declared = new Map(
    Object.entries(schema.properties ?? {}).map(
      ([wireName, definition]) =>
        [
          direction === "encode" ? definition.property : wireName,
          { wireName, definition },
        ] as const,
    ),
  );
  const patterns = Object.entries(schema.patternProperties ?? {}).map(
    ([pattern, child]) => [new RegExp(pattern, "u"), child] as const,
  );
  return Object.keys(value).map((sourceName) => {
    const property = declared.get(sourceName);
    const wireName = property?.wireName ?? sourceName;
    const schemas: WireSchema[] = property === undefined ? [] : [property.definition.schema];
    for (const [pattern, child] of patterns) if (pattern.test(wireName)) schemas.push(child);
    const additional = schemas.length === 0;
    if (
      additional &&
      schema.additionalProperties !== undefined &&
      schema.additionalProperties !== false
    )
      schemas.push(schema.additionalProperties);
    return {
      sourceName,
      wireName,
      targetName: direction === "encode" ? wireName : (property?.definition.property ?? sourceName),
      schemas,
      additional,
    };
  });
}

function validateWireValueWithContext(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  dynamicScope: DynamicScope,
  context: ValidationContext,
  ignoreContentMediaType: boolean = schema.ignoreContentMediaType === true,
): Evaluation {
  const cached = cachedValidation(
    context,
    value,
    schema,
    direction,
    options,
    dynamicScope,
    ignoreContentMediaType,
  );
  if (cached !== undefined) return cached;
  const evaluation: Evaluation = { properties: new Set(), indexes: new Set() };
  assertFiniteJSONNumbers(value, context.finiteSeen);
  const scope = extendDynamicScope(dynamicScope, schema);
  if (schema.boolean === false) throw new TypeError("schema is false");
  if (value === undefined) return evaluation;
  const dynamicTarget = resolveDynamicReference(schema, scope);
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
    const referenced = components[schema.reference];
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
  if (schema.types !== undefined && !schema.types.some((type) => valueMatchesType(value, type))) {
    throw new TypeError(`expected ${schema.types.join(" | ")}`);
  }
  if (schema.constValue !== undefined && !wireValueEquals(value, schema.constValue)) {
    throw new TypeError("value does not match const");
  }
  if (
    schema.enumValues !== undefined &&
    !schema.enumValues.some((item) => wireValueEquals(value, item))
  ) {
    throw new TypeError("value is not in enum");
  }
  if (typeof value === "number") {
    if (schema.multipleOf !== undefined && !isMultipleOf(value, schema.multipleOf))
      throw new TypeError(`must be a multiple of ${schema.multipleOf}`);
    if (schema.maximum !== undefined && value > schema.maximum)
      throw new TypeError(`must be <= ${schema.maximum}`);
    if (schema.exclusiveMaximum !== undefined && value >= schema.exclusiveMaximum)
      throw new TypeError(`must be < ${schema.exclusiveMaximum}`);
    if (schema.minimum !== undefined && value < schema.minimum)
      throw new TypeError(`must be >= ${schema.minimum}`);
    if (schema.exclusiveMinimum !== undefined && value <= schema.exclusiveMinimum)
      throw new TypeError(`must be > ${schema.exclusiveMinimum}`);
  }
  if (typeof value === "string") {
    if (schema.minLength !== undefined && [...value].length < schema.minLength)
      throw new TypeError(`must have length >= ${schema.minLength}`);
    if (schema.maxLength !== undefined && [...value].length > schema.maxLength)
      throw new TypeError(`must have length <= ${schema.maxLength}`);
    if (schema.pattern !== undefined && !new RegExp(schema.pattern, "u").test(value))
      throw new TypeError(`must match pattern ${schema.pattern}`);
    if (
      schema.formatAssertion &&
      schema.format !== undefined &&
      !matchesWireFormat(value, schema.format)
    )
      throw new TypeError(`must match format ${schema.format}`);
  }
  if (schema.oneOf !== undefined) {
    const matches = matchingSchemasForControlFlow(
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
    const matches = matchingSchemasForControlFlow(
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
    const matches = schemaMatchesForControlFlow(
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
    const branch = matches ? schema.then : schema.else;
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
  if (!ignoreContentMediaType && schema.contentSchema !== undefined && typeof value === "string") {
    validateWireValueWithContext(
      context.decodeContent(value, schema, components, ignoreContentMediaType),
      schema.contentSchema,
      components,
      direction,
      options,
      scope,
      context,
    );
  }
  if (Array.isArray(value)) {
    for (let index = 0; index < value.length; index++) {
      if (!Object.hasOwn(value, index)) throw new TypeError("must not contain sparse items");
    }
    if (schema.minItems !== undefined && value.length < schema.minItems)
      throw new TypeError(`must contain at least ${schema.minItems} items`);
    if (schema.maxItems !== undefined && value.length > schema.maxItems)
      throw new TypeError(`must contain at most ${schema.maxItems} items`);
    if (schema.uniqueItems && !hasUniqueWireValues(value))
      throw new TypeError("must contain unique items");
    if (schema.contains !== undefined) {
      const matches = value.filter((item, index) => {
        const matches = schemaMatchesForControlFlow(
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
      const minimum = schema.minContains ?? 1;
      if (matches < minimum) throw new TypeError(`must contain at least ${minimum} matching items`);
      if (schema.maxContains !== undefined && matches > schema.maxContains)
        throw new TypeError(`must contain at most ${schema.maxContains} matching items`);
    }
    for (const [index, item] of value.entries()) {
      const itemSchema = schema.prefixItems?.[index] ?? schema.items;
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
    if (schema.unevaluatedItems !== undefined) {
      for (const [index, item] of value.entries()) {
        if (evaluation.indexes.has(index)) continue;
        if (schema.unevaluatedItems === false)
          throw new TypeError(`unexpected unevaluated item ${index}`);
        validateWireValueWithContext(
          item,
          schema.unevaluatedItems,
          components,
          direction,
          options,
          scope,
          context,
        );
        evaluation.indexes.add(index);
      }
    }
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
  if (schema.minProperties !== undefined && Object.keys(value).length < schema.minProperties)
    throw new TypeError(`must contain at least ${schema.minProperties} properties`);
  if (schema.maxProperties !== undefined && Object.keys(value).length > schema.maxProperties)
    throw new TypeError(`must contain at most ${schema.maxProperties} properties`);
  const properties = schema.properties ?? {};
  const allowed = new Set<string>();
  for (const [wireName, definition] of Object.entries(properties)) {
    const sourceName = direction === "encode" ? definition.property : wireName;
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
      } catch (cause) {
        throw new TypeError(
          `property ${wireName}: ${cause instanceof Error ? cause.message : "invalid value"}`,
          { cause },
        );
      }
    }
  }
  for (const required of schema.required ?? []) {
    const definition = properties[required];
    const sourceName =
      direction === "encode" && definition !== undefined ? definition.property : required;
    if (!Object.hasOwn(value, sourceName) || value[sourceName] === undefined) {
      throw new TypeError(`missing required property ${required}`);
    }
  }
  for (const [property, required] of Object.entries(schema.dependentRequired ?? {})) {
    const sourceProperty =
      direction === "encode" && properties[property] !== undefined
        ? properties[property].property
        : property;
    if (!Object.hasOwn(value, sourceProperty) || value[sourceProperty] === undefined) continue;
    for (const dependency of required) {
      const sourceDependency =
        direction === "encode" && properties[dependency] !== undefined
          ? properties[dependency].property
          : dependency;
      if (!Object.hasOwn(value, sourceDependency) || value[sourceDependency] === undefined) {
        throw new TypeError(`property ${property} requires property ${dependency}`);
      }
    }
  }
  for (const [property, dependency] of Object.entries(schema.dependentSchemas ?? {})) {
    const sourceProperty =
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
  const classified = classifyWireProperties(value, schema, direction);
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
  if (schema.propertyNames !== undefined) {
    for (const property of classified)
      validateWireValueWithContext(
        property.wireName,
        schema.propertyNames,
        components,
        direction,
        options,
        scope,
        context,
      );
  }
  if (schema.additionalProperties === false && options.unknownProperties === "reject") {
    for (const key of Object.keys(value)) {
      if (!allowed.has(key)) throw new TypeError(`unexpected property ${key}`);
    }
  }
  if (schema.unevaluatedProperties !== undefined) {
    for (const [key, item] of Object.entries(value)) {
      if (evaluation.properties.has(key)) continue;
      if (schema.unevaluatedProperties === false) {
        if (options.unknownProperties === "reject")
          throw new TypeError(`unexpected unevaluated property ${key}`);
        continue;
      }
      validateWireValueWithContext(
        item,
        schema.unevaluatedProperties,
        components,
        direction,
        options,
        scope,
        context,
      );
      evaluation.properties.add(key);
    }
  }
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

/** Implements the standard JSON Schema 2020-12 format-assertion registry. Unknown formats remain application-defined annotations. */
function matchesWireFormat(value: string, format: string): boolean {
  switch (format.toLowerCase()) {
    case "date-time":
      return matchesWireDateTime(value);
    case "date":
      return matchesWireDate(value);
    case "time":
      return matchesWireTime(value);
    case "duration":
      return /^P(?!$)(?:\d+Y)?(?:\d+M)?(?:\d+D)?(?:T(?=\d)(?:\d+H)?(?:\d+M)?(?:\d+(?:\.\d+)?S)?)?$/i.test(
        value,
      );
    case "email":
      return /^[^\s@]+@[^\s@]+\.[^\s@]+$/u.test(value);
    case "idn-email":
      return /^[^\s@]+@[^\s@]+$/u.test(value);
    case "hostname":
      return matchesWireHostname(value);
    case "idn-hostname":
      return matchesWireIDNHostname(value);
    case "ipv4":
      return matchesWireIPv4(value);
    case "ipv6":
      return matchesWireIPv6(value);
    case "uri":
      return matchesWireURI(value, true, false);
    case "uri-reference":
      return matchesWireURI(value, false, false);
    case "iri":
      return matchesWireURI(value, true, true);
    case "iri-reference":
      return matchesWireURI(value, false, true);
    case "uuid":
      return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value);
    case "uri-template":
      return matchesWireURITemplate(value);
    case "json-pointer":
      return /^(?:\/(?:[^~/]|~[01])*)*$/u.test(value);
    case "relative-json-pointer":
      return /^(?:0|[1-9][0-9]*)(?:#|(?:\/(?:[^~/]|~[01])*)*)$/u.test(value);
    case "regex":
      try {
        new RegExp(value, "u");
        return true;
      } catch {
        return false;
      }
    default:
      return true;
  }
}

function matchesWireDate(value: string): boolean {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/u.exec(value);
  if (match === null) return false;
  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const date = new Date(Date.UTC(year, month - 1, day));
  return (
    date.getUTCFullYear() === year && date.getUTCMonth() === month - 1 && date.getUTCDate() === day
  );
}

function matchesWireTime(value: string): boolean {
  const match = /^(\d{2}):(\d{2}):(\d{2})(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/iu.exec(value);
  if (match === null) return false;
  const hour = Number(match[1]);
  const minute = Number(match[2]);
  const second = Number(match[3]);
  return hour <= 23 && minute <= 59 && second <= 60;
}

function matchesWireDateTime(value: string): boolean {
  const split = value.indexOf("T") >= 0 ? value.split("T", 2) : value.split("t", 2);
  return split.length === 2 && matchesWireDate(split[0]!) && matchesWireTime(split[1]!);
}

function matchesWireHostname(value: string): boolean {
  if (value.length === 0 || value.length > 253 || /[^\x00-\x7f]/u.test(value)) return false;
  const normalized = value.endsWith(".") ? value.slice(0, -1) : value;
  return (
    normalized.length > 0 &&
    normalized.split(".").every((label) => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/iu.test(label))
  );
}

function matchesWireIDNHostname(value: string): boolean {
  if (/\s/u.test(value) || value.length === 0) return false;
  try {
    return matchesWireHostname(new URL("http://" + value).hostname);
  } catch {
    return false;
  }
}

function matchesWireIPv4(value: string): boolean {
  const segments = value.split(".");
  return (
    segments.length === 4 &&
    segments.every((segment) => /^(?:0|[1-9][0-9]{0,2})$/u.test(segment) && Number(segment) <= 255)
  );
}

function matchesWireIPv6(value: string): boolean {
  if (!value.includes(":")) return false;
  try {
    return new URL("http://[" + value + "]").hostname.length > 0;
  } catch {
    return false;
  }
}

function matchesWireURI(value: string, absolute: boolean, allowUnicode: boolean): boolean {
  if (/[\u0000-\u001f\u007f\s]/u.test(value) || (!allowUnicode && /[^\x00-\x7f]/u.test(value)))
    return false;
  try {
    const parsed = new URL(value, "https://format.invalid/");
    return !absolute || (/^[a-z][a-z0-9+.-]*:/iu.test(value) && parsed.protocol !== "");
  } catch {
    return false;
  }
}

function matchesWireURITemplate(value: string): boolean {
  if (/[\u0000-\u001f\u007f\s]/u.test(value)) return false;
  let depth = 0;
  for (const character of value) {
    if (character === "{") depth++;
    else if (character === "}") {
      depth--;
      if (depth < 0) return false;
    }
  }
  return depth === 0;
}

function discriminatorVariant(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
): WireSchema | undefined {
  if (!isRecord(value) || schema.discriminator === undefined) return undefined;
  const property = schema.discriminator.property;
  const candidate = value[property];
  if (typeof candidate !== "string") return schema.discriminator.defaultMapping;
  return schema.discriminator.mapping?.[candidate] ?? schema.discriminator.defaultMapping;
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

function assertFiniteJSONNumbers(value: unknown, seen = new WeakSet<object>()): void {
  if (typeof value === "number") {
    if (!Number.isFinite(value)) throw new TypeError("must be a finite JSON number");
    return;
  }
  if (typeof value !== "object" || value === null || seen.has(value)) return;
  seen.add(value);
  if (Array.isArray(value)) {
    for (const item of value) assertFiniteJSONNumbers(item, seen);
    return;
  }
  for (const item of Object.values(value)) assertFiniteJSONNumbers(item, seen);
}

function wireValueEquals(left: unknown, right: unknown): boolean {
  if (typeof left === "number" && typeof right === "number") return left === right;
  if (Object.is(left, right)) return true;
  if (Array.isArray(left) && Array.isArray(right)) {
    if (left.length !== right.length) return false;
    for (let index = 0; index < left.length; index++) {
      if (Object.hasOwn(left, index) !== Object.hasOwn(right, index)) return false;
      if (Object.hasOwn(left, index) && !wireValueEquals(left[index], right[index])) return false;
    }
    return true;
  }
  if (isRecord(left) && isRecord(right)) {
    const leftKeys = Object.keys(left).sort();
    const rightKeys = Object.keys(right).sort();
    return (
      leftKeys.length === rightKeys.length &&
      leftKeys.every(
        (key, index) => key === rightKeys[index] && wireValueEquals(left[key], right[key]),
      )
    );
  }
  return false;
}

function wireValueFingerprint(value: unknown): string {
  if (value === null) return "null";
  switch (typeof value) {
    case "undefined":
      return "undefined";
    case "boolean":
      return value ? "boolean:true" : "boolean:false";
    case "number":
      return `number:${value === 0 ? "0" : String(value)}`;
    case "string":
      return `string:${JSON.stringify(value)}`;
    case "bigint":
      return `bigint:${value.toString()}`;
    case "symbol":
      return `symbol:${String(value)}`;
    case "function":
      return "function";
  }
  if (Array.isArray(value)) {
    return `array:[${value
      .map((item, index) => (Object.hasOwn(value, index) ? wireValueFingerprint(item) : "<sparse>"))
      .join(",")}]`;
  }
  if (isRecord(value)) {
    const keys = Object.keys(value).sort();
    return `object:{${keys
      .map((key) => `${JSON.stringify(key)}:${wireValueFingerprint(value[key])}`)
      .join(",")}}`;
  }
  return `object:${Object.prototype.toString.call(value)}`;
}

function hasUniqueWireValues(values: readonly unknown[]): boolean {
  const buckets = new Map<string, unknown[]>();
  for (const value of values) {
    const fingerprint = wireValueFingerprint(value);
    const bucket = buckets.get(fingerprint);
    if (bucket !== undefined) {
      if (bucket.some((previous) => wireValueEquals(previous, value))) return false;
      bucket.push(value);
    } else {
      buckets.set(fingerprint, [value]);
    }
  }
  return true;
}

function isMultipleOf(value: number, divisor: number): boolean {
  if (!Number.isFinite(value) || !Number.isFinite(divisor) || divisor <= 0) return false;
  const [numerator, numeratorScale] = decimalInteger(value);
  const [denominator, denominatorScale] = decimalInteger(divisor);
  const scale = numeratorScale - denominatorScale;
  // Number's finite decimal representation bounds this exponent to 632.
  return scale >= 0
    ? (numerator * 10n ** BigInt(scale)) % denominator === 0n
    : numerator % (denominator * 10n ** BigInt(-scale)) === 0n;
}

function decimalInteger(value: number): readonly [bigint, number] {
  const [mantissa = "0", exponent = "0"] = String(value).split("e");
  const point = mantissa.indexOf(".");
  const fractional = point < 0 ? 0 : mantissa.length - point - 1;
  return [BigInt(mantissa.replace(".", "")), Number(exponent) - fractional];
}

function schemaMatchesForControlFlow(
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

function matchingSchemasForControlFlow(
  value: unknown,
  schemas: readonly WireSchema[],
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  dynamicScope: DynamicScope,
  context: ValidationContext,
): readonly WireSchema[] {
  const strictMatches = schemas.filter((schema) =>
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
  return schemas.filter((schema) =>
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
    validateWireValueWithContext(
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

/** Wire validation and mapping for plans that do not require extended schema content media. */
export const jsonWireCodec = /* @__PURE__ */ createWireCodec(decodeSchemaContent);
