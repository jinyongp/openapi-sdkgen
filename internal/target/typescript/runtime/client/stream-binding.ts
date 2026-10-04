import { registerHTTPErrorMethod } from "../http/response/http-errors.js";
import type { OperationDefinition } from "../http/operation.js";
import type { OperationStream, RequestOptions } from "../http/request.js";
import type { RequestFunction } from "../http/request/request-execution-types.js";

/** Canonical streaming executor contract for generated operation imports. */
export type { RequestFunction } from "../http/request/request-execution-types.js";
import {
  splitOptionalOperationArguments,
  type OptionalOperationArguments,
} from "./callable-arguments.js";

/** Binds generated streaming operation metadata with the same input and options dispatch as decoded calls. */
export function bindStreamOperation<Input, Item, Options extends RequestOptions = RequestOptions>(
  request: RequestFunction,
  operation: OperationDefinition,
  hasInput: boolean,
  inputOptional: boolean = false,
  defaultAccept?: string,
): (...args: readonly unknown[]) => OperationStream<Item> {
  const streamOptions: (options: Options | undefined) => Options | undefined = (
    options: Options | undefined,
  ): Options | undefined => {
    if (defaultAccept === undefined || options?.accept !== undefined) return options;
    return { ...options, accept: defaultAccept } as Options;
  };
  const stream: (...args: readonly unknown[]) => OperationStream<Item> = (
    ...args: readonly unknown[]
  ): OperationStream<Item> => {
    if (!hasInput)
      return request.stream<Item>(
        operation,
        undefined,
        streamOptions(args[0] as Options | undefined),
      );
    if (!inputOptional)
      return request.stream<Item>(
        operation,
        args[0] as Input,
        streamOptions(args[1] as Options | undefined),
      );
    const [input, options]: OptionalOperationArguments<Input, Options> =
      splitOptionalOperationArguments<Input, Options>(args);
    return request.stream<Item>(operation, input, streamOptions(options));
  };
  return registerHTTPErrorMethod(stream, operation.route);
}
