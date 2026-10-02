import type { RouteTypeIdentity } from "./identity.js";
import type { BinaryBody } from "./request.js";

/** Preserves the public value projection without importing a document-wide route map. */
export type OperationPublicType<Value> = PublicValue<Value, never>;

// Reuse an already visited recursive projection while expanding ordinary
// object aliases for public editor help. The public generic keeps one argument.
type PublicValue<Value, Seen> = Value extends Seen
  ? Value
  : Value extends BinaryBody
    ? Value
    : Value extends (...args: any[]) => any
      ? Value
      : Value extends readonly unknown[]
        ? { [Key in keyof Value]: PublicValue<Value[Key], Seen | Value> }
        : Value extends object
          ? { [Key in keyof Value]: PublicValue<Value[Key], Seen | Value> }
          : Value;

/** Raw resource capability with the same exact route brand as the public route helpers. */
export interface OperationResourceRawCapability<Call, Route> {
  readonly raw: Call & RouteTypeIdentity<Route>;
}
