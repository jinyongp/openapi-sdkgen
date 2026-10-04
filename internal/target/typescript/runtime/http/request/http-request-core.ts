import type { OperationDefinition } from "../operation.js";
import type { RequestOptions, OperationStream } from "../request.js";
import type {
  RequestContext,
  RequestExecutionServices,
  StreamingRequestExecutionServices,
} from "../http-types.js";
import type { BufferedRequestFunction, RequestFunction } from "./request-execution-types.js";
import { assertReadableResponseHeaders } from "../http-execution-support.js";
import { createStreamingRawResponseReader } from "../response/http-response-stream-raw.js";
import { createBufferedRequestCore } from "./http-buffered-core.js";
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
  const { options, baseURL, codecs, streamCodecs, fetchImplementation }: RequestContext = context;
  const createOperationStream: Partial<StreamingRequestExecutionServices>["createOperationStream"] =
    services.createOperationStream;
  const request: BufferedRequestFunction = createBufferedRequestCore(context, services, {
    assertResponseHeaders: assertReadableResponseHeaders,
    readStreamingRaw: createStreamingRawResponseReader(services.decodeResponseHeaders),
  });
  if (createOperationStream === undefined) return request;
  const stream: <Item>(
    operation: OperationDefinition,
    input?: unknown,
    requestOptions?: RequestOptions,
  ) => OperationStream<Item> = <Item>(
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
  return Object.assign(request, { stream });
}
