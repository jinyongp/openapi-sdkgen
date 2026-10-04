import type { WireCodec } from "../../schema/wire-types.js";
import type { MediaCodec } from "../../media/media-codec-types.js";
import type { StreamCodec } from "../../stream/stream-protocol-types.js";
import type { WireBodyDefinition } from "../../media/media-contract-types.js";
import type { ClientOptions } from "../configuration.js";
import type { OperationDefinition, ParameterDefinition } from "../operation.js";
import type { RequestOptions } from "../request.js";
import type { EncodedRequest, RequestExecutionServices, QueryEncoder } from "../http-types.js";
import type { HTTPCodecExtensions } from "../../media/media-service-types.js";
import { isRecord } from "../../shared/runtime-support.js";
import { isPromise } from "../http-execution-support.js";
import { assertSafeOperationPath } from "./http-request-path-safety.js";
import { appendRawHeaders, setHeader } from "./http-request-headers.js";
import { rejectUndefinedArrayValues } from "./http-request-input.js";
import type { BasicRequestParameterServices } from "./http-request-parameter-types.js";
import type { BufferedResponseServices } from "../response/http-response-service-types.js";

/** Buffered JSON contracts use the same validators, URL rules, response and error services. */
export function composeBasicHTTPServices(
  wire: WireCodec,
  extensions: HTTPCodecExtensions,
  parameters: BasicRequestParameterServices,
  responses: BufferedResponseServices,
  queryEncoder?: QueryEncoder,
): RequestExecutionServices {
  const { encodeParameter, serializePathParameter, resolveBaseURL }: BasicRequestParameterServices =
    parameters;
  function encodeRequest(
    baseURL: string | undefined,
    client: ClientOptions,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    _streamCodecs: ReadonlyMap<string, StreamCodec>,
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
        const parameter: ParameterDefinition | undefined = operation.parameters?.find(
          (value: ParameterDefinition): boolean => value.location === "path" && value.name === name,
        );
        const property: string = parameter?.property ?? name;
        if (!Object.hasOwn(pathValues, property) || pathValues[property] === undefined)
          throw new TypeError(`Missing path parameter ${name}`);
        return serializePathParameter(
          parameter,
          name,
          encodeParameter(operation, parameter, pathValues[property]),
        );
      },
    );
    assertSafeOperationPath(path);
    const url: URL = new URL(
      resolveBaseURL(options.baseURL ?? baseURL, client.origin, client.server, operation) +
        (path.startsWith("/") ? path : `/${path}`),
    );
    const headers: Headers = new Headers();
    if (queryEncoder !== undefined) {
      const queryValues: Record<string, unknown> = isRecord(values["query"]) ? values["query"] : {};
      rejectUndefinedArrayValues(queryValues);
      for (const parameter of operation.parameters ?? []) {
        if (
          parameter.location === "query" &&
          parameter.required &&
          (!Object.hasOwn(queryValues, parameter.property) ||
            queryValues[parameter.property] === undefined)
        )
          throw new TypeError(`Missing required query parameter ${parameter.name}`);
      }
      const query: string = queryEncoder(queryValues, operation, "query");
      if (query !== "") url.search = `${url.search}${url.search === "" ? "?" : "&"}${query}`;
    }
    const contractNames: ReadonlySet<string> = new Set(operation.headerNames ?? []);
    appendRawHeaders(headers, client.headers, contractNames);
    appendRawHeaders(headers, options.headers, contractNames);
    setHeader(headers, "Authorization", options.authorization ?? client.authorization);
    setHeader(headers, "Accept", options.accept);
    setHeader(headers, "X-CSRF-Token", options.csrfToken);
    setHeader(headers, "X-Request-Id", options.requestID);
    const redirect: RequestRedirect | undefined =
      options.authorization !== undefined ||
      client.authorization !== undefined ||
      options.csrfToken !== undefined
        ? "error"
        : undefined;
    const finish: (body?: BodyInit) => EncodedRequest = (body?: BodyInit): EncodedRequest => ({
      url: url.href,
      headers,
      ...(body === undefined ? {} : { body }),
      ...(redirect === undefined ? {} : { redirect }),
    });
    if (!Object.hasOwn(values, "body") || values["body"] === undefined) {
      if (operation.requestBodyRequired) throw new TypeError("Missing required request body");
      return finish();
    }
    rejectUndefinedArrayValues(values["body"]);
    const definition: WireBodyDefinition | undefined = operation.requestBodies?.[0];
    const contentType: string = operation.contentType ?? "application/json";
    const value: unknown =
      definition === undefined
        ? values["body"]
        : wire.transformWireValue(
            values["body"],
            definition.schema,
            operation.inputSchemas ?? {},
            "encode",
          );
    const body: BodyInit | Promise<BodyInit> = extensions.encodeRequestBody(
      contentType,
      value,
      codecs,
      definition?.schema,
      operation.inputSchemas ?? {},
      definition,
      options.multipartHeaders,
      options.multipartContentTypes,
    );
    headers.set("Content-Type", contentType);
    return isPromise(body) ? body.then(finish) : finish(body);
  }
  return { ...responses, encodeRequest };
}
