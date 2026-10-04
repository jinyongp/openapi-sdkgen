import type { RawResponse, RequestOptions } from "../http/request.js";
import type { InputOperationCall, OperationCall } from "./callable-types.js";
import {
  bindResourceInputStream,
  bindResourceNoInputStream,
  bindResourceOptionalInputStream,
} from "./resource-stream-binding.js";
import { bindResourceLinks, bindResourcePagination } from "./resource-helper-binding.js";

/** Binds arbitrary resource calls using the canonical input and capability owners. */
export function bindPathOperation<
  FullInput,
  Input,
  Output,
  Options extends RequestOptions = RequestOptions,
  Raw = RawResponse<Output>,
>(
  operation: Pick<InputOperationCall<FullInput, Output, Options, Raw>, "raw">,
  path: Readonly<Record<string, unknown>>,
  hasInput: boolean,
  inputOptional: boolean = false,
): OperationCall<Input, Output, Options, Raw> {
  const streamed: object = (
    hasInput
      ? inputOptional
        ? bindResourceOptionalInputStream(operation, path)
        : bindResourceInputStream(operation, path)
      : bindResourceNoInputStream(operation, path)
  ) as object;
  return bindResourcePagination(
    bindResourceLinks(streamed, operation) as object,
    operation,
  ) as OperationCall<Input, Output, Options, Raw>;
}

/** Binds a generated exact call without re-expanding its recursive public types. */
export function bindGeneratedPathOperation(
  operation: object,
  path: Readonly<Record<string, unknown>>,
  hasInput: boolean,
  inputOptional: boolean = false,
): unknown {
  return bindPathOperation<unknown, unknown, unknown, RequestOptions, unknown>(
    operation as Pick<InputOperationCall<unknown, unknown, RequestOptions, unknown>, "raw">,
    path,
    hasInput,
    inputOptional,
  );
}
