import type { RequestOptions } from "../http/request.js";
import type { OperationOptionsArguments } from "./callable-types.js";
import {
  bindResourceMethods,
  createResourceInput,
  type ResourceInputMerger,
  type ResourceOperation,
  type ResourcePath,
} from "./resource-binding-support.js";

/** Binds a resource with required remaining invocation input. */
export function bindResourceInput(operation: object, path: ResourcePath): unknown {
  const callable: ResourceOperation = operation as ResourceOperation;
  const merge: ResourceInputMerger = createResourceInput(path);
  return bindResourceMethods(
    (input: unknown, ...options: OperationOptionsArguments<RequestOptions>): Promise<unknown> =>
      callable(merge(input), ...options),
    (input: unknown, ...options: OperationOptionsArguments<RequestOptions>): Promise<unknown> =>
      callable.raw(merge(input), ...options),
    operation,
  );
}
