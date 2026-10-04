import type { HTTPBodyEncoder } from "./http-media-types.js";
export const encodeTextBody: HTTPBodyEncoder = (_contentType: string, value: unknown): BodyInit =>
  String(value);
