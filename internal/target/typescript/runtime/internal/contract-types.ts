import type { BinaryBody } from "./request.js";
import type { RouteTypeIdentity } from "./identity.js";

/** Preserves the public value projection without importing a document-wide route map. */
export type OperationPublicType<Value> = Value extends BinaryBody
  ? Value
  : Value extends (...args: any[]) => any
    ? Value
    : Value extends readonly unknown[]
      ? { [Key in keyof Value]: OperationPublicType<Value[Key]> }
      : Value extends object
        ? { [Key in keyof Value]: OperationPublicType<Value[Key]> }
        : Value;

/** Raw resource capability with the same exact route brand as the public route helpers. */
export interface OperationResourceRawCapability<Call, Route> {
  readonly raw: Call & RouteTypeIdentity<Route>;
}
