/** Stable error API, sharing one runtime implementation with request and wire modules. */
export {
  APIError,
  TransportErrorCode,
  isAPIError,
  isErrorCode,
  getErrorCode,
  getRequestID,
} from "../shared/runtime-support.js";
export type { APIErrorOptions, TransportError } from "../shared/runtime-support.js";
