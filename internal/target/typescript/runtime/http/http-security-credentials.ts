import type { SecuritySchemeDefinition } from "../security/security.js";
import type { SecurityCredentialHandler } from "../security/security-handler-types.js";
import type { SecurityCredentialResolver } from "./http-security-types.js";
import { securityCredentialError } from "../security/security-diagnostics.js";
/** Binds the prepared authentication handlers by scheme kind. */
export function createSecurityCredentialResolver(
  handlers: Readonly<Record<string, SecurityCredentialHandler>>,
): SecurityCredentialResolver {
  function credentialHandler(scheme: SecuritySchemeDefinition): SecurityCredentialHandler {
    const kind: string =
      scheme.type === "apiKey"
        ? "apiKey." + scheme.location
        : scheme.type === "http"
          ? scheme.scheme === "basic" || scheme.scheme === "bearer"
            ? "http." + scheme.scheme
            : "http"
          : scheme.type;
    const handler: SecurityCredentialHandler | undefined = handlers[kind];
    if (handler === undefined) throw securityCredentialError(scheme.name, "supported credential");
    return handler;
  }
  return credentialHandler;
}
