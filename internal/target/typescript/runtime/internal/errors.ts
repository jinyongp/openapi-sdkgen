/** Stable error API, sharing one runtime implementation with request and wire modules. */
export {
  APIError,
  TransportErrorCode,
  isAPIError,
  isErrorCode,
  getErrorCode,
  getRequestID,
} from "./runtime-support.js";
export type { APIErrorOptions, TransportError } from "./runtime-support.js";
