import type {
  WireCodec,
  MediaCodec,
  WireSchema,
  WireSchemas,
  WireBodyDefinition,
  WireEncodingDefinition,
} from "./wire-types.js";
import { isJSONMediaType, isXMLMediaType } from "./runtime-support.js";
import { normalizeMediaType } from "./http-execution-support.js";
import type { XMLCodec } from "./xml-types.js";
import type { HTTPHeaderDecoder } from "./http-media-types.js";
import type { MultipartResponseServices } from "./http-media-types.js";
import type { MultipartStreamPart, HTTPStreamDecodeOptions } from "./http-types.js";
import { MultipartByteFrames, type MultipartFramingFailure } from "./framing-multipart.js";
import { awaitAbortable } from "./http-execution-support.js";
import { parseStreamJSON } from "./stream-json.js";
export function createMultipartResponseServices(
  wire: WireCodec,
  xml: XMLCodec,
  decodeResponseHeaderValue: HTTPHeaderDecoder,
): MultipartResponseServices {
  const { validateWireValue }: WireCodec = wire;
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
    }: HTTPStreamDecodeOptions = options;
    let index: number = 0;
    for await (const part of decodeMultipartStreamParts(body, contentType, maxFrameBytes, signal)) {
      const frameSchema: WireSchema = prefixSchemas?.[index] ?? itemSchema;
      const frameEncoding: WireEncodingDefinition | undefined =
        prefixEncoding?.[index] ?? itemEncoding;
      index++;
      yield await awaitAbortable(
        decodeMultipartStreamPart(part, frameSchema, schemas, codecs, frameEncoding),
        signal,
      );
    }
  }

  const maxMultipartStreamHeaderBytes: 8192 = 8192;

  async function* decodeMultipartStreamParts(
    body: ReadableStream<Uint8Array>,
    contentType: string,
    maxFrameBytes?: number,
    signal?: AbortSignal,
  ): AsyncIterable<MultipartStreamPart> {
    const boundary: string | undefined =
      /(?:^|;)\s*boundary=(?:"([^"]+)"|([^;\s]+))/i.exec(contentType)?.[1] ??
      /(?:^|;)\s*boundary=(?:"([^"]+)"|([^;\s]+))/i.exec(contentType)?.[2];
    if (boundary === undefined || boundary === "")
      throw new TypeError("multipart response has no boundary parameter");
    const framing: MultipartByteFrames = new MultipartByteFrames(
      boundary,
      maxFrameBytes,
      (reason: MultipartFramingFailure): never => {
        if (reason === "opening") throw new TypeError("multipart opening boundary is malformed");
        if (reason === "boundary") throw new TypeError("multipart boundary is malformed");
        if (reason === "limit")
          throw new TypeError(`multipart stream frame exceeds ${maxFrameBytes} bytes`);
        throw new TypeError("multipart response ended before its closing boundary");
      },
    );
    const reader: ReadableStreamDefaultReader<Uint8Array<ArrayBufferLike>> = body.getReader();
    try {
      while (!framing.closed) {
        const { done, value }: ReadableStreamReadResult<Uint8Array<ArrayBufferLike>> =
          await awaitAbortable(reader.read(), signal);
        if (value !== undefined)
          for (const part of framing.push(value))
            yield parseMultipartStreamPart(part, maxFrameBytes);
        if (done) break;
      }
      framing.finish();
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
    const { headers, bytes }: MultipartStreamPart = part;
    for (const header of itemEncoding?.headers ?? []) {
      const value: string | null = headers.get(header.name);
      if (value === null) {
        if (header.required)
          throw new TypeError(`multipart part is missing required header ${header.name}`);
        continue;
      }
      const decoded: unknown = await decodeResponseHeaderValue(
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
    const declared: string | undefined = itemEncoding?.contentType?.split(",", 1)[0]?.trim();
    const rawPartContentType: string = headers.get("content-type") ?? declared ?? "text/plain";
    const partContentType: string = normalizeMediaType(rawPartContentType);
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
      return xml.decodeXML(new TextDecoder().decode(bytes), itemSchema, schemas);
    if (partContentType.startsWith("text/")) return new TextDecoder().decode(bytes);
    if (isBinaryMediaType(partContentType) || itemSchema.contentEncoding === "binary")
      return bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
    const codec: MediaCodec<unknown> | undefined = codecs.get(normalizeMediaType(partContentType));
    if (codec?.decode === undefined)
      throw new TypeError(`missing decode codec for multipart item ${partContentType}`);
    return codec.decode(
      new Response(ownedArrayBuffer(bytes), { headers: { "content-type": partContentType } }),
      { contentType: partContentType },
    );
  }

  function ownedArrayBuffer(bytes: Uint8Array): ArrayBuffer {
    const copy: Uint8Array<ArrayBuffer> = new Uint8Array(bytes.byteLength);
    copy.set(bytes);
    return copy.buffer;
  }

  function parseMultipartStreamPart(part: Uint8Array, maxFrameBytes?: number): MultipartStreamPart {
    const split: number = findStreamBytes(part, new Uint8Array([13, 10, 13, 10]));
    if (split < 0) throw new TypeError("multipart part has no header terminator");
    if (maxFrameBytes !== undefined && split > maxMultipartStreamHeaderBytes)
      throw new TypeError("multipart stream headers exceed 8192 bytes");
    const bytes: Uint8Array = part.subarray(split + 4);
    if (maxFrameBytes !== undefined && bytes.byteLength > maxFrameBytes)
      throw new TypeError(`multipart stream frame exceeds ${maxFrameBytes} bytes`);
    const headers: Headers = parseMultipartStreamHeaders(
      new TextDecoder().decode(part.slice(0, split)),
    );
    return { headers, bytes };
  }

  function findStreamBytes(source: Uint8Array, wanted: Uint8Array): number {
    if (wanted.length === 0) return 0;
    outer: for (let start: number = 0; start <= source.length - wanted.length; start++) {
      for (let index: number = 0; index < wanted.length; index++)
        if (source[start + index] !== wanted[index]) continue outer;
      return start;
    }
    return -1;
  }

  function parseMultipartStreamHeaders(source: string): Headers {
    const headers: Headers = new Headers();
    for (const line of source.split("\r\n")) {
      const separator: number = line.indexOf(":");
      if (separator <= 0) throw new TypeError("multipart part has a malformed header");
      headers.append(line.slice(0, separator).trim(), line.slice(separator + 1).trim());
    }
    return headers;
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
      const index: number = result.length;
      const schema: WireSchema =
        definition.schema.prefixItems?.[index] ?? definition.schema.items ?? {};
      const encoding: WireEncodingDefinition | undefined =
        definition.prefixEncoding?.[index] ?? definition.itemEncoding;
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
  return { decodeMultipartResponse, decodeStreamItems: decodeMultipartStreamItems };
}
