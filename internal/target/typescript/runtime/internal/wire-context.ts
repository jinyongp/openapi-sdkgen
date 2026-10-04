import type {
  WireSchema,
  WireSchemas,
  WireTransformOptions,
  DynamicScope,
  SchemaContentDecoder,
} from "./wire-types.js";
/** Per-call validation state, caches and selected optional assertion hooks. */
export interface ValidationContext {
  readonly handlers: WireValidationHandlers;
  readonly finiteSeen: WeakSet<object>;
  readonly validatedObjects: WeakMap<object, WeakMap<WireSchema, Map<string, Evaluation>>>;
  readonly schemaIDs: WeakMap<WireSchema, number>;
  mappedProperties?: WeakMap<object, ReadonlySet<string>>;
  nextSchemaID: number;
}

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
