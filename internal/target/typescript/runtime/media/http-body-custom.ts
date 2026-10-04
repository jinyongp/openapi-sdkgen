import type { HTTPBodyEncoder } from "./http-media-types.js";
import type { MediaCodec } from "./media-codec-types.js";
import { normalizeMediaType } from "../shared/runtime-support.js";
export const encodeCustomBody: HTTPBodyEncoder = (
  contentType: string,
  value: unknown,
  codecs: ReadonlyMap<string, MediaCodec<unknown>>,
): BodyInit | Promise<BodyInit> => {
  const codec: MediaCodec<unknown> | undefined = codecs.get(normalizeMediaType(contentType));
  if (codec?.encode === undefined) throw new TypeError(`missing encode codec for ${contentType}`);
  return codec.encode(value, { contentType });
};
