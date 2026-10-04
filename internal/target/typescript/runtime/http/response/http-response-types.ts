import type { WireBodyDefinition } from "../../media/media-contract-types.js";
import type { WireHeaderDefinition } from "../../media/media-contract-types.js";
export type { WireHeaderDefinition } from "../../media/media-contract-types.js";

/** Successful response representation understood by the runtime. */
export interface WireResponseDefinition extends WireBodyDefinition {
  /** Exact status code, `default`, or wildcard status such as `2XX`. */
  readonly status: string;
  readonly headers?: readonly WireHeaderDefinition[];
}
