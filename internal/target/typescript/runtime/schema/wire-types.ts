/** Minimal recursive schema used for runtime wire-name transformation. */
export interface WireSchema {
  /** Compiler-owned executable contract. Arbitrary-schema compatibility helpers ignore it. */
  readonly program?: SchemaProgram;
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

/** The typed wire operations required by HTTP serialization and decoding. */
export interface WireCodec {
  transformWireValue(
    value: unknown,
    schema: WireSchema,
    components: WireSchemas,
    direction: "encode" | "decode",
    options?: WireTransformOptions,
    dynamicScope?: DynamicScope,
  ): unknown;
  decodeWireValue(value: unknown, schema: WireSchema, components: WireSchemas): unknown;
  encodeWireValue(value: unknown, schema: WireSchema, components: WireSchemas): unknown;
  validateWireValue(
    value: unknown,
    schema: WireSchema,
    components: WireSchemas,
    direction: "encode" | "decode",
    options?: WireTransformOptions,
    dynamicScope?: DynamicScope,
  ): void;
}

/** Per-call validation state, caches and selected optional assertion hooks. */
export interface ValidationContext {
  readonly handlers: WireValidationHandlers;
  readonly execution: WireExecution;
  readonly finiteSeen: WeakSet<object>;
  readonly validatedObjects: WeakMap<object, WeakMap<WireSchema, Map<string, Evaluation>>>;
  schemaIDs?: WeakMap<WireSchema, number>;
  mappedProperties?: WeakMap<object, ReadonlySet<string>>;
  nextSchemaID: number;
}

/** Recursive execution ports shared by generated and compatibility codecs. */
export interface WireExecution {
  readonly validate: WireEvaluationFunction;
  readonly transform: WireTransformFunction;
  readonly matches: WireMatchFunction;
  readonly matching: WireMatchingFunction;
}
/** Validates one contract and returns its evaluated property and item names. */
export type WireEvaluationFunction = (
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  scope: DynamicScope,
  context: ValidationContext,
  ignoreContentMediaType?: boolean,
) => Evaluation;
/** Returns the accepted value in its destination representation. */
export type WireTransformFunction = (...args: Parameters<WireEvaluationFunction>) => unknown;
/** Tests one control-flow contract with the caller's execution context. */
export type WireMatchFunction = (...args: Parameters<WireEvaluationFunction>) => boolean;
/** Selects accepted correlated branches from an ordered group of contracts. */
export type WireMatchingFunction = (
  value: unknown,
  schemas: readonly WireSchema[],
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  scope: DynamicScope,
  context: ValidationContext,
) => readonly WireSchema[];

/** One statically generated node; it never interprets arbitrary schema fields. */
export interface SchemaProgram {
  readonly validate: WireEvaluationFunction;
  readonly transform?: WireTransformFunction;
  readonly views?: Readonly<Partial<Record<SchemaViewKind, SchemaProgram>>>;
}

/** Compiler-owned representation projections used by XML and inbound coercion. */
export type SchemaViewKind = "local" | "inherited" | "alternatives";

/** Properties and indexes evaluated by a successful schema branch. */
export interface Evaluation {
  readonly properties: Set<string>;
  readonly indexes: Set<number>;
}

/** Optional assertion operating within the canonical validation context. */
export type WireValidationHandler = (
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  scope: DynamicScope,
  context: ValidationContext,
  evaluation: Evaluation,
) => void;
/** Transforms the correlated composition branches that accept a value. */
export type WireCompositionTransform = (
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions,
  scope: DynamicScope,
  context: ValidationContext,
  representations: unknown[],
) => void;
/** Extends and resolves dynamic references in the caller's schema scope. */
export interface WireDynamicHandler {
  extend(scope: DynamicScope, schema: WireSchema): DynamicScope;
  resolve(schema: WireSchema, scope: DynamicScope): WireSchema | undefined;
}
/** Optional handlers supplied to the single wire validation algorithm. */
export interface WireValidationHandlers {
  readonly dynamic?: WireDynamicHandler;
  readonly decodeContent?: SchemaContentDecoder;
  readonly literal?: (value: unknown, schema: WireSchema) => void;
  readonly multipleOf?: (value: number, schema: WireSchema) => void;
  readonly number?: (value: number, schema: WireSchema) => void;
  readonly stringPattern?: (value: string, schema: WireSchema) => void;
  readonly string?: (value: string, schema: WireSchema) => void;
  readonly format?: (value: string, format: string) => boolean;
  readonly composition?: WireValidationHandler;
  readonly transformComposition?: WireCompositionTransform;
  readonly arrayBefore?: WireValidationHandler;
  readonly arrayUnique?: WireValidationHandler;
  readonly arrayContains?: WireValidationHandler;
  readonly arrayAfter?: WireValidationHandler;
  readonly objectBefore?: WireValidationHandler;
  readonly dependencies?: WireValidationHandler;
  readonly propertyNames?: WireValidationHandler;
  readonly objectAfter?: WireValidationHandler;
  readonly patterns?: (schema: WireSchema, name: string) => readonly WireSchema[];
}
