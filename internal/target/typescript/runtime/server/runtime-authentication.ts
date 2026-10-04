import { defineOwnDataProperty } from "../shared/runtime-support.js";
import type {
  InboundCookies,
  InboundSecurityCandidate,
  InboundSecuritySchemes,
} from "./runtime-types.js";
import { isRecord } from "../shared/runtime-support.js";
import { parseInboundCookies } from "./runtime-shared.js";
import { inboundCookieFirst } from "./runtime-shared.js";

/** Whether a non-empty effective OpenAPI Security Requirement Object applies. */
export function requiresInboundAuthentication(security: unknown): boolean {
  return (
    Array.isArray(security) &&
    security.length > 0 &&
    !security.some(
      (alternative: unknown): boolean =>
        isRecord(alternative) && Object.keys(alternative).length === 0,
    )
  );
}

/** Collects declared header/query/cookie credential candidates without authenticating them. */
export function collectInboundSecurityCandidates(
  request: Request,
  security: unknown,
  schemes: InboundSecuritySchemes,
  identities: Readonly<Record<string, string>> = {},
): Readonly<Record<string, InboundSecurityCandidate>> {
  const result: Record<string, InboundSecurityCandidate> = Object.create(null) as Record<
    string,
    InboundSecurityCandidate
  >;
  if (!Array.isArray(security)) return result;
  const url: URL = new URL(request.url);
  const cookies: InboundCookies = parseInboundCookies(request.headers.get("cookie"));
  for (const alternative of security) {
    if (!isRecord(alternative)) continue;
    for (const name of Object.keys(alternative)) {
      const identity: string = Object.hasOwn(identities, name) ? identities[name]! : name;
      if (Object.hasOwn(result, identity)) continue;
      const scheme: Readonly<Record<string, unknown>> | undefined = schemes[name];
      if (scheme === undefined || typeof scheme["type"] !== "string") continue;
      if (scheme["type"] === "apiKey") {
        const location: "query" | "header" | "cookie" | undefined =
          scheme["in"] === "header" || scheme["in"] === "query" || scheme["in"] === "cookie"
            ? scheme["in"]
            : undefined;
        const parameterName: string | undefined =
          typeof scheme["name"] === "string" ? scheme["name"] : undefined;
        if (location === undefined || parameterName === undefined) continue;
        const value: string | undefined =
          location === "header"
            ? (request.headers.get(parameterName) ?? undefined)
            : location === "query"
              ? (url.searchParams.get(parameterName) ?? undefined)
              : inboundCookieFirst(cookies.decoded[parameterName]);
        defineOwnDataProperty(result, identity, {
          scheme: identity,
          type: "apiKey",
          location,
          name: parameterName,
          ...(value === undefined ? {} : { value }),
        });
        continue;
      }
      const authorization: string | undefined = request.headers.get("authorization") ?? undefined;
      defineOwnDataProperty(result, identity, {
        scheme: identity,
        type: scheme["type"],
        ...(authorization === undefined ? {} : { value: authorization }),
      });
    }
  }
  return result;
}
