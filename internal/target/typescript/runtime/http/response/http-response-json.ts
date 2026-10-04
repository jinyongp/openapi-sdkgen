import type { WireCodec } from "../../schema/wire-types.js";
import type { OperationDefinition } from "../operation.js";
import type { RequestMetadata } from "../../shared/runtime-support.js";
import type { ResponseDecodeOptions } from "../http-types.js";
import type { BufferedResponseServices } from "./http-response-service-types.js";
import { selectResponseDefinition, responseContentType } from "../http-execution-support.js";
import { decodeBufferedResponseValue, responseDecodeFailure } from "./http-response-body.js";
import { createHTTPErrorFactory } from "./http-response-error.js";
import { emptyResponseHeaders, transformPreparedResponse } from "./http-response-wire.js";
/** Declared JSON contracts need no XML, scalar-text, sequential or multipart schema adapter. */
export function createJSONResponseServices(wire: WireCodec): BufferedResponseServices {
  function decodeResponseWireValue(
    operation: OperationDefinition,
    response: Response,
    value: unknown,
  ): unknown {
    const definition: ReturnType<typeof selectResponseDefinition> = selectResponseDefinition(
      operation,
      response,
      true,
    );
    return transformPreparedResponse(wire, definition, operation, value);
  }
  async function decodeResponse(
    _operation: OperationDefinition,
    response: Response,
    request: RequestMetadata,
    options: ResponseDecodeOptions,
  ): Promise<unknown> {
    if (
      response.status === 204 ||
      response.status === 205 ||
      responseContentType(response) === undefined ||
      response.body === null
    )
      return undefined;
    try {
      return await decodeBufferedResponseValue(response, options.codecs);
    } catch (cause: unknown) {
      throw responseDecodeFailure(response, request, cause);
    }
  }
  return {
    decodeResponse,
    decodeResponseHeaders: emptyResponseHeaders,
    decodeResponseWireValue,
    createHTTPError: createHTTPErrorFactory(wire),
  };
}
