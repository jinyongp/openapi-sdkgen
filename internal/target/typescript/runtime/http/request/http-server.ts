import type { OperationDefinition, ServerDefinition, ServerSelection } from "../operation.js";
import type { ServerURLExpander } from "./http-request-parameter-types.js";
import { operationDiagnosticName } from "../operation.js";
import { normalizeBaseURL } from "../http-execution-support.js";
/** Applies deployment and origin rules using the prepared server expansion policy. */
export function resolveServerBaseURL(
  baseURL: string | undefined,
  origin: string | undefined,
  selection: ServerSelection | undefined,
  operation: OperationDefinition,
  expand: ServerURLExpander,
): string {
  if (baseURL !== undefined) return baseURL;
  const servers: readonly ServerDefinition[] = operation.servers ?? [{ id: "#", url: "/" }];
  const server: ServerDefinition | undefined =
    selection?.id === undefined
      ? servers[0]
      : servers.find((item: ServerDefinition): boolean => item.id === selection.id);
  if (server === undefined)
    throw new TypeError(
      `Unknown server ${selection?.id} for operation ${operationDiagnosticName(operation)}`,
    );
  const expanded: string = expand(server, selection);
  try {
    return normalizeBaseURL(new URL(expanded).href);
  } catch (cause: unknown) {
    try {
      new URL(expanded);
    } catch {
      if (origin === undefined)
        throw new TypeError(
          `Server ${server.id} is relative; pass ClientOptions.origin or baseURL`,
        );
      const absoluteOrigin: string = normalizeOrigin(origin);
      return normalizeBaseURL(new URL(expanded, absoluteOrigin).href);
    }
    throw cause;
  }
}

function normalizeOrigin(value: string): string {
  const url: URL = new URL(value);
  if (
    (url.protocol !== "http:" && url.protocol !== "https:") ||
    url.pathname !== "/" ||
    url.search ||
    url.hash
  ) {
    throw new TypeError(
      "origin must be an absolute http(s) origin without path, query, or fragment",
    );
  }
  return url.origin;
}
/** Prepared server URLs contain no template variables. */
export function resolveStaticOperationBaseURL(
  baseURL: string | undefined,
  origin: string | undefined,
  selection: ServerSelection | undefined,
  operation: OperationDefinition,
): string {
  return resolveServerBaseURL(
    baseURL,
    origin,
    selection,
    operation,
    (server: ServerDefinition): string => server.url,
  );
}
