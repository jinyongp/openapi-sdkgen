import type { HTTPBodyEncoder } from "./http-media-types.js";
import type { MediaCodec } from "./wire-types.js";
import { normalizeMediaType } from "./http-execution-support.js";
export const encodeCustomBody: HTTPBodyEncoder = (
  contentType: string,
  value: unknown,
  codecs: ReadonlyMap<string, MediaCodec<unknown>>,
): BodyInit | Promise<BodyInit> => {
  const codec: MediaCodec<unknown> | undefined = codecs.get(normalizeMediaType(contentType));
  if (codec?.encode === undefined) throw new TypeError(`missing encode codec for ${contentType}`);
  return codec.encode(value, { contentType });
};
