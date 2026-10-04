import type { RawResponse, RequestOptions } from "../http/request.js";

type RequiredKeys<Value> = {
  [Key in keyof Value]-?: Record<never, never> extends Pick<Value, Key> ? never : Key;
}[keyof Value];

/** Options tuple that preserves required option fields. */
export type OperationOptionsArguments<Options extends RequestOptions> = [
  RequiredKeys<Options>,
] extends [never]
  ? [options?: Options]
  : [options: Options];

/** Callable generated operation that requires typed input. */
export interface InputOperationCall<Input, Output, Options extends RequestOptions, Raw> {
  /** Sends the request and returns the decoded response body. */
  (input: Input, ...options: OperationOptionsArguments<Options>): Promise<Output>;
  /** Sends the request and returns decoded data with HTTP response metadata. */
  raw(input: Input, ...options: OperationOptionsArguments<Options>): Promise<Raw>;
}

/** Callable generated operation with no input object. */
export interface NoInputOperationCall<Output, Options extends RequestOptions, Raw> {
  /** Sends the request and returns the decoded response body. */
  (...options: OperationOptionsArguments<Options>): Promise<Output>;
  /** Sends the request and returns decoded data with HTTP response metadata. */
  raw(...options: OperationOptionsArguments<Options>): Promise<Raw>;
}

/**
 * Callable operation surface selected from whether the operation accepts input.
 *
 * Generated clients specialize this type with operation-specific input, output,
 * options, and raw-response types.
 */
export type OperationCall<
  Input,
  Output,
  Options extends RequestOptions = RequestOptions,
  Raw = RawResponse<Output>,
> = [Input] extends [never]
  ? NoInputOperationCall<Output, Options, Raw>
  : InputOperationCall<Input, Output, Options, Raw>;
