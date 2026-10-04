import type { WireSchema } from "../schema/wire-types.js";
import type { StreamFraming } from "../stream/stream-protocol-types.js";

/** Request or response body representation understood by the runtime. */
export interface WireBodyDefinition {
  /** Exact media type, excluding parameters such as charset. */
  readonly contentType: string;
  /** Wire transformation schema for this representation. */
  readonly schema: WireSchema;
  /** Whether the Media Type Object explicitly declares a complete-content schema. */
  readonly schemaDeclared?: true;
  /** Compiler-selected raw binary representation, independent of MIME heuristics. */
  readonly binary?: true;
  /** Compiler-selected sequential framing; omitted for ordinary media. */
  readonly streamFraming?: StreamFraming;
  /** OpenAPI 3.2 schema for one streamed response item. */
  readonly itemSchema?: WireSchema;
  /** Per-property Encoding Object declarations for form request bodies. */
  readonly encoding?: readonly WireEncodingDefinition[];
  /** Positional Encoding Objects for the first parts of a multipart body. */
  readonly prefixEncoding?: readonly WireEncodingDefinition[];
  /** Positional Encoding Object applied to the remaining multipart parts. */
  readonly itemEncoding?: WireEncodingDefinition;
}

/** One OpenAPI Encoding Object declaration for a form request-body property. */
export interface WireEncodingDefinition {
  /** Form property name. Omitted for positional multipart encodings. */
  readonly name?: string;
  readonly contentType?: string;
  readonly style?: string;
  readonly explode?: boolean;
  readonly allowReserved?: boolean;
  readonly headers?: readonly WireMultipartHeaderDefinition[];
  /** Nested Encoding Objects for an embedded form or multipart representation. */
  readonly encoding?: readonly WireEncodingDefinition[];
  /** Nested positional Encoding Objects for an embedded multipart representation. */
  readonly prefixEncoding?: readonly WireEncodingDefinition[];
  /** Nested streaming positional Encoding Object for an embedded multipart representation. */
  readonly itemEncoding?: WireEncodingDefinition;
}

/** A Header Object attached to one multipart part by an Encoding Object. */
export interface WireMultipartHeaderDefinition {
  readonly name: string;
  readonly required?: boolean;
  readonly style?: string;
  readonly explode?: boolean;
  readonly contentType?: string;
  readonly schema: WireSchema;
}

/** Generated response-header decoding metadata. */
export interface WireHeaderDefinition {
  readonly name: string;
  readonly property: string;
  readonly required?: boolean;
  /** Header serialization style. OpenAPI defaults this to `simple`. */
  readonly style?: string;
  /** Whether an object uses `name=value` entries instead of alternating tokens. */
  readonly explode?: boolean;
  /** The sole Header Object content media type, when content is used instead of schema. */
  readonly contentType?: string;
  readonly schema: WireSchema;
}
