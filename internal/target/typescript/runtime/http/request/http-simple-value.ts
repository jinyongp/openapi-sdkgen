import { isRecord } from "../../shared/runtime-support.js";

/** Serializes an OpenAPI simple-style value without path escaping. */
export function serializeSimpleValue(value: unknown, explode: boolean): string {
  if (Array.isArray(value)) return value.map(String).join(",");
  if (isRecord(value)) {
    return Object.entries(value)
      .filter((entry: [string, unknown]): boolean => entry[1] !== undefined)
      .flatMap(([key, item]: [string, unknown]): string | string[] =>
        explode ? `${key}=${String(item)}` : [key, String(item)],
      )
      .join(",");
  }
  return String(value);
}
