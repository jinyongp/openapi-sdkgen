import type { WireBodyDefinition } from "../../media/media-contract-types.js";
import type { WireCodec } from "../../schema/wire-types.js";
import type {
  OperationDefinition,
  ParameterDefinition,
  ServerDefinition,
  ServerVariableDefinition,
  ServerSelection,
} from "../operation.js";
import { isRecord } from "../../shared/runtime-support.js";
import { operationDiagnosticName } from "../operation.js";
import {
  mediaTypeMatches,
  mediaTypeMatchScore,
  normalizeBaseURL,
} from "../http-execution-support.js";

const reservedHeaders: ReadonlySet<string> = /* @__PURE__ */ new Set([
  "accept",
  "authorization",
  "content-type",
  "x-csrf-token",
  "x-request-id",
]);

/** Rejects serialized dot segments before URL normalization can change the route. */
export function assertSafeOperationPath(path: string): void {
  for (const segment of path.split("/")) {
    const dots: string = segment.toLowerCase().replaceAll("%2e", ".");
    if (dots === "." || dots === "..")
      throw new TypeError(
        "Operation path contains a URL dot-segment after parameter serialization",
      );
  }
}

/** Selects the most specific declared request body media representation. */
export function selectRequestBodyDefinition(
  bodies: readonly WireBodyDefinition[],
  contentType: string,
): WireBodyDefinition | undefined {
  return bodies
    .filter((body: WireBodyDefinition): boolean => mediaTypeMatches(body.contentType, contentType))
    .sort(
      (left: WireBodyDefinition, right: WireBodyDefinition): number =>
        mediaTypeMatchScore(right.contentType, contentType) -
        mediaTypeMatchScore(left.contentType, contentType),
    )[0];
}

/** Resolves explicit client defaults or the declared server and its variables. */
export function resolveOperationBaseURL(
  baseURL: string | undefined,
  origin: string | undefined,
  selection: ServerSelection | undefined,
  operation: OperationDefinition,
): string {
  if (baseURL !== undefined) return baseURL;
  const servers: readonly ServerDefinition[] = operation.servers ?? [{ id: "#", url: "/" }];
  const server: ServerDefinition | undefined =
    selection?.id === undefined
      ? servers[0]
      : servers.find((item: ServerDefinition): boolean => item.id === selection.id);
  if (server === undefined)
    throw new TypeError(
      `Unknown server ${selection?.id} for operation ${operationDiagnosticName(operation)}`,
    );
  const variables: Readonly<Record<string, string>> = selection?.variables ?? {};
  const expanded: string = server.url.replace(/\{([^}]+)\}/g, (_: string, name: string): string => {
    const definition: ServerVariableDefinition | undefined = server.variables?.find(
      (item: ServerVariableDefinition): boolean => item.name === name,
    );
    if (definition === undefined)
      throw new TypeError(`Server ${server.id} has no variable ${name}`);
    const value: string = variables[name] ?? definition.defaultValue;
    if (definition.enumValues !== undefined && !definition.enumValues.includes(value)) {
      throw new TypeError(
        `Server variable ${name} must be one of ${definition.enumValues.join(", ")}`,
      );
    }
    return value;
  });
  try {
    return normalizeBaseURL(new URL(expanded).href);
  } catch (cause: unknown) {
    try {
      new URL(expanded);
    } catch {
      if (origin === undefined)
        throw new TypeError(
          `Server ${server.id} is relative; pass ClientOptions.origin or baseURL`,
        );
      const absoluteOrigin: string = normalizeOrigin(origin);
      return normalizeBaseURL(new URL(expanded, absoluteOrigin).href);
    }
    throw cause;
  }
}

function normalizeOrigin(value: string): string {
  const url: URL = new URL(value);
  if (
    (url.protocol !== "http:" && url.protocol !== "https:") ||
    url.pathname !== "/" ||
    url.search ||
    url.hash
  ) {
    throw new TypeError(
      "origin must be an absolute http(s) origin without path, query, or fragment",
    );
  }
  return url.origin;
}

/** Appends extra headers while rejecting reserved and declared header collisions. */
export function appendRawHeaders(
  target: Headers,
  source: HeadersInit | undefined,
  contractNames: ReadonlySet<string>,
): void {
  if (source === undefined) return;
  const incoming: Headers = new Headers(source);
  incoming.forEach((value: string, name: string): void => {
    const lower: string = name.toLowerCase();
    if (reservedHeaders.has(lower) || contractNames.has(lower)) {
      throw new TypeError(`Raw header ${name} must use its typed option`);
    }
    target.set(name, value);
  });
}

/** Sets a typed header when its value is supplied. */
export function setHeader(headers: Headers, name: string, value: string | undefined): void {
  if (value !== undefined) headers.set(name, value);
}

/** Rejects undefined array items before request validation and serialization. */
export function rejectUndefinedArrayValues(value: unknown): void {
  if (Array.isArray(value)) {
    for (const item of value) {
      if (item === undefined) {
        throw new TypeError("Request arrays cannot contain undefined");
      }
      rejectUndefinedArrayValues(item);
    }
    return;
  }
  if (isRecord(value)) {
    for (const item of Object.values(value)) rejectUndefinedArrayValues(item);
  }
}

function serializePathValue(value: unknown, explode: boolean, arraySeparator: string): string {
  if (Array.isArray(value))
    return value
      .map((item: unknown): string => encodeURIComponent(String(item)))
      .join(explode ? arraySeparator : ",");
  if (isRecord(value)) {
    return Object.entries(value)
      .filter((entry: [string, unknown]): boolean => entry[1] !== undefined)
      .flatMap(([key, item]: [string, unknown]): string | string[] =>
        explode
          ? `${encodeURIComponent(key)}=${encodeURIComponent(String(item))}`
          : [encodeURIComponent(key), encodeURIComponent(String(item))],
      )
      .join(explode ? arraySeparator : ",");
  }
  return encodeURIComponent(String(value));
}

/** Serializes a schema-based path parameter without a media codec dependency. */
export function serializeSchemaPathParameter(
  parameter: ParameterDefinition | undefined,
  name: string,
  value: unknown,
): string {
  const style: string = parameter?.style ?? "simple";
  const explode: boolean = parameter?.explode ?? false;
  const encoded: string = serializePathValue(value, explode, style === "label" ? "." : ",");
  if (style === "label") return `.${encoded}`;
  if (style !== "matrix") return encoded;
  if (Array.isArray(value) && explode)
    return value
      .map(
        (item: unknown): string =>
          `;${encodeURIComponent(name)}=${encodeURIComponent(String(item))}`,
      )
      .join("");
  if (isRecord(value) && explode)
    return Object.entries(value)
      .filter((entry: [string, unknown]): boolean => entry[1] !== undefined)
      .map(
        ([key, item]: [string, unknown]): string =>
          `;${encodeURIComponent(key)}=${encodeURIComponent(String(item))}`,
      )
      .join("");
  return `;${encodeURIComponent(name)}=${encoded}`;
}

type ParameterWireEncoder = (
  operation: OperationDefinition,
  parameter: ParameterDefinition | undefined,
  value: unknown,
) => unknown;
/** Reuses the canonical sort mapping and parameter validation before serialization. */
export function createParameterEncoder(wire: WireCodec): ParameterWireEncoder {
  const { transformWireValue }: WireCodec = wire;
  function encodeParameterWireValue(
    operation: OperationDefinition,
    parameter: ParameterDefinition | undefined,
    value: unknown,
  ): unknown {
    if (parameter?.sort !== undefined && Array.isArray(value)) {
      value = value.map((entry: unknown): string => {
        if (
          !isRecord(entry) ||
          typeof entry["field"] !== "string" ||
          typeof entry["direction"] !== "string"
        ) {
          throw new TypeError(`Invalid structured sort value for ${parameter.name}`);
        }
        const wire: string | undefined =
          parameter.sort?.[`${entry["field"]}\u0000${entry["direction"]}`];
        if (wire === undefined)
          throw new TypeError(`Invalid structured sort value for ${parameter.name}`);
        return wire;
      });
    }
    return parameter?.schema === undefined
      ? value
      : transformWireValue(value, parameter.schema, operation.inputSchemas ?? {}, "encode");
  }

  return encodeParameterWireValue;
}

/** Finds a normalized parameter by its public property and request location. */
export function findParameterByProperty(
  operation: OperationDefinition,
  location: ParameterDefinition["location"],
  property: string,
): ParameterDefinition | undefined {
  return operation.parameters?.find(
    (parameter: ParameterDefinition): boolean =>
      parameter.location === location && parameter.property === property,
  );
}
