import { xmlWireCodec, encodeXML, decodeXML } from "../internal/codecs.js";
import type { ServerCodecContext, ServerBound } from "./runtime-types.js";
import { bindServerCodec } from "./runtime-codecs.js";
import { decodeLegacyXML } from "./runtime-legacy-xml.js";
import { decodeInboundFormValue } from "./runtime-parameters.js";
import { decodeInboundJSONFrames } from "./runtime-json-stream.js";
import { decodeInboundSSEFrames } from "./runtime-sse-stream.js";
import { decodeInboundMultipartStream } from "./runtime-multipart.js";
export { InboundRequestError } from "./runtime-errors.js";
export type {
  Authenticate,
  InboundBodyOptions,
  InboundBodyPlan,
  InboundParameterDefinition,
  InboundParameterValues,
  InboundRequestContext,
  InboundResponse,
  InboundResponseDefinition,
  InboundResponseOptions,
  InboundSchema,
  InboundSchemas,
  InboundSecurityCandidate,
  InboundSecuritySchemes,
} from "./runtime-types.js";
import { decodeInboundParameters as decodeInboundParametersImpl } from "./runtime-parameters.js";
export { matchInboundRoute } from "./runtime-shared.js";
export { requiresInboundAuthentication } from "./runtime-authentication.js";
export { collectInboundSecurityCandidates } from "./runtime-authentication.js";
import { decodeInboundBody as decodeInboundBodyImpl } from "./runtime-body.js";
export { normalizeInboundMediaCodecs } from "./runtime-shared.js";
export { normalizeInboundStreamCodecs } from "./runtime-shared.js";
import { responseFromHandler as responseFromHandlerImpl } from "./runtime-response.js";
const codecContext: ServerCodecContext = {
  wire: xmlWireCodec,
  xml: { encodeXML, decodeXML },
  decodeFormValue: decodeInboundFormValue,
  decodeLegacyXML,
  frames: {
    "line-delimited-json": decodeInboundJSONFrames,
    "json-sequence": decodeInboundJSONFrames,
    sse: decodeInboundSSEFrames,
    multipart: decodeInboundMultipartStream,
  },
};
export const decodeInboundParameters: ServerBound<typeof decodeInboundParametersImpl> =
  /* @__PURE__ */ bindServerCodec(codecContext, decodeInboundParametersImpl);
export const decodeInboundBody: ServerBound<typeof decodeInboundBodyImpl> =
  /* @__PURE__ */ bindServerCodec(codecContext, decodeInboundBodyImpl);
export const responseFromHandler: ServerBound<typeof responseFromHandlerImpl> =
  /* @__PURE__ */ bindServerCodec(codecContext, responseFromHandlerImpl);
