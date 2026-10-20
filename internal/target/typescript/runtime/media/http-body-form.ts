import type { HTTPBodyEncoder } from "./http-media-types.js";
import type { WireEncodingDefinition, WireBodyDefinition } from "./media-contract-types.js";
import type { MediaCodec } from "./media-codec-types.js";
import type { WireSchema, WireSchemas } from "../schema/wire-types.js";
import type { OperationDefinition, ParameterDefinition } from "../http/operation.js";
import { composeQueryParameterEncoder, serializeQuery } from "../http/request/http-query-core.js";
import type { QueryParameterEncoder } from "../http/request/http-query-core.js";
import { isRecord, isJSONMediaType } from "../shared/runtime-support.js";
const encodeStyle: QueryParameterEncoder = composeQueryParameterEncoder(
  (
    _operation: OperationDefinition,
    _parameter: ParameterDefinition | undefined,
    value: unknown,
  ): unknown => value,
);
export const encodeFormBody: HTTPBodyEncoder = (
  _contentType: string,
  value: unknown,
  _codecs: ReadonlyMap<string, MediaCodec<unknown>>,
  _schema: WireSchema | undefined,
  _schemas: WireSchemas,
  definition: WireBodyDefinition | undefined,
): BodyInit => {
  if (!isRecord(value)) throw new TypeError("form body must be an object");
  const parts: string[] = [];
  for (const [name, item] of Object.entries(value)) {
    if (item === undefined) continue;
    const encoding: WireEncodingDefinition | undefined = definition?.encoding?.find(
      (entry: WireEncodingDefinition): boolean => entry.name === name,
    );
    if (
      encoding?.style !== undefined ||
      encoding?.explode !== undefined ||
      encoding?.allowReserved !== undefined
    ) {
      const style: string = encoding.style ?? "form";
      const operation: OperationDefinition = {
        route: "",
        method: "POST",
        path: "",
        envelope: "",
        parameters: [
          {
            name,
            property: name,
            location: "query",
            style,
            explode: encoding.explode ?? style === "form",
            allowReserved: encoding.allowReserved ?? false,
            allowEmptyValue: true,
          },
        ],
      };
      parts.push(serializeQuery(encodeStyle({ [name]: item }, operation, "query")));
      continue;
    }
    const form: URLSearchParams = new URLSearchParams();
    const json: boolean =
      encoding?.contentType !== undefined
        ? isJSONMediaType(encoding.contentType)
        : isRecord(item) || (Array.isArray(item) && item.some(isRecord));
    if (json) form.append(name, JSON.stringify(item));
    else if (Array.isArray(item)) for (const entry of item) form.append(name, String(entry));
    else form.append(name, String(item));
    parts.push(form.toString());
  }
  return parts.filter((part: string): boolean => part !== "").join("&");
};
