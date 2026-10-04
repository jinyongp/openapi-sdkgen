import { registerHTTPErrorMethod } from "../http/response/http-errors.js";

/** Registers both invocation identities and attaches the raw capability. */
export function bindOperationMethods(call: object, raw: object, route: string): unknown {
  registerHTTPErrorMethod(call, route);
  registerHTTPErrorMethod(raw, route);
  return Object.assign(call, { raw });
}
