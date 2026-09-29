import { bufferedXMLCodecExtensions, xmlWireCodec } from "./codecs.js";
import { createHTTPServices } from "./http-core.js";

/** Buffered XML/JSON services for plans without form, multipart or streaming request bodies. */
export const bufferedXMLRequestServices = /* @__PURE__ */ createHTTPServices(
  xmlWireCodec,
  bufferedXMLCodecExtensions,
);
