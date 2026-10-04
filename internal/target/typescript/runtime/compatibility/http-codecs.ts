import {
  decodeWireValue,
  encodeWireValue,
  transformWireValue,
  validateWireValue,
} from "./codecs.js";
import { createHTTPServices } from "./http-core.js";
import { createAdvancedHTTPServices } from "./http-advanced.js";
import type {
  RequestExecutionServices,
  StreamingRequestExecutionServices,
  AdvancedHTTPServices,
} from "../http/http-types.js";
import type { WireCodec } from "../schema/wire-types.js";

function createFullRequestServices(): StreamingRequestExecutionServices {
  const wire: WireCodec = {
    decodeWireValue,
    encodeWireValue,
    transformWireValue,
    validateWireValue,
  };
  // The callback is only used during a request, after both service sets exist.
  const advanced: AdvancedHTTPServices = createAdvancedHTTPServices(
    (): RequestExecutionServices => services,
    wire,
  );
  const services: RequestExecutionServices = createHTTPServices(wire, advanced);
  return { ...services, createOperationStream: advanced.createOperationStream };
}

/** Full request services share the same core algorithms as specialized providers. */
export const fullRequestServices: StreamingRequestExecutionServices =
  /* @__PURE__ */ createFullRequestServices();
