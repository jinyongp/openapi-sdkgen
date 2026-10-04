import type {
  ServerCodecContext,
  BoundServerFunction,
  InboundSchema,
  InboundSchemas,
} from "./runtime-types.js";
import type { WireSchema, WireSchemas } from "../schema/wire-types.js";
import type { XMLCodec } from "../media/xml/xml-types.js";
export function requireServerXML(context: ServerCodecContext): XMLCodec {
  if (context.xml === undefined) throw new TypeError("Required XML service is missing");
  return context.xml;
}
export function requireServerHook<Hook>(hook: Hook | undefined): Hook {
  if (hook === undefined) throw new TypeError("Required inbound service is missing");
  return hook;
}
export function decodeXMLBody(
  context: ServerCodecContext,
  source: string,
  schema: InboundSchema | undefined,
  schemas: InboundSchemas,
  wireSchema: WireSchema | undefined = undefined,
  wireSchemas: WireSchemas | undefined = undefined,
): unknown {
  // Generic inbound helpers also accept raw schemas and preserve their XML
  // namespace/node mapping. Both generated and generic routes share this hook.
  return requireServerHook(context.decodeLegacyXML)(
    context,
    source,
    schema,
    schemas,
    wireSchema,
    wireSchemas,
  );
}
export function bindServerCodec<Arguments extends unknown[], Result>(
  context: ServerCodecContext,
  handler: (context: ServerCodecContext, ...args: Arguments) => Result,
): BoundServerFunction<Arguments, Result> {
  return (...args: Arguments): Result => handler(context, ...args);
}
