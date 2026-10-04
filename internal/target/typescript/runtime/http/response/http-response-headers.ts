import type { WireCodec } from "../../schema/wire-types.js";
import type { WireResponseDefinition } from "./http-response-types.js";
import type { MediaCodec } from "../../media/media-codec-types.js";
import type { OperationDefinition } from "../operation.js";
import type { HTTPCodecExtensions } from "../../media/media-service-types.js";
import type { RequestExecutionServices } from "../http-types.js";
import { defineOwnDataProperty } from "../../shared/runtime-support.js";
import { selectResponseDefinition } from "../http-execution-support.js";
import { createHeaderContentDecoder } from "./http-header-content.js";
/** Declared response headers require their own content and serialization handlers. */
export function createResponseHeaderDecoder(
  wire: WireCodec,
  extensions: HTTPCodecExtensions,
): RequestExecutionServices["decodeResponseHeaders"] {
  const { decodeWireValue, validateWireValue }: WireCodec = wire;
  async function decodeResponseHeaders(
    operation: OperationDefinition,
    response: Response,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
  ): Promise<Readonly<Record<string, unknown>>> {
    const definition: WireResponseDefinition | undefined = selectResponseDefinition(
      operation,
      response,
      false,
    );
    const values: Record<string, unknown> = Object.create(null) as Record<string, unknown>;
    for (const header of definition?.headers ?? []) {
      const value: string | null = response.headers.get(header.name);
      if (value === null) {
        if (header.required) throw new TypeError(`missing required response header ${header.name}`);
        continue;
      }
      const decoded: unknown = await decodeResponseHeaderValue(
        header.name,
        value,
        header.schema,
        header.contentType,
        header.explode,
        operation.outputSchemas ?? {},
        codecs,
      );
      validateWireValue(decoded, header.schema, operation.outputSchemas ?? {}, "decode");
      defineOwnDataProperty(
        values,
        header.property,
        decodeWireValue(decoded, header.schema, operation.outputSchemas ?? {}),
      );
    }
    return values;
  }

  const decodeResponseHeaderValue: ReturnType<typeof createHeaderContentDecoder> =
    createHeaderContentDecoder(
      wire,
      extensions.decodeXML === undefined ? undefined : { decodeXML: extensions.decodeXML },
    );

  return decodeResponseHeaders;
}
