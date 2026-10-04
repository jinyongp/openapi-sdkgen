import type { WireCodec } from "../../schema/wire-types.js";
import type { OperationDefinition } from "../operation.js";
import type { RequestMetadata } from "../../shared/runtime-support.js";
import type { WireResponseDefinition } from "./http-response-types.js";
import type { BufferedResponseServices } from "./http-response-service-types.js";
import { APIError } from "../../shared/runtime-support.js";
import { registerHTTPError } from "./http-errors.js";
import { selectResponseDefinition, serverError } from "../http-execution-support.js";
/** Shares error provenance and mutation-time schema validation across HTTP response profiles. */
export function createHTTPErrorFactory(
  wire: WireCodec,
): BufferedResponseServices["createHTTPError"] {
  const { transformWireValue }: WireCodec = wire;
  function createHTTPError(
    operation: OperationDefinition,
    response: Response,
    request: RequestMetadata,
    data: unknown,
  ): APIError {
    const definition: WireResponseDefinition | undefined = selectResponseDefinition(
      operation,
      response,
      true,
    );
    const error: APIError = serverError(
      response,
      request,
      data,
      definition?.contentType === "" ? undefined : definition?.contentType,
    );
    if (definition === undefined) return error;
    return registerHTTPError(error, operation.route, (current: unknown): void => {
      if (response.ok || selectResponseDefinition(operation, response, true) !== definition)
        throw new TypeError("HTTP response no longer matches its declared representation");
      if (definition.contentType === "") {
        if (current !== undefined) throw new TypeError("bodyless HTTP response has a body");
      } else {
        // Validate mapped public names through the canonical output encoder,
        // including decoded content schemas and recursive/composed mappings.
        transformWireValue(current, definition.schema, operation.outputSchemas ?? {}, "encode");
      }
    });
  }
  return createHTTPError;
}
