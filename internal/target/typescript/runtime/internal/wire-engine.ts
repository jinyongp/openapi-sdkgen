/** Forwards the canonical type contracts without importing their implementation. */
export type * from "./wire-types.js";
export { extendDynamicScope, resolveDynamicReference } from "./wire-dynamic.js";
export { decodeSchemaContent } from "./wire-content.js";
import type { SchemaContentDecoder, WireCodec } from "./wire-types.js";
import { createWireCodec as createCoreWireCodec } from "./wire-core.js";
import { fullWireHandlers } from "./wire-handlers.js";
import { decodeSchemaContent } from "./wire-content.js";
/** Existing generic factory retains arbitrary-schema capabilities. */
export function createWireCodec(decodeContent: SchemaContentDecoder): WireCodec {
  return createCoreWireCodec({ ...fullWireHandlers, decodeContent });
}
export const jsonWireCodec: WireCodec = /* @__PURE__ */ createWireCodec(decodeSchemaContent);
