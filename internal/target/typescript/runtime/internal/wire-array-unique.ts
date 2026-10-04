import type { WireSchema } from "./wire-types.js";
import { wireValueEquals } from "./wire-equality.js";
import { isRecord } from "./runtime-support.js";
export function arrayUnique(value: unknown, schema: WireSchema): void {
  if (!Array.isArray(value)) return;
  if (schema.uniqueItems && !hasUniqueWireValues(value))
    throw new TypeError("must contain unique items");
}
function wireValueFingerprint(value: unknown): string {
  if (value === null) return "null";
  switch (typeof value) {
    case "undefined":
      return "undefined";
    case "boolean":
      return value ? "boolean:true" : "boolean:false";
    case "number":
      return `number:${value === 0 ? "0" : String(value)}`;
    case "string":
      return `string:${JSON.stringify(value)}`;
    case "bigint":
      return `bigint:${value.toString()}`;
    case "symbol":
      return `symbol:${String(value)}`;
    case "function":
      return "function";
  }
  if (Array.isArray(value)) {
    return `array:[${value
      .map((item: unknown, index: number): string =>
        Object.hasOwn(value, index) ? wireValueFingerprint(item) : "<sparse>",
      )
      .join(",")}]`;
  }
  if (isRecord(value)) {
    const keys: string[] = Object.keys(value).sort();
    return `object:{${keys
      .map((key: string): string => `${JSON.stringify(key)}:${wireValueFingerprint(value[key])}`)
      .join(",")}}`;
  }
  return `object:${Object.prototype.toString.call(value)}`;
}

function hasUniqueWireValues(values: readonly unknown[]): boolean {
  const buckets: Map<string, unknown[]> = new Map<string, unknown[]>();
  for (const value of values) {
    const fingerprint: string = wireValueFingerprint(value);
    const bucket: unknown[] | undefined = buckets.get(fingerprint);
    if (bucket !== undefined) {
      if (bucket.some((previous: unknown): boolean => wireValueEquals(previous, value)))
        return false;
      bucket.push(value);
    } else {
      buckets.set(fingerprint, [value]);
    }
  }
  return true;
}
