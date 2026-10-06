import type { WireCodec, WireSchema, WireSchemas } from "../schema/wire-types.js";
import type { MediaCodec } from "./media-codec-types.js";
import type {
  WireBodyDefinition,
  WireEncodingDefinition,
  WireMultipartHeaderDefinition,
} from "./media-contract-types.js";
import { isJSONMediaType, isXMLMediaType, isRecord } from "../shared/runtime-support.js";
import { normalizeMediaType } from "../shared/runtime-support.js";
import { isBinaryMediaType } from "./media-type.js";
import type { XMLCodec } from "./xml/xml-types.js";
import type { HTTPHeaderDecoder } from "./http-media-types.js";
import type {
  StreamProtocolEncodeOptions,
  EncodedStreamRequestBody,
  MultipartPartValue,
} from "./media-service-types.js";
import type { HTTPBodyEncoder, MultipartRequestServices } from "./http-media-types.js";
import { wireArrayItemSchema, wirePropertySchema } from "../schema/schema-query.js";
export function createMultipartRequestServices(
  wire: WireCodec,
  xml: XMLCodec,
  decodeResponseHeaderValue: HTTPHeaderDecoder,
): MultipartRequestServices {
  const { validateWireValue }: WireCodec = wire;
  const encodeBody: HTTPBodyEncoder = (
    contentType: string,
    value: unknown,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    schema: WireSchema | undefined,
    schemas: WireSchemas,
    definition: WireBodyDefinition | undefined,
    multipartHeaders: Readonly<Record<string, HeadersInit>> | undefined,
    multipartContentTypes: Readonly<Record<string, string>> | undefined,
  ): BodyInit | Promise<BodyInit> => {
    const normalizedContentType: string = normalizeMediaType(contentType);
    if (definition?.prefixEncoding !== undefined || definition?.itemEncoding !== undefined) {
      if (!Array.isArray(value)) throw new TypeError("positional multipart body must be an array");
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
      definition?.encoding?.some(
        (entry: WireEncodingDefinition): boolean => (entry.headers?.length ?? 0) > 0,
      ) ||
      multipartHeaders !== undefined
    ) {
      return encodeMultipartBody(
        value,
        definition?.encoding,
        multipartHeaders,
        schema,
        schemas,
        codecs,
        multipartContentTypes,
      );
    }
    return encodeMultipartForm(
      value,
      definition?.encoding,
      schema,
      schemas,
      codecs,
      multipartContentTypes,
    );
  };
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
    const boundary: string = `----openapi-sdkgen-${multipartBoundaryToken()}`;
    const chunks: BlobPart[] = [];
    for (const [index, value] of values.entries()) {
      const definition: WireEncodingDefinition | undefined =
        prefixEncoding?.[index] ?? itemEncoding;
      const itemSchema: WireSchema = schema?.prefixItems?.[index] ?? schema?.items ?? {};
      const binary: WireSchema | undefined = multipartBinarySchema(itemSchema, value, schemas);
      const selectedContentType: string = resolveMultipartContentType(
        definition?.contentType ?? binary?.binaryContentType,
        suppliedContentTypes?.[String(index)],
        defaultMultipartContentType(itemSchema),
        value,
      );
      const {
        body,
        contentType: partContentType,
        filename,
      }: MultipartPartValue = await multipartPartValue(
        value,
        selectedContentType,
        itemSchema,
        definition,
        schemas,
        codecs,
      );
      const headers: string = await multipartPartHeaders(
        undefined,
        definition,
        suppliedHeaders?.[String(index)],
        partContentType,
        filename,
        schemas,
        codecs,
      );
      chunks.push(`--${boundary}\r\n${headers}\r\n\r\n`, body, "\r\n");
    }
    chunks.push(`--${boundary}--\r\n`);
    return new Blob(chunks, { type: `${contentType}; boundary=${boundary}` });
  }

  function encodeStreamingMultipartBody(
    values: AsyncIterable<unknown>,
    options: StreamProtocolEncodeOptions,
  ): EncodedStreamRequestBody {
    const boundary: string = `----openapi-sdkgen-${multipartBoundaryToken()}`;
    const iterator: AsyncIterator<unknown, unknown, unknown> = values[Symbol.asyncIterator]();
    const encoder: TextEncoder = new TextEncoder();
    let index: number = 0;
    const body: ReadableStream<Uint8Array<ArrayBufferLike>> = new ReadableStream<Uint8Array>({
      async pull(
        controller: ReadableStreamDefaultController<Uint8Array<ArrayBufferLike>>,
      ): Promise<void> {
        try {
          const next: IteratorResult<unknown, unknown> = await iterator.next();
          if (next.done) {
            controller.enqueue(encoder.encode(`--${boundary}--\r\n`));
            controller.close();
            return;
          }
          const value: unknown = next.value;
          const frameSchema: WireSchema = options.prefixSchemas?.[index] ?? options.frameSchema;
          const frameEncoding: WireEncodingDefinition | undefined =
            options.prefixEncoding?.[index] ?? options.itemEncoding;
          const selectedContentType: string = resolveMultipartContentType(
            frameEncoding?.contentType ??
              multipartBinarySchema(frameSchema, value, options.schemas)?.binaryContentType,
            options.suppliedContentTypes?.[String(index)],
            defaultMultipartContentType(frameSchema),
            value,
          );
          const part: MultipartPartValue = await multipartPartValue(
            value,
            selectedContentType,
            frameSchema,
            frameEncoding,
            options.schemas,
            options.codecs,
          );
          const frameBytes: number = new Blob([part.body]).size;
          if (frameBytes > options.context.maxFrameBytes)
            throw new TypeError(
              `multipart stream frame exceeds ${options.context.maxFrameBytes} bytes`,
            );
          const headers: string = await multipartPartHeaders(
            undefined,
            frameEncoding,
            options.suppliedHeaders?.[String(index)],
            part.contentType,
            part.filename,
            options.schemas,
            options.codecs,
          );
          index++;
          const bytes: ArrayBuffer = await new Blob([
            `--${boundary}\r\n${headers}\r\n\r\n`,
            part.body,
            "\r\n",
          ]).arrayBuffer();
          controller.enqueue(new Uint8Array(bytes));
        } catch (cause: unknown) {
          controller.error(cause);
          try {
            await iterator.return?.();
          } catch {
            /* original error wins */
          }
        }
      },
      async cancel(reason: unknown): Promise<void> {
        await iterator.return?.(reason);
      },
    });
    return { body, contentType: `${options.contentType}; boundary=${boundary}` };
  }

  function defaultMultipartContentType(schema: WireSchema): string {
    if (schema.binaryInput === true) return schema.binaryContentType ?? "application/octet-stream";
    const types: readonly string[] = schema.types ?? [];
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
    const candidate: string =
      selected ?? (value instanceof Blob && value.type !== "" ? value.type : fallback);
    if (declared === undefined || declared.trim() === "") return candidate;
    const allowed: string[] = declared
      .split(",")
      .map((item: string): string => item.trim())
      .filter((item: string): boolean => item !== "");
    const exact: string[] = allowed.filter((item: string): boolean => !item.includes("*"));
    if (selected === undefined && allowed.length === 1 && exact.length === 1) return exact[0]!;
    if (
      allowed.some((item: string): boolean =>
        mediaRangeMatches(normalizeMediaType(item), normalizeMediaType(candidate)),
      )
    )
      return candidate;
    if (selected === undefined && exact.length === 1) return exact[0]!;
    throw new TypeError(
      `multipart part content type ${candidate} is not permitted by ${declared}; select one with RequestOptions.multipartContentTypes`,
    );
  }

  function multipartFieldSchema(
    schema: WireSchema | undefined,
    name: string,
    schemas: WireSchemas,
    index: number | undefined,
  ): WireSchema {
    const field: WireSchema =
      schema === undefined ? {} : (wirePropertySchema(schema, name, schemas) ?? {});
    return index === undefined ? field : (wireArrayItemSchema(field, index, schemas) ?? {});
  }

  function explodeMultipartField(
    value: unknown,
    definition: WireEncodingDefinition | undefined,
  ): boolean {
    return Array.isArray(value) && (definition?.explode ?? definition?.contentType === undefined);
  }

  async function namedMultipartPartValue(
    value: unknown,
    definition: WireEncodingDefinition | undefined,
    schema: WireSchema,
    schemas: WireSchemas,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    selected: string | undefined,
  ): Promise<MultipartPartValue> {
    const binary: WireSchema | undefined = multipartBinarySchema(schema, value, schemas);
    const contentType: string | undefined =
      definition?.contentType === undefined && selected === undefined && binary === undefined
        ? undefined
        : resolveMultipartContentType(
            definition?.contentType ?? binary?.binaryContentType,
            selected,
            defaultMultipartContentType(schema),
            value,
          );
    return multipartPartValue(value, contentType, schema, definition, schemas, codecs);
  }

  function multipartBinarySchema(
    schema: WireSchema,
    value: unknown,
    schemas: WireSchemas,
    seen: ReadonlySet<WireSchema> = new Set(),
  ): WireSchema | undefined {
    if (seen.has(schema)) return undefined;
    if (schema.binaryInput === true) return schema;
    const nestedSeen: Set<WireSchema> = new Set(seen);
    nestedSeen.add(schema);
    const branches: WireSchema[] = [
      ...(schema.allOf ?? []),
      ...(schema.anyOf ?? []),
      ...(schema.oneOf ?? []),
    ];
    if (schema.reference !== undefined && schemas[schema.reference] !== undefined)
      branches.push(schemas[schema.reference]!);
    for (const branch of branches) {
      try {
        wire.validateWireValue(value, branch, schemas, "encode");
      } catch {
        continue;
      }
      const binary: WireSchema | undefined = multipartBinarySchema(
        branch,
        value,
        schemas,
        nestedSeen,
      );
      if (binary !== undefined) return binary;
    }
    return undefined;
  }

  async function encodeMultipartForm(
    fields: Record<string, unknown>,
    encoding: readonly WireEncodingDefinition[] | undefined,
    schema: WireSchema | undefined,
    schemas: WireSchemas,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    suppliedContentTypes: Readonly<Record<string, string>> | undefined,
  ): Promise<FormData> {
    const form: FormData = new FormData();
    for (const [name, field] of Object.entries(fields)) {
      if (field === undefined) continue;
      const definition: WireEncodingDefinition | undefined = encoding?.find(
        (entry: WireEncodingDefinition): boolean => entry.name === name,
      );
      const exploded: boolean = explodeMultipartField(field, definition);
      const values: readonly unknown[] = exploded ? (field as unknown[]) : [field];
      for (let index: number = 0; index < values.length; index++) {
        const part: MultipartPartValue = await namedMultipartPartValue(
          values[index],
          definition,
          multipartFieldSchema(schema, name, schemas, exploded ? index : undefined),
          schemas,
          codecs,
          suppliedContentTypes?.[name],
        );
        if (typeof part.body === "string" && part.contentType === undefined)
          form.append(name, part.body);
        else {
          const blob: Blob = new Blob(
            [part.body],
            part.contentType === undefined ? {} : { type: part.contentType },
          );
          if (part.filename === undefined) form.append(name, blob);
          else form.append(name, blob, part.filename);
        }
      }
    }
    return form;
  }

  function mediaRangeMatches(range: string, value: string): boolean {
    const [rangeType, rangeSubtype]: string[] = range.split("/", 2);
    const [valueType, valueSubtype]: string[] = value.split("/", 2);
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
    suppliedContentTypes: Readonly<Record<string, string>> | undefined,
  ): Promise<Blob> {
    const boundary: string = `----openapi-sdkgen-${multipartBoundaryToken()}`;
    const chunks: BlobPart[] = [];
    const definitions: Map<string | undefined, WireEncodingDefinition> = new Map(
      (encoding ?? []).map(
        (definition: WireEncodingDefinition): [string | undefined, WireEncodingDefinition] => [
          definition.name,
          definition,
        ],
      ),
    );
    for (const [name, fieldValue] of Object.entries(fields)) {
      if (fieldValue === undefined) continue;
      const definition: WireEncodingDefinition | undefined = definitions.get(name);
      const values: unknown[] = explodeMultipartField(fieldValue, definition)
        ? (fieldValue as unknown[])
        : [fieldValue];
      for (let index: number = 0; index < values.length; index++) {
        const item: unknown = values[index];
        const propertySchema: WireSchema = multipartFieldSchema(
          schema,
          name,
          schemas,
          explodeMultipartField(fieldValue, definition) ? index : undefined,
        );
        const { body, contentType, filename }: MultipartPartValue = await namedMultipartPartValue(
          item,
          definition,
          propertySchema,
          schemas,
          codecs,
          suppliedContentTypes?.[name],
        );
        const headers: string = await multipartPartHeaders(
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
  ): Promise<MultipartPartValue> {
    if (
      declaredContentType !== undefined &&
      normalizeMediaType(declaredContentType).startsWith("multipart/")
    ) {
      if (!Array.isArray(value)) throw new TypeError("nested multipart part must be an array");
      const nested: Blob = await encodePositionalMultipartBody(
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
      const file: File | undefined =
        typeof File !== "undefined" && value instanceof File ? value : undefined;
      const contentType: string | undefined = (declaredContentType ?? value.type) || undefined;
      return {
        body: value,
        ...(contentType === undefined ? {} : { contentType }),
        ...(file === undefined ? {} : { filename: file.name }),
      };
    }
    if (value instanceof ArrayBuffer)
      return { body: value, contentType: declaredContentType ?? "application/octet-stream" };
    if (ArrayBuffer.isView(value)) {
      const bytes: Uint8Array<ArrayBuffer> = new Uint8Array(value.byteLength);
      bytes.set(new Uint8Array(value.buffer, value.byteOffset, value.byteLength));
      return { body: bytes.buffer, contentType: declaredContentType ?? "application/octet-stream" };
    }
    if (typeof value === "string" && multipartBinarySchema(schema, value, schemas) !== undefined) {
      return { body: value, contentType: declaredContentType ?? "application/octet-stream" };
    }
    if (declaredContentType !== undefined && requiresMultipartPartCodec(declaredContentType)) {
      const codec: MediaCodec<unknown> | undefined = codecs.get(
        normalizeMediaType(declaredContentType),
      );
      if (codec?.encode === undefined)
        throw new TypeError(`missing encode codec for multipart part ${declaredContentType}`);
      return {
        body: multipartCodecBody(await codec.encode(value, { contentType: declaredContentType })),
        contentType: declaredContentType,
      };
    }
    if (declaredContentType !== undefined && isXMLMediaType(declaredContentType)) {
      return { body: xml.encodeXML(value, schema, schemas), contentType: declaredContentType };
    }
    if (
      (declaredContentType !== undefined && isJSONMediaType(declaredContentType)) ||
      isRecord(value) ||
      Array.isArray(value)
    ) {
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
    const normalized: string = normalizeMediaType(contentType);
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
    const headers: Headers = new Headers(supplied);
    const declared: Map<string, WireMultipartHeaderDefinition> = new Map(
      (definition?.headers ?? []).map(
        (header: WireMultipartHeaderDefinition): [string, WireMultipartHeaderDefinition] => [
          header.name.toLowerCase(),
          header,
        ],
      ),
    );
    for (const header of definition?.headers ?? []) {
      const value: string | null = headers.get(header.name);
      if (value === null && header.required)
        throw new TypeError(`missing required multipart header ${name}.${header.name}`);
      if (value !== null) await validateMultipartHeaderValue(name, header, value, schemas, codecs);
    }
    for (const [headerName] of headers) {
      const normalized: string = headerName.toLowerCase();
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
    const lines: string[] =
      name === undefined
        ? filename === undefined || headers.has("content-disposition")
          ? []
          : [`Content-Disposition: attachment; filename="${escapeMultipartToken(filename)}"`]
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
    const decoded: unknown = await decodeResponseHeaderValue(
      `${part ?? "position"}.${header.name}`,
      value,
      header.schema,
      header.contentType,
      header.explode,
      schemas,
      codecs,
      "multipart",
    );
    validateWireValue(decoded, header.schema, schemas, "decode");
  }

  function escapeMultipartToken(value: string): string {
    if (/\r|\n/.test(value))
      throw new TypeError("multipart names and filenames cannot contain line breaks");
    return value.replaceAll("\\", "\\\\").replaceAll('"', '\\"');
  }

  function multipartBoundaryToken(): string {
    const random: `${string}-${string}-${string}-${string}-${string}` =
      globalThis.crypto?.randomUUID?.();
    return random === undefined ? `${Date.now()}-${Math.random().toString(16).slice(2)}` : random;
  }

  return { encodeBody, encodeStream: encodeStreamingMultipartBody };
}
