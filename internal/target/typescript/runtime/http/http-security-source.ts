import type { ClientOptions } from "./configuration.js";
import type { RequestOptions } from "./request.js";
import type { EncodedRequest, SDKSecuritySource } from "./http-types.js";
import type { SecuritySchemeDefinition } from "../security/security.js";
import {
  authorizationProtocol,
  authorizationSecuritySource,
} from "./http-security-authorization.js";
/** Resolves configured credentials for the general set of security schemes. */
export function securitySourceForScheme(
  options: ClientOptions,
  requestOptions: RequestOptions,
  encoded: EncodedRequest,
  credentials: RequestCredentials | undefined,
  scheme: SecuritySchemeDefinition,
  allowMutualTLS: boolean,
): SDKSecuritySource {
  if (usesAuthorizationHeader(scheme))
    return authorizationSecuritySource(options, requestOptions, encoded, (value: string): boolean =>
      matchesAuthorizationScheme(scheme, value),
    );
  if (isCSRFHeaderScheme(scheme)) {
    if (requestOptions.csrfToken === undefined) return { state: "none" };
    const value: string = encoded.headers.get("X-CSRF-Token") ?? "";
    return value === ""
      ? { state: "conflict", location: "X-CSRF-Token header" }
      : { state: "satisfied", kind: "header", name: "X-CSRF-Token", value };
  }
  if (scheme.type === "apiKey" && scheme.location === "cookie" && credentials === "include") {
    return { state: "satisfied", kind: "cookie" };
  }
  if (scheme.type === "mutualTLS" && allowMutualTLS && options.transport?.capabilities?.mutualTLS) {
    return { state: "satisfied", kind: "mutualTLS" };
  }
  return { state: "none" };
}

function usesAuthorizationHeader(scheme: SecuritySchemeDefinition): boolean {
  return (
    scheme.type === "http" ||
    scheme.type === "oauth2" ||
    scheme.type === "openIdConnect" ||
    (scheme.type === "apiKey" &&
      scheme.location === "header" &&
      scheme.parameterName?.toLowerCase() === "authorization")
  );
}

function isCSRFHeaderScheme(scheme: SecuritySchemeDefinition): boolean {
  return (
    scheme.type === "apiKey" &&
    scheme.location === "header" &&
    scheme.parameterName?.toLowerCase() === "x-csrf-token"
  );
}

function matchesAuthorizationScheme(scheme: SecuritySchemeDefinition, value: string): boolean {
  if (value === "") return false;
  if (scheme.type === "apiKey") return true;
  const protocol: string | undefined = authorizationProtocol(value);
  if (protocol === undefined) return false;
  if (scheme.type === "oauth2" || scheme.type === "openIdConnect") return protocol === "bearer";
  return protocol === scheme.scheme?.toLowerCase();
}
