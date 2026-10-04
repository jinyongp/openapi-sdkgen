import { isRecord } from "../shared/runtime-support.js";
import { inheritHTTPErrorMethod } from "../http/response/http-errors.js";
import type { RawResponse, RequestOptions } from "../http/request.js";
import type {
  InputOperationCall,
  OperationCall,
  OperationOptionsArguments,
} from "./callable-types.js";
import {
  operationOptionsArguments,
  splitOptionalOperationArguments,
  type OptionalOperationArguments,
} from "./callable-arguments.js";

/**
 * Binds resource path parameters to an input operation.
 *
 * Used to implement generated instance builders such as `api.products(productID)`.
 */
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
  // Generated exact calls remain callable at runtime even when their public
  // decoded-call signature is intentionally hidden (for example, an operation
  // with no successful buffered response). Resource binding only requires the
  // public raw capability at its boundary, then restores that generated runtime invariant.
  const callable: InputOperationCall<FullInput, Output, Options, Raw> =
    operation as InputOperationCall<FullInput, Output, Options, Raw>;
  const mergeInput: (input: Input | undefined) => FullInput = (
    input: Input | undefined,
  ): FullInput =>
    ({
      ...(isRecord(input) ? input : {}),
      path,
    }) as FullInput;
  const call:
    | ((input: Input, ...options: OperationOptionsArguments<Options>) => Promise<Output>)
    | ((...options: OperationOptionsArguments<Options>) => Promise<Output>) = hasInput
    ? inputOptional
      ? (...args: readonly unknown[]): Promise<Output> => {
          const [input, options]: OptionalOperationArguments<Input, Options> =
            splitOptionalOperationArguments<Input, Options>(args);
          return callable(mergeInput(input), ...operationOptionsArguments(options));
        }
      : (input: Input, ...options: OperationOptionsArguments<Options>): Promise<Output> =>
          callable(mergeInput(input), ...options)
    : (...options: OperationOptionsArguments<Options>): Promise<Output> =>
        callable(mergeInput(undefined), ...options);
  const raw:
    | ((input: Input, ...options: OperationOptionsArguments<Options>) => Promise<Raw>)
    | ((...options: OperationOptionsArguments<Options>) => Promise<Raw>) = hasInput
    ? inputOptional
      ? (...args: readonly unknown[]): Promise<Raw> => {
          const [input, options]: OptionalOperationArguments<Input, Options> =
            splitOptionalOperationArguments<Input, Options>(args);
          return callable.raw(mergeInput(input), ...operationOptionsArguments(options));
        }
      : (input: Input, ...options: OperationOptionsArguments<Options>): Promise<Raw> =>
          callable.raw(mergeInput(input), ...options)
    : (...options: OperationOptionsArguments<Options>): Promise<Raw> =>
        callable.raw(mergeInput(undefined), ...options);
  const sourceStream: ((...args: readonly unknown[]) => unknown) | undefined = (
    callable as InputOperationCall<FullInput, Output, Options, Raw> & OptionalStreamCall
  ).stream;
  const stream:
    | ((input: Input, ...options: OperationOptionsArguments<Options>) => unknown)
    | ((...options: OperationOptionsArguments<Options>) => unknown)
    | undefined =
    sourceStream === undefined
      ? undefined
      : hasInput
        ? inputOptional
          ? (...args: readonly unknown[]): unknown => {
              const [input, options]: OptionalOperationArguments<Input, Options> =
                splitOptionalOperationArguments<Input, Options>(args);
              return sourceStream(mergeInput(input), options);
            }
          : (input: Input, ...options: OperationOptionsArguments<Options>): unknown =>
              sourceStream(mergeInput(input), options[0])
        : (...options: OperationOptionsArguments<Options>): unknown =>
            sourceStream(mergeInput(undefined), options[0]);
  // Helpers have their own invocation input contract. Preserve their identity;
  // only call/raw/stream merge the resource path into an operation input.
  const links: unknown = Reflect.get(callable, "links");
  const paginate: unknown = Reflect.get(callable, "paginate");
  inheritHTTPErrorMethod(call, callable);
  inheritHTTPErrorMethod(raw, callable);
  if (stream !== undefined) inheritHTTPErrorMethod(stream, callable);
  return Object.assign(
    call,
    { raw },
    stream === undefined ? {} : { stream },
    links === undefined ? {} : { links },
    paginate === undefined ? {} : { paginate },
  ) as OperationCall<Input, Output, Options, Raw>;
}

/** Binds a generated exact call without re-expanding its recursive public types. */
export function bindGeneratedPathOperation(
  operation: object,
  path: Readonly<Record<string, unknown>>,
  hasInput: boolean,
  inputOptional: boolean = false,
): unknown {
  // Generated resource modules declare their exact callable surface. Only
  // this runtime boundary merges path values; those public types stay intact.
  return bindPathOperation<unknown, unknown, unknown, RequestOptions, unknown>(
    operation as Pick<InputOperationCall<unknown, unknown, RequestOptions, unknown>, "raw">,
    path,
    hasInput,
    inputOptional,
  );
}

type OptionalStreamCall = {
  readonly stream?: (...args: readonly unknown[]) => unknown;
};
