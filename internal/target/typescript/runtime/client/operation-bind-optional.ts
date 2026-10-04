import type { OperationDefinition } from "../http/operation.js";
import type { RequestOptions, RawResponse } from "../http/request.js";
import type { BufferedRequestFunction } from "../http/request/request-execution-types.js";
import { bindOperationMethods } from "./operation-binding.js";

/** Canonical request executor contract for generated operation imports. */
export type { BufferedRequestFunction } from "../http/request/request-execution-types.js";
/** Canonical inline descriptor constructor for generated operation imports. */
export { wireProperties as createWireProperties } from "../schema/wire-properties.js";

import {
  splitOptionalOperationArguments,
  type OptionalOperationArguments,
} from "./callable-arguments.js";

/** Binds the canonical optional input invocation contract. */
export function bindOptionalInputOperation(
  request: BufferedRequestFunction,
  operation: OperationDefinition,
): unknown {
  return bindOperationMethods(
    (...args: readonly unknown[]): Promise<unknown> => {
      const [input, options]: OptionalOperationArguments<unknown, RequestOptions> =
        splitOptionalOperationArguments(args);
      return request(operation, input, options);
    },
    (...args: readonly unknown[]): Promise<RawResponse<unknown>> => {
      const [input, options]: OptionalOperationArguments<unknown, RequestOptions> =
        splitOptionalOperationArguments(args);
      return request.raw(operation, input, options);
    },
    operation.route,
  );
}
