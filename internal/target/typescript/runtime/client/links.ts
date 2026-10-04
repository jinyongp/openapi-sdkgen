import { isAPIError } from "../shared/runtime-support.js";
import type { APIError } from "../shared/runtime-support.js";
import { defineOwnDataProperty, isRecord } from "../shared/runtime-support.js";
import type { RawResponse } from "../http/request.js";
import type { LinkDefinition, LinkInputOverride } from "./links-types.js";
/** Forwards the canonical type contracts without importing their implementation. */
export type * from "./links-types.js";

/** Resolves an OpenAPI Link Object into the generated target operation input. */
export function resolveLinkInput<Input>(
  response: RawResponse<unknown> | APIError,
  definition: LinkDefinition,
  sourceInput?: unknown,
): Input {
  const source: RawResponse<unknown, Readonly<Record<string, unknown>>> = normalizeLinkResponse(
    response,
  );
  const result: Record<string, unknown> = Object.create(null) as Record<string, unknown>;
  for (const assignment of definition.parameters ?? []) {
    let section: Record<string, unknown> | undefined = result[assignment.location] as
      | Record<string, unknown>
      | undefined;
    if (section === undefined) {
      section = Object.create(null) as Record<string, unknown>;
      defineOwnDataProperty(result, assignment.location, section);
    }
    defineOwnDataProperty(
      section,
      assignment.property,
      evaluateLinkValue(source, assignment.value, sourceInput),
    );
  }
  if (definition.requestBody !== undefined)
    defineOwnDataProperty(
      result,
      "body",
      evaluateLinkValue(source, definition.requestBody, sourceInput),
    );
  return result as Input;
}

function normalizeLinkResponse(response: RawResponse<unknown> | APIError): RawResponse<unknown> {
  if (!isAPIError(response)) return response;
  if (response.response === undefined || response.status === undefined)
    throw new TypeError("Link requires an APIError with an HTTP response");
  return {
    status: response.status,
    data: response.data,
    headers: Object.fromEntries(response.response.headers.entries()),
    request: response.request,
    response: response.response,
  };
}

/** Merges Link-derived defaults with explicit target input without mutating either value. */
export function mergeLinkInput<Input>(
  defaults: Input,
  override: LinkInputOverride<Input> | undefined,
): Input {
  if (!isRecord(defaults) || !isRecord(override)) return (override ?? defaults) as Input;
  const result: Record<string, unknown> = Object.create(null) as Record<string, unknown>;
  for (const [section, value] of Object.entries(defaults))
    defineOwnDataProperty(result, section, value);
  for (const [section, value] of Object.entries(override)) {
    const existing: unknown = result[section];
    if (isRecord(existing) && isRecord(value)) {
      const merged: Record<string, unknown> = Object.create(null) as Record<string, unknown>;
      for (const [key, item] of Object.entries(existing)) defineOwnDataProperty(merged, key, item);
      for (const [key, item] of Object.entries(value)) defineOwnDataProperty(merged, key, item);
      defineOwnDataProperty(result, section, merged);
    } else {
      defineOwnDataProperty(result, section, value);
    }
  }
  return result as Input;
}

function evaluateLinkValue(
  response: RawResponse<unknown>,
  value: unknown,
  sourceInput: unknown,
): unknown {
  if (isRecord(value) && isRecord(value["x-sdkgen-link-request-parameter"])) {
    const parameter: Record<string, unknown> = value["x-sdkgen-link-request-parameter"];
    const section: string | undefined =
      typeof parameter["section"] === "string" ? parameter["section"] : undefined;
    const property: string | undefined =
      typeof parameter["property"] === "string" ? parameter["property"] : undefined;
    const pointer: string | undefined =
      typeof parameter["pointer"] === "string" ? parameter["pointer"] : undefined;
    if (section === undefined || property === undefined || pointer === undefined)
      throw new TypeError("invalid generated Link request parameter expression");
    const input: Record<string, unknown> | undefined =
      isRecord(sourceInput) && isRecord(sourceInput[section]) ? sourceInput[section] : undefined;
    const item: unknown = input?.[property];
    return pointer === "" ? item : jsonPointerValue(item, pointer);
  }
  if (typeof value !== "string" || !value.startsWith("$")) return value;
  if (value === "$url") return response.response.url;
  if (value === "$statusCode" || value === "$response.statusCode") return response.status;
  const bodyPrefix: "$response.body" = "$response.body";
  if (value === bodyPrefix) return response.data;
  if (value.startsWith(bodyPrefix + "#"))
    return jsonPointerValue(response.data, value.slice(bodyPrefix.length + 1));
  const header: RegExpExecArray | null =
    /^\$response\.header\.([A-Za-z0-9!#$%&'*+.^_`|~-]+)$/i.exec(value);
  if (header !== null) return response.response.headers.get(header[1]!);
  const requestBodyPrefix: "$request.body" = "$request.body";
  if (value === requestBodyPrefix) return isRecord(sourceInput) ? sourceInput["body"] : undefined;
  if (value.startsWith(requestBodyPrefix + "#"))
    return jsonPointerValue(
      isRecord(sourceInput) ? sourceInput["body"] : undefined,
      value.slice(requestBodyPrefix.length + 1),
    );
  const requestParameter: RegExpExecArray | null =
    /^\$request\.(path|query|header|cookie)\.([^#]+)(#.*)?$/.exec(value);
  if (requestParameter !== null) {
    const section: string =
      requestParameter[1] === "header"
        ? "headerParams"
        : requestParameter[1] === "cookie"
          ? "cookieParams"
          : requestParameter[1]!;
    const input: Record<string, unknown> | undefined =
      isRecord(sourceInput) && isRecord(sourceInput[section]) ? sourceInput[section] : undefined;
    const item: unknown = input?.[requestParameter[2]!];
    return requestParameter[3] === undefined
      ? item
      : jsonPointerValue(item, requestParameter[3]!.slice(1));
  }
  throw new TypeError(`unsupported OpenAPI Link runtime expression ${value}`);
}

function jsonPointerValue(value: unknown, pointer: string): unknown {
  if (pointer === "") return value;
  if (!pointer.startsWith("/")) throw new TypeError(`invalid JSON Pointer ${pointer}`);
  let current: unknown = value;
  for (const token of pointer.slice(1).split("/")) {
    const key: string = token.replaceAll("~1", "/").replaceAll("~0", "~");
    if (Array.isArray(current)) {
      if (!/^(0|[1-9][0-9]*)$/.test(key))
        throw new TypeError(`JSON Pointer array token ${key} is invalid`);
      current = current[Number(key)];
      continue;
    }
    if (!isRecord(current) || !Object.hasOwn(current, key)) return undefined;
    current = current[key];
  }
  return current;
}
