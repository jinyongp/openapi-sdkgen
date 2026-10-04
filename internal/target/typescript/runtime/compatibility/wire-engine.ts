/** Forwards the canonical type contracts without importing their implementation. */
export type * from "./wire-contracts.js";
export { extendDynamicScope, resolveDynamicReference } from "../schema/references/wire-dynamic.js";
export { decodeSchemaContent } from "../schema/content/wire-content.js";
import type { SchemaContentDecoder, WireCodec } from "../schema/wire-types.js";
import { createWireCodec as createCoreWireCodec } from "../schema/wire-core.js";
import { fullWireHandlers } from "./wire-handlers.js";
import { decodeSchemaContent } from "../schema/content/wire-content.js";
/** Existing generic factory retains arbitrary-schema capabilities. */
export function createWireCodec(decodeContent: SchemaContentDecoder): WireCodec {
  return createCoreWireCodec({ ...fullWireHandlers, decodeContent });
}
export const jsonWireCodec: WireCodec = /* @__PURE__ */ createWireCodec(decodeSchemaContent);
