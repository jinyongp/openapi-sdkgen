import { bufferedXMLCodecExtensions, xmlWireCodec } from "./codecs.js";
import { createHTTPServices } from "./http-core.js";
import type { RequestExecutionServices } from "./http-types.js";

/** Buffered XML/JSON services for plans without form, multipart or streaming request bodies. */
export const bufferedXMLRequestServices: RequestExecutionServices =
  /* @__PURE__ */ createHTTPServices(xmlWireCodec, bufferedXMLCodecExtensions);
