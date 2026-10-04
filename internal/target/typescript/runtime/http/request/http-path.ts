import { isRecord } from "../../shared/runtime-support.js";
import type { ParameterDefinition } from "../operation.js";
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
