import { inheritHTTPErrorMethod } from "../http/response/http-errors.js";
import type { RequestOptions } from "../http/request.js";
import type { OperationOptionsArguments } from "./callable-types.js";
import { bindResourceInput } from "./resource-bind-input.js";
import { bindResourceNoInput } from "./resource-bind-none.js";
import { bindResourceOptionalInput } from "./resource-bind-optional.js";
import {
  splitOptionalOperationArguments,
  type OptionalOperationArguments,
} from "./callable-arguments.js";
import {
  createResourceInput,
  type ResourceInputMerger,
  type ResourcePath,
} from "./resource-binding-support.js";

type ResourceStream = (input: unknown, options: RequestOptions | undefined) => unknown;

function bindStream(bound: object, operation: object, stream: object): unknown {
  inheritHTTPErrorMethod(stream, operation);
  return Object.assign(bound, { stream });
}

/** Adds a stream resource call with no remaining invocation input. */
export function bindResourceNoInputStream(operation: object, path: ResourcePath): unknown {
  const bound: object = bindResourceNoInput(operation, path) as object;
  const stream: ResourceStream | undefined = Reflect.get(operation, "stream") as
    | ResourceStream
    | undefined;
  if (stream === undefined) return bound;
  const merge: ResourceInputMerger = createResourceInput(path);
  return bindStream(
    bound,
    operation,
    (...options: OperationOptionsArguments<RequestOptions>): unknown =>
      stream(merge(undefined), options[0]),
  );
}

/** Adds a stream resource call with required remaining invocation input. */
export function bindResourceInputStream(operation: object, path: ResourcePath): unknown {
  const bound: object = bindResourceInput(operation, path) as object;
  const stream: ResourceStream | undefined = Reflect.get(operation, "stream") as
    | ResourceStream
    | undefined;
  if (stream === undefined) return bound;
  const merge: ResourceInputMerger = createResourceInput(path);
  return bindStream(
    bound,
    operation,
    (input: unknown, ...options: OperationOptionsArguments<RequestOptions>): unknown =>
      stream(merge(input), options[0]),
  );
}

/** Adds a stream resource call with optional remaining invocation input. */
export function bindResourceOptionalInputStream(operation: object, path: ResourcePath): unknown {
  const bound: object = bindResourceOptionalInput(operation, path) as object;
  const stream: ResourceStream | undefined = Reflect.get(operation, "stream") as
    | ResourceStream
    | undefined;
  if (stream === undefined) return bound;
  const merge: ResourceInputMerger = createResourceInput(path);
  return bindStream(bound, operation, (...args: readonly unknown[]): unknown => {
    const [input, options]: OptionalOperationArguments<unknown, RequestOptions> =
      splitOptionalOperationArguments<unknown, RequestOptions>(args);
    return stream(merge(input), options);
  });
}
