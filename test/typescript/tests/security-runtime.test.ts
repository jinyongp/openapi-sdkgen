import { describe, expect, it, vi } from "vitest";
import { createRequest } from "../../../internal/target/typescript/runtime/compatibility/http.js";
import { TransportErrorCode } from "../../../internal/target/typescript/runtime/shared/runtime-support.js";
import type { ClientOptions } from "../../../internal/target/typescript/runtime/http/configuration.js";
import type { OperationDefinition } from "../../../internal/target/typescript/runtime/http/operation.js";
import type { RequestOptions } from "../../../internal/target/typescript/runtime/http/request.js";
import type {
  SecurityCredentials,
  SecuritySchemeDefinition,
} from "../../../internal/target/typescript/runtime/security/security.js";

const key: SecuritySchemeDefinition = {
  name: "Key",
  type: "apiKey",
  location: "header",
  parameterName: "X-API-Key",
};
const bearer: SecuritySchemeDefinition = { name: "Bearer", type: "http", scheme: "bearer" };
const operation = (
  security: NonNullable<OperationDefinition["security"]>,
): OperationDefinition => ({
  route: "GET /secure",
  method: "GET",
  path: "/secure",
  envelope: "",
  security,
});
const secured = operation([{ id: "key", schemes: [key] }]);
const response = async () => new Response(null, { status: 204 });

describe("security requirement and credential boundaries", () => {
  it.each([
    {
      name: "absent requirement",
      target: operation([]),
      selection: "unknown",
      code: TransportErrorCode.SECURITY_REQUIREMENT_INVALID,
    },
    {
      name: "explicit selection for a sole requirement",
      target: secured,
      selection: "key",
      code: TransportErrorCode.SECURITY_REQUIREMENT_INVALID,
    },
    {
      name: "missing alternative selection",
      target: operation([
        { id: "key", schemes: [key] },
        { id: "anonymous", schemes: [] },
      ]),
      selection: undefined,
      code: TransportErrorCode.SECURITY_REQUIREMENT_REQUIRED,
    },
    {
      name: "unknown alternative selection",
      target: operation([
        { id: "key", schemes: [key] },
        { id: "anonymous", schemes: [] },
      ]),
      selection: "missing",
      code: TransportErrorCode.SECURITY_REQUIREMENT_INVALID,
    },
  ])(
    "rejects $name before acquiring credentials or fetching",
    async ({ target, selection, code }) => {
      const fetch = vi.fn(response);
      const securityProvider = vi.fn(() => ({
        Key: { kind: "api-key" as const, value: "secret" },
      }));
      const request = createRequest({
        baseURL: "https://api.example.test",
        fetch,
        securityProvider,
      });
      const options: RequestOptions & { securityRequirement?: string } =
        selection === undefined ? {} : { securityRequirement: selection };
      await expect(request(target, undefined, options)).rejects.toMatchObject({ code });
      expect(securityProvider).not.toHaveBeenCalled();
      expect(fetch).not.toHaveBeenCalled();
    },
  );

  it("uses a selected anonymous alternative without credentials or redirect restriction", async () => {
    const fetch = vi.fn<typeof globalThis.fetch>(response);
    const securityProvider = vi.fn(() => {
      throw new Error("anonymous alternative must not request credentials");
    });
    const request = createRequest({ baseURL: "https://api.example.test", fetch, securityProvider });
    const options = { securityRequirement: "anonymous" };
    await expect(
      request(
        operation([
          { id: "key", schemes: [key] },
          { id: "anonymous", schemes: [] },
        ]),
        undefined,
        options as RequestOptions,
      ),
    ).resolves.toBeUndefined();
    expect(securityProvider).not.toHaveBeenCalled();
    expect(fetch).toHaveBeenCalledOnce();
    expect(fetch.mock.calls[0]?.[1]?.redirect).not.toBe("error");
  });

  it.each<{ name: string; credentials: unknown }>([
    { name: "null object", credentials: null },
    { name: "array", credentials: [] },
    { name: "omitted scheme", credentials: {} },
    {
      name: "undeclared scheme",
      credentials: {
        Key: { kind: "api-key", value: "secret" },
        Other: { kind: "api-key", value: "other" },
      },
    },
    {
      name: "wrong credential kind",
      credentials: { Key: { kind: "http-bearer", token: "secret" } },
    },
    { name: "empty API key", credentials: { Key: { kind: "api-key", value: "" } } },
  ])("rejects provider result with $name before fetch", async ({ credentials }) => {
    const fetch = vi.fn(response);
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch,
      securityProvider: async () => credentials as SecurityCredentials,
    });
    await expect(request(secured)).rejects.toMatchObject({
      code: TransportErrorCode.SECURITY_CREDENTIALS_INVALID,
    });
    expect(fetch).not.toHaveBeenCalled();
  });

  it.each<{ scheme: SecuritySchemeDefinition; credential: unknown }>([
    {
      scheme: { name: "Auth", type: "http", scheme: "basic" },
      credential: { kind: "http-basic", username: "user" },
    },
    {
      scheme: { name: "Auth", type: "http", scheme: "bearer" },
      credential: { kind: "http-bearer", token: "" },
    },
    {
      scheme: { name: "Auth", type: "http", scheme: "Digest" },
      credential: { kind: "http", value: "" },
    },
    { scheme: { name: "Auth", type: "oauth2" }, credential: { kind: "oauth2", token: 1 } },
    {
      scheme: { name: "Auth", type: "openIdConnect" },
      credential: { kind: "oauth2", token: "wrong-kind" },
    },
    {
      scheme: { name: "Auth", type: "mutualTLS" },
      credential: { kind: "api-key", value: "wrong-kind" },
    },
  ])("rejects malformed $scheme.type credential", async ({ scheme, credential }) => {
    const fetch = vi.fn(response);
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch,
      securityProvider: () => ({ Auth: credential }) as SecurityCredentials,
    });
    await expect(request(operation([{ id: "auth", schemes: [scheme] }]))).rejects.toMatchObject({
      code: TransportErrorCode.SECURITY_CREDENTIALS_INVALID,
    });
    expect(fetch).not.toHaveBeenCalled();
  });

  it("reports missing credentials distinctly from an invalid provider result", async () => {
    const fetch = vi.fn(response);
    await expect(
      createRequest({ baseURL: "https://api.example.test", fetch })(secured),
    ).rejects.toMatchObject({ code: TransportErrorCode.SECURITY_CREDENTIALS_REQUIRED });
    expect(fetch).not.toHaveBeenCalled();
  });

  it.each<{
    name: string;
    scheme: SecuritySchemeDefinition;
    options: ClientOptions;
    requestOptions?: RequestOptions;
  }>([
    { name: "Bearer header", scheme: bearer, options: { authorization: "Bearer caller" } },
    {
      name: "OAuth header",
      scheme: { name: "OAuth", type: "oauth2" },
      options: { authorization: "Bearer caller" },
    },
    {
      name: "API-key authorization",
      scheme: { ...key, parameterName: "Authorization" },
      options: { authorization: "caller-key" },
    },
    {
      name: "CSRF token",
      scheme: { ...key, parameterName: "X-CSRF-Token" },
      options: {},
      requestOptions: { csrfToken: "caller-csrf" },
    },
    {
      name: "ambient cookies",
      scheme: { ...key, location: "cookie", parameterName: "session" },
      options: { credentials: "include" },
    },
    {
      name: "mutual TLS transport",
      scheme: { name: "Certificate", type: "mutualTLS" },
      options: { transport: { fetch: response, capabilities: { mutualTLS: true } } },
    },
  ])(
    "does not acquire credentials again when $name satisfies the requirement",
    async ({ scheme, options, requestOptions }) => {
      const fetch = vi.fn<typeof globalThis.fetch>(response);
      const securityProvider = vi.fn(() => {
        throw new Error("already satisfied");
      });
      const request = createRequest({
        ...options,
        baseURL: "https://api.example.test",
        fetch,
        ...(options.transport ? { transport: { ...options.transport, fetch } } : {}),
        securityProvider,
      });
      await expect(
        request(operation([{ id: "provided", schemes: [scheme] }]), undefined, requestOptions),
      ).resolves.toBeUndefined();
      expect(securityProvider).not.toHaveBeenCalled();
      expect(fetch).toHaveBeenCalledOnce();
      expect(fetch.mock.calls[0]?.[1]?.redirect).toBe("error");
    },
  );

  it("allows identical shared credentials but rejects attempts to overwrite caller authorization", async () => {
    for (const [token, succeeds] of [
      ["caller", true],
      ["different", false],
    ] as const) {
      const fetch = vi.fn(response);
      const request = createRequest({
        baseURL: "https://api.example.test",
        authorization: "Bearer caller",
        fetch,
        securityProvider: async () => ({
          Bearer: { kind: "http-bearer", token },
          Key: { kind: "api-key", value: "secret" },
        }),
      });
      const pending = request(operation([{ id: "combined", schemes: [bearer, key] }]));
      if (succeeds) {
        await expect(pending).resolves.toBeUndefined();
        expect(fetch).toHaveBeenCalledOnce();
      } else {
        await expect(pending).rejects.toMatchObject({
          code: TransportErrorCode.SECURITY_CREDENTIALS_INVALID,
        });
        expect(fetch).not.toHaveBeenCalled();
      }
    }
  });

  it("rejects caller and provider collisions in API-key headers and query parameters", async () => {
    for (const location of ["header", "query"] as const) {
      const fetch = vi.fn(response);
      const request = createRequest({
        baseURL: "https://api.example.test",
        fetch,
        securityProvider: () => ({ Key: { kind: "api-key", value: "provider" } }),
      });
      const target = operation([
        { id: "key", schemes: [{ ...key, location, parameterName: "key" }] },
      ]);
      await expect(
        request(
          target,
          location === "query" ? { query: { key: "caller" } } : undefined,
          location === "header" ? { headers: { key: "caller" } } : undefined,
        ),
      ).rejects.toMatchObject({ code: TransportErrorCode.SECURITY_CREDENTIALS_INVALID });
      expect(fetch).not.toHaveBeenCalled();
    }
  });
});
