import type { RequestFunction } from "../client/callables.js";
import type { ClientOptions } from "../http/configuration.js";
import { createRequestContext } from "./http-core.js";
import { createRequestCore } from "./http-core.js";
import { fullRequestServices } from "./http-codecs.js";

/**
 * Creates the endpoint-neutral Fetch API request executor used by a generated client.
 *
 * The executor resolves paths, serializes OpenAPI parameters and bodies, maps wire
 * names, applies request options, handles cancellation/timeouts, decodes successful
 * responses, and normalizes failures as {@link APIError}. It never retries requests.
 *
 * @param options Client-wide base URL and transport defaults.
 * @returns Low-level request function used by generated operation bindings.
 */
export function createRequest(options: ClientOptions): RequestFunction {
  return createRequestCore(createRequestContext(options), fullRequestServices);
}
