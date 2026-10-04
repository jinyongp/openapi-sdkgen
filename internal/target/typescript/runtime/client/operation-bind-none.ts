import type { OperationDefinition } from "../http/operation.js";
import type { RequestOptions, RawResponse } from "../http/request.js";
import type { BufferedRequestFunction } from "../http/request/request-execution-types.js";
import { bindOperationMethods } from "./operation-binding.js";

/** Canonical request executor contract for generated operation imports. */
export type { BufferedRequestFunction } from "../http/request/request-execution-types.js";
/** Canonical inline descriptor constructor for generated operation imports. */
export { wireProperties as createWireProperties } from "../schema/wire-properties.js";

/** Binds an operation with request options as its first argument. */
export function bindNoInputOperation(
  request: BufferedRequestFunction,
  operation: OperationDefinition,
): unknown {
  return bindOperationMethods(
    (...options: readonly RequestOptions[]): Promise<unknown> =>
      request(operation, undefined, options[0]),
    (...options: readonly RequestOptions[]): Promise<RawResponse<unknown>> =>
      request.raw(operation, undefined, options[0]),
    operation.route,
  );
}
