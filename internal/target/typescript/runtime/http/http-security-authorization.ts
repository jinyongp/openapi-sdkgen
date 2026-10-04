import type { ClientOptions } from "./configuration.js";
import type { RequestOptions } from "./request.js";
import type { EncodedRequest, SDKSecuritySource } from "./http-types.js";
/** Reads the protocol from a nonempty Authorization header credential. */
export function authorizationProtocol(value: string): string | undefined {
  const separator: number = value.indexOf(" ");
  if (separator <= 0 || value.slice(separator + 1).trim() === "") return undefined;
  return value.slice(0, separator).toLowerCase();
}
/** Matches configured Authorization credentials without accepting raw reserved headers. */
export function authorizationSecuritySource(
  options: ClientOptions,
  requestOptions: RequestOptions,
  encoded: EncodedRequest,
  matches: (value: string) => boolean,
): SDKSecuritySource {
  if (requestOptions.authorization === undefined && options.authorization === undefined)
    return { state: "none" };
  const value: string = encoded.headers.get("Authorization") ?? "";
  return matches(value)
    ? { state: "satisfied", kind: "header", name: "Authorization", value }
    : { state: "conflict", location: "Authorization header" };
}
