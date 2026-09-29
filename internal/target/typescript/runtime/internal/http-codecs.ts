import {
  decodeWireValue,
  encodeWireValue,
  transformWireValue,
  validateWireValue,
} from "./codecs.js";
import { createHTTPServices } from "./http-core.js";
import { createAdvancedHTTPServices } from "./http-advanced.js";
import type { StreamingRequestExecutionServices } from "./http-types.js";

function createFullRequestServices(): StreamingRequestExecutionServices {
  const wire = { decodeWireValue, encodeWireValue, transformWireValue, validateWireValue };
  // The callback is only used during a request, after both service sets exist.
  const advanced = createAdvancedHTTPServices(() => services, wire);
  const services = createHTTPServices(wire, advanced);
  return { ...services, createOperationStream: advanced.createOperationStream };
}

/** Full request services share the same core algorithms as specialized providers. */
export const fullRequestServices = /* @__PURE__ */ createFullRequestServices();
