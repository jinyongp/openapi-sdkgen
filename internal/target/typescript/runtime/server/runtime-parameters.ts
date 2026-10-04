import type { MediaCodec } from "../media/media-codec-types.js";
import type { WireEncodingDefinition } from "../media/media-contract-types.js";
import type { WireSchema, WireSchemas } from "../schema/wire-types.js";
import { defineOwnDataProperty } from "../shared/runtime-support.js";
import {
  wireArrayItemSchema as inboundWireArrayItemSchema,
  wirePropertySchema as inboundWirePropertySchema,
  wireSchemaTypes as inboundWireSchemaTypes,
} from "../schema/schema-query.js";
import type {
  InboundCookies,
  InboundParameterDefinition,
  InboundParameterValues,
  InboundSchema,
  InboundSchemas,
  InboundSortValue,
  MutableInboundParameterValues,
  ServerCodecContext,
} from "./runtime-types.js";
import { parseInboundCookies } from "./runtime-shared.js";
import { resolveInboundSchema } from "./runtime-shared.js";
import { isRecord } from "../shared/runtime-support.js";
import { normalizeInboundMediaType } from "./runtime-shared.js";
import { decodeXMLBody } from "./runtime-codecs.js";
import { isInboundBinaryMedia } from "./runtime-shared.js";
import { inboundMediaCodec } from "./runtime-shared.js";
import { inboundSchemaRecord } from "./runtime-shared.js";
import { decodeInboundParameterValue } from "./runtime-shared.js";
import { requireServerHook } from "./runtime-codecs.js";
import { materializeInboundWireSchema } from "./runtime-shared.js";
import { inboundWireSchemaAlternatives } from "./runtime-shared.js";
import { schemaAcceptsType } from "./runtime-shared.js";
import { isInboundSchema } from "./runtime-shared.js";
import { mergeInboundSchemaValues } from "./runtime-shared.js";
import { decodeInboundFormContent } from "./runtime-shared.js";
import { inboundArrayItemSchema } from "./runtime-shared.js";
import { InboundRequestError } from "./runtime-errors.js";

/** Decodes declared inbound parameters before host authentication and handler execution. */
export async function decodeInboundParameters(
  codecContext: ServerCodecContext,
  request: Request,
  definitions: readonly InboundParameterDefinition[],
  schemas: InboundSchemas,
  wireSchemas: WireSchemas,
  codecs: ReadonlyMap<string, MediaCodec<unknown>> | undefined = undefined,
  pathParameters: Readonly<Record<string, string>> = {},
): Promise<InboundParameterValues> {
  const result: MutableInboundParameterValues = {
    path: Object.create(null) as Record<string, unknown>,
    query: Object.create(null) as Record<string, unknown>,
    querystring: Object.create(null) as Record<string, unknown>,
    headerParams: Object.create(null) as Record<string, unknown>,
    cookieParams: Object.create(null) as Record<string, unknown>,
  };
  const url: URL = new URL(request.url);
  const cookies: InboundCookies = parseInboundCookies(request.headers.get("cookie"));
  for (const definition of definitions) {
    let raw: unknown =
      definition.location === "path"
        ? pathParameters[definition.name]
        : definition.location === "header"
          ? request.headers.get(definition.name)
          : definition.location === "cookie"
            ? definition.style === "cookie"
              ? cookies.raw[definition.name]
              : cookies.decoded[definition.name]
            : definition.location === "querystring"
              ? url.search.slice(1)
              : url.searchParams.getAll(definition.name);
    if (
      definition.location === "query" &&
      definition.contentType === undefined &&
      definition.style === "form" &&
      Array.isArray(raw) &&
      raw.length === 1 &&
      raw[0] === "" &&
      definition.allowEmptyValue !== true
    )
      throw new InboundRequestError(
        new Response("Empty query parameter " + definition.name + " is not allowed", {
          status: 400,
        }),
      );
    if (
      definition.contentType === undefined &&
      definition.location === "query" &&
      (definition.style === "deepObject" || (definition.style === "form" && definition.explode)) &&
      inboundSchemaDescribesObject(resolveInboundSchema(definition.schema, schemas))
    )
      raw = decodeInboundQueryObject(codecContext, url, definition, schemas, wireSchemas);
    if (
      definition.contentType === undefined &&
      definition.location === "cookie" &&
      definition.style === "cookie" &&
      definition.explode &&
      inboundSchemaDescribesObject(resolveInboundSchema(definition.schema, schemas))
    )
      raw = decodeInboundCookieObject(codecContext, cookies.raw, definition, schemas, wireSchemas);
    const absent: boolean = Array.isArray(raw)
      ? raw.length === 0
      : isRecord(raw)
        ? Object.keys(raw).length === 0
        : raw === undefined || raw === null;
    if (absent) {
      if (definition.required)
        throw new InboundRequestError(
          new Response("Missing required parameter " + definition.name, { status: 400 }),
        );
      continue;
    }
    const value: unknown = await decodeInboundParameterContent(
      codecContext,
      raw,
      definition,
      schemas,
      wireSchemas,
      codecs,
    );
    try {
      codecContext.wire.validateWireValue(value, definition.wireSchema, wireSchemas, "decode");
      const section: Record<string, unknown> =
        definition.location === "header"
          ? result.headerParams
          : definition.location === "cookie"
            ? result.cookieParams
            : result[definition.location];
      defineOwnDataProperty(
        section,
        definition.property,
        decodeInboundSortValue(
          codecContext.wire.decodeWireValue(value, definition.wireSchema, wireSchemas),
          definition,
        ),
      );
    } catch (error: unknown) {
      throw new InboundRequestError(
        new Response(
          "Invalid parameter " +
            definition.name +
            ": " +
            (error instanceof Error ? error.message : "invalid value"),
          { status: 400 },
        ),
      );
    }
  }
  return result;
}

export function decodeInboundSortValue(
  value: unknown,
  definition: InboundParameterDefinition,
): unknown {
  if (definition.sort === undefined || !Array.isArray(value)) return value;
  return value.map((wire: unknown): InboundSortValue => {
    const match: [string, string] | undefined = Object.entries(definition.sort ?? {}).find(
      (entry: [string, string]): boolean => entry[1] === wire,
    );
    if (match === undefined) throw new TypeError("invalid declared sort wire value");
    const separator: number = match[0].indexOf("\u0000");
    return { field: match[0].slice(0, separator), direction: match[0].slice(separator + 1) };
  });
}

export async function decodeInboundParameterContent(
  codecContext: ServerCodecContext,
  raw: unknown,
  definition: InboundParameterDefinition,
  schemas: InboundSchemas,
  wireSchemas: WireSchemas,
  codecs: ReadonlyMap<string, MediaCodec<unknown>> | undefined,
): Promise<unknown> {
  if (isRecord(raw)) return raw;
  if (typeof raw !== "string" && !Array.isArray(raw)) return raw;
  let normalized: string | readonly string[] = raw as string | readonly string[];
  if (typeof normalized === "string" && definition.location === "path") {
    if (definition.style === "label" && normalized.startsWith("."))
      normalized = normalized.slice(1);
    if (definition.style === "matrix" && normalized.startsWith(";")) {
      const prefix: string = ";" + definition.name + "=";
      if (normalized.startsWith(prefix)) normalized = normalized.slice(prefix.length);
    }
  }
  const contentType: string = normalizeInboundMediaType(definition.contentType ?? "");
  const source: string | undefined = typeof normalized === "string" ? normalized : normalized[0];
  if (
    (contentType === "application/json" || contentType.endsWith("+json")) &&
    source !== undefined
  ) {
    try {
      return JSON.parse(source);
    } catch {
      throw new InboundRequestError(
        new Response("Invalid JSON parameter " + definition.name, { status: 400 }),
      );
    }
  }
  if (contentType.includes("xml") && source !== undefined) {
    try {
      return decodeXMLBody(
        codecContext,
        source,
        definition.schema,
        schemas,
        definition.wireSchema,
        wireSchemas,
      );
    } catch {
      throw new InboundRequestError(
        new Response("Invalid XML parameter " + definition.name, { status: 400 }),
      );
    }
  }
  if (contentType === "application/x-www-form-urlencoded" && source !== undefined)
    return decodeInboundParameterForm(
      codecContext,
      source,
      definition.schema,
      schemas,
      definition.wireSchema,
      wireSchemas,
    );
  if (
    contentType !== "" &&
    !contentType.startsWith("text/") &&
    !isInboundBinaryMedia(contentType, definition.schema)
  ) {
    const codec: MediaCodec<unknown> | undefined = inboundMediaCodec(codecs, contentType);
    if (codec?.decodeParameter === undefined)
      throw new InboundRequestError(new Response("Unsupported Media Type", { status: 415 }));
    return codec.decodeParameter(source ?? "", { contentType });
  }
  const schema: Readonly<Record<string, unknown>> = inboundSchemaRecord(
    resolveInboundSchema(definition.schema, schemas),
  );
  if (inboundSchemaDescribesObject(schema))
    return decodeInboundSerializedObject(
      codecContext,
      source,
      definition,
      schema,
      schemas,
      wireSchemas,
    );
  return decodeInboundParameterValue(
    codecContext,
    normalized,
    definition.schema,
    schemas,
    definition.wireSchema,
    wireSchemas,
  );
}

export async function decodeInboundParameterForm(
  codecContext: ServerCodecContext,
  source: string,
  schema: InboundSchema,
  schemas: InboundSchemas,
  wireSchema: WireSchema,
  wireSchemas: WireSchemas,
): Promise<unknown> {
  const parsed: URLSearchParams = new URLSearchParams(source);
  const value: Record<string, unknown> = Object.create(null) as Record<string, unknown>;
  for (const [name, entry] of parsed) {
    const previous: unknown = value[name];
    defineOwnDataProperty(
      value,
      name,
      previous === undefined
        ? entry
        : Array.isArray(previous)
          ? [...previous, entry]
          : [previous, entry],
    );
  }
  return requireServerHook(codecContext.decodeFormValue)(
    codecContext,
    value,
    schema,
    schemas,
    wireSchema,
    wireSchemas,
  );
}

/** Reconstructs an object encoded with OpenAPI simple, label, matrix, or form parameter styles. */
export function decodeInboundSerializedObject(
  codecContext: ServerCodecContext,
  source: string | undefined,
  definition: InboundParameterDefinition,
  schema: Readonly<Record<string, unknown>>,
  schemas: InboundSchemas,
  wireSchemas: WireSchemas,
): Readonly<Record<string, unknown>> {
  if (source === undefined) return Object.create(null) as Readonly<Record<string, unknown>>;
  let pairs: readonly (readonly [string, string])[];
  if (definition.style === "matrix") {
    if (definition.explode)
      pairs = source
        .split(";")
        .filter(Boolean)
        .flatMap((entry: string): readonly (readonly [string, string])[] =>
          splitInboundParameterPair(entry),
        );
    else
      pairs = splitInboundParameterTokens(
        source.startsWith(";" + definition.name + "=")
          ? source.slice(definition.name.length + 2)
          : source,
      );
  } else if (definition.style === "label") {
    const value: string = source.startsWith(".") ? source.slice(1) : source;
    pairs = definition.explode
      ? value
          .split(".")
          .flatMap((entry: string): readonly (readonly [string, string])[] =>
            splitInboundParameterPair(entry),
          )
      : splitInboundParameterTokens(value);
  } else {
    pairs = definition.explode
      ? source
          .split(",")
          .flatMap((entry: string): readonly (readonly [string, string])[] =>
            splitInboundParameterPair(entry),
          )
      : splitInboundParameterTokens(source);
  }
  const raw: Record<string, string> = Object.create(null) as Record<string, string>;
  for (const [name, value] of pairs) {
    defineOwnDataProperty(raw, name, value);
  }
  return decodeInboundParameterObjectValue(
    codecContext,
    raw,
    schema,
    schemas,
    definition.wireSchema,
    wireSchemas,
    true,
  );
}

export function splitInboundParameterPair(value: string): readonly (readonly [string, string])[] {
  const separator: number = value.indexOf("=");
  return separator < 0 ? [] : [[value.slice(0, separator), value.slice(separator + 1)]];
}

export function splitInboundParameterTokens(value: string): readonly (readonly [string, string])[] {
  const tokens: string[] = value.split(",");
  const pairs: [string, string][] = [];
  for (let index: number = 0; index + 1 < tokens.length; index += 2)
    pairs.push([tokens[index]!, tokens[index + 1]!]);
  return pairs;
}

export function decodeInboundQueryObject(
  codecContext: ServerCodecContext,
  url: URL,
  definition: InboundParameterDefinition,
  schemas: InboundSchemas,
  wireSchemas: WireSchemas,
): Readonly<Record<string, unknown>> {
  const schema: Readonly<Record<string, unknown>> = inboundSchemaRecord(
    resolveInboundSchema(definition.schema, schemas),
  );
  const properties: Record<string, unknown> = isRecord(schema["properties"])
    ? schema["properties"]
    : {};
  const raw: Record<string, string | readonly string[]> = Object.create(null) as Record<
    string,
    string | readonly string[]
  >;
  if (definition.style === "deepObject") {
    const prefix: string = definition.name + "[";
    for (const [name, value] of url.searchParams) {
      if (!name.startsWith(prefix) || !name.endsWith("]")) continue;
      const property: string = name.slice(prefix.length, -1);
      defineOwnDataProperty(raw, property, value);
    }
    return decodeInboundParameterObjectValue(
      codecContext,
      raw,
      schema,
      schemas,
      definition.wireSchema,
      wireSchemas,
      true,
    );
  }
  const names: Set<string> = new Set([...Object.keys(properties), ...url.searchParams.keys()]);
  for (const property of names) {
    const values: string[] = url.searchParams.getAll(property);
    if (values.length === 0) continue;
    defineOwnDataProperty(raw, property, values);
  }
  return decodeInboundParameterObjectValue(
    codecContext,
    raw,
    schema,
    schemas,
    definition.wireSchema,
    wireSchemas,
    false,
  );
}

/** Reconstructs an OpenAPI 3.2 cookie-style exploded object without URI decoding cookie text. */
export function decodeInboundCookieObject(
  codecContext: ServerCodecContext,
  cookies: Readonly<Record<string, string | readonly string[]>>,
  definition: InboundParameterDefinition,
  schemas: InboundSchemas,
  wireSchemas: WireSchemas,
): Readonly<Record<string, unknown>> {
  const schema: Readonly<Record<string, unknown>> = inboundSchemaRecord(
    resolveInboundSchema(definition.schema, schemas),
  );
  const properties: Record<string, unknown> = isRecord(schema["properties"])
    ? schema["properties"]
    : {};
  const raw: Record<string, string | readonly string[]> = Object.create(null) as Record<
    string,
    string | readonly string[]
  >;
  const names: Set<string> = new Set([...Object.keys(properties), ...Object.keys(cookies)]);
  for (const property of names) {
    const value: string | readonly string[] | undefined = cookies[property];
    if (value === undefined) continue;
    defineOwnDataProperty(raw, property, value);
  }
  return decodeInboundParameterObjectValue(
    codecContext,
    raw,
    schema,
    schemas,
    definition.wireSchema,
    wireSchemas,
    false,
  );
}

export function decodeInboundParameterObjectValue(
  codecContext: ServerCodecContext,
  raw: Readonly<Record<string, string | readonly string[]>>,
  schema: InboundSchema,
  schemas: InboundSchemas,
  wireSchema: WireSchema,
  wireSchemas: WireSchemas,
  preserveUnknown: boolean,
): Readonly<Record<string, unknown>> {
  wireSchema = materializeInboundWireSchema(wireSchema, wireSchemas);
  let fallback: Record<string, unknown> = Object.create(null) as Record<string, unknown>;
  for (const alternative of inboundWireSchemaAlternatives(wireSchema, wireSchemas)) {
    const result: Record<string, unknown> = Object.create(null) as Record<string, unknown>;
    for (const [name, value] of Object.entries(raw)) {
      const property: WireSchema | undefined =
        inboundWirePropertySchema(alternative, name, wireSchemas) ??
        inboundWirePropertySchema(wireSchema, name, wireSchemas);
      if (property === undefined && !preserveUnknown) continue;
      defineOwnDataProperty(
        result,
        name,
        property === undefined
          ? value
          : decodeInboundParameterValue(
              codecContext,
              value,
              schema,
              schemas,
              property,
              wireSchemas,
            ),
      );
    }
    fallback = result;
    try {
      codecContext.wire.validateWireValue(result, wireSchema, wireSchemas, "decode");
      return result;
    } catch {
      /* Try the next correlated object alternative. */
    }
  }
  return fallback;
}

export function inboundSchemaDescribesObject(schema: InboundSchema): boolean {
  const descriptor: Readonly<Record<string, unknown>> = inboundSchemaRecord(schema);
  return (
    schemaAcceptsType(descriptor["type"], "object") ||
    isRecord(descriptor["properties"]) ||
    isRecord(descriptor["patternProperties"]) ||
    isInboundSchema(descriptor["additionalProperties"])
  );
}

export function inboundPropertySchema(
  schema: Readonly<Record<string, unknown>>,
  name: string,
): InboundSchema | undefined {
  const properties: Record<string, unknown> = isRecord(schema["properties"])
    ? schema["properties"]
    : {};
  let result: InboundSchema | undefined = isInboundSchema(properties[name])
    ? properties[name]
    : undefined;
  let matched: boolean = result !== undefined;
  const patterns: Record<string, unknown> = isRecord(schema["patternProperties"])
    ? schema["patternProperties"]
    : {};
  for (const [pattern, candidate] of Object.entries(patterns)) {
    if (!isInboundSchema(candidate) || !new RegExp(pattern, "u").test(name)) continue;
    result = result === undefined ? candidate : mergeInboundSchemaValues(result, candidate);
    matched = true;
  }
  if (!matched && isInboundSchema(schema["additionalProperties"]))
    result = schema["additionalProperties"];
  return result;
}

/** Coerces form field strings using the wire schema before inbound validation. */
export async function decodeInboundFormValue(
  codecContext: ServerCodecContext,
  value: unknown,
  schema: InboundSchema | undefined,
  schemas: InboundSchemas,
  wireSchema: WireSchema | undefined = undefined,
  wireSchemas: WireSchemas = {},
  encoding: readonly WireEncodingDefinition[] | undefined = undefined,
  contentType: string | undefined = undefined,
  codecs: ReadonlyMap<string, MediaCodec<unknown>> | undefined = undefined,
): Promise<unknown> {
  if (schema === undefined) return value;
  if (wireSchema !== undefined) wireSchema = materializeInboundWireSchema(wireSchema, wireSchemas);
  const resolved: InboundSchema = resolveInboundSchema(schema, schemas);
  const descriptor: Readonly<Record<string, unknown>> = inboundSchemaRecord(resolved);
  if (value instanceof Blob) {
    const normalized: string = normalizeInboundMediaType(contentType ?? value.type);
    if (
      normalized === "application/json" ||
      normalized.endsWith("+json") ||
      normalized.includes("xml") ||
      codecs?.has(normalized) === true
    )
      return decodeInboundFormContent(
        codecContext,
        await value.text(),
        resolved,
        schemas,
        wireSchema,
        wireSchemas,
        normalized,
        codecs,
      );
    return value;
  }
  if (value instanceof ArrayBuffer || ArrayBuffer.isView(value)) return value;
  if (typeof value === "string" && contentType !== undefined) {
    const decoded: unknown = await decodeInboundFormContent(
      codecContext,
      value,
      resolved,
      schemas,
      wireSchema,
      wireSchemas,
      contentType,
      codecs,
    );
    if (decoded !== value)
      return requireServerHook(codecContext.decodeFormValue)(
        codecContext,
        decoded,
        resolved,
        schemas,
        wireSchema,
        wireSchemas,
        encoding,
        undefined,
        codecs,
      );
  }
  if (Array.isArray(value)) {
    if (wireSchema !== undefined) {
      let fallback: unknown = value;
      for (const alternative of inboundWireSchemaAlternatives(wireSchema, wireSchemas)) {
        if (!inboundWireSchemaTypes(alternative, wireSchemas).includes("array")) continue;
        const entries: unknown[] = value.flatMap((entry: unknown): unknown[] =>
          Array.isArray(entry) ? entry : [entry],
        );
        const decoded: unknown[] = await Promise.all(
          entries.map((entry: unknown, index: number): unknown => {
            const item: WireSchema | undefined = inboundWireArrayItemSchema(
              alternative,
              index,
              wireSchemas,
            );
            return item === undefined
              ? entry
              : requireServerHook(codecContext.decodeFormValue)(
                  codecContext,
                  entry,
                  {},
                  schemas,
                  item,
                  wireSchemas,
                  encoding,
                  contentType,
                  codecs,
                );
          }),
        );
        fallback = decoded;
        try {
          codecContext.wire.validateWireValue(decoded, wireSchema, wireSchemas, "decode");
          return decoded;
        } catch {
          /* Try the next correlated array alternative. */
        }
      }
      return fallback;
    }
    if (!Array.isArray(descriptor["prefixItems"]) && !isInboundSchema(descriptor["items"]))
      return value;
    const entries: unknown[] = value.flatMap((entry: unknown): unknown[] =>
      Array.isArray(entry) ? entry : [entry],
    );
    return Promise.all(
      entries.map((entry: unknown, index: number): Promise<unknown> =>
        requireServerHook(codecContext.decodeFormValue)(
          codecContext,
          entry,
          inboundArrayItemSchema(descriptor, index),
          schemas,
          wireSchema === undefined
            ? undefined
            : inboundWireArrayItemSchema(wireSchema, index, wireSchemas),
          wireSchemas,
          encoding,
          contentType,
          codecs,
        ),
      ),
    );
  }
  if (isRecord(value)) {
    let fallback: Record<string, unknown> = Object.create(null) as Record<string, unknown>;
    const alternatives: readonly (WireSchema | undefined)[] =
      wireSchema === undefined
        ? [undefined]
        : inboundWireSchemaAlternatives(wireSchema, wireSchemas);
    for (const alternative of alternatives) {
      const result: Record<string, unknown> = Object.create(null) as Record<string, unknown>;
      for (const [name, entry] of Object.entries(value)) {
        const property: InboundSchema | undefined = inboundPropertySchema(descriptor, name);
        const definition: WireEncodingDefinition | undefined = encoding?.find(
          (candidate: WireEncodingDefinition): boolean => candidate.name === name,
        );
        const wireProperty: WireSchema | undefined =
          alternative === undefined
            ? undefined
            : (inboundWirePropertySchema(alternative, name, wireSchemas) ??
              (wireSchema === undefined
                ? undefined
                : inboundWirePropertySchema(wireSchema, name, wireSchemas)));
        defineOwnDataProperty(
          result,
          name,
          wireSchema !== undefined && wireProperty === undefined
            ? entry
            : await requireServerHook(codecContext.decodeFormValue)(
                codecContext,
                entry,
                isInboundSchema(property) ? property : {},
                schemas,
                wireProperty,
                wireSchemas,
                definition?.encoding,
                definition?.contentType,
                codecs,
              ),
        );
      }
      fallback = result;
      if (wireSchema === undefined) return result;
      try {
        codecContext.wire.validateWireValue(result, wireSchema, wireSchemas, "decode");
        return result;
      } catch {
        /* Try the next correlated object alternative. */
      }
    }
    return fallback;
  }
  if (typeof value === "string")
    return decodeInboundParameterValue(
      codecContext,
      value,
      resolved,
      schemas,
      wireSchema,
      wireSchemas,
    );
  return value;
}
