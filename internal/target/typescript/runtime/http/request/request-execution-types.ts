import type { OperationDefinition } from "../operation.js";
import type { RequestOptions, RawResponse, OperationStream } from "../request.js";

/** Low-level request executor used by generated operation bindings. */
export interface BufferedRequestFunction {
  /**
   * Sends an operation and returns its decoded response body.
   *
   * @param operation Generated operation metadata.
   * @param input Generated path, query, header, cookie, and body input.
   * @param options Per-request transport options.
   */
  <Output>(
    operation: OperationDefinition,
    input?: unknown,
    options?: RequestOptions,
  ): Promise<Output>;
  /**
   * Sends an operation and returns its decoded body with HTTP response metadata.
   *
   * @param operation Generated operation metadata.
   * @param input Generated path, query, header, cookie, and body input.
   * @param options Per-request transport options.
   */
  raw<Output>(
    operation: OperationDefinition,
    input?: unknown,
    options?: RequestOptions,
  ): Promise<RawResponse<Output>>;
}

/** Full request executor, including streaming operations. */
export interface RequestFunction extends BufferedRequestFunction {
  /** Opens one declared streaming response and lazily decodes its items. */
  stream<Item>(
    operation: OperationDefinition,
    input?: unknown,
    options?: RequestOptions,
  ): OperationStream<Item>;
}
