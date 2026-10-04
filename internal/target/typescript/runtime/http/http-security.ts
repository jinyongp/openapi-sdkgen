import type { ClientOptions, SecurityCredentialContext } from "./configuration.js";
import type { OperationDefinition } from "./operation.js";
import type {
  SecurityCredential,
  SecurityCredentials,
  SecurityRequirementDefinition,
  SecuritySchemeDefinition,
} from "../security/security.js";
import type { RequestOptions } from "./request.js";
import type { Transport } from "../shared/transport.js";
import type {
  EncodedRequest,
  OperationRequestOptions,
  SDKSecuritySource,
  OperationSecurity,
} from "./http-types.js";
import type {
  SecurityCredentialHandler,
  SecurityCredentialHeader,
} from "../security/security-handler-types.js";
import { TransportErrorCode, defineOwnDataProperty, isRecord } from "../shared/runtime-support.js";
import { operationDiagnosticName } from "./operation.js";
import type { TransportError } from "../shared/runtime-support.js";
import { isPromise } from "./http-execution-support.js";
import { transportError } from "../shared/runtime-support.js";
import { securityCredentialError, securityCollision } from "../security/security-diagnostics.js";
export function createOperationSecurity(
  handlers: Readonly<Record<string, SecurityCredentialHandler>>,
): OperationSecurity {
  /** Applies the selected OpenAPI security requirement without sharing credential state across requests. */
  function applyOperationSecurity(
    options: ClientOptions,
    operation: OperationDefinition,
    encoded: EncodedRequest,
    requestOptions: OperationRequestOptions,
    credentials: RequestCredentials | undefined,
  ): EncodedRequest | Promise<EncodedRequest> {
    const declared: readonly SecurityRequirementDefinition[] | undefined = operation.security;
    const requestedID: string | undefined = requestOptions.securityRequirement;
    if (declared === undefined || declared.length === 0) {
      if (requestedID !== undefined)
        throw securityRequirementInvalid(
          "The operation does not declare an OpenAPI security requirement",
        );
      return encoded;
    }
    const requirements: Record<string, SecurityRequirementDefinition> = Object.create(
      null,
    ) as Record<string, SecurityRequirementDefinition>;
    for (const requirement of declared)
      defineOwnDataProperty(requirements, requirement.id, requirement);
    let selected: SecurityRequirementDefinition;
    if (declared.length === 1) {
      if (requestedID !== undefined) {
        throw securityRequirementInvalid(
          `Operation ${operationDiagnosticName(operation)} has one SDK-selected security requirement and does not accept an explicit selection`,
        );
      }
      selected = declared[0]!;
    } else {
      if (requestedID === undefined) {
        throw transportError(
          TransportErrorCode.SECURITY_REQUIREMENT_REQUIRED,
          `Operation ${operationDiagnosticName(operation)} requires an explicit OpenAPI security requirement`,
          undefined,
        );
      }
      const requested: SecurityRequirementDefinition | undefined = requirements[requestedID];
      if (requested === undefined) {
        throw securityRequirementInvalid(
          `Operation ${operationDiagnosticName(operation)} does not declare security requirement ${requestedID}`,
        );
      }
      selected = requested;
    }
    if (
      securityRequirementIsSatisfied(options, requestOptions, encoded, credentials, selected, true)
    ) {
      return applySelectedSecurityRequirement(
        options,
        requestOptions,
        encoded,
        credentials,
        selected,
        {},
        false,
      );
    }
    if (typeof options.securityProvider !== "function") {
      return applySelectedSecurityRequirement(
        options,
        requestOptions,
        encoded,
        credentials,
        selected,
        {},
        false,
      );
    }
    const context: SecurityCredentialContext = {
      operation: {
        route: operation.route,
        ...(operation.operationID === undefined ? {} : { operationID: operation.operationID }),
        method: operation.method,
        path: operation.path,
      },
      requirement: selected,
      origin: new URL(encoded.url).origin,
      ...(requestOptions.signal === undefined ? {} : { signal: requestOptions.signal }),
    };
    const suppliedCredentials:
      | Readonly<Record<string, SecurityCredential>>
      | Promise<Readonly<Record<string, SecurityCredential>>> = options.securityProvider(context);
    const apply: (resolved: SecurityCredentials) => EncodedRequest = (
      resolved: SecurityCredentials,
    ): EncodedRequest =>
      applySelectedSecurityRequirement(
        options,
        requestOptions,
        encoded,
        credentials,
        selected,
        resolved,
        true,
      );
    return isPromise(suppliedCredentials)
      ? suppliedCredentials.then(apply)
      : apply(suppliedCredentials);
  }

  function securityRequirementIsSatisfied(
    options: ClientOptions,
    requestOptions: RequestOptions,
    encoded: EncodedRequest,
    credentials: RequestCredentials | undefined,
    requirement: SecurityRequirementDefinition,
    allowMutualTLS: boolean,
  ): boolean {
    return requirement.schemes.every(
      (scheme: SecuritySchemeDefinition): boolean =>
        securitySourceForScheme(
          options,
          requestOptions,
          encoded,
          credentials,
          scheme,
          allowMutualTLS,
        ).state === "satisfied",
    );
  }

  function securitySourceForScheme(
    options: ClientOptions,
    requestOptions: RequestOptions,
    encoded: EncodedRequest,
    credentials: RequestCredentials | undefined,
    scheme: SecuritySchemeDefinition,
    allowMutualTLS: boolean,
  ): SDKSecuritySource {
    if (usesAuthorizationHeader(scheme)) {
      if (requestOptions.authorization === undefined && options.authorization === undefined)
        return { state: "none" };
      const value: string = encoded.headers.get("Authorization") ?? "";
      return matchesAuthorizationScheme(scheme, value)
        ? { state: "satisfied", kind: "header", name: "Authorization", value }
        : { state: "conflict", location: "Authorization header" };
    }
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
    if (
      scheme.type === "mutualTLS" &&
      allowMutualTLS &&
      options.transport?.capabilities?.mutualTLS
    ) {
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
    const separator: number = value.indexOf(" ");
    if (separator <= 0 || value.slice(separator + 1).trim() === "") return false;
    const protocol: string = value.slice(0, separator).toLowerCase();
    if (scheme.type === "oauth2" || scheme.type === "openIdConnect") return protocol === "bearer";
    return protocol === scheme.scheme?.toLowerCase();
  }

  function applySelectedSecurityRequirement(
    options: ClientOptions,
    requestOptions: RequestOptions,
    encoded: EncodedRequest,
    credentialsMode: RequestCredentials | undefined,
    requirement: SecurityRequirementDefinition,
    suppliedCredentials: unknown,
    providerReturned: boolean,
  ): EncodedRequest {
    if (!isRecord(suppliedCredentials)) {
      throw transportError(
        TransportErrorCode.SECURITY_CREDENTIALS_INVALID,
        "Security credential provider returned an invalid credentials object",
        undefined,
      );
    }
    const declaredNames: Set<string> = new Set(
      requirement.schemes.map((scheme: SecuritySchemeDefinition): string => scheme.name),
    );
    if (
      Object.keys(suppliedCredentials).some((name: string): boolean => !declaredNames.has(name))
    ) {
      throw transportError(
        TransportErrorCode.SECURITY_CREDENTIALS_INVALID,
        "Security credential provider returned credentials outside the selected requirement",
        undefined,
      );
    }
    const url: URL = new URL(encoded.url);
    for (const scheme of requirement.schemes) {
      const source: SDKSecuritySource = securitySourceForScheme(
        options,
        requestOptions,
        encoded,
        credentialsMode,
        scheme,
        true,
      );
      const credential: SecurityCredential | undefined = suppliedCredentials[scheme.name] as
        | SecurityCredential
        | undefined;
      if (source.state === "conflict") throw securityCollision(scheme.name, source.location);
      if (source.state === "satisfied") {
        if (credential === undefined) continue;
        if (source.kind === "header") {
          const header: SecurityCredentialHeader | undefined = securityCredentialHeader(
            scheme,
            credential,
          );
          if (
            header !== undefined &&
            header.name.toLowerCase() === source.name.toLowerCase() &&
            normalizeHeaderValue(header.name, header.value) === source.value
          )
            continue;
          throw securityCollision(scheme.name, source.name);
        }
        if (source.kind === "mutualTLS") {
          assertSecurityCredentialShape(scheme, credential);
          continue;
        }
        assertSecurityCredentialShape(scheme, credential);
        throw securityCollision(scheme.name, "ambient Cookie credentials");
      }
      if (credential === undefined) {
        if (providerReturned) {
          throw transportError(
            TransportErrorCode.SECURITY_CREDENTIALS_INVALID,
            `Security credential provider omitted scheme ${scheme.name}`,
            undefined,
          );
        }
        throw transportError(
          TransportErrorCode.SECURITY_CREDENTIALS_REQUIRED,
          `Security requirement ${requirement.id} requires credentials for scheme ${scheme.name}`,
          undefined,
        );
      }
      applySecurityCredential(options.transport, scheme, credential, encoded.headers, url);
    }
    return {
      ...encoded,
      url: url.href,
      ...(requirement.schemes.length === 0 ? {} : { redirect: "error" as const }),
    };
  }

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
  function assertSecurityCredentialShape(
    scheme: SecuritySchemeDefinition,
    credential: SecurityCredential,
  ): void {
    credentialHandler(scheme).validate(scheme, credential);
  }
  function securityCredentialHeader(
    scheme: SecuritySchemeDefinition,
    credential: SecurityCredential,
  ): SecurityCredentialHeader | undefined {
    assertSecurityCredentialShape(scheme, credential);
    return credentialHandler(scheme).header?.(scheme, credential);
  }
  function applySecurityCredential(
    transport: Transport | undefined,
    scheme: SecuritySchemeDefinition,
    credential: SecurityCredential,
    headers: Headers,
    url: URL,
  ): void {
    const header: SecurityCredentialHeader | undefined = securityCredentialHeader(
      scheme,
      credential,
    );
    if (header !== undefined) {
      if (headers.has(header.name)) throw securityCollision(scheme.name, `header ${header.name}`);
      headers.set(header.name, header.value);
      return;
    }
    credentialHandler(scheme).apply?.(transport, scheme, credential, headers, url);
  }
  function normalizeHeaderValue(name: string, value: string): string {
    const headers: Headers = new Headers();
    headers.set(name, value);
    return headers.get(name)!;
  }

  function securityRequirementInvalid(message: string): TransportError {
    return transportError(TransportErrorCode.SECURITY_REQUIREMENT_INVALID, message, undefined);
  }

  return applyOperationSecurity;
}
