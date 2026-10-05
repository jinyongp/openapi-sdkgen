import type { MediaCodec } from "../../media/media-codec-types.js";
import type { StreamCodec } from "../../stream/stream-protocol-types.js";
import type { WireBodyDefinition } from "../../media/media-contract-types.js";
import type { WireSchema, WireSchemas, WireCodec } from "../../schema/wire-types.js";
import type { ClientOptions } from "../configuration.js";
import {
  TransportErrorCode,
  isRecord,
  isJSONMediaType,
  isXMLMediaType,
} from "../../shared/runtime-support.js";
import type { OperationDefinition, ParameterDefinition } from "../operation.js";
import type { RequestOptions } from "../request.js";
import type { EncodedRequest, QueryPart, RequestExecutionServices } from "../http-types.js";
import type {
  EncodedStreamRequestBody,
  HTTPCodecExtensions,
} from "../../media/media-service-types.js";
import { normalizeMediaType, transportError } from "../../shared/runtime-support.js";
import {
  isPromise,
  isReadableStream,
  requireHTTPHook,
  resolveStreamCodec,
  resolveMaxStreamFrameBytes,
} from "../http-execution-support.js";
import { awaitAbortable } from "../../stream/stream-abort.js";
import {
  assertSafeOperationPath,
  selectRequestBodyDefinition,
  resolveOperationBaseURL,
  appendRawHeaders,
  setHeader,
  rejectUndefinedArrayValues,
  serializeSchemaPathParameter,
  createParameterEncoder,
  findParameterByProperty,
} from "./http-request-values.js";
import {
  createQueryParameterEncoder,
  assertQueryEmptyValueAllowed,
  appendQueryValue,
  serializeQuery,
} from "./http-query.js";
import { createResponseServices } from "../response/http-response-services.js";
import { createResponseHeaderDecoder } from "../response/http-response-headers.js";
import { serializeSimpleValue } from "./http-simple-value.js";
/** Shared parameter, header and response algorithms without an implicit advanced-media import. */
export function createHTTPServices(
  wire: WireCodec,
  extensions: HTTPCodecExtensions,
): RequestExecutionServices {
  const { transformWireValue }: WireCodec = wire;
  const appendQuerySync: ReturnType<typeof createQueryParameterEncoder> =
    createQueryParameterEncoder(wire, serializeContentParameterSync);
  const encodeParameterWireValue: ReturnType<typeof createParameterEncoder> =
    createParameterEncoder(wire);
  const {
    decodeResponse,
    decodeResponseHeaders,
    decodeResponseWireValue,
    createHTTPError,
  }: ReturnType<typeof createResponseServices> = createResponseServices(
    wire,
    extensions,
    createResponseHeaderDecoder(wire, extensions),
  );

  function encodeRequest(
    baseURL: string | undefined,
    client: ClientOptions,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    streamCodecs: ReadonlyMap<string, StreamCodec>,
    operation: OperationDefinition,
    input: unknown,
    options: RequestOptions,
  ): EncodedRequest | Promise<EncodedRequest> {
    const pending: EncodedRequest | Promise<EncodedRequest> = hasCustomParameterInput(
      operation,
      input,
    )
      ? encodeRequestAsync(baseURL, client, codecs, streamCodecs, operation, input, options)
      : encodeRequestSynchronous(baseURL, client, codecs, streamCodecs, operation, input, options);
    const finish: (encoded: EncodedRequest) => EncodedRequest = (
      encoded: EncodedRequest,
    ): EncodedRequest => {
      let result: EncodedRequest =
        options.authorization !== undefined ||
        client.authorization !== undefined ||
        options.csrfToken !== undefined
          ? { ...encoded, redirect: "error" as const }
          : encoded;
      if (isReadableStream(result.body)) {
        const tracked: TrackedRequestBody = trackRequestBodyStream(result.body);
        result = {
          ...result,
          body: tracked.body,
          bodyFailure: tracked.failure,
          bodyCancel: tracked.cancel,
        };
      }
      return result;
    };
    return isPromise(pending) ? pending.then(finish) : finish(pending);
  }

  function trackRequestBodyStream(source: ReadableStream<Uint8Array>): TrackedRequestBody {
    const reader: ReadableStreamDefaultReader<Uint8Array<ArrayBufferLike>> = source.getReader();
    let failure: unknown;
    let released: boolean = false;
    let cancelPromise: Promise<void> | undefined;
    const release: () => void = (): void => {
      if (released) return;
      released = true;
      reader.releaseLock();
    };
    const cancel: (reason?: unknown) => Promise<void> = (reason?: unknown): Promise<void> => {
      if (released) return Promise.resolve();
      cancelPromise ??= (async (): Promise<undefined> => {
        try {
          await reader.cancel(reason);
        } finally {
          release();
        }
      })();
      return cancelPromise;
    };
    return {
      body: new ReadableStream<Uint8Array>({
        async pull(
          controller: ReadableStreamDefaultController<Uint8Array<ArrayBufferLike>>,
        ): Promise<void> {
          try {
            const next: ReadableStreamReadResult<Uint8Array<ArrayBufferLike>> = await reader.read();
            if (next.done) {
              release();
              controller.close();
              return;
            }
            controller.enqueue(next.value);
          } catch (cause: unknown) {
            failure = cause;
            release();
            controller.error(cause);
          }
        },
        async cancel(reason: unknown): Promise<void> {
          await cancel(reason);
        },
      }),
      failure: (): unknown => failure,
      cancel,
    };
  }

  function hasCustomParameterInput(operation: OperationDefinition, input: unknown): boolean {
    const values: Record<string, unknown> = isRecord(input) ? input : {};
    for (const parameter of operation.parameters ?? []) {
      if (parameter.contentType === undefined || !requiresParameterCodec(parameter.contentType))
        continue;
      const source: unknown =
        parameter.location === "path"
          ? values["path"]
          : parameter.location === "header"
            ? values["headerParams"]
            : parameter.location === "cookie"
              ? values["cookieParams"]
              : parameter.location === "querystring"
                ? values["querystring"]
                : values["query"];
      if (isRecord(source) && source[parameter.property] !== undefined) return true;
    }
    return false;
  }

  function requiresParameterCodec(contentType: string): boolean {
    return (
      !isJSONMediaType(contentType) &&
      !isXMLMediaType(contentType) &&
      contentType.toLowerCase() !== "application/x-www-form-urlencoded" &&
      !contentType.toLowerCase().startsWith("text/")
    );
  }

  function encodeRequestSynchronous(
    baseURL: string | undefined,
    client: ClientOptions,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    streamCodecs: ReadonlyMap<string, StreamCodec>,
    operation: OperationDefinition,
    input: unknown,
    options: RequestOptions,
  ): EncodedRequest | Promise<EncodedRequest> {
    const values: Record<string, unknown> = isRecord(input) ? input : {};
    const pathValues: Record<string, unknown> = isRecord(values["path"]) ? values["path"] : {};
    rejectUndefinedArrayValues(pathValues);
    const path: string = operation.path.replaceAll(
      /\{([^}]+)\}/g,
      (_: string, name: string): string => {
        const parameter: ParameterDefinition | undefined = findParameter(operation, "path", name);
        const property: string = parameter?.property ?? name;
        const rawValue: unknown = pathValues[property];
        if (!Object.hasOwn(pathValues, property) || rawValue === undefined)
          throw new TypeError(`Missing path parameter ${name}`);
        return serializePathParameterSync(
          parameter,
          name,
          encodeParameterWireValue(operation, parameter, rawValue),
          operation.inputSchemas ?? {},
        );
      },
    );
    assertSafeOperationPath(path);
    const url: URL = new URL(
      resolveOperationBaseURL(options.baseURL ?? baseURL, client.origin, client.server, operation) +
        (path.startsWith("/") ? path : `/${path}`),
    );
    const queryValues: Record<string, unknown> = isRecord(values["query"]) ? values["query"] : {};
    const querystringValues: Record<string, unknown> = isRecord(values["querystring"])
      ? values["querystring"]
      : {};
    rejectUndefinedArrayValues(queryValues);
    rejectUndefinedArrayValues(querystringValues);
    const query: QueryPart[] = [
      ...appendQuerySync(queryValues, operation, "query"),
      ...appendQuerySync(querystringValues, operation, "querystring"),
    ];
    if (query.length > 0)
      url.search = `${url.search}${url.search === "" ? "?" : "&"}${serializeQuery(query)}`;
    const contractHeaderNames: Set<string> = new Set(
      [
        ...(operation.headerNames ?? []),
        ...(operation.parameters ?? [])
          .filter((parameter: ParameterDefinition): boolean => parameter.location === "header")
          .map((parameter: ParameterDefinition): string => parameter.name),
      ].map((name: string): string => name.toLowerCase()),
    );
    const headers: Headers = new Headers();
    appendRawHeaders(headers, client.headers, contractHeaderNames);
    appendRawHeaders(headers, options.headers, contractHeaderNames);
    const headerParams: Record<string, unknown> = {
      ...(isRecord(values["headerParams"]) ? values["headerParams"] : {}),
    };
    rejectUndefinedArrayValues(headerParams);
    for (const [property, value] of Object.entries(headerParams)) {
      if (value === undefined) continue;
      const parameter: ParameterDefinition | undefined = findParameterByProperty(
        operation,
        "header",
        property,
      );
      const name: string = parameter?.name ?? property;
      const serialized: string =
        parameter?.contentType === undefined
          ? serializeSimpleValue(
              encodeParameterWireValue(operation, parameter, value),
              parameter?.explode ?? false,
            )
          : serializeContentParameterSync(
              encodeParameterWireValue(operation, parameter, value),
              parameter.contentType,
              parameter.schema,
              operation.inputSchemas ?? {},
            );
      headers.set(name, serialized);
    }
    setHeader(headers, "Authorization", options.authorization ?? client.authorization);
    setHeader(headers, "Accept", options.accept);
    setHeader(headers, "X-CSRF-Token", options.csrfToken);
    setHeader(headers, "X-Request-Id", options.requestID);
    const cookieValues: Record<string, unknown> = isRecord(values["cookieParams"])
      ? values["cookieParams"]
      : {};
    rejectUndefinedArrayValues(cookieValues);
    assertRequiredParameters(
      operation,
      pathValues,
      queryValues,
      querystringValues,
      headerParams,
      cookieValues,
    );
    const cookies: string[] = Object.entries(cookieValues)
      .filter((entry: [string, unknown]): entry is [string, unknown] => entry[1] !== undefined)
      .flatMap(([property, value]: [string, unknown]): string[] =>
        serializeCookieSync(operation, property, value),
      );
    if (cookies.length > 0) {
      if (!client.transport?.capabilities?.cookieJar)
        throw transportError(
          TransportErrorCode.TRANSPORT_CAPABILITY_REQUIRED,
          "Sending declared cookie parameters requires a cookie-jar transport",
          undefined,
        );
      headers.set("Cookie", cookies.join("; "));
    }
    if (!Object.hasOwn(values, "body") || values["body"] === undefined) {
      if (operation.requestBodyRequired) throw new TypeError("Missing required request body");
      return { url: url.href, headers };
    }
    rejectUndefinedArrayValues(values["body"]);
    let contentType: string = operation.contentType ?? "application/json";
    let bodyValue: unknown = values["body"];
    const requestBodies: readonly WireBodyDefinition[] | undefined = operation.requestBodies;
    const needsSelection: boolean =
      requestBodies !== undefined &&
      (requestBodies.length > 1 ||
        requestBodies.some((body: WireBodyDefinition): boolean => body.contentType.includes("*")));
    if (needsSelection) {
      if (
        !isRecord(values["body"]) ||
        typeof values["body"]["contentType"] !== "string" ||
        !Object.hasOwn(values["body"], "value")
      )
        throw new TypeError("request body media range requires { contentType, value }");
      const selected: WireBodyDefinition | undefined = selectRequestBodyDefinition(
        requestBodies!,
        values["body"]["contentType"],
      );
      if (selected === undefined)
        throw new TypeError(
          `request body content type ${values["body"]["contentType"]} is not declared by this operation`,
        );
      contentType = values["body"]["contentType"];
      bodyValue = values["body"]["value"];
    }
    const definition: WireBodyDefinition | undefined =
      requestBodies === undefined
        ? undefined
        : selectRequestBodyDefinition(requestBodies, contentType);
    const streamSource: AsyncIterable<unknown> | undefined =
      definition?.itemSchema !== undefined && isStreamSource(bodyValue)
        ? normalizeStreamSource(bodyValue, options.signal)
        : undefined;
    if (
      definition?.itemSchema !== undefined &&
      streamSource === undefined &&
      definition.schemaDeclared !== true
    )
      throw new TypeError("streaming request body must be a StreamSource");
    const selectedStreamCodec: StreamCodec<unknown, unknown> | undefined = resolveStreamCodec(
      contentType,
      options.streamCodec,
      streamCodecs,
    );
    const finishStream: (encoded: EncodedStreamRequestBody) => EncodedRequest = (
      encoded: EncodedStreamRequestBody,
    ): EncodedRequest => {
      headers.set("Content-Type", encoded.contentType);
      return { url: url.href, headers, body: encoded.body };
    };
    if (definition?.itemSchema !== undefined && streamSource !== undefined) {
      const stream: EncodedStreamRequestBody | Promise<EncodedStreamRequestBody> = requireHTTPHook(
        extensions.encodeIncrementalStreamRequestBody,
      )(streamSource, {
        contentType,
        streamFraming: definition.streamFraming,
        itemSchema: definition.itemSchema,
        schemas: operation.inputSchemas ?? {},
        streamCodec: selectedStreamCodec,
        maxFrameBytes: resolveMaxStreamFrameBytes(
          options.maxStreamFrameBytes ?? client.maxStreamFrameBytes,
        ),
        signal: options.signal,
        itemEncoding: definition.itemEncoding,
        suppliedHeaders: options.multipartHeaders,
        suppliedContentTypes: options.multipartContentTypes,
        codecs,
      });
      return isPromise(stream) ? stream.then(finishStream) : finishStream(stream);
    }
    if (
      streamSource === undefined &&
      definition?.schemaDeclared === true &&
      definition.streamFraming !== undefined
    ) {
      const stream: EncodedStreamRequestBody | Promise<EncodedStreamRequestBody> = requireHTTPHook(
        extensions.encodeCompleteSequentialRequestBody,
      )(bodyValue, {
        contentType,
        streamFraming: definition.streamFraming,
        schema: definition.schema,
        schemas: operation.inputSchemas ?? {},
        streamCodec: selectedStreamCodec,
        maxFrameBytes: resolveMaxStreamFrameBytes(
          options.maxStreamFrameBytes ?? client.maxStreamFrameBytes,
        ),
        signal: options.signal,
        prefixEncoding: definition.prefixEncoding,
        itemEncoding: definition.itemEncoding,
        suppliedHeaders: options.multipartHeaders,
        suppliedContentTypes: options.multipartContentTypes,
        codecs,
      });
      return isPromise(stream) ? stream.then(finishStream) : finishStream(stream);
    }
    const body: BodyInit | Promise<BodyInit> = extensions.encodeRequestBody(
      contentType,
      encodeRequestWireValue(operation, contentType, bodyValue),
      codecs,
      definition?.schema,
      operation.inputSchemas ?? {},
      definition,
      options.multipartHeaders,
      options.multipartContentTypes,
    );
    const finish: (resolved: BodyInit | ReadableStream<Uint8Array>) => EncodedRequest = (
      resolved: BodyInit | ReadableStream<Uint8Array>,
    ): EncodedRequest => {
      if (!(resolved instanceof FormData))
        headers.set(
          "Content-Type",
          normalizeMediaType(contentType).startsWith("multipart/") && resolved instanceof Blob
            ? resolved.type
            : contentType,
        );
      return { url: url.href, headers, body: resolved };
    };
    return isPromise(body) ? body.then(finish) : finish(body);
  }

  async function encodeRequestAsync(
    baseURL: string | undefined,
    client: ClientOptions,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    streamCodecs: ReadonlyMap<string, StreamCodec>,
    operation: OperationDefinition,
    input: unknown,
    options: RequestOptions,
  ): Promise<EncodedRequest> {
    const values: Record<string, unknown> = isRecord(input) ? input : {};
    const pathValues: Record<string, unknown> = isRecord(values["path"]) ? values["path"] : {};
    rejectUndefinedArrayValues(pathValues);
    let path: string = operation.path;
    for (const match of operation.path.matchAll(/\{([^}]+)\}/g)) {
      const name: string = match[1]!;
      const parameter: ParameterDefinition | undefined = findParameter(operation, "path", name);
      const property: string = parameter?.property ?? name;
      const rawValue: unknown = pathValues[property];
      if (!Object.hasOwn(pathValues, property) || rawValue === undefined) {
        throw new TypeError(`Missing path parameter ${name}`);
      }
      const value: unknown = encodeParameterWireValue(operation, parameter, rawValue);
      path = path.replace(
        match[0],
        await serializePathParameter(parameter, name, value, operation.inputSchemas ?? {}, codecs),
      );
    }
    assertSafeOperationPath(path);
    const operationBaseURL: string = resolveOperationBaseURL(
      options.baseURL ?? baseURL,
      client.origin,
      client.server,
      operation,
    );
    const url: URL = new URL(operationBaseURL + (path.startsWith("/") ? path : `/${path}`));
    const queryValues: Record<string, unknown> = isRecord(values["query"]) ? values["query"] : {};
    const querystringValues: Record<string, unknown> = isRecord(values["querystring"])
      ? values["querystring"]
      : {};
    rejectUndefinedArrayValues(queryValues);
    rejectUndefinedArrayValues(querystringValues);
    const query: QueryPart[] = [
      ...(await appendQuery(queryValues, operation, codecs, "query")),
      ...(await appendQuery(querystringValues, operation, codecs, "querystring")),
    ];
    if (query.length > 0)
      url.search = `${url.search}${url.search === "" ? "?" : "&"}${serializeQuery(query)}`;

    const contractHeaderNames: Set<string> = new Set(
      [
        ...(operation.headerNames ?? []),
        ...(operation.parameters ?? [])
          .filter((parameter: ParameterDefinition): boolean => parameter.location === "header")
          .map((parameter: ParameterDefinition): string => parameter.name),
      ].map((name: string): string => name.toLowerCase()),
    );
    const headers: Headers = new Headers();
    appendRawHeaders(headers, client.headers, contractHeaderNames);
    appendRawHeaders(headers, options.headers, contractHeaderNames);

    const headerParams: Record<string, unknown> = {
      ...(isRecord(values["headerParams"]) ? values["headerParams"] : {}),
    };
    rejectUndefinedArrayValues(headerParams);
    for (const [property, value] of Object.entries(headerParams)) {
      if (value === undefined) continue;
      const parameter: ParameterDefinition | undefined = findParameterByProperty(
        operation,
        "header",
        property,
      );
      const name: string = parameter?.name ?? property;
      const encodedValue: unknown = encodeParameterWireValue(operation, parameter, value);
      const serialized: string =
        parameter?.contentType === undefined
          ? serializeSimpleValue(encodedValue, parameter?.explode ?? false)
          : await serializeContentParameter(
              encodedValue,
              parameter.contentType,
              parameter.schema,
              operation.inputSchemas ?? {},
              codecs,
            );
      headers.set(name, serialized);
    }
    setHeader(headers, "Authorization", options.authorization ?? client.authorization);
    setHeader(headers, "Accept", options.accept);
    setHeader(headers, "X-CSRF-Token", options.csrfToken);
    setHeader(headers, "X-Request-Id", options.requestID);

    const cookieValues: Record<string, unknown> = isRecord(values["cookieParams"])
      ? values["cookieParams"]
      : {};
    rejectUndefinedArrayValues(cookieValues);
    assertRequiredParameters(
      operation,
      pathValues,
      queryValues,
      querystringValues,
      headerParams,
      cookieValues,
    );
    const cookiePromises: Promise<string[]>[] = Object.entries(cookieValues)
      .filter((entry: [string, unknown]): entry is [string, unknown] => entry[1] !== undefined)
      .map(async ([property, value]: [string, unknown]): Promise<string[]> =>
        serializeCookie(operation, property, value, codecs),
      );
    const cookies: string[] = (await Promise.all(cookiePromises)).flat();
    if (cookies.length > 0) {
      if (!client.transport?.capabilities?.cookieJar) {
        throw transportError(
          TransportErrorCode.TRANSPORT_CAPABILITY_REQUIRED,
          "Sending declared cookie parameters requires a cookie-jar transport",
          undefined,
        );
      }
      headers.set("Cookie", cookies.join("; "));
    }

    if (!Object.hasOwn(values, "body") || values["body"] === undefined) {
      if (operation.requestBodyRequired) throw new TypeError("Missing required request body");
      return { url: url.href, headers };
    }
    rejectUndefinedArrayValues(values["body"]);
    let contentType: string = operation.contentType ?? "application/json";
    let bodyValue: unknown = values["body"];
    const requestBodies: readonly WireBodyDefinition[] | undefined = operation.requestBodies;
    const needsSelection: boolean =
      requestBodies !== undefined &&
      (requestBodies.length > 1 ||
        requestBodies.some((body: WireBodyDefinition): boolean => body.contentType.includes("*")));
    if (needsSelection) {
      if (
        !isRecord(values["body"]) ||
        typeof values["body"]["contentType"] !== "string" ||
        !Object.hasOwn(values["body"], "value")
      )
        throw new TypeError("request body media range requires { contentType, value }");
      const selected: WireBodyDefinition | undefined = selectRequestBodyDefinition(
        requestBodies!,
        values["body"]["contentType"],
      );
      if (selected === undefined)
        throw new TypeError(
          `request body content type ${values["body"]["contentType"]} is not declared by this operation`,
        );
      contentType = values["body"]["contentType"];
      bodyValue = values["body"]["value"];
    }
    const definition: WireBodyDefinition | undefined =
      requestBodies === undefined
        ? undefined
        : selectRequestBodyDefinition(requestBodies, contentType);
    const streamSource: AsyncIterable<unknown> | undefined =
      definition?.itemSchema !== undefined && isStreamSource(bodyValue)
        ? normalizeStreamSource(bodyValue, options.signal)
        : undefined;
    if (
      definition?.itemSchema !== undefined &&
      streamSource === undefined &&
      definition.schemaDeclared !== true
    )
      throw new TypeError("streaming request body must be a StreamSource");
    const selectedStreamCodec: StreamCodec<unknown, unknown> | undefined = resolveStreamCodec(
      contentType,
      options.streamCodec,
      streamCodecs,
    );
    const finishStream: (encoded: EncodedStreamRequestBody) => EncodedRequest = (
      encoded: EncodedStreamRequestBody,
    ): EncodedRequest => {
      headers.set("Content-Type", encoded.contentType);
      return { url: url.href, headers, body: encoded.body };
    };
    if (definition?.itemSchema !== undefined && streamSource !== undefined) {
      const stream: EncodedStreamRequestBody = await requireHTTPHook(
        extensions.encodeIncrementalStreamRequestBody,
      )(streamSource, {
        contentType,
        streamFraming: definition.streamFraming,
        itemSchema: definition.itemSchema,
        schemas: operation.inputSchemas ?? {},
        streamCodec: selectedStreamCodec,
        maxFrameBytes: resolveMaxStreamFrameBytes(
          options.maxStreamFrameBytes ?? client.maxStreamFrameBytes,
        ),
        signal: options.signal,
        itemEncoding: definition.itemEncoding,
        suppliedHeaders: options.multipartHeaders,
        suppliedContentTypes: options.multipartContentTypes,
        codecs,
      });
      return finishStream(stream);
    }
    if (
      streamSource === undefined &&
      definition?.schemaDeclared === true &&
      definition.streamFraming !== undefined
    ) {
      const stream: EncodedStreamRequestBody = await requireHTTPHook(
        extensions.encodeCompleteSequentialRequestBody,
      )(bodyValue, {
        contentType,
        streamFraming: definition.streamFraming,
        schema: definition.schema,
        schemas: operation.inputSchemas ?? {},
        streamCodec: selectedStreamCodec,
        maxFrameBytes: resolveMaxStreamFrameBytes(
          options.maxStreamFrameBytes ?? client.maxStreamFrameBytes,
        ),
        signal: options.signal,
        prefixEncoding: definition.prefixEncoding,
        itemEncoding: definition.itemEncoding,
        suppliedHeaders: options.multipartHeaders,
        suppliedContentTypes: options.multipartContentTypes,
        codecs,
      });
      return finishStream(stream);
    }
    const body: BodyInit | Promise<BodyInit> = extensions.encodeRequestBody(
      contentType,
      encodeRequestWireValue(operation, contentType, bodyValue),
      codecs,
      definition?.schema,
      operation.inputSchemas ?? {},
      definition,
      options.multipartHeaders,
      options.multipartContentTypes,
    );
    const finish: (resolved: BodyInit | ReadableStream<Uint8Array>) => EncodedRequest = (
      resolved: BodyInit | ReadableStream<Uint8Array>,
    ): EncodedRequest => {
      if (!(resolved instanceof FormData)) {
        const resolvedContentType: string =
          normalizeMediaType(contentType).startsWith("multipart/") && resolved instanceof Blob
            ? resolved.type
            : contentType;
        headers.set("Content-Type", resolvedContentType);
      }
      return { url: url.href, headers, body: resolved };
    };
    return finish(await body);
  }

  function assertRequiredParameters(
    operation: OperationDefinition,
    pathValues: Record<string, unknown>,
    queryValues: Record<string, unknown>,
    querystringValues: Record<string, unknown>,
    headerValues: Record<string, unknown>,
    cookieValues: Record<string, unknown>,
  ): void {
    for (const parameter of operation.parameters ?? []) {
      if (!parameter.required) continue;
      const values: Record<string, unknown> =
        parameter.location === "path"
          ? pathValues
          : parameter.location === "query"
            ? queryValues
            : parameter.location === "querystring"
              ? querystringValues
              : parameter.location === "header"
                ? headerValues
                : cookieValues;
      if (!Object.hasOwn(values, parameter.property) || values[parameter.property] === undefined) {
        throw new TypeError(`Missing required ${parameter.location} parameter ${parameter.name}`);
      }
    }
  }

  /** Encodes a validated wire value using the OpenAPI XML Object rules. */
  function encodeRequestWireValue(
    operation: OperationDefinition,
    contentType: string,
    value: unknown,
  ): unknown {
    const definition: WireBodyDefinition | undefined =
      operation.requestBodies === undefined
        ? undefined
        : selectRequestBodyDefinition(operation.requestBodies, contentType);
    return definition === undefined
      ? value
      : transformWireValue(value, definition.schema, operation.inputSchemas ?? {}, "encode");
  }

  async function appendQuery(
    query: Readonly<Record<string, unknown>>,
    operation: OperationDefinition,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    location: "query" | "querystring",
  ): Promise<QueryPart[]> {
    const result: QueryPart[] = [];
    for (const [property, value] of Object.entries(query)) {
      if (value === undefined) continue;
      const parameter: ParameterDefinition | undefined = findParameterByProperty(
        operation,
        location,
        property,
      );
      if (parameter?.location === "querystring") {
        await appendQuerystring(
          result,
          encodeParameterWireValue(operation, parameter, value),
          parameter,
          operation.inputSchemas ?? {},
          codecs,
        );
        continue;
      }
      await appendQueryParameter(
        result,
        parameter?.name ?? property,
        encodeParameterWireValue(operation, parameter, value),
        parameter,
        operation.inputSchemas ?? {},
        codecs,
      );
    }
    return result;
  }

  async function appendQueryParameter(
    query: QueryPart[],
    name: string,
    value: unknown,
    parameter: ParameterDefinition | undefined,
    components: WireSchemas,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
  ): Promise<void> {
    assertQueryEmptyValueAllowed(value, parameter);
    if (parameter?.contentType !== undefined) {
      appendQueryValue(
        query,
        name,
        await serializeContentParameter(
          value,
          parameter.contentType,
          parameter.schema,
          components,
          codecs,
        ),
        parameter?.allowReserved ?? false,
      );
      return;
    }
    const style: string = parameter?.style ?? "form";
    const explode: boolean = parameter?.explode ?? true;
    if (style === "deepObject" && isRecord(value)) {
      for (const [key, item] of Object.entries(value)) {
        if (item !== undefined)
          appendQueryValue(query, `${name}[${key}]`, item, parameter?.allowReserved ?? false);
      }
      return;
    }
    if (Array.isArray(value)) {
      if (style === "spaceDelimited")
        appendQueryValue(
          query,
          name,
          value.map(String).join(" "),
          parameter?.allowReserved ?? false,
        );
      else if (style === "pipeDelimited")
        appendQueryValue(
          query,
          name,
          value.map(String).join("|"),
          parameter?.allowReserved ?? false,
        );
      else if (explode)
        for (const item of value)
          appendQueryValue(query, name, item, parameter?.allowReserved ?? false);
      else
        appendQueryValue(
          query,
          name,
          value.map(String).join(","),
          parameter?.allowReserved ?? false,
        );
      return;
    }
    if (isRecord(value) && style === "form") {
      const entries: [string, unknown][] = Object.entries(value).filter(
        (entry: [string, unknown]): boolean => entry[1] !== undefined,
      );
      if (explode)
        for (const [key, item] of entries)
          appendQueryValue(query, key, item, parameter?.allowReserved ?? false);
      else
        appendQueryValue(
          query,
          name,
          entries
            .flatMap(([key, item]: [string, unknown]): string[] => [key, String(item)])
            .join(","),
          parameter?.allowReserved ?? false,
        );
      return;
    }
    if (isRecord(value) && (style === "spaceDelimited" || style === "pipeDelimited")) {
      const separator: " " | "|" = style === "spaceDelimited" ? " " : "|";
      const entries: [string, unknown][] = Object.entries(value).filter(
        (entry: [string, unknown]): boolean => entry[1] !== undefined,
      );
      const serialized: string = explode
        ? entries
            .map(([key, item]: [string, unknown]): string => `${key}=${String(item)}`)
            .join(separator)
        : entries
            .flatMap(([key, item]: [string, unknown]): string[] => [key, String(item)])
            .join(separator);
      appendQueryValue(query, name, serialized, parameter?.allowReserved ?? false);
      return;
    }
    appendQueryValue(query, name, value, parameter?.allowReserved ?? false);
  }

  async function appendQuerystring(
    query: QueryPart[],
    value: unknown,
    parameter: ParameterDefinition,
    components: WireSchemas,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
  ): Promise<void> {
    const contentType: string | undefined = parameter.contentType?.toLowerCase();
    if (contentType === "application/x-www-form-urlencoded") {
      if (!isRecord(value)) throw new TypeError("querystring form content must be an object");
      for (const [name, item] of Object.entries(value)) {
        if (item === undefined) continue;
        if (Array.isArray(item))
          for (const entry of item) query.push({ name, value: String(entry) });
        else query.push({ name, value: String(item) });
      }
      return;
    }
    if (contentType === "application/json") {
      query.push({ raw: encodeURIComponent(JSON.stringify(value)) });
      return;
    }
    query.push({
      raw: encodeURIComponent(
        await serializeContentParameter(
          value,
          parameter.contentType ?? "text/plain",
          parameter.schema,
          components,
          codecs,
        ),
      ),
    });
  }

  function findParameter(
    operation: OperationDefinition,
    location: ParameterDefinition["location"],
    name: string,
  ): ParameterDefinition | undefined {
    return operation.parameters?.find(
      (parameter: ParameterDefinition): boolean =>
        parameter.location === location && parameter.name === name,
    );
  }

  async function serializePathParameter(
    parameter: ParameterDefinition | undefined,
    name: string,
    value: unknown,
    components: WireSchemas,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
  ): Promise<string> {
    if (parameter?.contentType !== undefined) {
      return encodeURIComponent(
        await serializeContentParameter(
          value,
          parameter.contentType,
          parameter.schema,
          components,
          codecs,
        ),
      );
    }
    return serializeSchemaPathParameter(parameter, name, value);
  }

  function serializePathParameterSync(
    parameter: ParameterDefinition | undefined,
    name: string,
    value: unknown,
    components: WireSchemas,
  ): string {
    if (parameter?.contentType !== undefined)
      return encodeURIComponent(
        serializeContentParameterSync(value, parameter.contentType, parameter.schema, components),
      );
    return serializeSchemaPathParameter(parameter, name, value);
  }

  async function serializeContentParameter(
    value: unknown,
    contentType: string,
    schema: WireSchema | undefined = undefined,
    components: WireSchemas = {},
    codecs: ReadonlyMap<string, MediaCodec<unknown>> = new Map(),
  ): Promise<string> {
    if (isJSONMediaType(contentType)) return JSON.stringify(value);
    if (isXMLMediaType(contentType))
      return requireHTTPHook(extensions.encodeXML)(value, schema ?? {}, components);
    if (contentType.toLowerCase() === "application/x-www-form-urlencoded") {
      if (!isRecord(value)) return String(value);
      const form: URLSearchParams = new URLSearchParams();
      for (const [name, item] of Object.entries(value)) {
        if (item === undefined) continue;
        if (Array.isArray(item)) for (const entry of item) form.append(name, String(entry));
        else form.append(name, String(item));
      }
      return form.toString();
    }
    if (contentType.toLowerCase().startsWith("text/")) return String(value);
    const codec: MediaCodec<unknown> | undefined = codecs.get(normalizeMediaType(contentType));
    if (codec?.encodeParameter === undefined)
      throw new TypeError(`missing parameter encode codec for ${contentType}`);
    return await codec.encodeParameter(value, { contentType });
  }

  function serializeContentParameterSync(
    value: unknown,
    contentType: string,
    schema: WireSchema | undefined = undefined,
    components: WireSchemas = {},
  ): string {
    if (isJSONMediaType(contentType)) return JSON.stringify(value);
    if (isXMLMediaType(contentType))
      return requireHTTPHook(extensions.encodeXML)(value, schema ?? {}, components);
    if (contentType.toLowerCase() === "application/x-www-form-urlencoded") {
      if (!isRecord(value)) return String(value);
      const form: URLSearchParams = new URLSearchParams();
      for (const [name, item] of Object.entries(value)) {
        if (item === undefined) continue;
        if (Array.isArray(item)) for (const entry of item) form.append(name, String(entry));
        else form.append(name, String(item));
      }
      return form.toString();
    }
    if (contentType.toLowerCase().startsWith("text/")) return String(value);
    throw new TypeError(`missing parameter encode codec for ${contentType}`);
  }

  async function serializeCookie(
    operation: OperationDefinition,
    property: string,
    value: unknown,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
  ): Promise<string[]> {
    const parameter: ParameterDefinition | undefined = findParameterByProperty(
      operation,
      "cookie",
      property,
    );
    const name: string = parameter?.name ?? property;
    const preserve: boolean = parameter?.style === "cookie";
    const pair: (key: string, item: unknown) => string = (key: string, item: unknown): string =>
      `${preserve ? key : encodeURIComponent(key)}=${preserve ? String(item ?? "") : encodeURIComponent(String(item ?? ""))}`;
    value = encodeParameterWireValue(operation, parameter, value);
    if (parameter?.contentType !== undefined) {
      return [
        pair(
          name,
          await serializeContentParameter(
            value,
            parameter.contentType,
            parameter.schema,
            operation.inputSchemas ?? {},
            codecs,
          ),
        ),
      ];
    }
    if (Array.isArray(value)) {
      if (parameter?.explode ?? true) {
        return value.map((item: unknown): string => pair(name, item));
      }
      return [pair(name, value.map(String).join(","))];
    }
    if (isRecord(value) && (parameter?.explode ?? true)) {
      return Object.entries(value)
        .filter((entry: [string, unknown]): boolean => entry[1] !== undefined)
        .map(([key, item]: [string, unknown]): string => pair(key, item));
    }
    return [pair(name, serializeSimpleValue(value, false))];
  }

  function serializeCookieSync(
    operation: OperationDefinition,
    property: string,
    value: unknown,
  ): string[] {
    const parameter: ParameterDefinition | undefined = findParameterByProperty(
      operation,
      "cookie",
      property,
    );
    const name: string = parameter?.name ?? property;
    const preserve: boolean = parameter?.style === "cookie";
    const pair: (key: string, item: unknown) => string = (key: string, item: unknown): string =>
      `${preserve ? key : encodeURIComponent(key)}=${preserve ? String(item ?? "") : encodeURIComponent(String(item ?? ""))}`;
    value = encodeParameterWireValue(operation, parameter, value);
    if (parameter?.contentType !== undefined)
      return [
        pair(
          name,
          serializeContentParameterSync(
            value,
            parameter.contentType,
            parameter.schema,
            operation.inputSchemas ?? {},
          ),
        ),
      ];
    if (Array.isArray(value))
      return (parameter?.explode ?? true)
        ? value.map((item: unknown): string => pair(name, item))
        : [pair(name, value.map(String).join(","))];
    if (isRecord(value) && (parameter?.explode ?? true))
      return Object.entries(value)
        .filter((entry: [string, unknown]): boolean => entry[1] !== undefined)
        .map(([key, item]: [string, unknown]): string => pair(key, item));
    return [pair(name, serializeSimpleValue(value, false))];
  }

  function isStreamSource(
    value: unknown,
  ): value is AsyncIterable<unknown> | ReadableStream<unknown> {
    return isAsyncIterable(value) || isReadableStreamLike(value);
  }

  function normalizeStreamSource(
    source: AsyncIterable<unknown> | ReadableStream<unknown>,
    signal: AbortSignal | undefined,
  ): AsyncIterable<unknown> {
    return {
      [Symbol.asyncIterator](): AsyncIterator<unknown> {
        const iterator: AsyncIterator<unknown, unknown, unknown> = isAsyncIterable(source)
          ? source[Symbol.asyncIterator]()
          : readableStreamIterator(source);
        let done: boolean = false;
        return {
          async next(): Promise<IteratorResult<unknown>> {
            if (done) return { done: true, value: undefined };
            try {
              const next: IteratorResult<unknown, unknown> = await awaitAbortable(
                Promise.resolve(iterator.next()),
                signal,
              );
              if (done || next.done) {
                done = true;
                return { done: true, value: undefined };
              }
              return next;
            } catch (cause: unknown) {
              if (signal?.aborted && !done) {
                done = true;
                void Promise.resolve(iterator.return?.(signal.reason)).catch(
                  (): undefined => undefined,
                );
              }
              throw cause;
            }
          },
          async return(reason?: unknown): Promise<IteratorResult<unknown>> {
            if (done) return { done: true, value: undefined };
            done = true;
            const close: Promise<IteratorResult<unknown, unknown> | undefined> = Promise.resolve(
              iterator.return?.(reason),
            );
            if (signal?.aborted) void close.catch((): undefined => undefined);
            else await close.catch((): undefined => undefined);
            return { done: true, value: undefined };
          },
        };
      },
    };
  }

  function readableStreamIterator(source: ReadableStream<unknown>): AsyncIterator<unknown> {
    const reader: ReadableStreamDefaultReader<unknown> = source.getReader();
    let released: boolean = false;
    const release: () => void = (): void => {
      if (released) return;
      released = true;
      reader.releaseLock();
    };
    return {
      async next(): Promise<IteratorResult<unknown>> {
        try {
          const next: ReadableStreamReadResult<unknown> = await reader.read();
          if (next.done) {
            release();
            return { done: true, value: undefined };
          }
          return { done: false, value: next.value };
        } catch (cause: unknown) {
          release();
          throw cause;
        }
      },
      async return(reason?: unknown): Promise<IteratorResult<unknown>> {
        try {
          await reader.cancel(reason);
        } finally {
          release();
        }
        return { done: true, value: undefined };
      },
    };
  }

  function isReadableStreamLike(value: unknown): value is ReadableStream<unknown> {
    return (
      value !== null &&
      typeof value === "object" &&
      typeof (value as ReadableStream<unknown>).getReader === "function"
    );
  }

  function isAsyncIterable(value: unknown): value is AsyncIterable<unknown> {
    return (
      value !== null &&
      typeof value === "object" &&
      typeof (value as AsyncIterable<unknown>)[Symbol.asyncIterator] === "function"
    );
  }
  return {
    encodeRequest,
    decodeResponse,
    decodeResponseHeaders,
    decodeResponseWireValue,
    createHTTPError,
  };
}

type TrackedRequestBody = {
  readonly body: ReadableStream<Uint8Array>;
  readonly failure: () => unknown;
  readonly cancel: (reason?: unknown) => Promise<void>;
};
