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
import type { SecurityCredentialHeader } from "../security/security-handler-types.js";
import { TransportErrorCode, isRecord } from "../shared/runtime-support.js";
import type {
  OperationSecurityPolicy,
  SecuritySourceResolver,
  SecurityCredentialResolver,
} from "./http-security-types.js";
import { securityRequirementInvalid } from "./http-security-errors.js";
import { isPromise } from "./http-execution-support.js";
import { transportError } from "../shared/runtime-support.js";
import { securityCollision } from "../security/security-diagnostics.js";
/** Composes provider resolution, credential validation and collision checks with prepared policies. */
export function composeOperationSecurity(policy: OperationSecurityPolicy): OperationSecurity {
  const securitySourceForScheme: SecuritySourceResolver = policy.source;
  const credentialHandler: SecurityCredentialResolver = policy.credential;
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
    const selected: SecurityRequirementDefinition = policy.select(operation, declared, requestedID);
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

  return applyOperationSecurity;
}
