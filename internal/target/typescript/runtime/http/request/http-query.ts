import type { WireCodec, WireSchema, WireSchemas } from "../../schema/wire-types.js";
import type { OperationDefinition, ParameterDefinition } from "../operation.js";
import type { QueryPart, QueryEncoder } from "../http-types.js";
import { isRecord } from "../../shared/runtime-support.js";
import { createParameterEncoder, findParameterByProperty } from "./http-request-values.js";
type ContentParameterEncoder = (
  value: unknown,
  contentType: string,
  schema: WireSchema | undefined,
  components: WireSchemas,
) => string;
type QueryParameterEncoder = (
  query: Readonly<Record<string, unknown>>,
  operation: OperationDefinition,
  location: "query" | "querystring",
) => QueryPart[];
/** Reuses synchronous OpenAPI query serialization independently from custom parameter codecs. */
export function createQueryParameterEncoder(
  wire: WireCodec,
  encodeContent?: ContentParameterEncoder,
): QueryParameterEncoder {
  const encodeParameterWireValue: ReturnType<typeof createParameterEncoder> =
    createParameterEncoder(wire);
  function serializeContentParameterSync(
    value: unknown,
    contentType: string,
    schema: WireSchema | undefined,
    components: WireSchemas,
  ): string {
    if (encodeContent === undefined)
      throw new TypeError(`missing parameter encode codec for ${contentType}`);
    return encodeContent(value, contentType, schema, components);
  }
  function appendQuerySync(
    query: Readonly<Record<string, unknown>>,
    operation: OperationDefinition,
    location: "query" | "querystring",
  ): QueryPart[] {
    const result: QueryPart[] = [];
    for (const [property, rawValue] of Object.entries(query)) {
      if (rawValue === undefined) continue;
      const parameter: ParameterDefinition | undefined = findParameterByProperty(
        operation,
        location,
        property,
      );
      const value: unknown = encodeParameterWireValue(operation, parameter, rawValue);
      if (parameter?.location === "querystring") {
        appendQuerystringSync(result, value, parameter, operation.inputSchemas ?? {});
        continue;
      }
      const name: string = parameter?.name ?? property;
      assertQueryEmptyValueAllowed(value, parameter);
      if (parameter?.contentType !== undefined) {
        appendQueryValue(
          result,
          name,
          serializeContentParameterSync(
            value,
            parameter.contentType,
            parameter.schema,
            operation.inputSchemas ?? {},
          ),
          parameter.allowReserved ?? false,
        );
        continue;
      }
      const style: string = parameter?.style ?? "form";
      const explode: boolean = parameter?.explode ?? true;
      if (style === "deepObject" && isRecord(value)) {
        for (const [key, item] of Object.entries(value))
          if (item !== undefined)
            appendQueryValue(result, `${name}[${key}]`, item, parameter?.allowReserved ?? false);
        continue;
      }
      if (Array.isArray(value)) {
        if (style === "spaceDelimited")
          appendQueryValue(
            result,
            name,
            value.map(String).join(" "),
            parameter?.allowReserved ?? false,
          );
        else if (style === "pipeDelimited")
          appendQueryValue(
            result,
            name,
            value.map(String).join("|"),
            parameter?.allowReserved ?? false,
          );
        else if (explode)
          for (const item of value)
            appendQueryValue(result, name, item, parameter?.allowReserved ?? false);
        else
          appendQueryValue(
            result,
            name,
            value.map(String).join(","),
            parameter?.allowReserved ?? false,
          );
        continue;
      }
      if (isRecord(value) && style === "form") {
        const entries: [string, unknown][] = Object.entries(value).filter(
          (entry: [string, unknown]): boolean => entry[1] !== undefined,
        );
        if (explode)
          for (const [key, item] of entries)
            appendQueryValue(result, key, item, parameter?.allowReserved ?? false);
        else
          appendQueryValue(
            result,
            name,
            entries
              .flatMap(([key, item]: [string, unknown]): string[] => [key, String(item)])
              .join(","),
            parameter?.allowReserved ?? false,
          );
        continue;
      }
      if (isRecord(value) && (style === "spaceDelimited" || style === "pipeDelimited")) {
        const separator: " " | "|" = style === "spaceDelimited" ? " " : "|";
        const entries: [string, unknown][] = Object.entries(value).filter(
          (entry: [string, unknown]): boolean => entry[1] !== undefined,
        );
        appendQueryValue(
          result,
          name,
          explode
            ? entries
                .map(([key, item]: [string, unknown]): string => `${key}=${String(item)}`)
                .join(separator)
            : entries
                .flatMap(([key, item]: [string, unknown]): string[] => [key, String(item)])
                .join(separator),
          parameter?.allowReserved ?? false,
        );
        continue;
      }
      appendQueryValue(result, name, value, parameter?.allowReserved ?? false);
    }
    return result;
  }

  function appendQuerystringSync(
    query: QueryPart[],
    value: unknown,
    parameter: ParameterDefinition,
    components: WireSchemas,
  ): void {
    const contentType: string | undefined = parameter.contentType?.toLowerCase();
    if (contentType === "application/x-www-form-urlencoded") {
      if (!isRecord(value)) throw new TypeError("querystring form content must be an object");
      for (const [name, item] of Object.entries(value)) {
        if (item === undefined) continue;
        if (Array.isArray(item))
          for (const entry of item) query.push({ name, value: String(entry) });
        else query.push({ name, value: String(item) });
      }
      return;
    }
    if (contentType === "application/json") {
      query.push({ raw: encodeURIComponent(JSON.stringify(value)) });
      return;
    }
    query.push({
      raw: encodeURIComponent(
        serializeContentParameterSync(
          value,
          parameter.contentType ?? "text/plain",
          parameter.schema,
          components,
        ),
      ),
    });
  }

  return appendQuerySync;
}
/** Binds the same parameter validation and escaping to a complete query string. */
export function createQueryEncoder(wire: WireCodec): QueryEncoder {
  const encode: QueryParameterEncoder = createQueryParameterEncoder(wire);
  return (
    query: Readonly<Record<string, unknown>>,
    operation: OperationDefinition,
    location: "query" | "querystring",
  ): string => serializeQuery(encode(query, operation, location));
}
/** Enforces the declared empty-value policy for a form query parameter. */
export function assertQueryEmptyValueAllowed(
  value: unknown,
  parameter: ParameterDefinition | undefined,
): void {
  if (
    value === "" &&
    parameter !== undefined &&
    parameter.location === "query" &&
    parameter.contentType === undefined &&
    parameter.style === "form" &&
    parameter.allowEmptyValue !== true
  )
    throw new TypeError(`Empty query parameter ${parameter.name} requires allowEmptyValue: true`);
}

/** Records a query value without losing its reserved-character policy. */
export function appendQueryValue(
  query: QueryPart[],
  name: string,
  value: unknown,
  allowReserved: boolean,
): void {
  if (
    isRecord(value) &&
    typeof value["field"] === "string" &&
    typeof value["direction"] === "string"
  ) {
    query.push({ name, value: `${value["field"]}:${value["direction"]}`, allowReserved });
    return;
  }
  if (typeof value === "object" && value !== null) {
    query.push({ name, value: JSON.stringify(value), allowReserved });
    return;
  }
  query.push({ name, value: String(value), allowReserved });
}

/** Escapes query parts using their declared OpenAPI reserved-character policy. */
export function serializeQuery(query: readonly QueryPart[]): string {
  return query
    .map(
      (part: QueryPart): string =>
        part.raw ??
        `${encodeURIComponent(part.name ?? "")}=${part.allowReserved ? encodeReservedQueryValue(part.value ?? "") : encodeURIComponent(part.value ?? "")}`,
    )
    .join("&");
}

function encodeReservedQueryValue(value: string): string {
  return encodeURIComponent(value)
    .replace(/%25([0-9a-f]{2})/gi, "%$1")
    .replace(/%3A|%2F|%3F|%40|%21|%24|%27|%28|%29|%2A|%2C|%3B|%3D/gi, (encoded: string): string =>
      decodeURIComponent(encoded),
    );
}
