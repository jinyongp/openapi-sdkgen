import { MultipartByteFrames } from "../stream/framing/framing-multipart.js";
import type { MultipartFramingFailure } from "../stream/framing/framing-multipart.js";
import type { MediaCodec } from "../media/media-codec-types.js";
import type { WireEncodingDefinition } from "../media/media-contract-types.js";
import type { WireSchema, WireSchemas } from "../schema/wire-types.js";
import type {
  InboundMultipartPartDecodeOptions,
  InboundProtocolDecodeOptions,
  InboundSchema,
  ServerCodecContext,
} from "./runtime-types.js";
import { awaitInboundAbortable } from "./runtime-shared.js";
import { inboundSequenceItemSchema } from "./runtime-shared.js";
import { decodeXMLBody } from "./runtime-codecs.js";
import { isInboundBinaryMedia } from "./runtime-shared.js";
import { decodeInboundParameterValue } from "./runtime-shared.js";
import { decodeInboundFormContent } from "./runtime-shared.js";
import { validateInboundWireValue } from "./runtime-shared.js";
import { InboundRequestError } from "./runtime-errors.js";

export async function* decodeInboundMultipartStream(
  codecContext: ServerCodecContext,
  body: ReadableStream<Uint8Array>,
  options: InboundProtocolDecodeOptions,
): AsyncIterable<unknown> {
  const { rawContentType, maxFrameBytes, signal }: InboundProtocolDecodeOptions = options;
  const match: RegExpExecArray | null = /(?:^|;)\s*boundary=(?:"([^"]+)"|([^;\s]+))/i.exec(
    rawContentType,
  );
  const boundary: string | undefined = match?.[1] ?? match?.[2];
  if (boundary === undefined || boundary === "")
    throw new InboundRequestError(new Response("Invalid multipart boundary", { status: 400 }));
  const framing: MultipartByteFrames = new MultipartByteFrames(
    boundary,
    maxFrameBytes,
    (reason: MultipartFramingFailure): never => {
      const message: string =
        reason === "limit"
          ? "Multipart frame exceeds maxStreamFrameBytes"
          : reason === "incomplete"
            ? "Invalid multipart body"
            : "Invalid multipart boundary";
      throw new InboundRequestError(new Response(message, { status: 400 }));
    },
  );
  const reader: ReadableStreamDefaultReader<Uint8Array<ArrayBufferLike>> = body.getReader();
  let count: number = 0;
  try {
    while (!framing.closed) {
      const next: ReadableStreamReadResult<Uint8Array<ArrayBufferLike>> =
        await awaitInboundAbortable(reader.read(), signal);
      if (next.value !== undefined)
        for (const part of framing.push(next.value)) {
          const frameSchema: InboundSchema | undefined = options.complete
            ? inboundSequenceItemSchema(options.schema, options.schemas, count)
            : options.schema;
          const frameEncoding: WireEncodingDefinition | undefined = options.complete
            ? (options.prefixEncoding?.[count] ?? options.itemEncoding)
            : options.itemEncoding;
          yield await decodeInboundMultipartPart(codecContext, part, {
            schema: frameSchema,
            schemas: options.schemas,
            itemEncoding: frameEncoding,
            wireSchemas: options.wireSchemas,
            codecs: options.codecs,
            maxFrameBytes,
          });
          count++;
        }
      if (next.done) break;
    }
    framing.finish();
  } finally {
    try {
      await reader.cancel(signal.reason);
    } finally {
      reader.releaseLock();
    }
  }
}

export async function decodeInboundMultipartPart(
  codecContext: ServerCodecContext,
  part: Uint8Array,
  options: InboundMultipartPartDecodeOptions,
): Promise<unknown> {
  const {
    schema,
    schemas,
    itemEncoding,
    wireSchemas,
    codecs,
    maxFrameBytes,
  }: InboundMultipartPartDecodeOptions = options;
  const split: number = findInboundBytes(part, new Uint8Array([13, 10, 13, 10]));
  if (split < 0)
    throw new InboundRequestError(new Response("Invalid multipart part", { status: 400 }));
  if (split > 8192)
    throw new InboundRequestError(
      new Response("Multipart headers exceed stream limit", { status: 400 }),
    );
  const headers: Headers = parseInboundMultipartHeaders(
    new TextDecoder().decode(part.slice(0, split)),
  );
  await validateInboundMultipartEncodingHeaders(
    codecContext,
    headers,
    itemEncoding,
    wireSchemas,
    codecs,
  );
  const bytes: Uint8Array = part.subarray(split + 4);
  if (bytes.byteLength > maxFrameBytes)
    throw new InboundRequestError(
      new Response("Multipart frame exceeds maxStreamFrameBytes", { status: 400 }),
    );
  const rawContentType: string =
    headers.get("content-type") ??
    itemEncoding?.contentType?.split(",", 1)[0]?.trim() ??
    "text/plain";
  const normalized: string = rawContentType.split(";", 1)[0]!.trim().toLowerCase();
  let value: unknown;
  if (normalized === "application/json" || normalized.endsWith("+json")) {
    try {
      value = JSON.parse(new TextDecoder().decode(bytes));
    } catch {
      throw new InboundRequestError(new Response("Invalid multipart JSON item", { status: 400 }));
    }
  } else if (normalized.includes("xml")) {
    try {
      value = decodeXMLBody(
        codecContext,
        new TextDecoder().decode(bytes),
        schema,
        schemas,
        undefined,
        undefined,
      );
    } catch {
      throw new InboundRequestError(new Response("Invalid multipart XML item", { status: 400 }));
    }
  } else if (isInboundBinaryMedia(normalized, schema)) {
    return bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
  } else value = new TextDecoder().decode(bytes);
  return value;
}

export async function validateInboundMultipartEncodingHeaders(
  codecContext: ServerCodecContext,
  headers: Headers,
  encoding: WireEncodingDefinition | undefined,
  wireSchemas: WireSchemas | undefined,
  codecs: ReadonlyMap<string, MediaCodec<unknown>> | undefined,
): Promise<void> {
  const schemas: Readonly<Record<string, WireSchema>> = wireSchemas ?? {};
  for (const header of encoding?.headers ?? []) {
    const raw: string | null = headers.get(header.name);
    if (raw === null) {
      if (header.required)
        throw new InboundRequestError(
          new Response(`Missing required multipart header ${header.name}`, { status: 400 }),
        );
      continue;
    }
    const decoded: unknown =
      header.contentType === undefined
        ? decodeInboundParameterValue(codecContext, raw, {}, {}, header.schema, schemas)
        : await decodeInboundFormContent(
            codecContext,
            raw,
            {},
            {},
            header.schema,
            schemas,
            header.contentType,
            codecs,
          );
    validateInboundWireValue(
      codecContext,
      decoded,
      header.schema,
      schemas,
      `multipart header ${header.name}`,
    );
  }
}

export function parseInboundMultipartHeaders(source: string): Headers {
  const headers: Headers = new Headers();
  for (const line of source.split("\r\n")) {
    const separator: number = line.indexOf(":");
    if (separator <= 0)
      throw new InboundRequestError(new Response("Invalid multipart header", { status: 400 }));
    headers.append(line.slice(0, separator).trim(), line.slice(separator + 1).trim());
  }
  return headers;
}

export function findInboundBytes(source: Uint8Array, wanted: Uint8Array): number {
  outer: for (let start: number = 0; start <= source.length - wanted.length; start++) {
    for (let index: number = 0; index < wanted.length; index++)
      if (source[start + index] !== wanted[index]) continue outer;
    return start;
  }
  return -1;
}
