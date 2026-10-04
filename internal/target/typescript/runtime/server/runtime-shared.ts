import { defineOwnDataProperty } from "../internal/objects.js";
import type {
  MediaCodec,
  StreamCodec,
  StreamContext,
  StreamProtocol,
  StreamReader,
  WireProperty,
  WireSchema,
  WireSchemas,
} from "../internal/wire-types.js";
import {
  wireArrayItemSchema as inboundWireArrayItemSchema,
  wireSchemaTypes as inboundWireSchemaTypes,
} from "../internal/schema-query.js";
import type { Mutable } from "../internal/objects.js";
import type {
  InboundBodyOptions,
  InboundBodyPlan,
  InboundCookies,
  InboundProtocolItems,
  InboundSchema,
  InboundSchemas,
  InboundWireSchemaConjunction,
  RequiredStreamSignal,
  ServerCodecContext,
} from "./runtime-types.js";
import { decodeXMLBody } from "./runtime-codecs.js";
import { decodeInboundBuiltInStreamFrames } from "./runtime-stream.js";
import { InboundRequestError } from "./runtime-errors.js";

/** Matches a host webhook route template and returns decoded parameter values. */
export function matchInboundRoute(
  template: string | undefined,
  pathname: string,
): Readonly<Record<string, string>> | undefined {
  if (template === undefined || !template.startsWith("/")) return undefined;
  const expected: string[] = template.split("/").slice(1);
  const actual: string[] = pathname.split("/").slice(1);
  if (expected.length !== actual.length) return undefined;
  const result: Record<string, string> = Object.create(null) as Record<string, string>;
  for (let index: number = 0; index < expected.length; index++) {
    const segment: string = expected[index]!;
    const value: string = actual[index]!;
    const match: RegExpExecArray | null = /^\{([^{}\/]+)\}$/.exec(segment);
    if (match === null) {
      if (segment !== value) return undefined;
      continue;
    }
    try {
      defineOwnDataProperty(result, match[1]!, decodeURIComponent(value));
    } catch {
      return undefined;
    }
  }
  return result;
}

export function decodeInboundParameterValue(
  codecContext: ServerCodecContext,
  raw: string | readonly string[],
  schema: InboundSchema,
  schemas: InboundSchemas,
  wireSchema: WireSchema | undefined = undefined,
  wireSchemas: WireSchemas = {},
): unknown {
  if (wireSchema !== undefined) {
    wireSchema = materializeInboundWireSchema(wireSchema, wireSchemas);
    let fallback: unknown = Array.isArray(raw) ? raw[0] : raw;
    for (const alternative of inboundWireSchemaAlternatives(wireSchema, wireSchemas)) {
      const candidate: unknown = decodeInboundParameterValueForWireSchema(
        codecContext,
        raw,
        alternative,
        wireSchemas,
      );
      fallback = candidate;
      try {
        codecContext.wire.validateWireValue(candidate, wireSchema, wireSchemas, "decode");
        return candidate;
      } catch {
        /* Try the next correlated schema alternative. */
      }
    }
    return fallback;
  }
  const descriptor: Readonly<Record<string, unknown>> = inboundSchemaRecord(
    resolveInboundSchema(schema, schemas),
  );
  const values: readonly string[] = typeof raw === "string" ? [raw] : raw;
  const types: readonly string[] = inboundSchemaTypes(descriptor, schemas);
  const candidates: unknown[] = [];
  const value: string = values[0]!;
  if (types.includes("string")) candidates.push(value);
  if (types.includes("boolean") && (value === "true" || value === "false"))
    candidates.push(value === "true");
  if (types.includes("integer")) {
    const number: number = Number(value);
    if (Number.isInteger(number)) candidates.push(number);
  }
  if (types.includes("number")) {
    const number: number = Number(value);
    if (Number.isFinite(number)) candidates.push(number);
  }
  if (types.includes("array")) {
    const entries: string[] = values.flatMap((value: string): string[] => value.split(","));
    const array: unknown[] = entries.map((entry: string, index: number): unknown =>
      decodeInboundParameterValue(
        codecContext,
        entry,
        inboundArrayItemSchema(descriptor, index),
        schemas,
        wireSchema === undefined
          ? undefined
          : inboundWireArrayItemSchema(wireSchema, index, wireSchemas),
        wireSchemas,
      ),
    );
    if (values.length > 1) candidates.unshift(array);
    else candidates.push(array);
  }
  if (candidates.length === 0) candidates.push(value);
  return candidates[0];
}

export function decodeInboundParameterValueForWireSchema(
  codecContext: ServerCodecContext,
  raw: string | readonly string[],
  schema: WireSchema,
  wireSchemas: WireSchemas,
): unknown {
  const values: readonly string[] = typeof raw === "string" ? [raw] : raw;
  const value: string = values[0]!;
  const types: readonly string[] = inboundWireSchemaTypes(schema, wireSchemas);
  const candidates: unknown[] = [value];
  if (types.includes("boolean") && (value === "true" || value === "false"))
    candidates.push(value === "true");
  if (types.includes("integer")) {
    const number: number = Number(value);
    if (Number.isInteger(number)) candidates.push(number);
  }
  if (types.includes("number")) {
    const number: number = Number(value);
    if (Number.isFinite(number)) candidates.push(number);
  }
  if (types.includes("array")) {
    const entries: string[] = values.flatMap((entry: string): string[] => entry.split(","));
    const array: unknown[] = entries.map((entry: string, index: number): unknown => {
      const item: WireSchema | undefined = inboundWireArrayItemSchema(schema, index, wireSchemas);
      return item === undefined
        ? entry
        : decodeInboundParameterValue(codecContext, entry, {}, {}, item, wireSchemas);
    });
    if (values.length > 1) candidates.unshift(array);
    else candidates.push(array);
  }
  if (candidates.length === 0) candidates.push(value);
  for (const candidate of candidates) {
    try {
      codecContext.wire.validateWireValue(candidate, schema, wireSchemas, "decode");
      return candidate;
    } catch {
      /* Try the next lossless representation. */
    }
  }
  return candidates[0];
}

export function inboundSchemaTypes(
  schema: InboundSchema,
  schemas: InboundSchemas,
  seen: ReadonlySet<object> = new Set(),
): readonly string[] {
  if (!isRecord(schema) || seen.has(schema)) return [];
  const nestedSeen: Set<object> = new Set(seen);
  nestedSeen.add(schema);
  const descriptor: Readonly<Record<string, unknown>> = inboundSchemaRecord(
    resolveInboundSchema(schema, schemas),
  );
  const result: Set<string> = new Set<string>();
  const declared: unknown[] = Array.isArray(descriptor["type"])
    ? descriptor["type"]
    : [descriptor["type"]];
  for (const value of declared) if (typeof value === "string") result.add(value);
  for (const keyword of ["allOf", "oneOf", "anyOf"]) {
    const variants: unknown[] = Array.isArray(descriptor[keyword]) ? descriptor[keyword] : [];
    for (const variant of variants)
      if (isInboundSchema(variant))
        for (const value of inboundSchemaTypes(variant, schemas, nestedSeen)) result.add(value);
  }
  return [...result];
}

export function inboundArrayItemSchema(
  schema: Readonly<Record<string, unknown>>,
  index: number,
): InboundSchema {
  const prefixItems: unknown[] = Array.isArray(schema["prefixItems"]) ? schema["prefixItems"] : [];
  if (isInboundSchema(prefixItems[index])) return prefixItems[index];
  return isInboundSchema(schema["items"]) ? schema["items"] : {};
}

export function materializeInboundWireSchema(
  schema: WireSchema,
  schemas: WireSchemas,
  dynamicScope: readonly WireSchema[] = [],
  seen: ReadonlySet<WireSchema> = new Set(),
): WireSchema {
  if (seen.has(schema)) return schema;
  const nestedSeen: Set<WireSchema> = new Set(seen);
  nestedSeen.add(schema);
  const scope: readonly WireSchema[] =
    schema.dynamicAnchor === undefined ? dynamicScope : [...dynamicScope, schema];
  const result: Mutable<WireSchema> = { ...schema };
  const conjunctions: WireSchema[] = [
    ...(schema.allOf ?? []).map((branch: WireSchema): WireSchema =>
      materializeInboundWireSchema(branch, schemas, scope, nestedSeen),
    ),
  ];
  if (schema.reference !== undefined && schemas[schema.reference] !== undefined) {
    conjunctions.push(
      materializeInboundWireSchema(schemas[schema.reference]!, schemas, scope, nestedSeen),
    );
    delete result.reference;
  }
  if (schema.dynamicReference !== undefined) {
    const target: WireSchema =
      scope.find(
        (candidate: WireSchema): boolean =>
          candidate.dynamicAnchor === schema.dynamicReference!.anchor,
      ) ?? schema.dynamicReference.fallback;
    conjunctions.push(materializeInboundWireSchema(target, schemas, scope, nestedSeen));
    delete result.dynamicReference;
  }
  if (schema.properties !== undefined) {
    result.properties = Object.fromEntries(
      Object.entries(schema.properties).map(
        ([name, definition]: [string, WireProperty]): [string, WireProperty] => [
          name,
          {
            ...definition,
            schema: materializeInboundWireSchema(definition.schema, schemas, scope, nestedSeen),
          },
        ],
      ),
    );
  }
  if (schema.patternProperties !== undefined)
    result.patternProperties = Object.fromEntries(
      Object.entries(schema.patternProperties).map(
        ([pattern, value]: [string, WireSchema]): [string, WireSchema] => [
          pattern,
          materializeInboundWireSchema(value, schemas, scope, nestedSeen),
        ],
      ),
    );
  if (schema.dependentSchemas !== undefined)
    result.dependentSchemas = Object.fromEntries(
      Object.entries(schema.dependentSchemas).map(
        ([name, value]: [string, WireSchema]): [string, WireSchema] => [
          name,
          materializeInboundWireSchema(value, schemas, scope, nestedSeen),
        ],
      ),
    );
  if (schema.items !== undefined)
    result.items = materializeInboundWireSchema(schema.items, schemas, scope, nestedSeen);
  if (schema.prefixItems !== undefined)
    result.prefixItems = schema.prefixItems.map((item: WireSchema): WireSchema =>
      materializeInboundWireSchema(item, schemas, scope, nestedSeen),
    );
  if (schema.additionalProperties !== undefined && schema.additionalProperties !== false)
    result.additionalProperties = materializeInboundWireSchema(
      schema.additionalProperties,
      schemas,
      scope,
      nestedSeen,
    );
  if (schema.unevaluatedProperties !== undefined && schema.unevaluatedProperties !== false)
    result.unevaluatedProperties = materializeInboundWireSchema(
      schema.unevaluatedProperties,
      schemas,
      scope,
      nestedSeen,
    );
  if (schema.unevaluatedItems !== undefined && schema.unevaluatedItems !== false)
    result.unevaluatedItems = materializeInboundWireSchema(
      schema.unevaluatedItems,
      schemas,
      scope,
      nestedSeen,
    );
  if (schema.oneOf !== undefined)
    result.oneOf = schema.oneOf.map((branch: WireSchema): WireSchema =>
      materializeInboundWireSchema(branch, schemas, scope, nestedSeen),
    );
  if (schema.anyOf !== undefined)
    result.anyOf = schema.anyOf.map((branch: WireSchema): WireSchema =>
      materializeInboundWireSchema(branch, schemas, scope, nestedSeen),
    );
  if (schema.contains !== undefined)
    result.contains = materializeInboundWireSchema(schema.contains, schemas, scope, nestedSeen);
  if (schema.not !== undefined)
    result.not = materializeInboundWireSchema(schema.not, schemas, scope, nestedSeen);
  if (schema.if !== undefined)
    result.if = materializeInboundWireSchema(schema.if, schemas, scope, nestedSeen);
  if (schema.then !== undefined)
    result.then = materializeInboundWireSchema(schema.then, schemas, scope, nestedSeen);
  if (schema.else !== undefined)
    result.else = materializeInboundWireSchema(schema.else, schemas, scope, nestedSeen);
  if (schema.contentSchema !== undefined)
    result.contentSchema = materializeInboundWireSchema(
      schema.contentSchema,
      schemas,
      scope,
      nestedSeen,
    );
  if (conjunctions.length === 0) delete result.allOf;
  else result.allOf = conjunctions;
  return result;
}

export function inboundWireSchemaAlternatives(
  schema: WireSchema,
  schemas: WireSchemas,
  seen: ReadonlySet<WireSchema> = new Set(),
): readonly WireSchema[] {
  if (seen.has(schema)) return [{}];
  const nestedSeen: Set<WireSchema> = new Set(seen);
  nestedSeen.add(schema);
  const own: Mutable<WireSchema> = { ...schema };
  delete own.reference;
  delete own.allOf;
  delete own.oneOf;
  delete own.anyOf;
  let alternatives: WireSchema[] = [own];
  const conjunctions: (readonly WireSchema[])[] = [];
  if (schema.dynamicReference !== undefined)
    conjunctions.push(
      inboundWireSchemaAlternatives(schema.dynamicReference.fallback, schemas, nestedSeen),
    );
  if (schema.reference !== undefined && schemas[schema.reference] !== undefined)
    conjunctions.push(
      inboundWireSchemaAlternatives(schemas[schema.reference]!, schemas, nestedSeen),
    );
  for (const branch of schema.allOf ?? [])
    conjunctions.push(inboundWireSchemaAlternatives(branch, schemas, nestedSeen));
  for (const choices of [schema.oneOf, schema.anyOf]) {
    if (choices === undefined) continue;
    conjunctions.push(
      choices.flatMap((branch: WireSchema): readonly WireSchema[] =>
        inboundWireSchemaAlternatives(branch, schemas, nestedSeen),
      ),
    );
  }
  for (const choices of conjunctions) {
    alternatives = alternatives.flatMap((base: WireSchema): InboundWireSchemaConjunction[] =>
      choices.map((choice: WireSchema): InboundWireSchemaConjunction => ({
        allOf: [base, choice],
      })),
    );
  }
  return alternatives;
}

export async function decodeInboundFormContent(
  codecContext: ServerCodecContext,
  value: unknown,
  schema: InboundSchema,
  schemas: InboundSchemas,
  wireSchema: WireSchema | undefined,
  wireSchemas: WireSchemas,
  contentType: string | undefined,
  codecs: ReadonlyMap<string, MediaCodec<unknown>> | undefined,
): Promise<unknown> {
  if (typeof value !== "string" || contentType === undefined) return value;
  const normalized: string = normalizeInboundMediaType(contentType);
  if (normalized === "application/json" || normalized.endsWith("+json")) {
    try {
      return JSON.parse(value);
    } catch {
      throw new InboundRequestError(new Response("Invalid form JSON field", { status: 400 }));
    }
  }
  if (normalized.includes("xml")) {
    try {
      return decodeXMLBody(codecContext, value, schema, schemas, wireSchema, wireSchemas);
    } catch {
      throw new InboundRequestError(new Response("Invalid form XML field", { status: 400 }));
    }
  }
  const codec: MediaCodec<unknown> | undefined = codecs?.get(normalized);
  if (codec?.decodeParameter === undefined) return value;
  try {
    return await codec.decodeParameter(value, { contentType });
  } catch {
    throw new InboundRequestError(new Response("Invalid form field", { status: 400 }));
  }
}

export function parseInboundCookies(header: string | null): InboundCookies {
  const raw: Record<string, string | string[]> = Object.create(null) as Record<
    string,
    string | string[]
  >;
  const decoded: Record<string, string | string[]> = Object.create(null) as Record<
    string,
    string | string[]
  >;
  if (header === null) return { raw, decoded };
  for (const item of header.split(";")) {
    const index: number = item.indexOf("=");
    if (index < 0) continue;
    const name: string = item.slice(0, index).trim();
    if (name === "") continue;
    const value: string = item.slice(index + 1).trim();
    appendInboundCookie(raw, name, value);
    try {
      appendInboundCookie(decoded, decodeURIComponent(name), decodeURIComponent(value));
    } catch {
      appendInboundCookie(decoded, name, value);
    }
  }
  return { raw, decoded };
}

export function appendInboundCookie(
  target: Record<string, string | string[]>,
  name: string,
  value: string,
): void {
  const previous: string | string[] | undefined = target[name];
  defineOwnDataProperty(
    target,
    name,
    previous === undefined
      ? value
      : Array.isArray(previous)
        ? [...previous, value]
        : [previous, value],
  );
}

export function inboundCookieFirst(
  value: string | readonly string[] | undefined,
): string | undefined {
  return typeof value === "string" ? value : value?.[0];
}

export function inboundMediaTypeMatchScore(pattern: string, actual: string): number {
  const normalized: string = normalizeInboundMediaType(pattern);
  if (normalized === normalizeInboundMediaType(actual)) return 3;
  if (normalized.includes("*+")) return 2;
  if (normalized.includes("*")) return 1;
  return 0;
}

export function decodeInboundWireValue(
  codecContext: ServerCodecContext,
  value: unknown,
  schema: WireSchema | undefined,
  schemas: WireSchemas | undefined,
): unknown {
  if (schema === undefined || value === undefined) return value;
  try {
    return codecContext.wire.decodeWireValue(value, schema, schemas ?? {});
  } catch {
    throw new InboundRequestError(new Response("Invalid request body", { status: 400 }));
  }
}

export function validateInboundWireValue(
  codecContext: ServerCodecContext,
  value: unknown,
  schema: WireSchema | undefined,
  schemas: WireSchemas | undefined,
  label: string,
): void {
  if (schema === undefined || value === undefined) return;
  try {
    codecContext.wire.validateWireValue(value, schema, schemas ?? {}, "decode");
  } catch (error: unknown) {
    throw new InboundRequestError(
      new Response(
        "Invalid " + label + ": " + (error instanceof Error ? error.message : "invalid value"),
        { status: 400 },
      ),
    );
  }
}

export function normalizeInboundMediaCodecs(
  codecs: Readonly<Record<string, MediaCodec<unknown>>> | undefined,
): ReadonlyMap<string, MediaCodec<unknown>> {
  const result: Map<string, MediaCodec<unknown>> = new Map<string, MediaCodec<unknown>>();
  for (const [mediaType, codec] of Object.entries(codecs ?? {})) {
    const normalized: string = normalizeInboundMediaType(mediaType);
    if (normalized === "" || normalized.includes("/ ") || !normalized.includes("/"))
      throw new TypeError("invalid inbound codec media type " + mediaType);
    if (result.has(normalized))
      throw new TypeError("duplicate inbound codec media type " + mediaType);
    result.set(normalized, codec);
  }
  return result;
}

export function inboundMediaCodec(
  codecs: ReadonlyMap<string, MediaCodec<unknown>> | undefined,
  contentType: string,
): MediaCodec<unknown> | undefined {
  if (codecs === undefined) return undefined;
  return codecs.get(normalizeInboundMediaType(contentType));
}

export function resolveInboundStreamFrameBytes(value: number | undefined): number {
  const resolved: number = value ?? 1024 * 1024;
  if (!Number.isSafeInteger(resolved) || resolved <= 0)
    throw new TypeError("maxStreamFrameBytes must be a positive safe integer");
  return resolved;
}

export function normalizeInboundStreamCodecs(
  codecs: Readonly<Record<string, StreamCodec>> | undefined,
): ReadonlyMap<string, StreamCodec> {
  const result: Map<string, StreamCodec<unknown, unknown>> = new Map<string, StreamCodec>();
  for (const [mediaType, codec] of Object.entries(codecs ?? {})) {
    const normalized: string = normalizeInboundMediaType(mediaType);
    if (normalized === "" || normalized.includes("/ ") || !normalized.includes("/"))
      throw new TypeError("invalid inbound stream codec media type " + mediaType);
    if (result.has(normalized))
      throw new TypeError("duplicate inbound stream codec media type " + mediaType);
    result.set(normalized, codec);
  }
  return result;
}

export function inboundStreamCodec(
  codecs: ReadonlyMap<string, StreamCodec> | undefined,
  contentType: string,
): StreamCodec | undefined {
  return codecs?.get(normalizeInboundMediaType(contentType));
}

export async function* decodeInboundStreamBody(
  codecContext: ServerCodecContext,
  body: ReadableStream<Uint8Array>,
  rawContentType: string,
  contentType: string,
  signal: AbortSignal,
  options: InboundBodyOptions & InboundBodyPlan,
): AsyncIterable<unknown> {
  const { items }: InboundProtocolItems = decodeInboundProtocolItems(
    codecContext,
    body,
    rawContentType,
    contentType,
    signal,
    options,
  );
  let count: number = 0;
  try {
    for await (const value of items) {
      validateInboundWireValue(
        codecContext,
        value,
        options.wireSchema,
        options.wireSchemas,
        "stream item",
      );
      count++;
      yield decodeInboundWireValue(codecContext, value, options.wireSchema, options.wireSchemas);
    }
    if (options.required && count === 0)
      throw new InboundRequestError(new Response("Request body is required", { status: 400 }));
  } catch (error: unknown) {
    if (error instanceof InboundRequestError) throw error;
    throw new InboundRequestError(new Response("Invalid stream item", { status: 400 }));
  }
}

export async function decodeInboundCompleteSequentialBody(
  codecContext: ServerCodecContext,
  body: ReadableStream<Uint8Array>,
  rawContentType: string,
  contentType: string,
  signal: AbortSignal,
  options: InboundBodyOptions & InboundBodyPlan,
): Promise<unknown> {
  const { items }: InboundProtocolItems = decodeInboundProtocolItems(
    codecContext,
    body,
    rawContentType,
    contentType,
    signal,
    options,
  );
  const values: unknown[] = [];
  try {
    for await (const value of items) values.push(value);
    validateInboundWireValue(
      codecContext,
      values,
      options.wireSchema,
      options.wireSchemas,
      "request body",
    );
    return decodeInboundWireValue(codecContext, values, options.wireSchema, options.wireSchemas);
  } catch (error: unknown) {
    if (error instanceof InboundRequestError) throw error;
    throw new InboundRequestError(new Response("Invalid request body", { status: 400 }));
  }
}

export function decodeInboundProtocolItems(
  codecContext: ServerCodecContext,
  body: ReadableStream<Uint8Array>,
  rawContentType: string,
  contentType: string,
  signal: AbortSignal,
  options: InboundBodyOptions & InboundBodyPlan,
): InboundProtocolItems {
  const maxFrameBytes: number = resolveInboundStreamFrameBytes(options.maxStreamFrameBytes);
  const context: StreamContext & RequiredStreamSignal = {
    contentType: rawContentType,
    maxFrameBytes,
    signal,
  };
  const codec: StreamCodec<unknown, unknown> | undefined = inboundStreamCodec(
    options.streamCodecs,
    contentType,
  );
  const frames: AsyncIterable<unknown> =
    codec?.protocol === undefined
      ? decodeInboundBuiltInStreamFrames(codecContext, body, {
          rawContentType,
          streamFraming: options.streamFraming,
          schema: options.schema,
          schemas: options.schemas,
          complete: options.stream !== true,
          prefixEncoding: options.stream === true ? undefined : options.prefixEncoding,
          itemEncoding: options.itemEncoding,
          wireSchema: options.wireSchema,
          wireSchemas: options.wireSchemas,
          codecs: options.codecs,
          maxFrameBytes,
          signal,
        })
      : decodeInboundCustomProtocol(body, codec.protocol, context);
  return {
    items: decodeInboundStreamApplicationItems(frames, codec, context),
  };
}

export function decodeInboundStreamApplicationItems(
  frames: AsyncIterable<unknown>,
  codec: StreamCodec | undefined,
  context: StreamContext,
): AsyncIterable<unknown> {
  if (codec?.adapter !== undefined) return codec.adapter.decode(frames, context);
  return frames;
}

export async function* decodeInboundCustomProtocol(
  body: ReadableStream<Uint8Array>,
  protocol: StreamProtocol<unknown>,
  context: StreamContext & RequiredStreamSignal,
): AsyncIterable<unknown> {
  const reader: StreamReader = createInboundMediaStreamReader(
    body,
    context.maxFrameBytes,
    context.signal,
  );
  const frames: AsyncIterable<unknown> = protocol.decode(reader, context);
  const iterator: AsyncIterator<unknown, unknown, unknown> = frames[Symbol.asyncIterator]();
  try {
    while (true) {
      const next: IteratorResult<unknown, unknown> = await awaitInboundAbortable(
        Promise.resolve(iterator.next()),
        context.signal,
      );
      if (next.done) return;
      yield next.value;
    }
  } finally {
    await reader.cancel(context.signal.reason);
    if (iterator.return !== undefined) {
      const close: Promise<IteratorResult<unknown, unknown>> = Promise.resolve(iterator.return());
      if (context.signal.aborted) void close.catch((): undefined => undefined);
      else await close.catch((): undefined => undefined);
    }
  }
}

export function createInboundMediaStreamReader(
  body: ReadableStream<Uint8Array>,
  maximum: number,
  signal: AbortSignal,
): StreamReader {
  const reader: ReadableStreamDefaultReader<Uint8Array<ArrayBufferLike>> = body.getReader();
  let pending: Uint8Array<ArrayBufferLike> = new Uint8Array();
  let done: boolean = false;
  let cancelled: boolean = false;
  const cancel: (reason?: unknown) => Promise<void> = async (reason?: unknown): Promise<void> => {
    if (cancelled) return;
    cancelled = true;
    try {
      await reader.cancel(reason);
    } finally {
      reader.releaseLock();
    }
  };
  return {
    async read(maxBytes: number): Promise<Uint8Array | null> {
      if (!Number.isSafeInteger(maxBytes) || maxBytes <= 0 || maxBytes > maximum)
        throw new TypeError("stream protocol read exceeds maxStreamFrameBytes");
      while (pending.byteLength === 0 && !done) {
        const next: ReadableStreamReadResult<Uint8Array<ArrayBufferLike>> =
          await awaitInboundAbortable(reader.read(), signal);
        done = next.done;
        if (next.value !== undefined) pending = next.value;
      }
      if (pending.byteLength === 0) {
        await cancel();
        return null;
      }
      const value: Uint8Array<ArrayBuffer> = pending.slice(0, maxBytes);
      pending = pending.subarray(value.byteLength);
      return value;
    },
    cancel,
  };
}

export function awaitInboundAbortable<Value>(
  value: Promise<Value>,
  signal: AbortSignal,
): Promise<Value> {
  if (signal.aborted) {
    void value.catch((): undefined => undefined);
    return Promise.reject(signal.reason);
  }
  return new Promise(
    (
      resolve: (value: Value | PromiseLike<Value>) => void,
      reject: (reason?: unknown) => void,
    ): void => {
      const onAbort: () => void = (): void => reject(signal.reason);
      signal.addEventListener("abort", onAbort, { once: true });
      value.then(
        (result: Value): void => {
          signal.removeEventListener("abort", onAbort);
          resolve(result);
        },
        (cause: unknown): void => {
          signal.removeEventListener("abort", onAbort);
          reject(cause);
        },
      );
    },
  );
}

export async function* emptyInboundStream(): AsyncIterable<unknown> {
  return;
}

export function inboundSequenceItemSchema(
  schema: InboundSchema | undefined,
  schemas: InboundSchemas,
  index: number,
): InboundSchema | undefined {
  if (schema === undefined) return undefined;
  const resolved: InboundSchema = resolveInboundSchema(schema, schemas);
  if (typeof resolved === "boolean") return undefined;
  const descriptor: Readonly<Record<string, unknown>> = inboundSchemaRecord(resolved);
  const prefixItems: unknown[] = Array.isArray(descriptor["prefixItems"])
    ? descriptor["prefixItems"]
    : [];
  const candidate: unknown = prefixItems[index] ?? descriptor["items"];
  return isInboundSchema(candidate) ? resolveInboundSchema(candidate, schemas) : undefined;
}

export function isInboundBinaryMedia(
  contentType: string,
  schema: InboundSchema | undefined,
): boolean {
  const descriptor: Readonly<Record<string, unknown>> = inboundSchemaRecord(schema);
  return (
    contentType === "application/octet-stream" ||
    contentType.startsWith("image/") ||
    contentType.startsWith("audio/") ||
    contentType.startsWith("video/") ||
    descriptor["format"] === "binary" ||
    descriptor["contentEncoding"] === "binary"
  );
}

export function inboundMediaTypeMatches(expected: string, actual: string): boolean {
  const normalized: string = expected.toLowerCase();
  if (normalized === actual || (normalized.endsWith("+json") && actual.endsWith("+json")))
    return true;
  const [expectedType, expectedSubtype]: string[] = normalized.split("/", 2);
  const [actualType, actualSubtype]: string[] = actual.split("/", 2);
  if (
    expectedType === undefined ||
    expectedSubtype === undefined ||
    actualType === undefined ||
    actualSubtype === undefined
  )
    return false;
  if (expectedType !== "*" && expectedType !== actualType) return false;
  if (expectedSubtype === "*") return true;
  if (expectedSubtype.startsWith("*+") && actualSubtype.endsWith(expectedSubtype.slice(1)))
    return true;
  return false;
}

export function resolveInboundSchema(
  schema: InboundSchema,
  schemas: InboundSchemas,
  resolving: ReadonlySet<string> = new Set(),
): InboundSchema {
  if (typeof schema === "boolean") return schema;
  const descriptor: Readonly<Record<string, unknown>> = inboundSchemaRecord(schema);
  const reference: string | undefined =
    typeof descriptor["$ref"] === "string" ? descriptor["$ref"] : undefined;
  const name: string | undefined =
    reference === undefined ? undefined : inboundComponentReferenceName(reference);
  let resolved: InboundSchema = schema;
  if (name !== undefined && !resolving.has(name)) {
    const target: InboundSchema | undefined = schemas[name];
    if (target !== undefined) {
      const nestedResolving: Set<string> = new Set(resolving);
      nestedResolving.add(name);
      const base: InboundSchema = resolveInboundSchema(target, schemas, nestedResolving);
      if (base === false) return false;
      const siblings: Record<string, unknown> = Object.fromEntries(
        Object.entries(descriptor).filter(([key]: [string, unknown]): boolean => key !== "$ref"),
      );
      resolved =
        base === true ? siblings : mergeInboundSchemaRecords(inboundSchemaRecord(base), siblings);
    }
  }
  if (typeof resolved === "boolean") return resolved;
  let effective: Readonly<Record<string, unknown>> = inboundSchemaRecord(resolved);
  for (const part of Array.isArray(effective["allOf"]) ? effective["allOf"] : []) {
    if (!isInboundSchema(part)) continue;
    const nested: InboundSchema = resolveInboundSchema(part, schemas, resolving);
    if (nested === false) return false;
    if (nested !== true)
      effective = mergeInboundSchemaRecords(effective, inboundSchemaRecord(nested));
  }
  return effective;
}

export function mergeInboundSchemaRecords(
  left: Readonly<Record<string, unknown>>,
  right: Readonly<Record<string, unknown>>,
): Readonly<Record<string, unknown>> {
  const merged: Record<string, unknown> = { ...left, ...right };
  if (isRecord(left["properties"]) || isRecord(right["properties"])) {
    const leftProperties: Record<string, unknown> = isRecord(left["properties"])
      ? left["properties"]
      : {};
    const rightProperties: Record<string, unknown> = isRecord(right["properties"])
      ? right["properties"]
      : {};
    const properties: Record<string, unknown> = { ...leftProperties, ...rightProperties };
    for (const name of Object.keys(properties)) {
      const leftProperty: unknown = leftProperties[name];
      const rightProperty: unknown = rightProperties[name];
      if (isInboundSchema(leftProperty) && isInboundSchema(rightProperty))
        properties[name] = mergeInboundSchemaValues(leftProperty, rightProperty);
    }
    merged["properties"] = properties;
  }
  if (isInboundSchema(left["items"]) && isInboundSchema(right["items"]))
    merged["items"] = mergeInboundSchemaValues(left["items"], right["items"]);
  const leftAllOf: unknown[] = Array.isArray(left["allOf"]) ? left["allOf"] : [];
  const rightAllOf: unknown[] = Array.isArray(right["allOf"]) ? right["allOf"] : [];
  const conjunctions: unknown[] = [...leftAllOf, ...rightAllOf];
  for (const keyword of ["oneOf", "anyOf"]) {
    const leftVariants: unknown[] = Array.isArray(left[keyword]) ? left[keyword] : [];
    const rightVariants: unknown[] = Array.isArray(right[keyword]) ? right[keyword] : [];
    if (leftVariants.length !== 0 && rightVariants.length !== 0)
      conjunctions.push({ [keyword]: leftVariants });
  }
  if (conjunctions.length !== 0) merged["allOf"] = conjunctions;
  const leftPrefixItems: unknown = left["prefixItems"];
  const rightPrefixItems: unknown = right["prefixItems"];
  if (Array.isArray(leftPrefixItems) && Array.isArray(rightPrefixItems)) {
    const maximum: number = Math.max(leftPrefixItems.length, rightPrefixItems.length);
    merged["prefixItems"] = Array.from(
      { length: maximum },
      (_: unknown, index: number): unknown => {
        const leftItem: unknown = leftPrefixItems[index];
        const rightItem: unknown = rightPrefixItems[index];
        return isInboundSchema(leftItem) && isInboundSchema(rightItem)
          ? mergeInboundSchemaValues(leftItem, rightItem)
          : (rightItem ?? leftItem);
      },
    );
  }
  return merged;
}

export function mergeInboundSchemaValues(left: InboundSchema, right: InboundSchema): InboundSchema {
  if (left === false || right === false) return false;
  if (left === true) return right;
  if (right === true) return left;
  return mergeInboundSchemaRecords(left, right);
}

export function inboundComponentReferenceName(reference: string): string | undefined {
  if (!reference.startsWith("#")) return undefined;
  let pointer: string;
  try {
    pointer = decodeURIComponent(reference.slice(1));
  } catch {
    return undefined;
  }
  const prefix: "/components/schemas/" = "/components/schemas/";
  if (!pointer.startsWith(prefix)) return undefined;
  const token: string = pointer.slice(prefix.length);
  if (token === "" || token.includes("/")) return undefined;
  return token.replaceAll("~1", "/").replaceAll("~0", "~");
}

export function schemaAcceptsType(type: unknown, wanted: string): boolean {
  return Array.isArray(type) ? type.includes(wanted) : type === wanted;
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function isInboundSchema(value: unknown): value is InboundSchema {
  return typeof value === "boolean" || isRecord(value);
}

export function inboundSchemaRecord(
  value: InboundSchema | undefined,
): Readonly<Record<string, unknown>> {
  return isRecord(value) ? value : {};
}

export function normalizeInboundMediaType(value: string): string {
  return value.split(";", 1)[0]!.trim().toLowerCase();
}

export function inboundResponseStatusMatches(declared: string, actual: number): boolean {
  if (declared === "default") return true;
  if (/^[1-5][0-9][0-9]$/.test(declared)) return Number(declared) === actual;
  if (/^[1-5][Xx][Xx]$/.test(declared)) return Number(declared[0]) === Math.floor(actual / 100);
  return false;
}
