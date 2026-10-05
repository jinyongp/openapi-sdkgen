import type { RequestContext, RequestExecutionServices } from "../http-types.js";
import type { BufferedRequestFunction } from "./request-execution-types.js";
import { assertReadableResponseHeaders } from "../http-execution-support.js";
import { createBufferedRequestCore } from "./http-buffered-core.js";

/** Buffered general HTTP preserves readable headers without sequential-response support. */
export function createGeneralBufferedRequestCore(
  context: RequestContext,
  services: RequestExecutionServices,
): BufferedRequestFunction {
  return createBufferedRequestCore(context, services, {
    assertResponseHeaders: assertReadableResponseHeaders,
  });
}
