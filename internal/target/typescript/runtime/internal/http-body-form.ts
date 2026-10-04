import type { HTTPBodyEncoder } from "./http-media-types.js";
import type {
  WireEncodingDefinition,
  MediaCodec,
  WireSchema,
  WireSchemas,
  WireBodyDefinition,
} from "./wire-types.js";
import { isRecord, isJSONMediaType } from "./runtime-support.js";
export const encodeFormBody: HTTPBodyEncoder = (
  _contentType: string,
  value: unknown,
  _codecs: ReadonlyMap<string, MediaCodec<unknown>>,
  _schema: WireSchema | undefined,
  _schemas: WireSchemas,
  definition: WireBodyDefinition | undefined,
): BodyInit => {
  if (!isRecord(value)) throw new TypeError("form body must be an object");
  const form: URLSearchParams = new URLSearchParams();
  for (const [name, item] of formEntries(value, definition?.encoding)) form.append(name, item);
  return form;
};
function formEntries(
  value: Record<string, unknown>,
  encoding: readonly WireEncodingDefinition[] | undefined,
): readonly [string, string][] {
  const result: [string, string][] = [];
  for (const [name, item] of Object.entries(value)) {
    if (item === undefined) continue;
    const definition: WireEncodingDefinition | undefined = encoding?.find(
      (entry: WireEncodingDefinition): boolean => entry.name === name,
    );
    if (definition?.contentType !== undefined && isJSONMediaType(definition.contentType)) {
      result.push([name, JSON.stringify(item)]);
      continue;
    }
    const explode: boolean = definition?.explode ?? true;
    if (Array.isArray(item)) {
      if (explode) for (const entry of item) result.push([name, String(entry)]);
      else result.push([name, item.map(String).join(",")]);
      continue;
    }
    if (isRecord(item)) {
      const entries: [string, unknown][] = Object.entries(item).filter(
        (entry: [string, unknown]): entry is [string, unknown] => entry[1] !== undefined,
      );
      if (explode) for (const [key, entry] of entries) result.push([key, String(entry)]);
      else
        result.push([
          name,
          entries
            .flatMap(([key, entry]: [string, unknown]): string[] => [key, String(entry)])
            .join(","),
        ]);
      continue;
    }
    result.push([name, String(item)]);
  }
  return result;
}
