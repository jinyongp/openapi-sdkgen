import type { WireCodec } from "../../schema/wire-types.js";
import type { RequestExecutionServices, QueryEncoder } from "../http-types.js";
import type { HTTPCodecExtensions } from "../../media/media-service-types.js";
import {
  createParameterEncoder,
  serializeSchemaPathParameter,
  resolveOperationBaseURL,
} from "./http-request-values.js";
import { createResponseServices } from "../response/http-response-services.js";
import { composeBasicHTTPServices } from "./http-basic-core.js";
/** Generic compatibility composition retains the complete normalized contract behavior. */
export function createBasicHTTPServices(
  wire: WireCodec,
  extensions: HTTPCodecExtensions,
  queryEncoder?: QueryEncoder,
): RequestExecutionServices {
  return composeBasicHTTPServices(
    wire,
    extensions,
    {
      encodeParameter: createParameterEncoder(wire),
      serializePathParameter: serializeSchemaPathParameter,
      resolveBaseURL: resolveOperationBaseURL,
    },
    createResponseServices(wire, extensions),
    queryEncoder,
  );
}
