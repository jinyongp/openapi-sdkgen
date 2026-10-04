import type {
  MediaCodec,
  StreamCodec,
  StreamContext,
  StreamFraming,
  WireCodec,
  WireEncodingDefinition,
  WireHeaderDefinition,
  WireSchema,
  WireSchemas,
} from "../internal/wire-types.js";
import type { Mutable } from "../internal/objects.js";
import type { XMLCodec } from "../internal/xml-types.js";

/** Metadata provided to host-owned inbound authentication policy. */
export interface InboundRequestContext {
  readonly request: Request;
  readonly operationID: string;
  readonly method: string;
  readonly path: string;
  /** Decoded parameters grouped by location and keyed by exact OpenAPI names. */
  readonly params: InboundParameterValues;
  readonly security: unknown;
  readonly securityCandidates: Readonly<Record<string, InboundSecurityCandidate>>;
}

/** Exact decoded inbound parameter containers, separated by OpenAPI location. */
export interface InboundParameterValues {
  readonly path: Readonly<Record<string, unknown>>;
  readonly query: Readonly<Record<string, unknown>>;
  readonly querystring: Readonly<Record<string, unknown>>;
  readonly headerParams: Readonly<Record<string, unknown>>;
  readonly cookieParams: Readonly<Record<string, unknown>>;
}

/** Raw candidate derived from one declared inbound security scheme. */
export interface InboundSecurityCandidate {
  readonly scheme: string;
  readonly type: string;
  readonly location?: "header" | "query" | "cookie";
  readonly name?: string;
  readonly value?: string;
}

/** Lossless Security Scheme Object map used only to collect request candidates. */
export type InboundSecuritySchemes = Readonly<Record<string, Readonly<Record<string, unknown>>>>;

/** One generated inbound query, header, or cookie parameter. */
export interface InboundParameterDefinition {
  readonly location: "path" | "query" | "querystring" | "header" | "cookie";
  readonly name: string;
  readonly property: string;
  readonly style: string;
  readonly explode: boolean;
  readonly allowReserved: boolean;
  readonly allowEmptyValue?: boolean;
  readonly required: boolean;
  readonly contentType?: string | undefined;
  readonly schema: InboundSchema;
  /** Full generated input schema: validates wire names then maps them to TS names. */
  readonly wireSchema: WireSchema;
  readonly sort?: Readonly<Record<string, string>>;
}

/** Return void to continue or a Response to reject the inbound request. */
export type Authenticate = (
  context: InboundRequestContext,
) => void | Response | Promise<void | Response>;

export interface InboundCookies {
  readonly raw: Readonly<Record<string, string | readonly string[]>>;
  readonly decoded: Readonly<Record<string, string | readonly string[]>>;
}

/** Framework-neutral response produced by an inbound generated handler. */
export interface InboundResponse {
  readonly status: number;
  readonly contentType?: string | undefined;
  readonly headers?: HeadersInit | undefined;
  /** Typed values keyed by generated response-header property names. */
  readonly headerValues?: Readonly<Record<string, unknown>> | undefined;
  readonly body?: unknown;
}

/** One generated response representation accepted by an inbound handler. */
export interface InboundResponseDefinition {
  /** Exact status code, status range (for example 2XX), or default. */
  readonly status: string;
  /** Exact generated response media type, when the response has a body. */
  readonly contentType?: string | undefined;
  /** Output-projected wire schema used to validate and encode the body. */
  readonly schema?: WireSchema | undefined;
  /** Declared response headers validated before the response is returned. */
  readonly headers?: readonly WireHeaderDefinition[] | undefined;
}

/** Generated response plans and component schemas for one inbound endpoint. */
export interface InboundResponseOptions {
  readonly schemas: WireSchemas;
  readonly responses: readonly InboundResponseDefinition[];
  readonly codecs?: ReadonlyMap<string, MediaCodec<unknown>> | undefined;
}

/** JSON Schema fragments used by generated inbound request validation. */
export type InboundSchema = Readonly<Record<string, unknown>> | boolean;

/** Component schemas used to resolve local inbound $ref values. */
export type InboundSchemas = Readonly<Record<string, InboundSchema>>;

/** One declared media representation selected from an inbound request body. */
export interface InboundBodyPlan {
  readonly contentType: string;
  readonly binary: boolean;
  readonly stream: boolean;
  readonly streamFraming?: StreamFraming | undefined;
  readonly schema?: InboundSchema | undefined;
  readonly wireSchema?: WireSchema | undefined;
  readonly encoding?: readonly WireEncodingDefinition[] | undefined;
  readonly prefixEncoding?: readonly WireEncodingDefinition[] | undefined;
  readonly itemEncoding?: WireEncodingDefinition | undefined;
}

/** Body contract selected from an OpenAPI Request Body Object. */
export interface InboundBodyOptions {
  readonly required: boolean;
  readonly plans: readonly InboundBodyPlan[];
  readonly schemas: InboundSchemas;
  /** Generated wire-name mapping for the decoded body or stream item. */
  readonly wireSchema?: WireSchema | undefined;
  /** Generated component mappings used by wireSchema. */
  readonly wireSchemas?: WireSchemas | undefined;
  /** Host codecs for declared custom complete inbound media values. */
  readonly codecs?: ReadonlyMap<string, MediaCodec<unknown>> | undefined;
  /** Stream protocol/adapter overrides for inbound sequential media. */
  readonly streamCodecs?: ReadonlyMap<string, StreamCodec> | undefined;
  /** Maximum total bytes accepted for one complete inbound request body. */
  readonly maxBodyBytes?: number | undefined;
  /** Maximum byte count a custom inbound stream protocol may request in one read. */
  readonly maxStreamFrameBytes?: number | undefined;
}

export interface InboundProtocolDecodeOptions {
  readonly rawContentType: string;
  readonly streamFraming: StreamFraming | undefined;
  readonly schema: InboundSchema | undefined;
  readonly schemas: InboundSchemas;
  readonly complete: boolean;
  readonly prefixEncoding: readonly WireEncodingDefinition[] | undefined;
  readonly itemEncoding: WireEncodingDefinition | undefined;
  readonly wireSchema: WireSchema | undefined;
  readonly wireSchemas: WireSchemas | undefined;
  readonly codecs: ReadonlyMap<string, MediaCodec<unknown>> | undefined;
  readonly maxFrameBytes: number;
  readonly signal: AbortSignal;
}

export interface InboundMultipartPartDecodeOptions {
  readonly schema: InboundSchema | undefined;
  readonly schemas: InboundSchemas;
  readonly itemEncoding: WireEncodingDefinition | undefined;
  readonly wireSchemas: WireSchemas | undefined;
  readonly codecs: ReadonlyMap<string, MediaCodec<unknown>> | undefined;
  readonly maxFrameBytes: number;
}

export interface InboundXMLNode {
  readonly name: string;
  readonly attributes: Readonly<Record<string, string>>;
  readonly children: InboundXMLNode[];
  text: string;
  hasText: boolean;
}

export type MutableInboundParameterValues = {
  -readonly [Key in keyof InboundParameterValues]: Mutable<InboundParameterValues[Key]>;
};

export type InboundSortValue = { field: string; direction: string };

export type InboundWireSchemaConjunction = Pick<Required<WireSchema>, "allOf">;

export type InboundProtocolItems = {
  readonly items: AsyncIterable<unknown>;
};

export type RequiredStreamSignal = {
  readonly [Key in "signal"]: NonNullable<StreamContext[Key]>;
};
export interface ServerCodecContext {
  readonly wire: WireCodec;
  readonly xml?: XMLCodec;
  readonly decodeLegacyXML?: typeof import("./runtime-legacy-xml.js").decodeLegacyXML;
  readonly decodeFormValue?: typeof import("./runtime-parameters.js").decodeInboundFormValue;
  readonly frames?: Readonly<
    Partial<Record<Exclude<StreamFraming, "custom">, InboundFrameDecoder>>
  >;
}
export type InboundFrameDecoder = (
  codecContext: ServerCodecContext,
  body: ReadableStream<Uint8Array>,
  options: InboundProtocolDecodeOptions,
) => AsyncIterable<unknown>;
export type BoundServerFunction<Arguments extends unknown[], Result> = (
  ...args: Arguments
) => Result;
export type ServerBound<Function> = Function extends (
  context: ServerCodecContext,
  ...args: infer Arguments
) => infer Result
  ? BoundServerFunction<Arguments, Result>
  : never;
