import type { RequestExecutionServices } from "../http-types.js";
/** Canonical response decoding and error ports shared by buffered HTTP services. */
export type BufferedResponseServices = Pick<
  RequestExecutionServices,
  "decodeResponse" | "decodeResponseHeaders" | "decodeResponseWireValue" | "createHTTPError"
>;
