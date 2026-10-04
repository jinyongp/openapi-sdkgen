import type { HTTPBodyEncoder } from "./http-media-types.js";
import type { MediaCodec } from "./wire-types.js";
import { encodeCustomBody } from "./http-body-custom.js";
export const encodeBinaryBody: HTTPBodyEncoder = (
  contentType: string,
  value: unknown,
  codecs: ReadonlyMap<string, MediaCodec<unknown>>,
): BodyInit | Promise<BodyInit> => {
  if (value instanceof Blob || value instanceof ArrayBuffer || ArrayBuffer.isView(value))
    return value as BodyInit;
  return encodeCustomBody(
    contentType,
    value,
    codecs,
    undefined,
    {},
    undefined,
    undefined,
    undefined,
  );
};
