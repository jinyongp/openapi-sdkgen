import type {
  OperationDefinition,
  ParameterDefinition,
  ServerDefinition,
  ServerSelection,
} from "../operation.js";
/** Validates and transforms a parameter before HTTP serialization. */
export type ParameterWireEncoder = (
  operation: OperationDefinition,
  parameter: ParameterDefinition | undefined,
  value: unknown,
) => unknown;
/** Serializes an already transformed path parameter. */
export type PathParameterSerializer = (
  parameter: ParameterDefinition | undefined,
  name: string,
  value: unknown,
) => string;
/** Resolves deployment overrides and selected operation servers. */
export type OperationBaseURLResolver = (
  baseURL: string | undefined,
  origin: string | undefined,
  selection: ServerSelection | undefined,
  operation: OperationDefinition,
) => string;
/** Expands one selected normalized server URL. */
export type ServerURLExpander = (
  server: ServerDefinition,
  selection: ServerSelection | undefined,
) => string;
/** Canonical request policies for buffered JSON contracts. */
export interface BasicRequestParameterServices {
  readonly encodeParameter: ParameterWireEncoder;
  readonly serializePathParameter: PathParameterSerializer;
  readonly resolveBaseURL: OperationBaseURLResolver;
}
