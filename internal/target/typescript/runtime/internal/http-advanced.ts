import { createOperationStreamService } from "./http-stream.js";
import { decodeXML, encodeXML } from "./wire-xml.js";
import { isJSONMediaType, isXMLMediaType } from "./runtime-support.js";
import type {
  MediaCodec,
  StreamCodec,
  StreamContext,
  StreamFraming,
  WireBodyDefinition,
  WireEncodingDefinition,
  WireMultipartHeaderDefinition,
  WireSchema,
  WireSchemas,
} from "./wire-engine.js";
import type { ClientOptions } from "./configuration.js";
import { TransportErrorCode, isAPIError } from "./runtime-support.js";
import { defineOwnDataProperty, isRecord } from "./runtime-support.js";
import { operationDiagnosticName } from "./runtime-support.js";
import type { OperationDefinition } from "./operation.js";
import type {
  OperationStream,
  RequestOptions,
  ServerSentEvent,
  StreamResponseMetadata,
} from "./request.js";
import {
  decodeResponseStreamItems as decodeFramedResponseStreamItems,
  parseStreamJSON,
} from "./streaming.js";
import {
  applyOperationSecurity,
  assertReadableResponseHeaders,
  awaitAbortable,
  cancelResponseBody,
  cancelTrackedRequestBody,
  createAbortContext,
  isPromise,
  isReadableStream,
  normalizeMediaType,
  requestMetadata,
  selectResponseDefinition,
  serverError,
  transportError,
} from "./http-core.js";
import type {
  CompleteSequentialRequestOptions,
  EncodedRequest,
  EncodedStreamRequestBody,
  IncrementalStreamRequestOptions,
  MultipartStreamPart,
  HTTPStreamDecodeOptions,
  StreamProtocolEncodeOptions,
} from "./http-types.js";
import type { WireCodec } from "./wire-engine.js";
import type { AdvancedHTTPServices, RequestExecutionServices } from "./http-types.js";

/** Supplies the full XML, multipart and streaming implementations above the shared request algorithms. */
export function createAdvancedHTTPServices(
  getBase: () => RequestExecutionServices,
  wire: WireCodec,
): AdvancedHTTPServices {
  const { decodeWireValue, transformWireValue, validateWireValue } = wire;
  const createOperationStream = createOperationStreamService(
    getBase,
    decodeResponseStreamItems,
    wire,
  );
  const tolerantResponseTransformOptions = { unknownProperties: "preserve" } as const;

  async function* decodeResponseStreamItems(
    body: ReadableStream<Uint8Array>,
    options: HTTPStreamDecodeOptions,
  ): AsyncIterable<unknown> {
    yield* decodeFramedResponseStreamItems(body, {
      contentType: options.contentType,
      streamFraming: options.streamFraming,
      maxFrameBytes: options.maxFrameBytes,
      streamCodec: options.streamCodec,
      signal: options.signal,
      ...(options.streamFraming === "multipart"
        ? { multipartFrames: () => decodeMultipartStreamItems(body, options) }
        : {}),
    });
  }

  async function* decodeMultipartStreamItems(
    body: ReadableStream<Uint8Array>,
    options: HTTPStreamDecodeOptions,
  ): AsyncIterable<unknown> {
    const {
      contentType,
      itemSchema,
      prefixSchemas,
      schemas,
      codecs,
      prefixEncoding,
      itemEncoding,
      maxFrameBytes,
      signal,
    } = options;
    let index = 0;
    for await (const part of decodeMultipartStreamParts(body, contentType, maxFrameBytes, signal)) {
      const frameSchema = prefixSchemas?.[index] ?? itemSchema;
      const frameEncoding = prefixEncoding?.[index] ?? itemEncoding;
      index++;
      yield await awaitAbortable(
        decodeMultipartStreamPart(part, frameSchema, schemas, codecs, frameEncoding),
        signal,
      );
    }
  }

  const maxMultipartStreamHeaderBytes = 8192;

  async function* decodeMultipartStreamParts(
    body: ReadableStream<Uint8Array>,
    contentType: string,
    maxFrameBytes?: number,
    signal?: AbortSignal,
  ): AsyncIterable<MultipartStreamPart> {
    const boundary =
      /(?:^|;)\s*boundary=(?:"([^"]+)"|([^;\s]+))/i.exec(contentType)?.[1] ??
      /(?:^|;)\s*boundary=(?:"([^"]+)"|([^;\s]+))/i.exec(contentType)?.[2];
    if (boundary === undefined || boundary === "")
      throw new TypeError("multipart response has no boundary parameter");
    const encoder = new TextEncoder();
    const opening = encoder.encode(`--${boundary}`);
    const separator = encoder.encode(`\r\n--${boundary}`);
    const reader = body.getReader();
    let pending: Uint8Array<ArrayBufferLike> = new Uint8Array();
    let started = false;
    let closed = false;
    try {
      while (!closed) {
        const { done, value } = await awaitAbortable(reader.read(), signal);
        if (value !== undefined) pending = appendStreamBytes(pending, value);
        while (!closed) {
          if (!started) {
            const index = findStreamBytes(pending, opening);
            if (index < 0) break;
            const after = index + opening.length;
            if (pending.length < after + 2) break;
            if (pending[after] === 45 && pending[after + 1] === 45) {
              closed = true;
              pending = pending.slice(after + 2);
              continue;
            }
            if (pending[after] !== 13 || pending[after + 1] !== 10)
              throw new TypeError("multipart opening boundary is malformed");
            pending = pending.slice(after + 2);
            started = true;
            continue;
          }
          const index = findStreamBytes(pending, separator);
          if (index < 0) break;
          const after = index + separator.length;
          if (pending.length < after + 2) break;
          const closing = pending[after] === 45 && pending[after + 1] === 45;
          if (!closing && (pending[after] !== 13 || pending[after + 1] !== 10))
            throw new TypeError("multipart boundary is malformed");
          const part = pending.slice(0, index);
          pending = pending.slice(after + 2);
          yield parseMultipartStreamPart(part, maxFrameBytes);
          if (closing) closed = true;
        }
        if (maxFrameBytes !== undefined && !closed) {
          const maximumBuffered = started
            ? maxFrameBytes + maxMultipartStreamHeaderBytes + separator.length + 4
            : maxMultipartStreamHeaderBytes + opening.length + 2;
          if (pending.byteLength > maximumBuffered)
            throw new TypeError(`multipart stream frame exceeds ${maxFrameBytes} bytes`);
        }
        if (done) break;
      }
      if (!closed) throw new TypeError("multipart response ended before its closing boundary");
    } finally {
      try {
        await reader.cancel();
      } finally {
        reader.releaseLock();
      }
    }
  }

  async function decodeMultipartStreamPart(
    part: MultipartStreamPart,
    itemSchema: WireSchema,
    schemas: WireSchemas,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    itemEncoding: WireEncodingDefinition | undefined,
  ): Promise<unknown> {
    const { headers, bytes } = part;
    for (const header of itemEncoding?.headers ?? []) {
      const value = headers.get(header.name);
      if (value === null) {
        if (header.required)
          throw new TypeError(`multipart part is missing required header ${header.name}`);
        continue;
      }
      const decoded = await decodeResponseHeaderValue(
        header.name,
        value,
        header.schema,
        header.contentType,
        header.explode,
        schemas,
        codecs,
      );
      validateWireValue(decoded, header.schema, schemas, "decode");
    }
    const declared = itemEncoding?.contentType?.split(",", 1)[0]?.trim();
    const rawPartContentType = headers.get("content-type") ?? declared ?? "text/plain";
    const partContentType = normalizeMediaType(rawPartContentType);
    if (partContentType.startsWith("multipart/")) {
      return decodeMultipartResponse(
        new Blob([ownedArrayBuffer(bytes)]).stream(),
        rawPartContentType,
        {
          contentType: rawPartContentType,
          schema: itemSchema,
          ...(itemEncoding?.encoding === undefined ? {} : { encoding: itemEncoding.encoding }),
          ...(itemEncoding?.prefixEncoding === undefined
            ? {}
            : { prefixEncoding: itemEncoding.prefixEncoding }),
          ...(itemEncoding?.itemEncoding === undefined
            ? {}
            : { itemEncoding: itemEncoding.itemEncoding }),
        },
        schemas,
        codecs,
      );
    }
    if (isJSONMediaType(partContentType)) return parseStreamJSON(new TextDecoder().decode(bytes));
    if (isXMLMediaType(partContentType))
      return decodeXML(new TextDecoder().decode(bytes), itemSchema, schemas);
    if (partContentType.startsWith("text/")) return new TextDecoder().decode(bytes);
    if (isBinaryMediaType(partContentType) || itemSchema.contentEncoding === "binary")
      return bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
    const codec = codecs.get(normalizeMediaType(partContentType));
    if (codec?.decode === undefined)
      throw new TypeError(`missing decode codec for multipart item ${partContentType}`);
    return codec.decode(
      new Response(ownedArrayBuffer(bytes), { headers: { "content-type": partContentType } }),
      { contentType: partContentType },
    );
  }

  function ownedArrayBuffer(bytes: Uint8Array): ArrayBuffer {
    const copy = new Uint8Array(bytes.byteLength);
    copy.set(bytes);
    return copy.buffer;
  }

  function parseMultipartStreamPart(part: Uint8Array, maxFrameBytes?: number): MultipartStreamPart {
    const split = findStreamBytes(part, new Uint8Array([13, 10, 13, 10]));
    if (split < 0) throw new TypeError("multipart part has no header terminator");
    if (maxFrameBytes !== undefined && split > maxMultipartStreamHeaderBytes)
      throw new TypeError("multipart stream headers exceed 8192 bytes");
    const bytes = part.slice(split + 4);
    if (maxFrameBytes !== undefined && bytes.byteLength > maxFrameBytes)
      throw new TypeError(`multipart stream frame exceeds ${maxFrameBytes} bytes`);
    const headers = parseMultipartStreamHeaders(new TextDecoder().decode(part.slice(0, split)));
    return { headers, bytes };
  }

  function appendStreamBytes(left: Uint8Array, right: Uint8Array): Uint8Array {
    const result = new Uint8Array(left.length + right.length);
    result.set(left);
    result.set(right, left.length);
    return result;
  }

  function findStreamBytes(source: Uint8Array, wanted: Uint8Array): number {
    if (wanted.length === 0) return 0;
    outer: for (let start = 0; start <= source.length - wanted.length; start++) {
      for (let index = 0; index < wanted.length; index++)
        if (source[start + index] !== wanted[index]) continue outer;
      return start;
    }
    return -1;
  }

  function parseMultipartStreamHeaders(source: string): Headers {
    const headers = new Headers();
    for (const line of source.split("\r\n")) {
      const separator = line.indexOf(":");
      if (separator <= 0) throw new TypeError("multipart part has a malformed header");
      headers.append(line.slice(0, separator).trim(), line.slice(separator + 1).trim());
    }
    return headers;
  }

  async function decodeResponseHeaderValue(
    name: string,
    value: string,
    schema: WireSchema,
    contentType: string | undefined,
    explode: boolean | undefined,
    schemas: WireSchemas,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
  ): Promise<unknown> {
    if (contentType !== undefined) {
      const decoded = decodeHeaderContent(name, value, contentType);
      if (
        isJSONMediaType(contentType) ||
        contentType.toLowerCase() === "application/x-www-form-urlencoded"
      )
        return decoded;
      if (isXMLMediaType(contentType)) return decodeXML(value, schema, schemas);
      if (!contentType.toLowerCase().startsWith("text/")) {
        const codec = codecs.get(normalizeMediaType(contentType));
        if (codec?.decodeParameter === undefined)
          throw new TypeError(`missing decodeParameter codec for response header ${name}`);
        return codec.decodeParameter(value, { contentType });
      }
      value = decoded as string;
    }
    const resolved = resolveHeaderSchema(schema, schemas);
    if (resolved.types?.includes("array"))
      return value
        .split(",")
        .map((entry) => decodeResponseHeaderScalar(name, entry, resolved.items ?? {}, schemas));
    if (resolved.types?.includes("object") || resolved.properties !== undefined) {
      const result = Object.create(null) as Record<string, unknown>;
      const tokens = value.split(",");
      if (explode)
        for (const token of tokens) {
          const separator = token.indexOf("=");
          if (separator < 0) continue;
          const propertyName = token.slice(0, separator);
          const property = resolved.properties?.[propertyName];
          defineOwnDataProperty(
            result,
            propertyName,
            decodeResponseHeaderScalar(
              name,
              token.slice(separator + 1),
              property?.schema ?? {},
              schemas,
            ),
          );
        }
      else
        for (let index = 0; index + 1 < tokens.length; index += 2) {
          const property = resolved.properties?.[tokens[index]!];
          defineOwnDataProperty(
            result,
            tokens[index]!,
            decodeResponseHeaderScalar(name, tokens[index + 1]!, property?.schema ?? {}, schemas),
          );
        }
      return result;
    }
    return decodeResponseHeaderScalar(name, value, resolved, schemas);
  }

  function decodeResponseHeaderScalar(
    name: string,
    value: string,
    schema: WireSchema,
    schemas: WireSchemas,
  ): unknown {
    const resolved = resolveHeaderSchema(schema, schemas);
    if (resolved.types?.includes("integer")) {
      const parsed = Number(value);
      if (!Number.isInteger(parsed))
        throw new TypeError(`response header ${name} is not an integer`);
      return parsed;
    }
    if (resolved.types?.includes("number")) {
      const parsed = Number(value);
      if (!Number.isFinite(parsed)) throw new TypeError(`response header ${name} is not a number`);
      return parsed;
    }
    if (resolved.types?.includes("boolean")) {
      if (value === "true") return true;
      if (value === "false") return false;
      throw new TypeError(`response header ${name} is not a boolean`);
    }
    return value;
  }

  function resolveHeaderSchema(schema: WireSchema, schemas: WireSchemas): WireSchema {
    const referenced = schema.reference === undefined ? undefined : schemas[schema.reference];
    return referenced === undefined ? schema : resolveHeaderSchema(referenced, schemas);
  }

  function decodeHeaderContent(name: string, value: string, contentType: string): unknown {
    if (isJSONMediaType(contentType)) {
      try {
        return JSON.parse(value);
      } catch (cause) {
        throw new TypeError(`response header ${name} is not valid ${contentType}`, { cause });
      }
    }
    if (contentType.toLowerCase() === "application/x-www-form-urlencoded") {
      const result = Object.create(null) as Record<string, string | string[]>;
      for (const [key, item] of new URLSearchParams(value)) {
        const previous = result[key];
        defineOwnDataProperty(
          result,
          key,
          previous === undefined
            ? item
            : Array.isArray(previous)
              ? [...previous, item]
              : [previous, item],
        );
      }
      return result;
    }
    return value;
  }

  function encodeRequestBody(
    contentType: string,
    value: unknown,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    schema: WireSchema | undefined,
    schemas: WireSchemas,
    definition: WireBodyDefinition | undefined,
    multipartHeaders: Readonly<Record<string, HeadersInit>> | undefined,
    multipartContentTypes: Readonly<Record<string, string>> | undefined,
  ): BodyInit | Promise<BodyInit> {
    const normalizedContentType = contentType.toLowerCase();
    if (isJSONMediaType(normalizedContentType)) return JSON.stringify(value);
    if (normalizedContentType === "application/x-www-form-urlencoded") {
      if (!isRecord(value)) throw new TypeError("form body must be an object");
      const form = new URLSearchParams();
      for (const [name, item] of formEntries(value, definition?.encoding)) form.append(name, item);
      return form;
    }
    if (normalizedContentType.startsWith("multipart/")) {
      if (definition?.prefixEncoding !== undefined || definition?.itemEncoding !== undefined) {
        if (!Array.isArray(value))
          throw new TypeError("positional multipart body must be an array");
        return encodePositionalMultipartBody(
          contentType,
          value,
          definition.prefixEncoding,
          definition.itemEncoding,
          multipartHeaders,
          multipartContentTypes,
          schema,
          schemas,
          codecs,
        );
      }
      if (normalizedContentType !== "multipart/form-data")
        throw new TypeError(
          `named multipart encoding requires multipart/form-data, got ${contentType}`,
        );
      if (!isRecord(value)) throw new TypeError("multipart body must be an object");
      if (
        definition?.encoding?.some((entry) => (entry.headers?.length ?? 0) > 0) ||
        multipartHeaders !== undefined
      ) {
        return encodeMultipartBody(
          value,
          definition?.encoding,
          multipartHeaders,
          schema,
          schemas,
          codecs,
        );
      }
      const form = new FormData();
      const append = (
        name: string,
        item: unknown,
        definition: WireEncodingDefinition | undefined,
      ): void => {
        if (item instanceof Blob) form.append(name, item);
        else if (item instanceof ArrayBuffer) form.append(name, new Blob([item]));
        else if (ArrayBuffer.isView(item)) {
          const bytes = new Uint8Array(item.byteLength);
          bytes.set(new Uint8Array(item.buffer, item.byteOffset, item.byteLength));
          form.append(name, new Blob([bytes.buffer]));
        } else if (isRecord(item) || Array.isArray(item)) {
          form.append(
            name,
            new Blob([JSON.stringify(item)], {
              type: definition?.contentType ?? "application/json",
            }),
          );
        } else if (definition?.contentType !== undefined)
          form.append(name, new Blob([String(item)], { type: definition.contentType }));
        else form.append(name, String(item));
      };
      for (const [name, item] of Object.entries(value)) {
        if (item === undefined) continue;
        const encoding = definition?.encoding?.find((entry) => entry.name === name);
        if (Array.isArray(item) && encoding?.explode !== false)
          for (const entry of item) append(name, entry, encoding);
        else append(name, item, encoding);
      }
      return form;
    }
    if (isXMLMediaType(normalizedContentType)) return encodeXML(value, schema ?? {}, schemas);
    if (normalizedContentType.startsWith("text/")) return String(value);
    if (value instanceof Blob || value instanceof ArrayBuffer || ArrayBuffer.isView(value)) {
      return value as BodyInit;
    }
    const codec = codecs.get(normalizeMediaType(contentType));
    if (codec?.encode === undefined) throw new TypeError(`missing encode codec for ${contentType}`);
    return codec.encode(value, { contentType });
  }

  async function encodePositionalMultipartBody(
    contentType: string,
    values: readonly unknown[],
    prefixEncoding: readonly WireEncodingDefinition[] | undefined,
    itemEncoding: WireEncodingDefinition | undefined,
    suppliedHeaders: Readonly<Record<string, HeadersInit>> | undefined,
    suppliedContentTypes: Readonly<Record<string, string>> | undefined,
    schema: WireSchema | undefined,
    schemas: WireSchemas,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
  ): Promise<Blob> {
    const boundary = `----openapi-sdkgen-${multipartBoundaryToken()}`;
    const chunks: BlobPart[] = [];
    for (const [index, value] of values.entries()) {
      const definition = prefixEncoding?.[index] ?? itemEncoding;
      const itemSchema = schema?.prefixItems?.[index] ?? schema?.items ?? {};
      const selectedContentType = resolveMultipartContentType(
        definition?.contentType,
        suppliedContentTypes?.[String(index)],
        defaultMultipartContentType(itemSchema),
        value,
      );
      const { body, contentType: partContentType } = await multipartPartValue(
        value,
        selectedContentType,
        itemSchema,
        definition,
        schemas,
        codecs,
      );
      const headers = await multipartPartHeaders(
        undefined,
        definition,
        suppliedHeaders?.[String(index)],
        partContentType,
        undefined,
        schemas,
        codecs,
      );
      chunks.push(`--${boundary}\r\n${headers}\r\n\r\n`, body, "\r\n");
    }
    chunks.push(`--${boundary}--\r\n`);
    return new Blob(chunks, { type: `${contentType}; boundary=${boundary}` });
  }

  function encodeSequentialRequestWireBody(
    contentType: string,
    streamFraming: Exclude<StreamFraming, "multipart" | "custom">,
    values: AsyncIterable<unknown>,
    maxFrameBytes: number,
  ): ReadableStream<Uint8Array> {
    const encodeItem = sequentialRequestItemEncoder(streamFraming, contentType);
    const iterator = values[Symbol.asyncIterator]();
    const encoder = new TextEncoder();
    return new ReadableStream<Uint8Array>(
      {
        async pull(controller): Promise<void> {
          try {
            const next = await iterator.next();
            if (next.done) {
              controller.close();
              return;
            }
            const frame = encoder.encode(encodeItem(next.value));
            if (frame.byteLength > maxFrameBytes)
              throw new TypeError(`stream frame exceeds ${maxFrameBytes} bytes`);
            controller.enqueue(frame);
          } catch (cause) {
            controller.error(cause);
            try {
              await iterator.return?.();
            } catch {
              /* original error wins */
            }
          }
        },
        async cancel(reason): Promise<void> {
          await iterator.return?.(reason);
        },
      },
      { highWaterMark: 0 },
    );
  }

  function sequentialRequestItemEncoder(
    streamFraming: Exclude<StreamFraming, "multipart" | "custom">,
    contentType: string,
  ): (value: unknown) => string {
    if (streamFraming === "sse") return encodeSSERequestItem;
    if (streamFraming === "json-sequence")
      return (value) => `\u001e${encodeJSONStreamItem(value)}\n`;
    if (streamFraming === "line-delimited-json")
      return (value) => `${encodeJSONStreamItem(value)}\n`;
    throw new TypeError(`unsupported streaming request media type ${contentType}`);
  }

  function encodeJSONStreamItem(value: unknown): string {
    const encoded = JSON.stringify(value);
    if (encoded === undefined) throw new TypeError("stream item is not JSON-serializable");
    return encoded;
  }

  function encodeSSERequestItem(value: unknown): string {
    if (!isRecord(value)) throw new TypeError("SSE stream item must be an object");
    for (const field of Object.keys(value)) {
      if (field !== "data" && field !== "event" && field !== "id" && field !== "retry")
        throw new TypeError(`SSE stream item contains unsupported field ${field}`);
    }
    if (!Object.hasOwn(value, "data") || typeof value.data !== "string")
      throw new TypeError("SSE stream item data must be a string");

    const event = sseRequestStringField(value, "event");
    const id = sseRequestStringField(value, "id");
    if (event !== undefined && /[\r\n]/.test(event))
      throw new TypeError("SSE stream item event must not contain a line break");
    if (id !== undefined && /[\u0000\r\n]/.test(id))
      throw new TypeError("SSE stream item id must not contain NUL or a line break");

    let retry: number | undefined;
    if (Object.hasOwn(value, "retry") && value.retry !== undefined) {
      if (typeof value.retry !== "number" || !Number.isSafeInteger(value.retry) || value.retry < 0)
        throw new TypeError("SSE stream item retry must be a non-negative safe integer");
      retry = value.retry;
    }

    const lines: string[] = [];
    if (event !== undefined) lines.push(`event: ${event}`);
    for (const line of value.data.split(/\r\n|\r|\n/)) lines.push(`data: ${line}`);
    if (id !== undefined) lines.push(`id: ${id}`);
    if (retry !== undefined) lines.push(`retry: ${retry}`);
    return lines.join("\n") + "\n\n";
  }

  function sseRequestStringField(
    value: Readonly<Record<string, unknown>>,
    field: "event" | "id",
  ): string | undefined {
    if (!Object.hasOwn(value, field) || value[field] === undefined) return undefined;
    if (typeof value[field] !== "string")
      throw new TypeError(`SSE stream item ${field} must be a string`);
    return value[field];
  }

  function encodeIncrementalStreamRequestBody(
    values: AsyncIterable<unknown>,
    options: IncrementalStreamRequestOptions,
  ): EncodedStreamRequestBody | Promise<EncodedStreamRequestBody> {
    const context: StreamContext = {
      contentType: options.contentType,
      maxFrameBytes: options.maxFrameBytes,
      ...(options.signal === undefined ? {} : { signal: options.signal }),
    };
    const items = transformStreamingRequestItems(values, options.itemSchema, options.schemas);
    const frames = encodeStreamApplicationFrames(
      items,
      options.streamFraming,
      options.streamCodec,
      context,
    );
    return encodeStreamProtocolFrames(frames, {
      contentType: options.contentType,
      streamFraming: options.streamFraming,
      streamCodec: options.streamCodec,
      context,
      frameSchema: options.itemSchema,
      prefixSchemas: undefined,
      schemas: options.schemas,
      prefixEncoding: undefined,
      itemEncoding: options.itemEncoding,
      suppliedHeaders: options.suppliedHeaders,
      suppliedContentTypes: options.suppliedContentTypes,
      codecs: options.codecs,
    });
  }

  function encodeCompleteSequentialRequestBody(
    value: unknown,
    options: CompleteSequentialRequestOptions,
  ): EncodedStreamRequestBody | Promise<EncodedStreamRequestBody> {
    const transformed = transformWireValue(value, options.schema, options.schemas, "encode");
    if (!Array.isArray(transformed))
      throw new TypeError("complete sequential request body must be an array value");
    const context: StreamContext = {
      contentType: options.contentType,
      maxFrameBytes: options.maxFrameBytes,
      ...(options.signal === undefined ? {} : { signal: options.signal }),
    };
    const items = streamArrayValues(transformed);
    const frames = encodeStreamApplicationFrames(
      items,
      options.streamFraming,
      options.streamCodec,
      context,
    );
    return encodeStreamProtocolFrames(frames, {
      contentType: options.contentType,
      streamFraming: options.streamFraming,
      streamCodec: options.streamCodec,
      context,
      frameSchema: options.schema.items ?? {},
      prefixSchemas: options.schema.prefixItems,
      schemas: options.schemas,
      prefixEncoding: options.prefixEncoding,
      itemEncoding: options.itemEncoding,
      suppliedHeaders: options.suppliedHeaders,
      suppliedContentTypes: options.suppliedContentTypes,
      codecs: options.codecs,
    });
  }

  function encodeStreamApplicationFrames(
    items: AsyncIterable<unknown>,
    streamFraming: StreamFraming | undefined,
    streamCodec: StreamCodec | undefined,
    context: StreamContext,
  ): AsyncIterable<unknown> {
    if (streamCodec?.adapter !== undefined) return streamCodec.adapter.encode(items, context);
    if (streamCodec?.protocol === undefined && streamFraming === "sse")
      return encodeDefaultSSEJSONFrames(items);
    return items;
  }

  async function* encodeDefaultSSEJSONFrames(
    items: AsyncIterable<unknown>,
  ): AsyncIterable<ServerSentEvent> {
    for await (const item of items) {
      const data = JSON.stringify(item);
      if (data === undefined) throw new TypeError("SSE stream item must be JSON-serializable");
      yield { data };
    }
  }

  function encodeStreamProtocolFrames(
    frames: AsyncIterable<unknown>,
    options: StreamProtocolEncodeOptions,
  ): EncodedStreamRequestBody | Promise<EncodedStreamRequestBody> {
    if (options.streamCodec?.protocol !== undefined) {
      const encoded = options.streamCodec.protocol.encode(frames, options.context);
      const finish = (body: ReadableStream<Uint8Array>): EncodedStreamRequestBody => ({
        body,
        contentType: options.contentType,
      });
      return isPromise(encoded) ? encoded.then(finish) : finish(encoded);
    }
    if (options.streamFraming === "multipart") return encodeStreamingMultipartBody(frames, options);
    if (
      options.streamFraming === "line-delimited-json" ||
      options.streamFraming === "json-sequence" ||
      options.streamFraming === "sse"
    )
      return {
        body: encodeSequentialRequestWireBody(
          options.contentType,
          options.streamFraming,
          frames,
          options.context.maxFrameBytes,
        ),
        contentType: options.contentType,
      };
    throw new TypeError(`missing stream protocol for ${options.contentType}`);
  }

  async function* streamArrayValues(values: readonly unknown[]): AsyncIterable<unknown> {
    for (const value of values) yield value;
  }

  function transformStreamingRequestItems(
    values: AsyncIterable<unknown>,
    itemSchema: WireSchema,
    schemas: WireSchemas,
  ): AsyncIterable<unknown> {
    return mapAsyncIterable(values, (value) =>
      transformWireValue(value, itemSchema, schemas, "encode"),
    );
  }

  function mapAsyncIterable<Input, Output>(
    values: AsyncIterable<Input>,
    transform: (value: Input) => Output,
  ): AsyncIterable<Output> {
    return {
      [Symbol.asyncIterator](): AsyncIterator<Output> {
        const iterator = values[Symbol.asyncIterator]();
        let done = false;
        return {
          async next(): Promise<IteratorResult<Output>> {
            if (done) return { done: true, value: undefined as never };
            const next = await iterator.next();
            if (done || next.done) {
              done = true;
              return { done: true, value: undefined as never };
            }
            return { done: false, value: transform(next.value) };
          },
          async return(reason?: unknown): Promise<IteratorResult<Output>> {
            if (done) return { done: true, value: undefined as never };
            done = true;
            await iterator.return?.(reason);
            return { done: true, value: undefined as never };
          },
        };
      },
    };
  }

  function encodeStreamingMultipartBody(
    values: AsyncIterable<unknown>,
    options: StreamProtocolEncodeOptions,
  ): { readonly body: ReadableStream<Uint8Array>; readonly contentType: string } {
    const boundary = `----openapi-sdkgen-${multipartBoundaryToken()}`;
    const iterator = values[Symbol.asyncIterator]();
    const encoder = new TextEncoder();
    let index = 0;
    const body = new ReadableStream<Uint8Array>({
      async pull(controller): Promise<void> {
        try {
          const next = await iterator.next();
          if (next.done) {
            controller.enqueue(encoder.encode(`--${boundary}--\r\n`));
            controller.close();
            return;
          }
          const value = next.value;
          const frameSchema = options.prefixSchemas?.[index] ?? options.frameSchema;
          const frameEncoding = options.prefixEncoding?.[index] ?? options.itemEncoding;
          const selectedContentType = resolveMultipartContentType(
            frameEncoding?.contentType,
            options.suppliedContentTypes?.[String(index)],
            defaultMultipartContentType(frameSchema),
            value,
          );
          const part = await multipartPartValue(
            value,
            selectedContentType,
            frameSchema,
            frameEncoding,
            options.schemas,
            options.codecs,
          );
          const frameBytes = new Blob([part.body]).size;
          if (frameBytes > options.context.maxFrameBytes)
            throw new TypeError(
              `multipart stream frame exceeds ${options.context.maxFrameBytes} bytes`,
            );
          const headers = await multipartPartHeaders(
            undefined,
            frameEncoding,
            options.suppliedHeaders?.[String(index)],
            part.contentType,
            undefined,
            options.schemas,
            options.codecs,
          );
          index++;
          const bytes = await new Blob([
            `--${boundary}\r\n${headers}\r\n\r\n`,
            part.body,
            "\r\n",
          ]).arrayBuffer();
          controller.enqueue(new Uint8Array(bytes));
        } catch (cause) {
          controller.error(cause);
          try {
            await iterator.return?.();
          } catch {
            /* original error wins */
          }
        }
      },
      async cancel(reason): Promise<void> {
        await iterator.return?.(reason);
      },
    });
    return { body, contentType: `${options.contentType}; boundary=${boundary}` };
  }

  function defaultMultipartContentType(schema: WireSchema): string {
    const types = schema.types ?? [];
    if (types.includes("object") || types.includes("array")) return "application/json";
    if (types.includes("string"))
      return schema.contentEncoding === undefined ? "text/plain" : "application/octet-stream";
    if (types.includes("number") || types.includes("integer") || types.includes("boolean"))
      return "text/plain";
    return "application/octet-stream";
  }

  function resolveMultipartContentType(
    declared: string | undefined,
    selected: string | undefined,
    fallback: string,
    value: unknown,
  ): string {
    const candidate = normalizeMediaType(
      selected ?? (value instanceof Blob && value.type !== "" ? value.type : fallback),
    );
    if (declared === undefined || declared.trim() === "") return candidate;
    const allowed = declared
      .split(",")
      .map(normalizeMediaType)
      .filter((item) => item !== "");
    if (allowed.some((item) => mediaRangeMatches(item, candidate))) return candidate;
    const exact = allowed.filter((item) => !item.includes("*"));
    if (selected === undefined && exact.length === 1) return exact[0]!;
    throw new TypeError(
      `multipart part content type ${candidate} is not permitted by ${declared}; select one with RequestOptions.multipartContentTypes`,
    );
  }

  function mediaRangeMatches(range: string, value: string): boolean {
    const [rangeType, rangeSubtype] = range.split("/", 2);
    const [valueType, valueSubtype] = value.split("/", 2);
    return (
      (rangeType === "*" || rangeType === valueType) &&
      (rangeSubtype === "*" || rangeSubtype === valueSubtype)
    );
  }

  async function encodeMultipartBody(
    fields: Record<string, unknown>,
    encoding: readonly WireEncodingDefinition[] | undefined,
    suppliedHeaders: Readonly<Record<string, HeadersInit>> | undefined,
    schema: WireSchema | undefined,
    schemas: WireSchemas,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
  ): Promise<Blob> {
    const boundary = `----openapi-sdkgen-${multipartBoundaryToken()}`;
    const chunks: BlobPart[] = [];
    const definitions = new Map(
      (encoding ?? []).map((definition) => [definition.name, definition]),
    );
    for (const [name, fieldValue] of Object.entries(fields)) {
      if (fieldValue === undefined) continue;
      const definition = definitions.get(name);
      const values =
        Array.isArray(fieldValue) && definition?.explode !== false ? fieldValue : [fieldValue];
      for (const item of values) {
        const propertySchema = schema?.properties?.[name]?.schema ?? {};
        const { body, contentType, filename } = await multipartPartValue(
          item,
          definition?.contentType,
          propertySchema,
          definition,
          schemas,
          codecs,
        );
        const headers = await multipartPartHeaders(
          name,
          definition,
          suppliedHeaders?.[name],
          contentType,
          filename,
          schemas,
          codecs,
        );
        chunks.push(`--${boundary}\r\n${headers}\r\n\r\n`, body, "\r\n");
      }
    }
    chunks.push(`--${boundary}--\r\n`);
    return new Blob(chunks, { type: `multipart/form-data; boundary=${boundary}` });
  }

  async function multipartPartValue(
    value: unknown,
    declaredContentType: string | undefined,
    schema: WireSchema = {},
    definition: WireEncodingDefinition | undefined = undefined,
    schemas: WireSchemas = {},
    codecs: ReadonlyMap<string, MediaCodec<unknown>> = new Map(),
  ): Promise<{ body: BlobPart; contentType?: string; filename?: string }> {
    if (
      declaredContentType !== undefined &&
      normalizeMediaType(declaredContentType).startsWith("multipart/")
    ) {
      if (!Array.isArray(value)) throw new TypeError("nested multipart part must be an array");
      const nested = await encodePositionalMultipartBody(
        declaredContentType,
        value,
        definition?.prefixEncoding,
        definition?.itemEncoding,
        undefined,
        undefined,
        schema,
        schemas,
        codecs,
      );
      return { body: nested, contentType: nested.type };
    }
    if (value instanceof Blob) {
      const file = typeof File !== "undefined" && value instanceof File ? value : undefined;
      const contentType = (declaredContentType ?? value.type) || undefined;
      return {
        body: value,
        ...(contentType === undefined ? {} : { contentType }),
        ...(file === undefined ? {} : { filename: file.name }),
      };
    }
    if (value instanceof ArrayBuffer)
      return { body: value, contentType: declaredContentType ?? "application/octet-stream" };
    if (ArrayBuffer.isView(value)) {
      const bytes = new Uint8Array(value.byteLength);
      bytes.set(new Uint8Array(value.buffer, value.byteOffset, value.byteLength));
      return { body: bytes.buffer, contentType: declaredContentType ?? "application/octet-stream" };
    }
    if (declaredContentType !== undefined && requiresMultipartPartCodec(declaredContentType)) {
      const codec = codecs.get(normalizeMediaType(declaredContentType));
      if (codec?.encode === undefined)
        throw new TypeError(`missing encode codec for multipart part ${declaredContentType}`);
      return {
        body: multipartCodecBody(await codec.encode(value, { contentType: declaredContentType })),
        contentType: declaredContentType,
      };
    }
    if (declaredContentType !== undefined && isXMLMediaType(declaredContentType)) {
      return { body: encodeXML(value, schema, schemas), contentType: declaredContentType };
    }
    if (isRecord(value) || Array.isArray(value)) {
      return {
        body: JSON.stringify(value),
        contentType: declaredContentType ?? "application/json",
      };
    }
    return {
      body: String(value),
      ...(declaredContentType === undefined ? {} : { contentType: declaredContentType }),
    };
  }

  function requiresMultipartPartCodec(contentType: string): boolean {
    const normalized = normalizeMediaType(contentType);
    return (
      !isJSONMediaType(normalized) &&
      !isXMLMediaType(normalized) &&
      !normalized.startsWith("text/") &&
      normalized !== "application/x-www-form-urlencoded" &&
      !isBinaryMediaType(normalized)
    );
  }

  function multipartCodecBody(value: BodyInit): BlobPart {
    if (typeof value === "string" || value instanceof Blob || value instanceof ArrayBuffer)
      return value;
    if (ArrayBuffer.isView(value)) return value;
    if (value instanceof URLSearchParams) return value.toString();
    throw new TypeError(
      "multipart part codec must return a string, Blob, ArrayBuffer, ArrayBufferView, or URLSearchParams",
    );
  }

  async function multipartPartHeaders(
    name: string | undefined,
    definition: WireEncodingDefinition | undefined,
    supplied: HeadersInit | undefined,
    contentType: string | undefined,
    filename: string | undefined,
    schemas: WireSchemas,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
  ): Promise<string> {
    const headers = new Headers(supplied);
    const declared = new Map(
      (definition?.headers ?? []).map((header) => [header.name.toLowerCase(), header]),
    );
    for (const header of definition?.headers ?? []) {
      const value = headers.get(header.name);
      if (value === null && header.required)
        throw new TypeError(`missing required multipart header ${name}.${header.name}`);
      if (value !== null) await validateMultipartHeaderValue(name, header, value, schemas, codecs);
    }
    for (const [headerName] of headers) {
      const normalized = headerName.toLowerCase();
      if (
        normalized === "content-type" ||
        (name !== undefined && normalized === "content-disposition") ||
        !declared.has(normalized)
      ) {
        throw new TypeError(
          `multipart header ${name ?? "position"}.${headerName} is not declared by the Encoding Object`,
        );
      }
    }
    const lines =
      name === undefined
        ? []
        : [
            `Content-Disposition: form-data; name="${escapeMultipartToken(name)}"${filename === undefined ? "" : `; filename="${escapeMultipartToken(filename)}"`}`,
          ];
    if (contentType !== undefined && contentType !== "") lines.push(`Content-Type: ${contentType}`);
    for (const [headerName, value] of headers) lines.push(`${headerName}: ${value}`);
    return lines.join("\r\n");
  }

  async function validateMultipartHeaderValue(
    part: string | undefined,
    header: WireMultipartHeaderDefinition,
    value: string,
    schemas: WireSchemas,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
  ): Promise<void> {
    const decoded = await decodeMultipartHeaderValue(
      `${part ?? "position"}.${header.name}`,
      value,
      header.schema,
      header.contentType,
      header.explode,
      schemas,
      codecs,
    );
    validateWireValue(decoded, header.schema, schemas, "decode");
  }

  async function decodeMultipartHeaderValue(
    name: string,
    value: string,
    schema: WireSchema,
    contentType: string | undefined,
    explode: boolean | undefined,
    schemas: WireSchemas,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
  ): Promise<unknown> {
    if (contentType !== undefined) {
      const decoded = decodeHeaderContent(name, value, contentType);
      if (
        isJSONMediaType(contentType) ||
        contentType.toLowerCase() === "application/x-www-form-urlencoded"
      )
        return decoded;
      if (isXMLMediaType(contentType)) return decodeXML(value, schema, schemas);
      if (!contentType.toLowerCase().startsWith("text/")) {
        const codec = codecs.get(normalizeMediaType(contentType));
        if (codec?.decodeParameter === undefined)
          throw new TypeError(`missing decodeParameter codec for multipart header ${name}`);
        return codec.decodeParameter(value, { contentType });
      }
      value = decoded as string;
    }
    const resolved = resolveHeaderSchema(schema, schemas);
    if (resolved.types?.includes("array"))
      return value
        .split(",")
        .map((item) => decodeMultipartHeaderScalar(name, item, resolved.items ?? {}, schemas));
    if (resolved.types?.includes("object") || resolved.properties !== undefined) {
      const result = Object.create(null) as Record<string, unknown>;
      const tokens = value.split(",");
      if (explode)
        for (const token of tokens) {
          const separator = token.indexOf("=");
          if (separator < 0) continue;
          const property = token.slice(0, separator);
          defineOwnDataProperty(
            result,
            property,
            decodeMultipartHeaderScalar(
              name,
              token.slice(separator + 1),
              resolved.properties?.[property]?.schema ?? {},
              schemas,
            ),
          );
        }
      else
        for (let index = 0; index + 1 < tokens.length; index += 2)
          defineOwnDataProperty(
            result,
            tokens[index]!,
            decodeMultipartHeaderScalar(
              name,
              tokens[index + 1]!,
              resolved.properties?.[tokens[index]!]?.schema ?? {},
              schemas,
            ),
          );
      return result;
    }
    return decodeMultipartHeaderScalar(name, value, resolved, schemas);
  }

  function decodeMultipartHeaderScalar(
    name: string,
    value: string,
    schema: WireSchema,
    schemas: WireSchemas,
  ): unknown {
    const resolved = resolveHeaderSchema(schema, schemas);
    if (resolved.types?.includes("integer")) {
      const parsed = Number(value);
      if (!Number.isInteger(parsed))
        throw new TypeError(`multipart header ${name} is not an integer`);
      return parsed;
    }
    if (resolved.types?.includes("number")) {
      const parsed = Number(value);
      if (!Number.isFinite(parsed)) throw new TypeError(`multipart header ${name} is not a number`);
      return parsed;
    }
    if (resolved.types?.includes("boolean")) {
      if (value === "true") return true;
      if (value === "false") return false;
      throw new TypeError(`multipart header ${name} is not a boolean`);
    }
    return value;
  }

  function escapeMultipartToken(value: string): string {
    if (/\r|\n/.test(value))
      throw new TypeError("multipart names and filenames cannot contain line breaks");
    return value.replaceAll("\\", "\\\\").replaceAll('"', '\\"');
  }

  function multipartBoundaryToken(): string {
    const random = globalThis.crypto?.randomUUID?.();
    return random === undefined ? `${Date.now()}-${Math.random().toString(16).slice(2)}` : random;
  }

  function formEntries(
    value: Record<string, unknown>,
    encoding: readonly WireEncodingDefinition[] | undefined,
  ): readonly [string, string][] {
    const result: [string, string][] = [];
    for (const [name, item] of Object.entries(value)) {
      if (item === undefined) continue;
      const definition = encoding?.find((entry) => entry.name === name);
      if (definition?.contentType !== undefined && isJSONMediaType(definition.contentType)) {
        result.push([name, JSON.stringify(item)]);
        continue;
      }
      const explode = definition?.explode ?? true;
      if (Array.isArray(item)) {
        if (explode) for (const entry of item) result.push([name, String(entry)]);
        else result.push([name, item.map(String).join(",")]);
        continue;
      }
      if (isRecord(item)) {
        const entries = Object.entries(item).filter(
          (entry): entry is [string, unknown] => entry[1] !== undefined,
        );
        if (explode) for (const [key, entry] of entries) result.push([key, String(entry)]);
        else result.push([name, entries.flatMap(([key, entry]) => [key, String(entry)]).join(",")]);
        continue;
      }
      result.push([name, String(item)]);
    }
    return result;
  }

  async function decodeMultipartResponse(
    body: ReadableStream<Uint8Array>,
    contentType: string,
    definition: WireBodyDefinition,
    schemas: WireSchemas,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
  ): Promise<unknown[]> {
    const result: unknown[] = [];
    for await (const part of decodeMultipartStreamParts(body, contentType)) {
      const index = result.length;
      const schema = definition.schema.prefixItems?.[index] ?? definition.schema.items ?? {};
      const encoding = definition.prefixEncoding?.[index] ?? definition.itemEncoding;
      result.push(await decodeMultipartStreamPart(part, schema, schemas, codecs, encoding));
    }
    return result;
  }

  function isBinaryMediaType(contentType: string): boolean {
    return (
      contentType === "application/octet-stream" ||
      contentType.startsWith("image/") ||
      contentType.startsWith("audio/") ||
      contentType.startsWith("video/")
    );
  }
  return {
    createOperationStream,
    encodeRequestBody,
    encodeIncrementalStreamRequestBody,
    encodeCompleteSequentialRequestBody,
    decodeResponseStreamItems,
    decodeMultipartResponse,
    encodeXML,
    decodeXML,
  };
}
