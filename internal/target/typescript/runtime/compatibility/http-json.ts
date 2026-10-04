import type { BufferedRequestFunction } from "../http/request/request-execution-types.js";
import type { ClientOptions } from "../http/configuration.js";
import type { RequestExecutionServices } from "../http/http-types.js";
import { createRequestContext } from "./http-core.js";
import { createRequestCore } from "./http-core.js";
import { createHTTPServices } from "./http-core.js";
import { jsonWireCodec } from "./wire-engine.js";

/** Internal services for proven non-streaming, non-XML plans with JSON request bodies. */
export const jsonRequestServices: RequestExecutionServices = /* @__PURE__ */ createHTTPServices(
  jsonWireCodec,
  {
    encodeRequestBody(_contentType: string, value: unknown): BodyInit {
      return JSON.stringify(value);
    },
  },
);

/** Binds a proven JSON-only operation plan without importing advanced media implementations. */
export function createJSONRequest(options: ClientOptions): BufferedRequestFunction {
  return createRequestCore(createRequestContext(options), jsonRequestServices);
}
