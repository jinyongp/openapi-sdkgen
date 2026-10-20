import {
  appendQueryValue,
  serializeQuery,
  encodeStyleQueryValue,
} from "../../shared/query-serialization.js";
export { appendQueryValue, serializeQuery } from "../../shared/query-serialization.js";
import type { WireSchema, WireSchemas } from "../../schema/wire-types.js";
import type { OperationDefinition, ParameterDefinition } from "../operation.js";
import type { QueryPart, QueryEncoder } from "../http-types.js";
import { isRecord } from "../../shared/runtime-support.js";
import { findParameterByProperty } from "./http-request-input.js";
import type { ParameterWireEncoder } from "./http-request-parameter-types.js";
/** Encodes a media-based query or header parameter. */
export type ContentParameterEncoder = (
  value: unknown,
  contentType: string,
  schema: WireSchema | undefined,
  components: WireSchemas,
) => string;
/** Encodes a parameter with its operation-owned schema. */
export type QueryParameterEncoder = (
  query: Readonly<Record<string, unknown>>,
  operation: OperationDefinition,
  location: "query" | "querystring",
) => QueryPart[];
/** Reuses synchronous OpenAPI query serialization independently from custom parameter codecs. */
export function composeQueryParameterEncoder(
  encodeParameterWireValue: ParameterWireEncoder,
  encodeContent?: ContentParameterEncoder,
): QueryParameterEncoder {
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
      result.push(
        ...encodeStyleQueryValue(name, value, style, explode, parameter?.allowReserved ?? false),
      );
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
export function createPreparedQueryEncoder(encodeParameter: ParameterWireEncoder): QueryEncoder {
  const encode: QueryParameterEncoder = composeQueryParameterEncoder(encodeParameter);
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
