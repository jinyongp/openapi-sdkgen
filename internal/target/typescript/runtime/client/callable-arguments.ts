import { isRecord } from "../shared/runtime-support.js";
import type { ParameterDefinition } from "../http/operation.js";
import type { RequestOptions } from "../http/request.js";
import type { OperationOptionsArguments } from "./callable-types.js";

type OperationInputKey =
  | "body"
  | Exclude<ParameterDefinition["location"], "header" | "cookie">
  | `${Extract<ParameterDefinition["location"], "header" | "cookie">}Params`;

/** Resolved optional input and request options. */
export type OptionalOperationArguments<Input, Options> = readonly [
  Input | undefined,
  Options | undefined,
];

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

/** Resolves optional input without mistaking options for an input object. */
export function splitOptionalOperationArguments<Input, Options extends RequestOptions>(
  args: readonly unknown[],
): OptionalOperationArguments<Input, Options> {
  const [first, second]: readonly unknown[] = args;
  if (args.length > 1 || isGeneratedOperationInput(first)) {
    return [first as Input | undefined, second as Options | undefined];
  }
  return [undefined, first as Options | undefined];
}

/** Preserves the zero or one optional options argument convention. */
export function operationOptionsArguments<Options extends RequestOptions>(
  options: Options | undefined,
): OperationOptionsArguments<Options> {
  return (options === undefined ? [] : [options]) as OperationOptionsArguments<Options>;
}
