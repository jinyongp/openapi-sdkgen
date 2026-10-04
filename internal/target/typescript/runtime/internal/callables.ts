import { isRecord } from "./runtime-support.js";
import { registerHTTPErrorMethod, inheritHTTPErrorMethod } from "./http-errors.js";
import type { OperationDefinition, ParameterDefinition } from "./operation.js";
import type { OperationStream, RawResponse, RequestOptions } from "./request.js";

/** Shares inline descriptor construction through the existing operation import edge. */
export { wireProperties as createWireProperties } from "./wire-properties.js";

/** Low-level request executor used by generated operation bindings. */
export interface BufferedRequestFunction {
  /**
   * Sends an operation and returns its decoded response body.
   *
   * @param operation Generated operation metadata.
   * @param input Generated path, query, header, cookie, and body input.
   * @param options Per-request transport options.
   */
  <Output>(
    operation: OperationDefinition,
    input?: unknown,
    options?: RequestOptions,
  ): Promise<Output>;
  /**
   * Sends an operation and returns its decoded body with HTTP response metadata.
   *
   * @param operation Generated operation metadata.
   * @param input Generated path, query, header, cookie, and body input.
   * @param options Per-request transport options.
   */
  raw<Output>(
    operation: OperationDefinition,
    input?: unknown,
    options?: RequestOptions,
  ): Promise<RawResponse<Output>>;
}

/** Full request executor, including streaming operations. */
export interface RequestFunction extends BufferedRequestFunction {
  /** Opens one declared streaming response and lazily decodes its items. */
  stream<Item>(
    operation: OperationDefinition,
    input?: unknown,
    options?: RequestOptions,
  ): OperationStream<Item>;
}

type RequiredKeys<Value> = {
  [Key in keyof Value]-?: Record<never, never> extends Pick<Value, Key> ? never : Key;
}[keyof Value];

type OperationOptionsArguments<Options extends RequestOptions> = [RequiredKeys<Options>] extends [
  never,
]
  ? [options?: Options]
  : [options: Options];

type OperationInputKey =
  | "body"
  | Exclude<ParameterDefinition["location"], "header" | "cookie">
  | `${Extract<ParameterDefinition["location"], "header" | "cookie">}Params`;

type OptionalOperationArguments<Input, Options> = readonly [Input | undefined, Options | undefined];

const generatedOperationInputKeys: readonly OperationInputKey[] = [
  "body",
  "path",
  "query",
  "querystring",
  "headerParams",
  "cookieParams",
];

function isGeneratedOperationInput(value: unknown): boolean {
  return (
    isRecord(value) &&
    generatedOperationInputKeys.some((key: OperationInputKey): boolean => Object.hasOwn(value, key))
  );
}

function splitOptionalOperationArguments<Input, Options extends RequestOptions>(
  args: readonly unknown[],
): OptionalOperationArguments<Input, Options> {
  const [first, second]: readonly unknown[] = args;
  if (args.length > 1 || isGeneratedOperationInput(first)) {
    return [first as Input | undefined, second as Options | undefined];
  }
  return [undefined, first as Options | undefined];
}

function operationOptionsArguments<Options extends RequestOptions>(
  options: Options | undefined,
): OperationOptionsArguments<Options> {
  return (options === undefined ? [] : [options]) as OperationOptionsArguments<Options>;
}

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
  const call:
    | ((input: Input, ...options: OperationOptionsArguments<Options>) => Promise<Output>)
    | ((...options: OperationOptionsArguments<Options>) => Promise<Output>) = hasInput
    ? inputOptional
      ? (...args: readonly unknown[]): Promise<Output> => {
          const [input, options]: OptionalOperationArguments<Input, Options> =
            splitOptionalOperationArguments<Input, Options>(args);
          return request<Output>(operation, input, options);
        }
      : (input: Input, ...options: OperationOptionsArguments<Options>): Promise<Output> =>
          request<Output>(operation, input, options[0])
    : (...options: OperationOptionsArguments<Options>): Promise<Output> =>
        request<Output>(operation, undefined, options[0]);
  const raw:
    | ((
        input: Input,
        ...options: OperationOptionsArguments<Options>
      ) => Promise<RawResponse<Output, Readonly<Record<string, unknown>>>>)
    | ((
        ...options: OperationOptionsArguments<Options>
      ) => Promise<RawResponse<Output, Readonly<Record<string, unknown>>>>) = hasInput
    ? inputOptional
      ? (
          ...args: readonly unknown[]
        ): Promise<RawResponse<Output, Readonly<Record<string, unknown>>>> => {
          const [input, options]: OptionalOperationArguments<Input, Options> =
            splitOptionalOperationArguments<Input, Options>(args);
          return request.raw<Output>(operation, input, options);
        }
      : (
          input: Input,
          ...options: OperationOptionsArguments<Options>
        ): Promise<RawResponse<Output, Readonly<Record<string, unknown>>>> =>
          request.raw<Output>(operation, input, options[0])
    : (
        ...options: OperationOptionsArguments<Options>
      ): Promise<RawResponse<Output, Readonly<Record<string, unknown>>>> =>
        request.raw<Output>(operation, undefined, options[0]);
  registerHTTPErrorMethod(call, operation.route);
  registerHTTPErrorMethod(raw, operation.route);
  return Object.assign(call, { raw }) as OperationCall<Input, Output, Options, Raw>;
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

/**
 * Adds namespace members without colliding with Function prototype properties.
 * Generated runtime callables may expose only capabilities in their public type
 * (for example, stream-only operations). Decorating that surface requires an
 * object, not a public buffered-call signature, and must not add such a signature.
 */
export function assignCallableProperties<Call extends object, Members extends object>(
  call: Call,
  members: Members,
): Call & Members {
  for (const [key, value] of Object.entries(members)) {
    Object.defineProperty(call, key, {
      value,
      enumerable: true,
      configurable: true,
      writable: true,
    });
  }
  return call as Call & Members;
}

type OptionalStreamCall = {
  readonly stream?: (...args: readonly unknown[]) => unknown;
};
