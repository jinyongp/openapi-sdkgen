import { isRecord } from "../shared/runtime-support.js";
import { inheritHTTPErrorMethod } from "../http/response/http-errors.js";
import type { RequestOptions } from "../http/request.js";
import type { InputOperationCall } from "./callable-types.js";

/** Bound resource path values. */
export type ResourcePath = Readonly<Record<string, unknown>>;
/** Exact runtime call/raw contract behind generated resource declarations. */
export type ResourceOperation = InputOperationCall<unknown, unknown, RequestOptions, unknown>;
/** Merges invocation input with the resource's authoritative path. */
export type ResourceInputMerger = (input: unknown) => ResourcePath;
/** Compiler-selected resource input and capability binding. */
export type ResourcePathBinder = (operation: object, path: ResourcePath) => unknown;

/** Creates the canonical resource path merger. */
export function createResourceInput(path: ResourcePath): ResourceInputMerger {
  return (input: unknown): ResourcePath => ({ ...(isRecord(input) ? input : {}), path });
}

/** Registers call/raw error provenance and preserves callable identity. */
export function bindResourceMethods(call: object, raw: object, operation: object): unknown {
  inheritHTTPErrorMethod(call, operation);
  inheritHTTPErrorMethod(raw, operation);
  return Object.assign(call, { raw });
}
