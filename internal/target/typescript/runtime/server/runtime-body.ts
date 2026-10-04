import type { MediaCodec } from "../media/media-codec-types.js";
import { defineOwnDataProperty } from "../shared/runtime-support.js";
import type {
  InboundBodyOptions,
  InboundBodyPlan,
  InboundSchema,
  ServerCodecContext,
} from "./runtime-types.js";
import { inboundMediaTypeMatches } from "./runtime-shared.js";
import { inboundMediaTypeMatchScore } from "./runtime-shared.js";
import { emptyInboundStream } from "./runtime-shared.js";
import { decodeInboundStreamBody } from "./runtime-shared.js";
import { decodeInboundCompleteSequentialBody } from "./runtime-shared.js";
import { inboundMediaCodec } from "./runtime-shared.js";
import { requireServerHook } from "./runtime-codecs.js";
import { decodeXMLBody } from "./runtime-codecs.js";
import { validateInboundWireValue } from "./runtime-shared.js";
import { decodeInboundWireValue } from "./runtime-shared.js";
import { isInboundBinaryMedia } from "./runtime-shared.js";
import { InboundRequestError } from "./runtime-errors.js";

/** Decodes and validates one declared JSON, text, form, or XML request body. */
export async function decodeInboundBody(
  codecContext: ServerCodecContext,
  request: Request,
  options: InboundBodyOptions,
): Promise<unknown> {
  const rawContentType: string | null = request.headers.get("content-type");
  const contentType: string | undefined = rawContentType?.split(";", 1)[0]?.trim().toLowerCase();
  if (contentType === undefined && request.body === null && !options.required) return undefined;
  const plan: InboundBodyPlan | undefined =
    contentType === undefined ? undefined : selectInboundBodyPlan(options.plans, contentType);
  if (plan === undefined || contentType === undefined) {
    throw new InboundRequestError(new Response("Unsupported Media Type", { status: 415 }));
  }
  const value: unknown = await decodeSelectedInboundBody(
    codecContext,
    request,
    rawContentType ?? contentType,
    contentType,
    { ...options, ...plan },
  );
  return options.plans.length === 1 || value === undefined
    ? value
    : { contentType: plan.contentType, value };
}

export function selectInboundBodyPlan(
  plans: readonly InboundBodyPlan[],
  contentType: string,
): InboundBodyPlan | undefined {
  return plans
    .filter((plan: InboundBodyPlan): boolean =>
      inboundMediaTypeMatches(plan.contentType, contentType),
    )
    .sort(
      (left: InboundBodyPlan, right: InboundBodyPlan): number =>
        inboundMediaTypeMatchScore(right.contentType, contentType) -
        inboundMediaTypeMatchScore(left.contentType, contentType),
    )[0];
}

export async function decodeSelectedInboundBody(
  codecContext: ServerCodecContext,
  request: Request,
  rawContentType: string,
  contentType: string,
  options: InboundBodyOptions & InboundBodyPlan,
): Promise<unknown> {
  let value: unknown;
  if (options.stream === true) {
    if (request.body === null) {
      if (options.required)
        throw new InboundRequestError(new Response("Request body is required", { status: 400 }));
      return emptyInboundStream();
    }
    return decodeInboundStreamBody(
      codecContext,
      request.body,
      rawContentType,
      contentType,
      request.signal,
      options,
    );
  }

  const completeRequest: Request = await boundedCompleteInboundRequest(
    request,
    options.maxBodyBytes,
  );
  if (options.binary === true) {
    const bytes: ArrayBuffer = await completeRequest.arrayBuffer();
    if (bytes.byteLength === 0 && options.required)
      throw new InboundRequestError(new Response("Request body is required", { status: 400 }));
    return bytes;
  }
  if (options.streamFraming !== undefined) {
    if (completeRequest.body === null) {
      if (options.required)
        throw new InboundRequestError(new Response("Request body is required", { status: 400 }));
      return undefined;
    }
    return decodeInboundCompleteSequentialBody(
      codecContext,
      completeRequest.body,
      rawContentType,
      contentType,
      completeRequest.signal,
      options,
    );
  }
  if (!isGeneratedInboundMediaType(contentType, options.schema)) {
    const codec: MediaCodec<unknown> | undefined = inboundMediaCodec(options.codecs, contentType);
    if (codec?.decodeInbound === undefined)
      throw new InboundRequestError(new Response("Unsupported Media Type", { status: 415 }));
    try {
      value = await codec.decodeInbound(completeRequest, { contentType });
    } catch {
      throw new InboundRequestError(new Response("Invalid request body", { status: 400 }));
    }
  } else if (contentType === "multipart/form-data") {
    let form: FormData;
    try {
      form = await completeRequest.formData();
    } catch {
      throw new InboundRequestError(new Response("Invalid multipart form", { status: 400 }));
    }
    if ([...form.keys()].length === 0) {
      if (options.required)
        throw new InboundRequestError(new Response("Request body is required", { status: 400 }));
      return undefined;
    }
    const result: Record<string, unknown> = Object.create(null) as Record<string, unknown>;
    for (const [name, item] of form) {
      const previous: unknown = result[name];
      defineOwnDataProperty(
        result,
        name,
        previous === undefined
          ? item
          : Array.isArray(previous)
            ? [...previous, item]
            : [previous, item],
      );
    }
    value = await requireServerHook(codecContext.decodeFormValue)(
      codecContext,
      result,
      options.schema,
      options.schemas,
      options.wireSchema,
      options.wireSchemas,
      options.encoding,
      undefined,
      options.codecs,
    );
  } else {
    const text: string = await completeRequest.text();
    const missing: boolean = contentType === "text/plain" ? text === "" : text.trim() === "";
    if (missing) {
      if (options.required)
        throw new InboundRequestError(new Response("Request body is required", { status: 400 }));
      return undefined;
    }
    if (contentType === "application/json" || contentType.endsWith("+json")) {
      try {
        value = JSON.parse(text);
      } catch {
        throw new InboundRequestError(new Response("Invalid JSON", { status: 400 }));
      }
    } else if (contentType === "application/x-www-form-urlencoded") {
      const form: URLSearchParams = new URLSearchParams(text);
      const result: Record<string, unknown> = Object.create(null) as Record<string, unknown>;
      for (const [name, item] of form) {
        const previous: unknown = result[name];
        defineOwnDataProperty(
          result,
          name,
          previous === undefined
            ? item
            : Array.isArray(previous)
              ? [...previous, item]
              : [previous, item],
        );
      }
      value = await requireServerHook(codecContext.decodeFormValue)(
        codecContext,
        result,
        options.schema,
        options.schemas,
        options.wireSchema,
        options.wireSchemas,
        options.encoding,
        undefined,
        options.codecs,
      );
    } else if (contentType.includes("xml")) {
      try {
        value = decodeXMLBody(
          codecContext,
          text,
          options.schema,
          options.schemas,
          options.wireSchema,
          options.wireSchemas,
        );
      } catch (cause: unknown) {
        throw new InboundRequestError(new Response("Invalid XML", { status: 400 }));
      }
    } else value = text;
  }
  validateInboundWireValue(
    codecContext,
    value,
    options.wireSchema,
    options.wireSchemas,
    "request body",
  );
  return decodeInboundWireValue(codecContext, value, options.wireSchema, options.wireSchemas);
}

export function isGeneratedInboundMediaType(
  contentType: string,
  schema: InboundSchema | undefined,
): boolean {
  return (
    contentType === "application/json" ||
    contentType.endsWith("+json") ||
    contentType.startsWith("text/") ||
    contentType.includes("xml") ||
    contentType === "application/x-www-form-urlencoded" ||
    contentType === "multipart/form-data" ||
    isInboundBinaryMedia(contentType, schema)
  );
}

export function resolveInboundBodyBytes(value: number | undefined): number {
  const resolved: number = value ?? 8 * 1024 * 1024;
  if (!Number.isSafeInteger(resolved) || resolved <= 0)
    throw new TypeError("maxBodyBytes must be a positive safe integer");
  return resolved;
}

export function inboundBodyTooLargeError(): InboundRequestError {
  return new InboundRequestError(
    new Response("Request body exceeds maxBodyBytes", { status: 413 }),
  );
}

export function inboundContentLengthExceedsLimit(value: string | null, maxBytes: number): boolean {
  if (value === null || !/^\d+$/.test(value)) return false;
  const normalized: string = value.replace(/^0+/, "") || "0";
  const limit: string = String(maxBytes);
  return (
    normalized.length > limit.length || (normalized.length === limit.length && normalized > limit)
  );
}

export async function boundedCompleteInboundRequest(
  request: Request,
  configuredMaxBytes: number | undefined,
): Promise<Request> {
  const maxBytes: number = resolveInboundBodyBytes(configuredMaxBytes);
  if (inboundContentLengthExceedsLimit(request.headers.get("content-length"), maxBytes))
    throw inboundBodyTooLargeError();
  if (request.body === null) return request;

  const reader: ReadableStreamDefaultReader<Uint8Array<ArrayBuffer>> = request.body.getReader();
  const chunks: Uint8Array[] = [];
  let totalBytes: number = 0;
  try {
    while (true) {
      const next: ReadableStreamReadResult<Uint8Array<ArrayBuffer>> = await reader.read();
      if (next.done) break;
      if (next.value.byteLength > maxBytes - totalBytes) {
        throw inboundBodyTooLargeError();
      }
      totalBytes += next.value.byteLength;
      chunks.push(next.value);
    }
  } finally {
    reader.releaseLock();
  }

  const bytes: Uint8Array<ArrayBuffer> = new Uint8Array(totalBytes);
  let offset: number = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return new Request(request.url, {
    method: request.method,
    headers: request.headers,
    body: bytes,
    cache: request.cache,
    credentials: request.credentials,
    integrity: request.integrity,
    keepalive: request.keepalive,
    mode: request.mode,
    redirect: request.redirect,
    referrer: request.referrer,
    referrerPolicy: request.referrerPolicy,
    signal: request.signal,
  });
}
