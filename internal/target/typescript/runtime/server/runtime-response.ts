import type {
  MediaCodec,
  WireHeaderDefinition,
  WireSchema,
  WireSchemas,
} from "../internal/wire-types.js";
import { defineOwnDataProperty } from "../internal/objects.js";
import { decodeSimpleWireHeader } from "../internal/schema-query.js";
import type {
  InboundResponse,
  InboundResponseDefinition,
  InboundResponseOptions,
  ServerCodecContext,
} from "./runtime-types.js";
import { inboundResponseStatusMatches } from "./runtime-shared.js";
import { normalizeInboundMediaType } from "./runtime-shared.js";
import { inboundMediaTypeMatches } from "./runtime-shared.js";
import { inboundMediaTypeMatchScore } from "./runtime-shared.js";
import { requireServerXML } from "./runtime-codecs.js";
import { inboundMediaCodec } from "./runtime-shared.js";
import { isRecord } from "./runtime-shared.js";

export function assertInboundJSONSerializable(
  value: unknown,
  active: WeakSet<object> = new WeakSet(),
): void {
  if (value === null || typeof value === "string" || typeof value === "boolean") return;
  if (typeof value === "number") {
    if (!Number.isFinite(value)) throw new TypeError("JSON response numbers must be finite");
    return;
  }
  if (typeof value !== "object")
    throw new TypeError("JSON response contains a non-serializable value");
  if (active.has(value)) throw new TypeError("JSON response contains a cycle");
  active.add(value);
  if (Array.isArray(value)) {
    if (Object.getPrototypeOf(value) !== Array.prototype)
      throw new TypeError("JSON response arrays must use the standard array prototype");
    const keys: string[] = Object.keys(value);
    if (
      keys.length !== value.length ||
      keys.some((key: string, index: number): boolean => key !== String(index))
    )
      throw new TypeError("JSON response array has non-index properties");
    const names: string[] = Object.getOwnPropertyNames(value);
    const descriptors: Readonly<Record<string, PropertyDescriptor>> =
      Object.getOwnPropertyDescriptors(value) as Readonly<Record<string, PropertyDescriptor>>;
    const descriptorValues: readonly PropertyDescriptor[] = Object.values(
      descriptors,
    ) as readonly PropertyDescriptor[];
    if (
      names.length !== value.length + 1 ||
      names.some((name: string): boolean => name !== "length" && !/^(0|[1-9][0-9]*)$/.test(name)) ||
      descriptorValues.some(
        (descriptor: PropertyDescriptor): boolean => !Object.hasOwn(descriptor, "value"),
      ) ||
      Object.getOwnPropertySymbols(value).length !== 0
    )
      throw new TypeError("JSON response contains non-JSON array properties");
    for (let index: number = 0; index < value.length; index++) {
      if (!Object.hasOwn(value, index))
        throw new TypeError("JSON response contains a sparse array");
      assertInboundJSONSerializable(descriptors[String(index)]!.value, active);
    }
  } else {
    const prototype: unknown = Object.getPrototypeOf(value);
    if (prototype !== Object.prototype && prototype !== null)
      throw new TypeError("JSON response objects must be plain records");
    const names: string[] = Object.getOwnPropertyNames(value);
    const descriptors: Readonly<Record<string, PropertyDescriptor>> =
      Object.getOwnPropertyDescriptors(value) as Readonly<Record<string, PropertyDescriptor>>;
    const descriptorValues: readonly PropertyDescriptor[] = Object.values(
      descriptors,
    ) as readonly PropertyDescriptor[];
    if (
      Object.getOwnPropertySymbols(value).length !== 0 ||
      names.length !== Object.keys(value).length ||
      descriptorValues.some(
        (descriptor: PropertyDescriptor): boolean => !Object.hasOwn(descriptor, "value"),
      )
    )
      throw new TypeError("JSON response contains non-JSON properties");
    for (const descriptor of descriptorValues)
      assertInboundJSONSerializable(descriptor.value, active);
  }
  active.delete(value);
}

/** Converts and validates a handler value into its declared Fetch Response representation. */
export async function responseFromHandler(
  codecContext: ServerCodecContext,
  value: InboundResponse,
  options?: InboundResponseOptions,
): Promise<Response> {
  const headers: Headers = new Headers(value.headers);
  if ((value.status === 204 || value.status === 205) && value.body !== undefined)
    throw new TypeError("Responses with status 204 or 205 must not include a body");
  const statusDefinitions: InboundResponseDefinition[] =
    options?.responses.filter((definition: InboundResponseDefinition): boolean =>
      inboundResponseStatusMatches(definition.status, value.status),
    ) ?? [];
  if (options !== undefined && statusDefinitions.length === 0)
    throw new TypeError("response status " + value.status + " is not declared by this endpoint");
  const generatedHeaderNames: ReadonlySet<string> = await appendInboundResponseHeaderValues(
    codecContext,
    headers,
    value.headerValues,
    statusDefinitions.flatMap(
      (definition: InboundResponseDefinition): readonly WireHeaderDefinition[] =>
        definition.headers ?? [],
    ),
    options?.schemas ?? {},
    options?.codecs,
  );
  if (value.body === undefined) {
    if (
      options !== undefined &&
      !statusDefinitions.some(
        (definition: InboundResponseDefinition): boolean => definition.contentType === undefined,
      )
    )
      throw new TypeError("response status " + value.status + " requires a body");
    await validateInboundResponseHeaders(
      codecContext,
      headers,
      statusDefinitions.find(
        (definition: InboundResponseDefinition): boolean => definition.contentType === undefined,
      )?.headers,
      options?.schemas ?? {},
      options?.codecs,
      generatedHeaderNames,
    );
    return new Response(null, { status: value.status, headers });
  }
  const contentType: string =
    value.contentType ?? headers.get("content-type") ?? "application/json";
  const normalizedContentType: string = normalizeInboundMediaType(contentType);
  if (!headers.has("content-type")) headers.set("content-type", contentType);
  const definition: InboundResponseDefinition | undefined = statusDefinitions
    .filter(
      (entry: InboundResponseDefinition): boolean =>
        entry.contentType !== undefined &&
        inboundMediaTypeMatches(entry.contentType, normalizeInboundMediaType(contentType)),
    )
    .sort(
      (left: InboundResponseDefinition, right: InboundResponseDefinition): number =>
        inboundMediaTypeMatchScore(right.contentType ?? "", contentType) -
        inboundMediaTypeMatchScore(left.contentType ?? "", contentType),
    )[0];
  if (options !== undefined && definition === undefined)
    throw new TypeError(
      "response content type " + contentType + " is not declared for status " + value.status,
    );
  if (definition?.schema !== undefined)
    codecContext.wire.validateWireValue(value.body, definition.schema, options!.schemas, "encode");
  await validateInboundResponseHeaders(
    codecContext,
    headers,
    definition?.headers,
    options?.schemas ?? {},
    options?.codecs,
    generatedHeaderNames,
  );
  if (normalizedContentType === "application/json" || normalizedContentType.endsWith("+json")) {
    assertInboundJSONSerializable(value.body);
    return new Response(JSON.stringify(value.body), { status: value.status, headers });
  }
  if (normalizedContentType.includes("xml"))
    return new Response(
      requireServerXML(codecContext).encodeXML(
        value.body,
        definition?.schema ?? {},
        options?.schemas ?? {},
      ),
      {
        status: value.status,
        headers,
      },
    );
  if (normalizedContentType.startsWith("text/"))
    return new Response(String(value.body), { status: value.status, headers });
  if (
    value.body instanceof Blob ||
    value.body instanceof ArrayBuffer ||
    ArrayBuffer.isView(value.body)
  )
    return new Response(value.body as BodyInit, { status: value.status, headers });
  const codec: MediaCodec<unknown> | undefined = options?.codecs?.get(
    normalizeInboundMediaType(contentType),
  );
  if (codec?.encode === undefined) throw new TypeError("missing encode codec for " + contentType);
  return new Response(await codec.encode(value.body, { contentType }), {
    status: value.status,
    headers,
  });
}

export async function validateInboundResponseHeaders(
  codecContext: ServerCodecContext,
  headers: Headers,
  definitions: readonly WireHeaderDefinition[] | undefined,
  schemas: WireSchemas,
  codecs: ReadonlyMap<string, MediaCodec<unknown>> | undefined,
  generatedHeaderNames: ReadonlySet<string> = new Set(),
): Promise<void> {
  for (const definition of definitions ?? []) {
    if (generatedHeaderNames.has(definition.name.toLowerCase())) continue;
    const value: string | null = headers.get(definition.name);
    if (value === null) {
      if (definition.required)
        throw new TypeError("missing required response header " + definition.name);
      continue;
    }
    const decoded: unknown = await decodeInboundResponseHeaderValue(
      codecContext,
      value,
      definition,
      schemas,
      codecs,
    );
    codecContext.wire.validateWireValue(decoded, definition.schema, schemas, "decode");
  }
}

export async function decodeInboundResponseHeaderValue(
  codecContext: ServerCodecContext,
  value: string,
  definition: WireHeaderDefinition,
  schemas: WireSchemas,
  codecs: ReadonlyMap<string, MediaCodec<unknown>> | undefined,
): Promise<unknown> {
  const contentType: string = normalizeInboundMediaType(definition.contentType ?? "");
  if (contentType === "application/json" || contentType.endsWith("+json")) {
    try {
      return JSON.parse(value);
    } catch {
      throw new TypeError("invalid JSON response header " + definition.name);
    }
  }
  if (contentType === "application/x-www-form-urlencoded") {
    const form: Record<string, string | string[]> = Object.create(null) as Record<
      string,
      string | string[]
    >;
    for (const [name, item] of new URLSearchParams(value)) {
      const previous: string | string[] | undefined = form[name];
      defineOwnDataProperty(
        form,
        name,
        previous === undefined
          ? item
          : Array.isArray(previous)
            ? [...previous, item]
            : [previous, item],
      );
    }
    return form;
  }
  if (contentType.includes("xml"))
    return requireServerXML(codecContext).decodeXML(value, definition.schema, schemas);
  if (contentType !== "") {
    if (contentType.startsWith("text/")) return value;
    const codec: MediaCodec<unknown> | undefined = inboundMediaCodec(codecs, contentType);
    if (codec?.decodeParameter === undefined)
      throw new TypeError("missing decodeParameter codec for response header " + definition.name);
    return codec.decodeParameter(value, { contentType });
  }
  return decodeInboundSimpleHeader(
    codecContext,
    value,
    definition.schema,
    schemas,
    definition.explode ?? false,
  );
}

export function decodeInboundSimpleHeader(
  codecContext: ServerCodecContext,
  value: string,
  schema: WireSchema,
  schemas: WireSchemas,
  explode: boolean,
): unknown {
  return decodeSimpleWireHeader(
    value,
    schema,
    schemas,
    explode,
    (candidate: unknown, contract: WireSchema): void =>
      codecContext.wire.validateWireValue(candidate, contract, schemas, "decode"),
    "string",
  );
}

export async function appendInboundResponseHeaderValues(
  codecContext: ServerCodecContext,
  headers: Headers,
  values: Readonly<Record<string, unknown>> | undefined,
  definitions: readonly WireHeaderDefinition[],
  schemas: WireSchemas,
  codecs: ReadonlyMap<string, MediaCodec<unknown>> | undefined,
): Promise<ReadonlySet<string>> {
  const result: Set<string> = new Set<string>();
  if (values === undefined) return result;
  const byProperty: Map<string, WireHeaderDefinition> = new Map(
    definitions.map((definition: WireHeaderDefinition): [string, WireHeaderDefinition] => [
      definition.property,
      definition,
    ]),
  );
  for (const [property, value] of Object.entries(values)) {
    const definition: WireHeaderDefinition | undefined = byProperty.get(property);
    if (definition === undefined)
      throw new TypeError("undeclared response header property " + property);
    if (headers.has(definition.name))
      throw new TypeError(
        "response header is provided by both headers and headerValues: " + definition.name,
      );
    codecContext.wire.validateWireValue(value, definition.schema, schemas, "encode");
    headers.set(
      definition.name,
      await encodeInboundResponseHeaderValue(codecContext, value, definition, schemas, codecs),
    );
    result.add(definition.name.toLowerCase());
  }
  return result;
}

export async function encodeInboundResponseHeaderValue(
  codecContext: ServerCodecContext,
  value: unknown,
  definition: WireHeaderDefinition,
  schemas: WireSchemas,
  codecs: ReadonlyMap<string, MediaCodec<unknown>> | undefined,
): Promise<string> {
  const encoded: unknown = codecContext.wire.encodeWireValue(value, definition.schema, schemas);
  const contentType: string = normalizeInboundMediaType(definition.contentType ?? "");
  if (contentType === "application/json" || contentType.endsWith("+json"))
    return JSON.stringify(encoded);
  if (contentType.includes("xml"))
    return requireServerXML(codecContext).encodeXML(encoded, definition.schema, schemas);
  if (contentType === "application/x-www-form-urlencoded") return encodeInboundHeaderForm(encoded);
  if (contentType !== "" && !contentType.startsWith("text/")) {
    const codec: MediaCodec<unknown> | undefined = inboundMediaCodec(codecs, contentType);
    if (codec?.encodeParameter === undefined)
      throw new TypeError("missing encodeParameter codec for response header " + definition.name);
    return codec.encodeParameter(encoded, { contentType });
  }
  return encodeInboundSimpleHeader(encoded, definition.explode ?? false);
}

export function encodeInboundHeaderForm(value: unknown): string {
  if (!isRecord(value)) return String(value ?? "");
  const form: URLSearchParams = new URLSearchParams();
  for (const [name, item] of Object.entries(value)) {
    for (const entry of Array.isArray(item) ? item : [item])
      form.append(
        name,
        isRecord(entry) || Array.isArray(entry) ? JSON.stringify(entry) : String(entry ?? ""),
      );
  }
  return form.toString();
}

export function encodeInboundSimpleHeader(value: unknown, explode: boolean = false): string {
  if (Array.isArray(value))
    return value.map((item: unknown): string => String(item ?? "")).join(",");
  if (isRecord(value))
    return explode
      ? Object.entries(value)
          .map(([name, item]: [string, unknown]): string => name + "=" + String(item ?? ""))
          .join(",")
      : Object.entries(value)
          .flatMap(([name, item]: [string, unknown]): string[] => [name, String(item ?? "")])
          .join(",");
  return String(value ?? "");
}
