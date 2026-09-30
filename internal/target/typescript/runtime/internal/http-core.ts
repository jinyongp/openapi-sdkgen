import type {
  MediaCodec,
  StreamCodec,
  WireBodyDefinition,
  WireResponseDefinition,
  WireSchema,
  WireSchemas,
} from "./wire-engine.js";
import type { ClientOptions, SecurityCredentialContext } from "./configuration.js";
import { APIError, TransportErrorCode, isAPIError } from "./runtime-support.js";
import type { TransportError } from "./runtime-support.js";
import { defineOwnDataProperty, isRecord } from "./runtime-support.js";
import { operationDiagnosticName } from "./runtime-support.js";
import type { OperationDefinition, ParameterDefinition, ServerSelection } from "./operation.js";
import type { OperationStream, RawResponse, RequestMetadata, RequestOptions } from "./request.js";
import type {
  APIKeyCredential,
  HTTPBasicCredential,
  HTTPBearerCredential,
  HTTPCredential,
  OAuthCredential,
  SecurityCredential,
  SecurityCredentials,
  SecurityRequirementDefinition,
  SecuritySchemeDefinition,
} from "./security.js";
import type { Transport } from "./transport.js";
import type {
  AbortContext,
  EncodedRequest,
  EncodedStreamRequestBody,
  HTTPCodecExtensions,
  OperationRequestOptions,
  QueryPart,
  RequestContext,
  RequestExecutionServices,
  ResponseDecodeOptions,
  SDKSecuritySource,
  StreamingRequestExecutionServices,
} from "./http-types.js";
import { isJSONMediaType, isXMLMediaType } from "./runtime-support.js";
import type { WireCodec } from "./wire-engine.js";
import type { BufferedRequestFunction, RequestFunction } from "./callables.js";

/** Applies the selected OpenAPI security requirement without sharing credential state across requests. */
export function applyOperationSecurity(
  options: ClientOptions,
  operation: OperationDefinition,
  encoded: EncodedRequest,
  requestOptions: OperationRequestOptions,
  credentials: RequestCredentials | undefined,
): EncodedRequest | Promise<EncodedRequest> {
  const declared = operation.security;
  const requestedID = requestOptions.securityRequirement;
  if (declared === undefined || declared.length === 0) {
    if (requestedID !== undefined)
      throw securityRequirementInvalid(
        "The operation does not declare an OpenAPI security requirement",
      );
    return encoded;
  }
  const requirements = Object.create(null) as Record<string, SecurityRequirementDefinition>;
  for (const requirement of declared)
    defineOwnDataProperty(requirements, requirement.id, requirement);
  let selected: SecurityRequirementDefinition;
  if (declared.length === 1) {
    if (requestedID !== undefined) {
      throw securityRequirementInvalid(
        `Operation ${operationDiagnosticName(operation)} has one SDK-selected security requirement and does not accept an explicit selection`,
      );
    }
    selected = declared[0]!;
  } else {
    if (requestedID === undefined) {
      throw transportError(
        TransportErrorCode.SECURITY_REQUIREMENT_REQUIRED,
        `Operation ${operationDiagnosticName(operation)} requires an explicit OpenAPI security requirement`,
        undefined,
      );
    }
    const requested = requirements[requestedID];
    if (requested === undefined) {
      throw securityRequirementInvalid(
        `Operation ${operationDiagnosticName(operation)} does not declare security requirement ${requestedID}`,
      );
    }
    selected = requested;
  }
  if (
    securityRequirementIsSatisfied(options, requestOptions, encoded, credentials, selected, true)
  ) {
    return applySelectedSecurityRequirement(
      options,
      requestOptions,
      encoded,
      credentials,
      selected,
      {},
      false,
    );
  }
  if (typeof options.securityProvider !== "function") {
    return applySelectedSecurityRequirement(
      options,
      requestOptions,
      encoded,
      credentials,
      selected,
      {},
      false,
    );
  }
  const context: SecurityCredentialContext = {
    operation: {
      route: operation.route,
      ...(operation.operationID === undefined ? {} : { operationID: operation.operationID }),
      method: operation.method,
      path: operation.path,
    },
    requirement: selected,
    origin: new URL(encoded.url).origin,
    ...(requestOptions.signal === undefined ? {} : { signal: requestOptions.signal }),
  };
  const suppliedCredentials = options.securityProvider(context);
  const apply = (resolved: SecurityCredentials): EncodedRequest =>
    applySelectedSecurityRequirement(
      options,
      requestOptions,
      encoded,
      credentials,
      selected,
      resolved,
      true,
    );
  return isPromise(suppliedCredentials)
    ? suppliedCredentials.then(apply)
    : apply(suppliedCredentials);
}

function securityRequirementIsSatisfied(
  options: ClientOptions,
  requestOptions: RequestOptions,
  encoded: EncodedRequest,
  credentials: RequestCredentials | undefined,
  requirement: SecurityRequirementDefinition,
  allowMutualTLS: boolean,
): boolean {
  return requirement.schemes.every(
    (scheme) =>
      securitySourceForScheme(options, requestOptions, encoded, credentials, scheme, allowMutualTLS)
        .state === "satisfied",
  );
}

function securitySourceForScheme(
  options: ClientOptions,
  requestOptions: RequestOptions,
  encoded: EncodedRequest,
  credentials: RequestCredentials | undefined,
  scheme: SecuritySchemeDefinition,
  allowMutualTLS: boolean,
): SDKSecuritySource {
  if (usesAuthorizationHeader(scheme)) {
    if (requestOptions.authorization === undefined && options.authorization === undefined)
      return { state: "none" };
    const value = encoded.headers.get("Authorization") ?? "";
    return matchesAuthorizationScheme(scheme, value)
      ? { state: "satisfied", kind: "header", name: "Authorization", value }
      : { state: "conflict", location: "Authorization header" };
  }
  if (isCSRFHeaderScheme(scheme)) {
    if (requestOptions.csrfToken === undefined) return { state: "none" };
    const value = encoded.headers.get("X-CSRF-Token") ?? "";
    return value === ""
      ? { state: "conflict", location: "X-CSRF-Token header" }
      : { state: "satisfied", kind: "header", name: "X-CSRF-Token", value };
  }
  if (scheme.type === "apiKey" && scheme.location === "cookie" && credentials === "include") {
    return { state: "satisfied", kind: "cookie" };
  }
  if (scheme.type === "mutualTLS" && allowMutualTLS && options.transport?.capabilities?.mutualTLS) {
    return { state: "satisfied", kind: "mutualTLS" };
  }
  return { state: "none" };
}

function usesAuthorizationHeader(scheme: SecuritySchemeDefinition): boolean {
  return (
    scheme.type === "http" ||
    scheme.type === "oauth2" ||
    scheme.type === "openIdConnect" ||
    (scheme.type === "apiKey" &&
      scheme.location === "header" &&
      scheme.parameterName?.toLowerCase() === "authorization")
  );
}

function isCSRFHeaderScheme(scheme: SecuritySchemeDefinition): boolean {
  return (
    scheme.type === "apiKey" &&
    scheme.location === "header" &&
    scheme.parameterName?.toLowerCase() === "x-csrf-token"
  );
}

function matchesAuthorizationScheme(scheme: SecuritySchemeDefinition, value: string): boolean {
  if (value === "") return false;
  if (scheme.type === "apiKey") return true;
  const separator = value.indexOf(" ");
  if (separator <= 0 || value.slice(separator + 1).trim() === "") return false;
  const protocol = value.slice(0, separator).toLowerCase();
  if (scheme.type === "oauth2" || scheme.type === "openIdConnect") return protocol === "bearer";
  return protocol === scheme.scheme?.toLowerCase();
}

function applySelectedSecurityRequirement(
  options: ClientOptions,
  requestOptions: RequestOptions,
  encoded: EncodedRequest,
  credentialsMode: RequestCredentials | undefined,
  requirement: SecurityRequirementDefinition,
  suppliedCredentials: unknown,
  providerReturned: boolean,
): EncodedRequest {
  if (!isRecord(suppliedCredentials)) {
    throw transportError(
      TransportErrorCode.SECURITY_CREDENTIALS_INVALID,
      "Security credential provider returned an invalid credentials object",
      undefined,
    );
  }
  const declaredNames = new Set(requirement.schemes.map((scheme) => scheme.name));
  if (Object.keys(suppliedCredentials).some((name) => !declaredNames.has(name))) {
    throw transportError(
      TransportErrorCode.SECURITY_CREDENTIALS_INVALID,
      "Security credential provider returned credentials outside the selected requirement",
      undefined,
    );
  }
  const url = new URL(encoded.url);
  for (const scheme of requirement.schemes) {
    const source = securitySourceForScheme(
      options,
      requestOptions,
      encoded,
      credentialsMode,
      scheme,
      true,
    );
    const credential = suppliedCredentials[scheme.name] as SecurityCredential | undefined;
    if (source.state === "conflict") throw securityCollision(scheme.name, source.location);
    if (source.state === "satisfied") {
      if (credential === undefined) continue;
      if (source.kind === "header") {
        const header = securityCredentialHeader(scheme, credential);
        if (
          header !== undefined &&
          header.name.toLowerCase() === source.name.toLowerCase() &&
          normalizeHeaderValue(header.name, header.value) === source.value
        )
          continue;
        throw securityCollision(scheme.name, source.name);
      }
      if (source.kind === "mutualTLS") {
        assertSecurityCredentialShape(scheme, credential);
        continue;
      }
      assertSecurityCredentialShape(scheme, credential);
      throw securityCollision(scheme.name, "ambient Cookie credentials");
    }
    if (credential === undefined) {
      if (providerReturned) {
        throw transportError(
          TransportErrorCode.SECURITY_CREDENTIALS_INVALID,
          `Security credential provider omitted scheme ${scheme.name}`,
          undefined,
        );
      }
      throw transportError(
        TransportErrorCode.SECURITY_CREDENTIALS_REQUIRED,
        `Security requirement ${requirement.id} requires credentials for scheme ${scheme.name}`,
        undefined,
      );
    }
    applySecurityCredential(options.transport, scheme, credential, encoded.headers, url);
  }
  return {
    ...encoded,
    url: url.href,
    ...(requirement.schemes.length === 0 ? {} : { redirect: "error" as const }),
  };
}

function applySecurityCredential(
  transport: Transport | undefined,
  scheme: SecuritySchemeDefinition,
  credential: SecurityCredential,
  headers: Headers,
  url: URL,
): void {
  const header = securityCredentialHeader(scheme, credential);
  if (header !== undefined) {
    if (headers.has(header.name)) throw securityCollision(scheme.name, `header ${header.name}`);
    headers.set(header.name, header.value);
    return;
  }
  switch (scheme.type) {
    case "apiKey": {
      const apiKey = credential as APIKeyCredential;
      if (scheme.location === "query") {
        if (url.searchParams.has(scheme.parameterName!))
          throw securityCollision(scheme.name, `query parameter ${scheme.parameterName}`);
        url.searchParams.set(scheme.parameterName!, apiKey.value);
        return;
      }
      if (!transport?.capabilities?.cookieJar) {
        throw transportError(
          TransportErrorCode.TRANSPORT_CAPABILITY_REQUIRED,
          `Security scheme ${scheme.name} requires a cookie-jar transport`,
          undefined,
        );
      }
      if (headers.has("Cookie")) throw securityCollision(scheme.name, "Cookie header");
      headers.set(
        "Cookie",
        `${encodeURIComponent(scheme.parameterName!)}=${encodeURIComponent(apiKey.value)}`,
      );
      return;
    }
    case "http":
    case "oauth2":
    case "openIdConnect":
      return;
    case "mutualTLS":
      if (!transport?.capabilities?.mutualTLS) {
        throw transportError(
          TransportErrorCode.TRANSPORT_CAPABILITY_REQUIRED,
          `Security scheme ${scheme.name} requires a mutual-TLS transport`,
          undefined,
        );
      }
      return;
  }
}

function securityCredentialHeader(
  scheme: SecuritySchemeDefinition,
  credential: SecurityCredential,
): { readonly name: string; readonly value: string } | undefined {
  assertSecurityCredentialShape(scheme, credential);
  if (scheme.type === "apiKey") {
    const apiKey = credential as APIKeyCredential;
    return scheme.location === "header"
      ? { name: scheme.parameterName!, value: apiKey.value }
      : undefined;
  }
  if (scheme.type === "http") {
    if (scheme.scheme === "basic") {
      const basic = credential as HTTPBasicCredential;
      return {
        name: "Authorization",
        value: `Basic ${base64(`${basic.username}:${basic.password}`)}`,
      };
    }
    if (scheme.scheme === "bearer")
      return {
        name: "Authorization",
        value: `Bearer ${(credential as HTTPBearerCredential).token}`,
      };
    return {
      name: "Authorization",
      value: `${scheme.scheme} ${(credential as HTTPCredential).value}`,
    };
  }
  if (scheme.type === "oauth2" || scheme.type === "openIdConnect") {
    return { name: "Authorization", value: `Bearer ${(credential as OAuthCredential).token}` };
  }
  return undefined;
}

function assertSecurityCredentialShape(
  scheme: SecuritySchemeDefinition,
  credential: SecurityCredential,
): void {
  if (scheme.type === "apiKey") {
    if (
      credential?.kind !== "api-key" ||
      typeof credential.value !== "string" ||
      credential.value === ""
    )
      throw securityCredentialError(scheme.name, "api-key value");
    return;
  }
  if (scheme.type === "http") {
    if (scheme.scheme === "basic") {
      if (
        credential?.kind !== "http-basic" ||
        typeof credential.username !== "string" ||
        typeof credential.password !== "string"
      )
        throw securityCredentialError(scheme.name, "http-basic credential");
      return;
    }
    if (scheme.scheme === "bearer") {
      if (
        credential?.kind !== "http-bearer" ||
        typeof credential.token !== "string" ||
        credential.token === ""
      )
        throw securityCredentialError(scheme.name, "http-bearer token");
      return;
    }
    if (
      credential?.kind !== "http" ||
      typeof credential.value !== "string" ||
      credential.value === ""
    )
      throw securityCredentialError(scheme.name, "http credential");
    return;
  }
  if (scheme.type === "oauth2" || scheme.type === "openIdConnect") {
    if (
      credential?.kind !== scheme.type ||
      typeof credential.token !== "string" ||
      credential.token === ""
    )
      throw securityCredentialError(scheme.name, `${scheme.type} token`);
    return;
  }
  if (credential?.kind !== "mutual-tls")
    throw securityCredentialError(scheme.name, "mutual-tls credential");
}

function normalizeHeaderValue(name: string, value: string): string {
  const headers = new Headers();
  headers.set(name, value);
  return headers.get(name)!;
}

function securityRequirementInvalid(message: string): TransportError {
  return transportError(TransportErrorCode.SECURITY_REQUIREMENT_INVALID, message, undefined);
}

function securityCredentialError(scheme: string, expected: string): TransportError {
  return transportError(
    TransportErrorCode.SECURITY_CREDENTIALS_INVALID,
    `Security scheme ${scheme} requires ${expected}`,
    undefined,
  );
}

function securityCollision(scheme: string, location: string): TransportError {
  return transportError(
    TransportErrorCode.SECURITY_CREDENTIALS_INVALID,
    `Security scheme ${scheme} conflicts with caller-supplied ${location}`,
    undefined,
  );
}

function base64(value: string): string {
  const bytes = new TextEncoder().encode(value);
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary);
}

/** Checks transport capabilities required by declared response headers. */
export function assertReadableResponseHeaders(
  transport: Transport | undefined,
  operation: OperationDefinition,
): void {
  const readable = transport?.capabilities?.readableResponseHeaders;
  for (const response of operation.responses ?? []) {
    for (const header of response.headers ?? []) {
      if (!header.required || header.name.toLowerCase() !== "set-cookie") continue;
      if (
        readable === true ||
        (Array.isArray(readable) && readable.some((name) => name.toLowerCase() === "set-cookie"))
      )
        continue;
      throw transportError(
        TransportErrorCode.TRANSPORT_CAPABILITY_REQUIRED,
        "Reading required Set-Cookie response headers requires a capable transport",
        undefined,
      );
    }
  }
}

/** Finalizes a request body source while preserving the original request failure. */
export async function cancelTrackedRequestBody(
  cancel: ((reason?: unknown) => Promise<void>) | undefined,
  reason?: unknown,
): Promise<void> {
  if (cancel === undefined) return;
  await cancel(reason).catch(() => undefined);
}

/** Selects the most specific declared response for the actual status and media type. */
export function selectResponseDefinition(
  operation: OperationDefinition,
  response: Response,
  requireMediaMatch: boolean,
): WireResponseDefinition | undefined {
  const contentType = responseContentType(response);
  return operation.responses
    ?.filter((item) => {
      if (!statusMatches(item.status, response.status)) return false;
      if (!requireMediaMatch) return true;
      return contentType === undefined
        ? item.contentType === ""
        : mediaTypeMatches(item.contentType, contentType);
    })
    .sort((left, right) => {
      const statusDifference =
        statusMatchScore(right.status, response.status) -
        statusMatchScore(left.status, response.status);
      if (statusDifference !== 0) return statusDifference;
      return (
        mediaTypeMatchScore(right.contentType, contentType) -
        mediaTypeMatchScore(left.contentType, contentType)
      );
    })[0];
}

function statusMatches(pattern: string, status: number): boolean {
  if (pattern === String(status) || pattern === "default") return true;
  return /^\dXX$/i.test(pattern) && Number(pattern[0]) === Math.floor(status / 100);
}

function statusMatchScore(pattern: string, status: number): number {
  if (pattern === String(status)) return 3;
  if (/^\dXX$/i.test(pattern) && Number(pattern[0]) === Math.floor(status / 100)) return 2;
  return pattern === "default" ? 1 : 0;
}

/** Matches a concrete media type against an OpenAPI media range. */
export function mediaTypeMatches(pattern: string, actual: string): boolean {
  const expected = pattern.split(";", 1)[0]?.trim().toLowerCase() ?? "";
  const received = actual.split(";", 1)[0]?.trim().toLowerCase() ?? "";
  if (expected === received || expected === "*/*") return true;
  const [expectedType, expectedSubtype] = expected.split("/", 2);
  const [receivedType, receivedSubtype] = received.split("/", 2);
  if (
    expectedType === undefined ||
    expectedSubtype === undefined ||
    receivedType === undefined ||
    receivedSubtype === undefined
  )
    return false;
  if (expectedType !== "*" && expectedType !== receivedType) return false;
  if (expectedSubtype === "*") return true;
  if (expectedSubtype.startsWith("*+")) return receivedSubtype.endsWith(expectedSubtype.slice(1));
  return false;
}

/** Ranks exact, suffix-wildcard and general media matches. */
export function mediaTypeMatchScore(pattern: string, actual: string | undefined): number {
  if (actual === undefined) return 0;
  const normalized = normalizeMediaType(pattern);
  if (normalized === normalizeMediaType(actual)) return 3;
  if (normalized.includes("*+")) return 2;
  if (normalized.includes("*")) return 1;
  return 0;
}

/** Validates and normalizes the request base URL without accepting query or fragment components. */
export function normalizeBaseURL(value: string): string {
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    throw new TypeError("baseURL must be an absolute URL");
  }
  if ((url.protocol !== "http:" && url.protocol !== "https:") || url.search || url.hash) {
    throw new TypeError("baseURL must be an absolute http(s) URL without query or fragment");
  }
  url.pathname = url.pathname.replace(/\/+$/, "");
  return url.href.replace(/\/$/, "");
}

/** Combines request cancellation and timeout into request-local cleanup state. */
export function createAbortContext(
  signal: AbortSignal | undefined,
  timeoutMS: number | undefined,
  alwaysCreateSignal = false,
): AbortContext {
  if (timeoutMS !== undefined && (!Number.isFinite(timeoutMS) || timeoutMS <= 0)) {
    throw new TypeError("timeoutMS must be a positive finite number");
  }
  if (signal === undefined && timeoutMS === undefined && !alwaysCreateSignal) {
    return {
      signal: undefined,
      timedOut: () => false,
      aborted: () => false,
      cancel: () => undefined,
      cleanup: () => undefined,
    };
  }
  const controller = new AbortController();
  let timeoutReached = false;
  const forwardAbort = (): void => controller.abort(signal?.reason);
  if (signal?.aborted) forwardAbort();
  else signal?.addEventListener("abort", forwardAbort, { once: true });
  const timer =
    timeoutMS === undefined
      ? undefined
      : setTimeout(() => {
          timeoutReached = true;
          controller.abort();
        }, timeoutMS);
  return {
    signal: controller.signal,
    timedOut: () => timeoutReached,
    aborted: () => signal?.aborted === true,
    cancel: (reason?: unknown) => controller.abort(reason),
    cleanup: () => {
      if (timer !== undefined) clearTimeout(timer);
      signal?.removeEventListener("abort", forwardAbort);
    },
  };
}

/** Returns the normalized media type from the response Content-Type header. */
export function responseContentType(response: Response): string | undefined {
  return response.headers.get("content-type")?.split(";", 1)[0]?.trim().toLowerCase();
}

function normalizeCodecs(
  codecs: Readonly<Record<string, MediaCodec<unknown>>> | undefined,
): ReadonlyMap<string, MediaCodec<unknown>> {
  const result = new Map<string, MediaCodec<unknown>>();
  for (const [contentType, codec] of Object.entries(codecs ?? {})) {
    const normalized = normalizeMediaType(contentType);
    if (normalized === "" || result.has(normalized))
      throw new TypeError(`duplicate or invalid media codec ${contentType}`);
    result.set(normalized, codec);
  }
  return result;
}

function normalizeStreamCodecs(
  codecs: Readonly<Record<string, StreamCodec>> | undefined,
): ReadonlyMap<string, StreamCodec> {
  const result = new Map<string, StreamCodec>();
  for (const [contentType, codec] of Object.entries(codecs ?? {})) {
    const normalized = normalizeMediaType(contentType);
    if (normalized === "" || result.has(normalized))
      throw new TypeError(`duplicate or invalid stream codec ${contentType}`);
    result.set(normalized, codec);
  }
  return result;
}

/** Normalizes a media type for matching, excluding parameters. */
export function normalizeMediaType(contentType: string): string {
  return contentType.split(";", 1)[0]?.trim().toLowerCase() ?? "";
}

/** Recognizes a thenable returned by asynchronous request services. */
export function isPromise<Value>(value: Value | Promise<Value>): value is Promise<Value> {
  return typeof (value as Promise<Value>)?.then === "function";
}

/** Recognizes a byte stream used as a Fetch request body. */
export function isReadableStream(value: unknown): value is ReadableStream<Uint8Array> {
  return (
    value !== null &&
    typeof value === "object" &&
    typeof (value as ReadableStream<Uint8Array>).getReader === "function"
  );
}

/** Preserves response body and metadata in an API failure. */
export function serverError(response: Response, request: RequestMetadata, body: unknown): APIError {
  const envelope = isRecord(body) && isRecord(body.error) ? body.error : body;
  const error = isRecord(envelope) ? envelope : {};
  const code = typeof error.code === "string" ? error.code : `HTTP_${response.status}`;
  const message =
    typeof error.message === "string"
      ? error.message
      : typeof body === "string" && body.trim() !== ""
        ? body
        : `HTTP request failed with status ${response.status}`;
  return new APIError({
    code,
    message,
    request,
    status: response.status,
    details: error.details ?? error.fields,
    fields: error.fields,
    data: body,
    response,
  });
}

/** Captures request identifiers exposed by the response headers. */
export function requestMetadata(response: Response): RequestMetadata {
  const id = response.headers.get("x-request-id");
  return id === null ? {} : { id };
}

/** Constructs a typed transport error preserving the underlying cause. */
export function transportError(
  code: TransportErrorCode,
  message: string,
  cause: unknown,
): TransportError {
  return new APIError({ code, message, cause });
}

/** Constructs a transport failure retaining available request and response metadata. */
export function transportErrorFromCause(
  code: TransportErrorCode,
  message: string,
  cause: unknown,
  responseMetadata?: { request: RequestMetadata; status: number; response: Response },
): TransportError {
  if (isAPIError(cause)) {
    return new APIError({
      code,
      message,
      cause,
      request: cause.request,
      ...(cause.status === undefined ? {} : { status: cause.status }),
      ...(cause.response === undefined ? {} : { response: cause.response }),
    });
  }
  if (responseMetadata !== undefined) {
    return new APIError({ code, message, cause, ...responseMetadata });
  }
  return transportError(code, message, cause);
}

/** Cancels an outstanding response body without replacing the original request error. */
export async function cancelResponseBody(
  response: Response | undefined,
  reason?: unknown,
): Promise<void> {
  const body = response?.body;
  if (body === null || body === undefined || typeof body.cancel !== "function" || body.locked)
    return;
  await body.cancel(reason).catch(() => undefined);
}

/** Stops waiting when the request aborts and removes cancellation listeners after settlement. */
export function awaitAbortable<Value>(
  value: Promise<Value>,
  signal: AbortSignal | undefined,
): Promise<Value> {
  if (signal === undefined) return value;
  if (signal.aborted) {
    void value.catch(() => undefined);
    return Promise.reject(signal.reason);
  }
  return new Promise((resolve, reject) => {
    const onAbort = (): void => reject(signal.reason);
    signal.addEventListener("abort", onAbort, { once: true });
    value.then(
      (result) => {
        signal.removeEventListener("abort", onAbort);
        resolve(result);
      },
      (cause) => {
        signal.removeEventListener("abort", onAbort);
        reject(cause);
      },
    );
  });
}

/** Normalizes client defaults once; keeps request callbacks and mutable client state instance-local. */
export function createRequestContext(options: ClientOptions): RequestContext {
  const baseURL = options.baseURL === undefined ? undefined : normalizeBaseURL(options.baseURL);
  const fetchImplementation = options.transport?.fetch ?? options.fetch ?? globalThis.fetch;
  if (typeof fetchImplementation !== "function") {
    throw new TypeError("fetch is unavailable; pass ClientOptions.fetch");
  }
  const codecs = normalizeCodecs(options.codecs);
  const streamCodecs = normalizeStreamCodecs(options.streamCodecs);

  return { options, baseURL, fetchImplementation, codecs, streamCodecs };
}

/** Shared parameter, header and response algorithms without an implicit advanced-media import. */
export function createHTTPServices(
  wire: WireCodec,
  extensions: HTTPCodecExtensions,
): RequestExecutionServices {
  const { decodeWireValue, transformWireValue, validateWireValue } = wire;
  const reservedHeaders = /* @__PURE__ */ new Set([
    "accept",
    "authorization",
    "content-type",
    "x-csrf-token",
    "x-request-id",
  ]);

  const tolerantResponseTransformOptions = { unknownProperties: "preserve" } as const;

  function resolveMaxStreamFrameBytes(value: number | undefined): number {
    const resolved = value ?? 1024 * 1024;
    if (!Number.isSafeInteger(resolved) || resolved <= 0)
      throw new TypeError("maxStreamFrameBytes must be a positive safe integer");
    return resolved;
  }

  async function decodeResponseHeaders(
    operation: OperationDefinition,
    response: Response,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
  ): Promise<Readonly<Record<string, unknown>>> {
    const definition = selectResponseDefinition(operation, response, false);
    const values = Object.create(null) as Record<string, unknown>;
    for (const header of definition?.headers ?? []) {
      const value = response.headers.get(header.name);
      if (value === null) {
        if (header.required) throw new TypeError(`missing required response header ${header.name}`);
        continue;
      }
      const decoded = await decodeResponseHeaderValue(
        header.name,
        value,
        header.schema,
        header.contentType,
        header.explode,
        operation.outputSchemas ?? {},
        codecs,
      );
      validateWireValue(decoded, header.schema, operation.outputSchemas ?? {}, "decode");
      defineOwnDataProperty(
        values,
        header.property,
        decodeWireValue(decoded, header.schema, operation.outputSchemas ?? {}),
      );
    }
    return values;
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
      if (isXMLMediaType(contentType))
        return requireHTTPHook(extensions.decodeXML)(value, schema, schemas);
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

  function encodeRequest(
    baseURL: string | undefined,
    client: ClientOptions,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    streamCodecs: ReadonlyMap<string, StreamCodec>,
    operation: OperationDefinition,
    input: unknown,
    options: RequestOptions,
  ): EncodedRequest | Promise<EncodedRequest> {
    const pending = hasCustomParameterInput(operation, input)
      ? encodeRequestAsync(baseURL, client, codecs, streamCodecs, operation, input, options)
      : encodeRequestSynchronous(baseURL, client, codecs, streamCodecs, operation, input, options);
    const finish = (encoded: EncodedRequest): EncodedRequest => {
      let result =
        options.authorization !== undefined ||
        client.authorization !== undefined ||
        options.csrfToken !== undefined
          ? { ...encoded, redirect: "error" as const }
          : encoded;
      if (isReadableStream(result.body)) {
        const tracked = trackRequestBodyStream(result.body);
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

  function trackRequestBodyStream(source: ReadableStream<Uint8Array>): {
    readonly body: ReadableStream<Uint8Array>;
    readonly failure: () => unknown;
    readonly cancel: (reason?: unknown) => Promise<void>;
  } {
    const reader = source.getReader();
    let failure: unknown;
    let released = false;
    let cancelPromise: Promise<void> | undefined;
    const release = (): void => {
      if (released) return;
      released = true;
      reader.releaseLock();
    };
    const cancel = (reason?: unknown): Promise<void> => {
      if (released) return Promise.resolve();
      cancelPromise ??= (async () => {
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
        async pull(controller) {
          try {
            const next = await reader.read();
            if (next.done) {
              release();
              controller.close();
              return;
            }
            controller.enqueue(next.value);
          } catch (cause) {
            failure = cause;
            release();
            controller.error(cause);
          }
        },
        async cancel(reason) {
          await cancel(reason);
        },
      }),
      failure: () => failure,
      cancel,
    };
  }

  function hasCustomParameterInput(operation: OperationDefinition, input: unknown): boolean {
    const values = isRecord(input) ? input : {};
    for (const parameter of operation.parameters ?? []) {
      if (parameter.contentType === undefined || !requiresParameterCodec(parameter.contentType))
        continue;
      const source =
        parameter.location === "path"
          ? values.path
          : parameter.location === "header"
            ? values.headerParams
            : parameter.location === "cookie"
              ? values.cookieParams
              : parameter.location === "querystring"
                ? values.querystring
                : values.query;
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

  function assertSafeOperationPath(path: string): void {
    for (const segment of path.split("/")) {
      const dots = segment.toLowerCase().replaceAll("%2e", ".");
      if (dots === "." || dots === "..")
        throw new TypeError(
          "Operation path contains a URL dot-segment after parameter serialization",
        );
    }
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
    const values = isRecord(input) ? input : {};
    const pathValues = isRecord(values.path) ? values.path : {};
    rejectUndefinedArrayValues(pathValues);
    const path = operation.path.replaceAll(/\{([^}]+)\}/g, (_, name: string) => {
      const parameter = findParameter(operation, "path", name);
      const property = parameter?.property ?? name;
      const rawValue = pathValues[property];
      if (rawValue === undefined || rawValue === null)
        throw new TypeError(`Missing path parameter ${name}`);
      return serializePathParameterSync(
        parameter,
        name,
        encodeParameterWireValue(operation, parameter, rawValue),
        operation.inputSchemas ?? {},
      );
    });
    assertSafeOperationPath(path);
    const url = new URL(
      resolveOperationBaseURL(options.baseURL ?? baseURL, client.origin, client.server, operation) +
        (path.startsWith("/") ? path : `/${path}`),
    );
    const queryValues = isRecord(values.query) ? values.query : {};
    const querystringValues = isRecord(values.querystring) ? values.querystring : {};
    rejectUndefinedArrayValues(queryValues);
    rejectUndefinedArrayValues(querystringValues);
    const query = [
      ...appendQuerySync(queryValues, operation, "query"),
      ...appendQuerySync(querystringValues, operation, "querystring"),
    ];
    if (query.length > 0)
      url.search = `${url.search}${url.search === "" ? "?" : "&"}${serializeQuery(query)}`;
    const contractHeaderNames = new Set(
      [
        ...(operation.headerNames ?? []),
        ...(operation.parameters ?? [])
          .filter((parameter) => parameter.location === "header")
          .map((parameter) => parameter.name),
      ].map((name) => name.toLowerCase()),
    );
    const headers = new Headers();
    appendRawHeaders(headers, client.headers, contractHeaderNames);
    appendRawHeaders(headers, options.headers, contractHeaderNames);
    const headerParams = { ...(isRecord(values.headerParams) ? values.headerParams : {}) };
    rejectUndefinedArrayValues(headerParams);
    for (const [property, value] of Object.entries(headerParams)) {
      if (value === undefined) continue;
      const parameter = findParameterByProperty(operation, "header", property);
      const name = parameter?.name ?? property;
      const serialized =
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
    const cookieValues = isRecord(values.cookieParams) ? values.cookieParams : {};
    rejectUndefinedArrayValues(cookieValues);
    assertRequiredParameters(
      operation,
      pathValues,
      queryValues,
      querystringValues,
      headerParams,
      cookieValues,
    );
    const cookies = Object.entries(cookieValues)
      .filter((entry): entry is [string, unknown] => entry[1] !== undefined)
      .flatMap(([property, value]) => serializeCookieSync(operation, property, value));
    if (cookies.length > 0) {
      if (!client.transport?.capabilities?.cookieJar)
        throw transportError(
          TransportErrorCode.TRANSPORT_CAPABILITY_REQUIRED,
          "Sending declared cookie parameters requires a cookie-jar transport",
          undefined,
        );
      headers.set("Cookie", cookies.join("; "));
    }
    if (!Object.hasOwn(values, "body") || values.body === undefined) {
      if (operation.requestBodyRequired) throw new TypeError("Missing required request body");
      return { url: url.href, headers };
    }
    rejectUndefinedArrayValues(values.body);
    let contentType = operation.contentType ?? "application/json";
    let bodyValue: unknown = values.body;
    const requestBodies = operation.requestBodies;
    const needsSelection =
      requestBodies !== undefined &&
      (requestBodies.length > 1 || requestBodies.some((body) => body.contentType.includes("*")));
    if (needsSelection) {
      if (
        !isRecord(values.body) ||
        typeof values.body.contentType !== "string" ||
        !Object.hasOwn(values.body, "value")
      )
        throw new TypeError("request body media range requires { contentType, value }");
      const selected = selectRequestBodyDefinition(requestBodies!, values.body.contentType);
      if (selected === undefined)
        throw new TypeError(
          `request body content type ${values.body.contentType} is not declared by this operation`,
        );
      contentType = values.body.contentType;
      bodyValue = values.body.value;
    }
    const definition =
      requestBodies === undefined
        ? undefined
        : selectRequestBodyDefinition(requestBodies, contentType);
    const streamSource =
      definition?.itemSchema !== undefined && isStreamSource(bodyValue)
        ? normalizeStreamSource(bodyValue, options.signal)
        : undefined;
    if (
      definition?.itemSchema !== undefined &&
      streamSource === undefined &&
      definition.schemaDeclared !== true
    )
      throw new TypeError("streaming request body must be a StreamSource");
    const selectedStreamCodec = resolveStreamCodec(contentType, options.streamCodec, streamCodecs);
    const finishStream = (encoded: EncodedStreamRequestBody): EncodedRequest => {
      headers.set("Content-Type", encoded.contentType);
      return { url: url.href, headers, body: encoded.body };
    };
    if (definition?.itemSchema !== undefined && streamSource !== undefined) {
      const stream = requireHTTPHook(extensions.encodeIncrementalStreamRequestBody)(streamSource, {
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
      const stream = requireHTTPHook(extensions.encodeCompleteSequentialRequestBody)(bodyValue, {
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
    const body = extensions.encodeRequestBody(
      contentType,
      encodeRequestWireValue(operation, contentType, bodyValue),
      codecs,
      definition?.schema,
      operation.inputSchemas ?? {},
      definition,
      options.multipartHeaders,
      options.multipartContentTypes,
    );
    const finish = (resolved: BodyInit | ReadableStream<Uint8Array>): EncodedRequest => {
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
    const values = isRecord(input) ? input : {};
    const pathValues = isRecord(values.path) ? values.path : {};
    rejectUndefinedArrayValues(pathValues);
    let path = operation.path;
    for (const match of operation.path.matchAll(/\{([^}]+)\}/g)) {
      const name = match[1]!;
      const parameter = findParameter(operation, "path", name);
      const property = parameter?.property ?? name;
      const rawValue = pathValues[property];
      if (rawValue === undefined || rawValue === null) {
        throw new TypeError(`Missing path parameter ${name}`);
      }
      const value = encodeParameterWireValue(operation, parameter, rawValue);
      path = path.replace(
        match[0],
        await serializePathParameter(parameter, name, value, operation.inputSchemas ?? {}, codecs),
      );
    }
    assertSafeOperationPath(path);
    const operationBaseURL = resolveOperationBaseURL(
      options.baseURL ?? baseURL,
      client.origin,
      client.server,
      operation,
    );
    const url = new URL(operationBaseURL + (path.startsWith("/") ? path : `/${path}`));
    const queryValues = isRecord(values.query) ? values.query : {};
    const querystringValues = isRecord(values.querystring) ? values.querystring : {};
    rejectUndefinedArrayValues(queryValues);
    rejectUndefinedArrayValues(querystringValues);
    const query = [
      ...(await appendQuery(queryValues, operation, codecs, "query")),
      ...(await appendQuery(querystringValues, operation, codecs, "querystring")),
    ];
    if (query.length > 0)
      url.search = `${url.search}${url.search === "" ? "?" : "&"}${serializeQuery(query)}`;

    const contractHeaderNames = new Set(
      [
        ...(operation.headerNames ?? []),
        ...(operation.parameters ?? [])
          .filter((parameter) => parameter.location === "header")
          .map((parameter) => parameter.name),
      ].map((name) => name.toLowerCase()),
    );
    const headers = new Headers();
    appendRawHeaders(headers, client.headers, contractHeaderNames);
    appendRawHeaders(headers, options.headers, contractHeaderNames);

    const headerParams = {
      ...(isRecord(values.headerParams) ? values.headerParams : {}),
    };
    rejectUndefinedArrayValues(headerParams);
    for (const [property, value] of Object.entries(headerParams)) {
      if (value === undefined) continue;
      const parameter = findParameterByProperty(operation, "header", property);
      const name = parameter?.name ?? property;
      const encodedValue = encodeParameterWireValue(operation, parameter, value);
      const serialized =
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

    const cookieValues = isRecord(values.cookieParams) ? values.cookieParams : {};
    rejectUndefinedArrayValues(cookieValues);
    assertRequiredParameters(
      operation,
      pathValues,
      queryValues,
      querystringValues,
      headerParams,
      cookieValues,
    );
    const cookiePromises = Object.entries(cookieValues)
      .filter((entry): entry is [string, unknown] => entry[1] !== undefined)
      .map(async ([property, value]) => serializeCookie(operation, property, value, codecs));
    const cookies = (await Promise.all(cookiePromises)).flat();
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

    if (!Object.hasOwn(values, "body") || values.body === undefined) {
      if (operation.requestBodyRequired) throw new TypeError("Missing required request body");
      return { url: url.href, headers };
    }
    rejectUndefinedArrayValues(values.body);
    let contentType = operation.contentType ?? "application/json";
    let bodyValue: unknown = values.body;
    const requestBodies = operation.requestBodies;
    const needsSelection =
      requestBodies !== undefined &&
      (requestBodies.length > 1 || requestBodies.some((body) => body.contentType.includes("*")));
    if (needsSelection) {
      if (
        !isRecord(values.body) ||
        typeof values.body.contentType !== "string" ||
        !Object.hasOwn(values.body, "value")
      )
        throw new TypeError("request body media range requires { contentType, value }");
      const selected = selectRequestBodyDefinition(requestBodies!, values.body.contentType);
      if (selected === undefined)
        throw new TypeError(
          `request body content type ${values.body.contentType} is not declared by this operation`,
        );
      contentType = values.body.contentType;
      bodyValue = values.body.value;
    }
    const definition =
      requestBodies === undefined
        ? undefined
        : selectRequestBodyDefinition(requestBodies, contentType);
    const streamSource =
      definition?.itemSchema !== undefined && isStreamSource(bodyValue)
        ? normalizeStreamSource(bodyValue, options.signal)
        : undefined;
    if (
      definition?.itemSchema !== undefined &&
      streamSource === undefined &&
      definition.schemaDeclared !== true
    )
      throw new TypeError("streaming request body must be a StreamSource");
    const selectedStreamCodec = resolveStreamCodec(contentType, options.streamCodec, streamCodecs);
    const finishStream = (encoded: EncodedStreamRequestBody): EncodedRequest => {
      headers.set("Content-Type", encoded.contentType);
      return { url: url.href, headers, body: encoded.body };
    };
    if (definition?.itemSchema !== undefined && streamSource !== undefined) {
      const stream = await requireHTTPHook(extensions.encodeIncrementalStreamRequestBody)(
        streamSource,
        {
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
        },
      );
      return finishStream(stream);
    }
    if (
      streamSource === undefined &&
      definition?.schemaDeclared === true &&
      definition.streamFraming !== undefined
    ) {
      const stream = await requireHTTPHook(extensions.encodeCompleteSequentialRequestBody)(
        bodyValue,
        {
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
        },
      );
      return finishStream(stream);
    }
    const body = extensions.encodeRequestBody(
      contentType,
      encodeRequestWireValue(operation, contentType, bodyValue),
      codecs,
      definition?.schema,
      operation.inputSchemas ?? {},
      definition,
      options.multipartHeaders,
      options.multipartContentTypes,
    );
    const finish = (resolved: BodyInit | ReadableStream<Uint8Array>): EncodedRequest => {
      if (!(resolved instanceof FormData)) {
        const resolvedContentType =
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
      const values =
        parameter.location === "path"
          ? pathValues
          : parameter.location === "query"
            ? queryValues
            : parameter.location === "querystring"
              ? querystringValues
              : parameter.location === "header"
                ? headerValues
                : cookieValues;
      if (values[parameter.property] === undefined || values[parameter.property] === null) {
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
    const definition =
      operation.requestBodies === undefined
        ? undefined
        : selectRequestBodyDefinition(operation.requestBodies, contentType);
    return definition === undefined
      ? value
      : transformWireValue(value, definition.schema, operation.inputSchemas ?? {}, "encode");
  }

  function encodeParameterWireValue(
    operation: OperationDefinition,
    parameter: ParameterDefinition | undefined,
    value: unknown,
  ): unknown {
    if (parameter?.sort !== undefined && Array.isArray(value)) {
      value = value.map((entry) => {
        if (
          !isRecord(entry) ||
          typeof entry.field !== "string" ||
          typeof entry.direction !== "string"
        ) {
          throw new TypeError(`Invalid structured sort value for ${parameter.name}`);
        }
        const wire = parameter.sort?.[`${entry.field}\u0000${entry.direction}`];
        if (wire === undefined)
          throw new TypeError(`Invalid structured sort value for ${parameter.name}`);
        return wire;
      });
    }
    return parameter?.schema === undefined
      ? value
      : transformWireValue(value, parameter.schema, operation.inputSchemas ?? {}, "encode");
  }

  function decodeResponseWireValue(
    operation: OperationDefinition,
    response: Response,
    value: unknown,
  ): unknown {
    const definition = selectResponseDefinition(operation, response, true);
    if (
      definition !== undefined &&
      isXMLMediaType(definition.contentType) &&
      typeof value === "string"
    ) {
      value = requireHTTPHook(extensions.decodeXML)(
        value,
        definition.schema,
        operation.outputSchemas ?? {},
      );
    }
    return definition === undefined
      ? value
      : transformWireValue(
          value,
          definition.schema,
          operation.outputSchemas ?? {},
          "decode",
          tolerantResponseTransformOptions,
        );
  }

  function selectRequestBodyDefinition(
    bodies: readonly WireBodyDefinition[],
    contentType: string,
  ): WireBodyDefinition | undefined {
    return bodies
      .filter((body) => mediaTypeMatches(body.contentType, contentType))
      .sort(
        (left, right) =>
          mediaTypeMatchScore(right.contentType, contentType) -
          mediaTypeMatchScore(left.contentType, contentType),
      )[0];
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
      const parameter = findParameterByProperty(operation, location, property);
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
    const style = parameter?.style ?? "form";
    const explode = parameter?.explode ?? true;
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
      const entries = Object.entries(value).filter((entry) => entry[1] !== undefined);
      if (explode)
        for (const [key, item] of entries)
          appendQueryValue(query, key, item, parameter?.allowReserved ?? false);
      else
        appendQueryValue(
          query,
          name,
          entries.flatMap(([key, item]) => [key, String(item)]).join(","),
          parameter?.allowReserved ?? false,
        );
      return;
    }
    if (isRecord(value) && (style === "spaceDelimited" || style === "pipeDelimited")) {
      const separator = style === "spaceDelimited" ? " " : "|";
      const entries = Object.entries(value).filter((entry) => entry[1] !== undefined);
      const serialized = explode
        ? entries.map(([key, item]) => `${key}=${String(item)}`).join(separator)
        : entries.flatMap(([key, item]) => [key, String(item)]).join(separator);
      appendQueryValue(query, name, serialized, parameter?.allowReserved ?? false);
      return;
    }
    appendQueryValue(query, name, value, parameter?.allowReserved ?? false);
  }

  function appendQuerySync(
    query: Readonly<Record<string, unknown>>,
    operation: OperationDefinition,
    location: "query" | "querystring",
  ): QueryPart[] {
    const result: QueryPart[] = [];
    for (const [property, rawValue] of Object.entries(query)) {
      if (rawValue === undefined) continue;
      const parameter = findParameterByProperty(operation, location, property);
      const value = encodeParameterWireValue(operation, parameter, rawValue);
      if (parameter?.location === "querystring") {
        appendQuerystringSync(result, value, parameter, operation.inputSchemas ?? {});
        continue;
      }
      const name = parameter?.name ?? property;
      assertQueryEmptyValueAllowed(value, parameter);
      if (parameter?.contentType !== undefined) {
        appendQueryValue(
          result,
          name,
          serializeContentParameterSync(
            value,
            parameter.contentType,
            parameter.schema,
            operation.inputSchemas ?? {},
          ),
          parameter.allowReserved ?? false,
        );
        continue;
      }
      const style = parameter?.style ?? "form";
      const explode = parameter?.explode ?? true;
      if (style === "deepObject" && isRecord(value)) {
        for (const [key, item] of Object.entries(value))
          if (item !== undefined)
            appendQueryValue(result, `${name}[${key}]`, item, parameter?.allowReserved ?? false);
        continue;
      }
      if (Array.isArray(value)) {
        if (style === "spaceDelimited")
          appendQueryValue(
            result,
            name,
            value.map(String).join(" "),
            parameter?.allowReserved ?? false,
          );
        else if (style === "pipeDelimited")
          appendQueryValue(
            result,
            name,
            value.map(String).join("|"),
            parameter?.allowReserved ?? false,
          );
        else if (explode)
          for (const item of value)
            appendQueryValue(result, name, item, parameter?.allowReserved ?? false);
        else
          appendQueryValue(
            result,
            name,
            value.map(String).join(","),
            parameter?.allowReserved ?? false,
          );
        continue;
      }
      if (isRecord(value) && style === "form") {
        const entries = Object.entries(value).filter((entry) => entry[1] !== undefined);
        if (explode)
          for (const [key, item] of entries)
            appendQueryValue(result, key, item, parameter?.allowReserved ?? false);
        else
          appendQueryValue(
            result,
            name,
            entries.flatMap(([key, item]) => [key, String(item)]).join(","),
            parameter?.allowReserved ?? false,
          );
        continue;
      }
      if (isRecord(value) && (style === "spaceDelimited" || style === "pipeDelimited")) {
        const separator = style === "spaceDelimited" ? " " : "|";
        const entries = Object.entries(value).filter((entry) => entry[1] !== undefined);
        appendQueryValue(
          result,
          name,
          explode
            ? entries.map(([key, item]) => `${key}=${String(item)}`).join(separator)
            : entries.flatMap(([key, item]) => [key, String(item)]).join(separator),
          parameter?.allowReserved ?? false,
        );
        continue;
      }
      appendQueryValue(result, name, value, parameter?.allowReserved ?? false);
    }
    return result;
  }

  function appendQuerystringSync(
    query: QueryPart[],
    value: unknown,
    parameter: ParameterDefinition,
    components: WireSchemas,
  ): void {
    const contentType = parameter.contentType?.toLowerCase();
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
        serializeContentParameterSync(
          value,
          parameter.contentType ?? "text/plain",
          parameter.schema,
          components,
        ),
      ),
    });
  }

  function assertQueryEmptyValueAllowed(
    value: unknown,
    parameter: ParameterDefinition | undefined,
  ): void {
    if (
      value === "" &&
      parameter !== undefined &&
      parameter.location === "query" &&
      parameter.contentType === undefined &&
      parameter.style === "form" &&
      parameter.allowEmptyValue !== true
    )
      throw new TypeError(`Empty query parameter ${parameter.name} requires allowEmptyValue: true`);
  }

  function appendQueryValue(
    query: QueryPart[],
    name: string,
    value: unknown,
    allowReserved: boolean,
  ): void {
    if (isRecord(value) && typeof value.field === "string" && typeof value.direction === "string") {
      query.push({ name, value: `${value.field}:${value.direction}`, allowReserved });
      return;
    }
    if (typeof value === "object" && value !== null) {
      query.push({ name, value: JSON.stringify(value), allowReserved });
      return;
    }
    query.push({ name, value: String(value), allowReserved });
  }

  function serializeQuery(query: readonly QueryPart[]): string {
    return query
      .map(
        (part) =>
          part.raw ??
          `${encodeURIComponent(part.name ?? "")}=${part.allowReserved ? encodeReservedQueryValue(part.value ?? "") : encodeURIComponent(part.value ?? "")}`,
      )
      .join("&");
  }

  async function appendQuerystring(
    query: QueryPart[],
    value: unknown,
    parameter: ParameterDefinition,
    components: WireSchemas,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
  ): Promise<void> {
    const contentType = parameter.contentType?.toLowerCase();
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

  function encodeReservedQueryValue(value: string): string {
    return encodeURIComponent(value)
      .replace(/%25([0-9a-f]{2})/gi, "%$1")
      .replace(/%3A|%2F|%3F|%40|%21|%24|%27|%28|%29|%2A|%2C|%3B|%3D/gi, (encoded) =>
        decodeURIComponent(encoded),
      );
  }

  function findParameter(
    operation: OperationDefinition,
    location: ParameterDefinition["location"],
    name: string,
  ): ParameterDefinition | undefined {
    return operation.parameters?.find(
      (parameter) => parameter.location === location && parameter.name === name,
    );
  }

  function findParameterByProperty(
    operation: OperationDefinition,
    location: ParameterDefinition["location"],
    property: string,
  ): ParameterDefinition | undefined {
    return operation.parameters?.find(
      (parameter) => parameter.location === location && parameter.property === property,
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
    const style = parameter?.style ?? "simple";
    const explode = parameter?.explode ?? false;
    const encoded = serializePathValue(value, explode, style === "label" ? "." : ",");
    if (style === "label") return `.${encoded}`;
    if (style !== "matrix") return encoded;
    if (Array.isArray(value) && explode) {
      return value
        .map((item) => `;${encodeURIComponent(name)}=${encodeURIComponent(String(item))}`)
        .join("");
    }
    if (isRecord(value) && explode) {
      return Object.entries(value)
        .filter((entry) => entry[1] !== undefined)
        .map(([key, item]) => `;${encodeURIComponent(key)}=${encodeURIComponent(String(item))}`)
        .join("");
    }
    return `;${encodeURIComponent(name)}=${encoded}`;
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
    const style = parameter?.style ?? "simple";
    const explode = parameter?.explode ?? false;
    const encoded = serializePathValue(value, explode, style === "label" ? "." : ",");
    if (style === "label") return `.${encoded}`;
    if (style !== "matrix") return encoded;
    if (Array.isArray(value) && explode)
      return value
        .map((item) => `;${encodeURIComponent(name)}=${encodeURIComponent(String(item))}`)
        .join("");
    if (isRecord(value) && explode)
      return Object.entries(value)
        .filter((entry) => entry[1] !== undefined)
        .map(([key, item]) => `;${encodeURIComponent(key)}=${encodeURIComponent(String(item))}`)
        .join("");
    return `;${encodeURIComponent(name)}=${encoded}`;
  }

  function serializePathValue(value: unknown, explode: boolean, arraySeparator: string): string {
    if (Array.isArray(value))
      return value
        .map((item) => encodeURIComponent(String(item)))
        .join(explode ? arraySeparator : ",");
    if (isRecord(value)) {
      return Object.entries(value)
        .filter((entry) => entry[1] !== undefined)
        .flatMap(([key, item]) =>
          explode
            ? `${encodeURIComponent(key)}=${encodeURIComponent(String(item))}`
            : [encodeURIComponent(key), encodeURIComponent(String(item))],
        )
        .join(explode ? arraySeparator : ",");
    }
    return encodeURIComponent(String(value));
  }

  function serializeSimpleValue(value: unknown, explode: boolean): string {
    if (Array.isArray(value)) return value.map(String).join(",");
    if (isRecord(value)) {
      return Object.entries(value)
        .filter((entry) => entry[1] !== undefined)
        .flatMap(([key, item]) => (explode ? `${key}=${String(item)}` : [key, String(item)]))
        .join(",");
    }
    return String(value);
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
      const form = new URLSearchParams();
      for (const [name, item] of Object.entries(value)) {
        if (item === undefined) continue;
        if (Array.isArray(item)) for (const entry of item) form.append(name, String(entry));
        else form.append(name, String(item));
      }
      return form.toString();
    }
    if (contentType.toLowerCase().startsWith("text/")) return String(value);
    const codec = codecs.get(normalizeMediaType(contentType));
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
      const form = new URLSearchParams();
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
    const parameter = findParameterByProperty(operation, "cookie", property);
    const name = parameter?.name ?? property;
    const preserve = parameter?.style === "cookie";
    const pair = (key: string, item: unknown): string =>
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
        return value.map((item) => pair(name, item));
      }
      return [pair(name, value.map(String).join(","))];
    }
    if (isRecord(value) && (parameter?.explode ?? true)) {
      return Object.entries(value)
        .filter((entry) => entry[1] !== undefined)
        .map(([key, item]) => pair(key, item));
    }
    return [pair(name, serializeSimpleValue(value, false))];
  }

  function serializeCookieSync(
    operation: OperationDefinition,
    property: string,
    value: unknown,
  ): string[] {
    const parameter = findParameterByProperty(operation, "cookie", property);
    const name = parameter?.name ?? property;
    const preserve = parameter?.style === "cookie";
    const pair = (key: string, item: unknown): string =>
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
        ? value.map((item) => pair(name, item))
        : [pair(name, value.map(String).join(","))];
    if (isRecord(value) && (parameter?.explode ?? true))
      return Object.entries(value)
        .filter((entry) => entry[1] !== undefined)
        .map(([key, item]) => pair(key, item));
    return [pair(name, serializeSimpleValue(value, false))];
  }

  function resolveOperationBaseURL(
    baseURL: string | undefined,
    origin: string | undefined,
    selection: ServerSelection | undefined,
    operation: OperationDefinition,
  ): string {
    if (baseURL !== undefined) return baseURL;
    const servers = operation.servers ?? [{ id: "#", url: "/" }];
    const server =
      selection?.id === undefined ? servers[0] : servers.find((item) => item.id === selection.id);
    if (server === undefined)
      throw new TypeError(
        `Unknown server ${selection?.id} for operation ${operationDiagnosticName(operation)}`,
      );
    const variables = selection?.variables ?? {};
    const expanded = server.url.replace(/\{([^}]+)\}/g, (_, name: string) => {
      const definition = server.variables?.find((item) => item.name === name);
      if (definition === undefined)
        throw new TypeError(`Server ${server.id} has no variable ${name}`);
      const value = variables[name] ?? definition.defaultValue;
      if (definition.enumValues !== undefined && !definition.enumValues.includes(value)) {
        throw new TypeError(
          `Server variable ${name} must be one of ${definition.enumValues.join(", ")}`,
        );
      }
      return value;
    });
    try {
      return normalizeBaseURL(new URL(expanded).href);
    } catch (cause) {
      try {
        new URL(expanded);
      } catch {
        if (origin === undefined)
          throw new TypeError(
            `Server ${server.id} is relative; pass ClientOptions.origin or baseURL`,
          );
        const absoluteOrigin = normalizeOrigin(origin);
        return normalizeBaseURL(new URL(expanded, absoluteOrigin).href);
      }
      throw cause;
    }
  }

  function normalizeOrigin(value: string): string {
    const url = new URL(value);
    if (
      (url.protocol !== "http:" && url.protocol !== "https:") ||
      url.pathname !== "/" ||
      url.search ||
      url.hash
    ) {
      throw new TypeError(
        "origin must be an absolute http(s) origin without path, query, or fragment",
      );
    }
    return url.origin;
  }

  function appendRawHeaders(
    target: Headers,
    source: HeadersInit | undefined,
    contractNames: ReadonlySet<string>,
  ): void {
    if (source === undefined) return;
    const incoming = new Headers(source);
    incoming.forEach((value, name) => {
      const lower = name.toLowerCase();
      if (reservedHeaders.has(lower) || contractNames.has(lower)) {
        throw new TypeError(`Raw header ${name} must use its typed option`);
      }
      target.set(name, value);
    });
  }

  function setHeader(headers: Headers, name: string, value: string | undefined): void {
    if (value !== undefined) headers.set(name, value);
  }

  function rejectUndefinedArrayValues(value: unknown): void {
    if (Array.isArray(value)) {
      for (const item of value) {
        if (item === undefined) {
          throw new TypeError("Request arrays cannot contain undefined");
        }
        rejectUndefinedArrayValues(item);
      }
      return;
    }
    if (isRecord(value)) {
      for (const item of Object.values(value)) rejectUndefinedArrayValues(item);
    }
  }

  async function decodeResponse(
    operation: OperationDefinition,
    response: Response,
    request: RequestMetadata,
    options: ResponseDecodeOptions,
  ): Promise<unknown> {
    if (response.status === 204 || response.status === 205) return undefined;
    const contentType = responseContentType(response);
    if (contentType === undefined || response.body === null) return undefined;
    try {
      const definition = selectResponseDefinition(operation, response, true);
      const completeSequential = definition?.streamFraming !== undefined;
      if (completeSequential && definition !== undefined) {
        const values: unknown[] = [];
        const streamCodec = resolveStreamCodec(
          contentType,
          options.streamCodec,
          options.streamCodecs,
        );
        for await (const value of requireHTTPHook(extensions.decodeResponseStreamItems)(
          response.body,
          {
            contentType: response.headers.get("content-type") ?? contentType,
            streamFraming: definition.streamFraming,
            itemSchema: definition.schema.items ?? {},
            prefixSchemas: definition.schema.prefixItems,
            schemas: operation.outputSchemas ?? {},
            codecs: options.codecs,
            prefixEncoding: definition.prefixEncoding,
            itemEncoding: definition.itemEncoding,
            maxFrameBytes: resolveMaxStreamFrameBytes(options.maxStreamFrameBytes),
            streamCodec,
            signal: options.signal,
          },
        )) {
          values.push(value);
        }
        return values;
      }
      if (normalizeMediaType(contentType).startsWith("multipart/") && definition !== undefined) {
        return requireHTTPHook(extensions.decodeMultipartResponse)(
          response.body,
          response.headers.get("content-type") ?? contentType,
          definition,
          operation.outputSchemas ?? {},
          options.codecs,
        );
      }
      if (isJSONMediaType(contentType)) {
        return await response.json();
      }
      if (contentType.startsWith("text/") || contentType.includes("xml")) {
        return await response.text();
      }
      if (isBinaryMediaType(contentType)) return response.body;
      const codec = options.codecs.get(contentType);
      if (codec?.decode === undefined)
        throw new TypeError(`missing decode codec for ${contentType}`);
      return await codec.decode(response, { contentType });
    } catch (cause) {
      throw new APIError({
        code: TransportErrorCode.RESPONSE_DECODE_FAILED,
        message: "Failed to decode response body",
        request,
        status: response.status,
        response,
        cause,
      });
    }
  }

  function resolveStreamCodec(
    contentType: string,
    override: StreamCodec | undefined,
    defaults: ReadonlyMap<string, StreamCodec>,
  ): StreamCodec | undefined {
    return override ?? defaults.get(normalizeMediaType(contentType));
  }

  function isBinaryMediaType(contentType: string): boolean {
    return (
      contentType === "application/octet-stream" ||
      contentType.startsWith("image/") ||
      contentType.startsWith("audio/") ||
      contentType.startsWith("video/")
    );
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
        const iterator = isAsyncIterable(source)
          ? source[Symbol.asyncIterator]()
          : readableStreamIterator(source);
        let done = false;
        return {
          async next(): Promise<IteratorResult<unknown>> {
            if (done) return { done: true, value: undefined };
            try {
              const next = await awaitAbortable(Promise.resolve(iterator.next()), signal);
              if (done || next.done) {
                done = true;
                return { done: true, value: undefined };
              }
              return next;
            } catch (cause) {
              if (signal?.aborted && !done) {
                done = true;
                void Promise.resolve(iterator.return?.(signal.reason)).catch(() => undefined);
              }
              throw cause;
            }
          },
          async return(reason?: unknown): Promise<IteratorResult<unknown>> {
            if (done) return { done: true, value: undefined };
            done = true;
            const close = Promise.resolve(iterator.return?.(reason));
            if (signal?.aborted) void close.catch(() => undefined);
            else await close.catch(() => undefined);
            return { done: true, value: undefined };
          },
        };
      },
    };
  }

  function readableStreamIterator(source: ReadableStream<unknown>): AsyncIterator<unknown> {
    const reader = source.getReader();
    let released = false;
    const release = (): void => {
      if (released) return;
      released = true;
      reader.releaseLock();
    };
    return {
      async next(): Promise<IteratorResult<unknown>> {
        try {
          const next = await reader.read();
          if (next.done) release();
          return next;
        } catch (cause) {
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
  return { encodeRequest, decodeResponse, decodeResponseHeaders, decodeResponseWireValue };
}

function requireHTTPHook<Hook>(hook: Hook | undefined): Hook {
  if (hook === undefined)
    throw new TypeError("Operation execution plan is missing a required HTTP implementation");
  return hook;
}

/** Binds normalized client state to only the request services this operation needs. */
export function createRequestCore(
  context: RequestContext,
  services: StreamingRequestExecutionServices,
): RequestFunction;
/** Binds normalized client state to the request services required by an operation. */
export function createRequestCore(
  context: RequestContext,
  services: RequestExecutionServices,
): BufferedRequestFunction;
/** Binds normalized client state to the request services required by an operation. */
export function createRequestCore(
  context: RequestContext,
  services: RequestExecutionServices & Partial<StreamingRequestExecutionServices>,
): BufferedRequestFunction | RequestFunction {
  const { options, baseURL, fetchImplementation, codecs, streamCodecs } = context;
  const {
    encodeRequest,
    decodeResponse,
    decodeResponseHeaders,
    decodeResponseWireValue,
    createOperationStream,
  } = services;
  const execute = async <Output>(
    operation: OperationDefinition,
    input?: unknown,
    requestOptions: RequestOptions = {},
    raw = false,
  ): Promise<Output | RawResponse<Output>> => {
    const credentials = requestOptions.credentials ?? options.credentials;
    const timeoutMS = requestOptions.timeoutMS ?? options.timeoutMS;
    const abort = createAbortContext(requestOptions.signal, timeoutMS);
    let responseMetadata:
      | { request: RequestMetadata; status: number; response: Response }
      | undefined;
    let requestBodyFailure: (() => unknown) | undefined;
    let requestBodyCancel: ((reason?: unknown) => Promise<void>) | undefined;
    try {
      let encoded: EncodedRequest;
      try {
        if (abort.signal?.aborted) throw abort.signal.reason;
        const effectiveRequestOptions =
          abort.signal === undefined ? requestOptions : { ...requestOptions, signal: abort.signal };
        const pending = encodeRequest(
          baseURL,
          options,
          codecs,
          streamCodecs,
          operation,
          input,
          effectiveRequestOptions,
        );
        encoded = isPromise(pending) ? await awaitAbortable(pending, abort.signal) : pending;
        requestBodyFailure = encoded.bodyFailure;
        requestBodyCancel = encoded.bodyCancel;
        if (abort.signal?.aborted) throw abort.signal.reason;
        const secured = applyOperationSecurity(
          options,
          operation,
          encoded,
          effectiveRequestOptions,
          credentials,
        );
        encoded = isPromise(secured) ? await awaitAbortable(secured, abort.signal) : secured;
        requestBodyFailure = encoded.bodyFailure;
        requestBodyCancel = encoded.bodyCancel;
      } catch (cause) {
        if (abort.timedOut() || abort.aborted()) throw cause;
        if (isAPIError(cause)) throw cause;
        throw transportError(
          TransportErrorCode.REQUEST_ENCODE_FAILED,
          `Failed to encode ${operationDiagnosticName(operation)} request`,
          cause,
        );
      }
      const init: RequestInit = {
        method: operation.method,
        headers: encoded.headers,
        ...(encoded.redirect === undefined ? {} : { redirect: encoded.redirect }),
      };
      if (encoded.body !== undefined) {
        init.body = encoded.body as BodyInit;
        if (isReadableStream(encoded.body))
          (init as RequestInit & { duplex?: "half" }).duplex = "half";
      }
      if (abort.signal !== undefined) init.signal = abort.signal;
      if (credentials !== undefined) init.credentials = credentials;
      if (abort.signal?.aborted) throw abort.signal.reason;
      assertReadableResponseHeaders(options.transport, operation);
      const response = await awaitAbortable(fetchImplementation(encoded.url, init), abort.signal);
      const request = requestMetadata(response);
      responseMetadata = { request, status: response.status, response };
      const responseDefinition = selectResponseDefinition(operation, response, true);
      if (
        raw &&
        response.ok &&
        (responseDefinition?.itemSchema !== undefined ||
          responseDefinition?.streamFraming !== undefined)
      ) {
        const contentType = responseContentType(response);
        let headerValues: Readonly<Record<string, unknown>>;
        try {
          headerValues = await awaitAbortable(
            decodeResponseHeaders(operation, response, codecs),
            abort.signal,
          );
        } catch (cause) {
          await response.body?.cancel().catch(() => undefined);
          throw transportErrorFromCause(
            TransportErrorCode.RESPONSE_DECODE_FAILED,
            "Failed to decode response headers",
            cause,
            responseMetadata,
          );
        }
        return {
          status: response.status,
          ...(contentType === undefined ? {} : { contentType }),
          data: undefined as Output,
          headers: headerValues,
          request,
          response,
        };
      }
      let body: unknown;
      try {
        const decodedBody = await awaitAbortable(
          decodeResponse(operation, response, request, {
            codecs,
            streamCodecs,
            ...(requestOptions.streamCodec === undefined
              ? {}
              : { streamCodec: requestOptions.streamCodec }),
            maxStreamFrameBytes: requestOptions.maxStreamFrameBytes ?? options.maxStreamFrameBytes,
            ...(abort.signal === undefined ? {} : { signal: abort.signal }),
          }),
          abort.signal,
        );
        body = decodeResponseWireValue(operation, response, decodedBody);
      } catch (cause) {
        throw transportErrorFromCause(
          TransportErrorCode.RESPONSE_DECODE_FAILED,
          "Failed to decode response body",
          cause,
          responseMetadata,
        );
      }
      if (!response.ok) {
        throw serverError(response, request, body);
      }
      const data =
        operation.envelope === "data" && isRecord(body) && Object.hasOwn(body, "data")
          ? (body.data as Output)
          : (body as Output);
      if (!raw) return data;
      const contentType = responseContentType(response);
      let headerValues: Readonly<Record<string, unknown>>;
      try {
        headerValues = await awaitAbortable(
          decodeResponseHeaders(operation, response, codecs),
          abort.signal,
        );
      } catch (cause) {
        throw transportErrorFromCause(
          TransportErrorCode.RESPONSE_DECODE_FAILED,
          "Failed to decode response headers",
          cause,
          responseMetadata,
        );
      }
      return {
        status: response.status,
        ...(contentType === undefined ? {} : { contentType }),
        data: body as Output,
        headers: headerValues,
        request,
        response,
      };
    } catch (cause) {
      await cancelTrackedRequestBody(requestBodyCancel, cause);
      if (abort.timedOut()) {
        await cancelResponseBody(responseMetadata?.response, cause);
        throw transportErrorFromCause(
          TransportErrorCode.REQUEST_TIMEOUT,
          `Request timed out after ${timeoutMS}ms`,
          cause,
          responseMetadata,
        );
      }
      if (abort.aborted()) {
        await cancelResponseBody(responseMetadata?.response, cause);
        throw transportErrorFromCause(
          TransportErrorCode.REQUEST_ABORTED,
          "Request was aborted",
          cause,
          responseMetadata,
        );
      }
      if (isAPIError(cause)) throw cause;
      const bodyFailure = requestBodyFailure?.();
      if (bodyFailure !== undefined) {
        throw transportErrorFromCause(
          TransportErrorCode.REQUEST_ENCODE_FAILED,
          `Failed to encode ${operationDiagnosticName(operation)} request body stream`,
          bodyFailure,
          responseMetadata,
        );
      }
      throw transportError(TransportErrorCode.NETWORK_ERROR, "Network request failed", cause);
    } finally {
      await cancelTrackedRequestBody(requestBodyCancel);
      abort.cleanup();
    }
  };

  const request = <Output>(
    operation: OperationDefinition,
    input?: unknown,
    requestOptions?: RequestOptions,
  ): Promise<Output> => execute<Output>(operation, input, requestOptions, false) as Promise<Output>;
  const raw = <Output>(
    operation: OperationDefinition,
    input?: unknown,
    requestOptions?: RequestOptions,
  ): Promise<RawResponse<Output>> =>
    execute<Output>(operation, input, requestOptions, true) as Promise<RawResponse<Output>>;
  if (createOperationStream === undefined) return Object.assign(request, { raw });
  const stream = <Item>(
    operation: OperationDefinition,
    input?: unknown,
    requestOptions: RequestOptions = {},
  ): OperationStream<Item> =>
    createOperationStream<Item>(
      baseURL,
      options,
      codecs,
      streamCodecs,
      fetchImplementation,
      operation,
      input,
      requestOptions,
    );
  return Object.assign(request, { raw, stream });
}
