import type { HTTPBodyEncoder } from "./http-media-types.js";
/** Serializes a validated request value as a JSON body. */
export const encodeJSONBody: HTTPBodyEncoder = (_contentType: string, value: unknown): BodyInit =>
  JSON.stringify(value);
