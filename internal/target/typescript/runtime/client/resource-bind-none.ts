import type { RequestOptions } from "../http/request.js";
import type { OperationOptionsArguments } from "./callable-types.js";
import {
  bindResourceMethods,
  createResourceInput,
  type ResourceInputMerger,
  type ResourceOperation,
  type ResourcePath,
} from "./resource-binding-support.js";

/** Binds a resource whose remaining invocation input is empty. */
export function bindResourceNoInput(operation: object, path: ResourcePath): unknown {
  const callable: ResourceOperation = operation as ResourceOperation;
  const merge: ResourceInputMerger = createResourceInput(path);
  return bindResourceMethods(
    (...options: OperationOptionsArguments<RequestOptions>): Promise<unknown> =>
      callable(merge(undefined), ...options),
    (...options: OperationOptionsArguments<RequestOptions>): Promise<unknown> =>
      callable.raw(merge(undefined), ...options),
    operation,
  );
}
