import { isRecord } from "./runtime-support.js";
/** One query parameter value with its OpenAPI escaping and serialization rules. */
export interface QueryPart {
  readonly name?: string;
  readonly value?: string;
  readonly allowReserved?: boolean;
  readonly raw?: string;
}
/** Serializes one already encoded wire value for query or URLencoded form use. */
export function encodeStyleQueryValue(
  name: string,
  value: unknown,
  style: string,
  explode: boolean,
  allowReserved: boolean,
): QueryPart[] {
  const result: QueryPart[] = [];
  if (style === "deepObject" && isRecord(value)) {
    for (const [key, item] of Object.entries(value))
      if (item !== undefined) appendQueryValue(result, `${name}[${key}]`, item, allowReserved);
    return result;
  }
  if (Array.isArray(value)) {
    if (style === "spaceDelimited")
      appendQueryValue(result, name, value.map(String).join(" "), allowReserved);
    else if (style === "pipeDelimited")
      appendQueryValue(result, name, value.map(String).join("|"), allowReserved);
    else if (explode) for (const item of value) appendQueryValue(result, name, item, allowReserved);
    else appendFormTokens(result, name, value.map(String), allowReserved);
    return result;
  }
  if (isRecord(value) && style === "form") {
    const entries: [string, unknown][] = Object.entries(value).filter(
      (entry: [string, unknown]): boolean => entry[1] !== undefined,
    );
    if (explode)
      for (const [key, item] of entries) appendQueryValue(result, key, item, allowReserved);
    else
      appendFormTokens(
        result,
        name,
        entries.flatMap(([key, item]: [string, unknown]): string[] => [key, String(item)]),
        allowReserved,
      );
    return result;
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
      allowReserved,
    );
    return result;
  }
  appendQueryValue(result, name, value, allowReserved);
  return result;
}
function appendFormTokens(
  query: QueryPart[],
  name: string,
  tokens: readonly string[],
  allowReserved: boolean,
): void {
  const encode: (value: string) => string = allowReserved
    ? encodeReservedQueryValue
    : encodeURIComponent;
  query.push({ raw: `${encodeURIComponent(name)}=${tokens.map(encode).join(",")}` });
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
