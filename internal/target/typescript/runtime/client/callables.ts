import type { OperationDefinition } from "../http/operation.js";
import type { RawResponse, RequestOptions } from "../http/request.js";
import type { BufferedRequestFunction } from "../http/request/request-execution-types.js";
import type { OperationCall } from "./callable-types.js";
import { bindInputOperation } from "./operation-bind-input.js";
import { bindNoInputOperation } from "./operation-bind-none.js";
import { bindOptionalInputOperation } from "./operation-bind-optional.js";

/** Shares inline descriptor construction through the existing operation import edge. */
export { wireProperties as createWireProperties } from "../schema/wire-properties.js";
/** Canonical callable contracts. */
export type { InputOperationCall, NoInputOperationCall, OperationCall } from "./callable-types.js";
/** Canonical request executor contracts. */
export type {
  BufferedRequestFunction,
  RequestFunction,
} from "../http/request/request-execution-types.js";
/** Canonical resource binding with path and capability identity preservation. */
export { bindPathOperation, bindGeneratedPathOperation } from "./resource-binding.js";
/** Canonical streaming input and options dispatch. */
export { bindStreamOperation } from "./stream-binding.js";
/** Canonical callable namespace decoration. */
export { assignCallableProperties } from "./callable-properties.js";

/**
 * Binds generated operation metadata to a low-level request executor.
 *
 * Used by generated clients; applications normally call the generated operation instead.
 */
export function bindOperation<
  Input,
  Output,
  Options extends RequestOptions = RequestOptions,
  Raw = RawResponse<Output>,
>(
  request: BufferedRequestFunction,
  operation: OperationDefinition,
  hasInput: boolean,
  inputOptional: boolean = false,
): OperationCall<Input, Output, Options, Raw> {
  return (
    hasInput
      ? inputOptional
        ? bindOptionalInputOperation(request, operation)
        : bindInputOperation(request, operation)
      : bindNoInputOperation(request, operation)
  ) as OperationCall<Input, Output, Options, Raw>;
}

/**
 * Binds one generated operation without re-instantiating its public recursive
 * Input/Output graph through the runtime helper type system.
 *
 * Generated operation modules already declare the exact public callable
 * interface and cast this runtime-only result at that boundary.
 */
export function bindGeneratedOperation(
  request: BufferedRequestFunction,
  operation: OperationDefinition,
  hasInput: boolean,
  inputOptional: boolean = false,
): unknown {
  return bindOperation<unknown, unknown, RequestOptions, RawResponse<unknown>>(
    request,
    operation,
    hasInput,
    inputOptional,
  );
}
