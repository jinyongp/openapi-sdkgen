import { collectSelectionReferences } from "./selection.js";
import type { OperationReference, SelectionInput } from "./selection-types.js";
import type { ClientOptions } from "./configuration.js";
import type { RequestContext } from "./http-types.js";

/** Version of the generated lookup/provider protocol, not an authentication token. */
export const operationLoaderABI = 1;
const referenceKey = Symbol.for("openapi-sdkgen.operation-reference.v1");
declare const preparedIdentity: unique symbol;

/** Code-only preparation reusable across independently configured clients. */
export interface PreparedOperations<Selection = unknown> {
  readonly [preparedIdentity]: Selection;
}

/** Exact operation-ID or method/path lookup domain. */
export type OperationLookupKind = "operation" | "route";

/** One placement in the full document's collision-resolved resource tree. */
export interface OperationResourcePlacement {
  readonly path: readonly (string | null)[];
  readonly member: string;
  readonly pathParameters?: readonly string[];
  readonly hasInput?: boolean;
  readonly inputOptional?: boolean;
  readonly pagination?: boolean;
}

/** A compiler-owned execution module. Client state is supplied only when binding. */
export interface OperationExecutionProvider {
  readonly abi: number;
  readonly generation: string;
  readonly route: string;
  readonly operationID?: string;
  readonly resources?: readonly OperationResourcePlacement[];
  /** Literal lazy imports emitted only for this operation's direct Link targets. */
  readonly linkTargets?: Readonly<Record<string, () => Promise<OperationExecutionProvider>>>;
  bind(context: RequestContext): object;
  /** Connects response helpers without binding or loading their target operations. */
  bindLinks?(invoke: (route: string, args: readonly unknown[]) => Promise<unknown>): object;
}

/** Shares validated code while keeping operation callables inside each client. */
export type OperationProviderResolver = (
  route: string,
  loadProvider?: () => Promise<OperationExecutionProvider>,
) => Promise<OperationExecutionProvider>;

/** Fixed export shape of each generated lookup module. */
export interface OperationLookupEntry {
  readonly abi: number;
  readonly generation: string;
  readonly kind: OperationLookupKind;
  readonly key: string;
  readonly provider: OperationExecutionProvider;
}

/** The client implementation is loaded during preparation, not reference lookup. */
export interface PreparedClientModule {
  createSelectedClient(
    options: ClientOptions,
    providers: readonly OperationExecutionProvider[],
    resolve: OperationProviderResolver,
  ): object;
}

/** Compiler-supplied code identity and module boundaries. */
export interface OperationLoaderConfiguration {
  readonly generation: string;
  readonly baseURL: URL;
  readonly loadClient: () => Promise<PreparedClientModule>;
  /** Public roots of a generation selection; Link dependencies resolve privately. */
  readonly publicRoutes?: readonly string[];
  /** Internal seam for deterministic module-loading tests; never a consumer option. */
  readonly importModule?: (url: string) => Promise<unknown>;
}

type ReferenceData = {
  readonly abi: number;
  readonly generation: string;
  readonly kind: OperationLookupKind;
  readonly key: string;
  readonly provider?: OperationExecutionProvider;
};

type PreparedData = {
  readonly providers: readonly OperationExecutionProvider[];
  readonly client: PreparedClientModule;
};

/** A failure at the SDK code-loading boundary; the platform's original error is retained. */
export class OperationPreparationError extends Error {
  readonly name = "OperationPreparationError";
  constructor(
    readonly stage: "INPUT" | "MODULE_LOAD" | "IDENTITY" | "BINDING",
    message: string,
    options?: ErrorOptions,
  ) {
    super(message, options);
  }
}

function assertLookupKey(key: string): void {
  // UTF-8 replaces isolated surrogates. Reject them before hashing exact identities.
  for (let index = 0; index < key.length; index++) {
    const code = key.charCodeAt(index);
    if (code >= 0xd800 && code <= 0xdbff) {
      const next = key.charCodeAt(++index);
      if (!(next >= 0xdc00 && next <= 0xdfff)) {
        throw new OperationPreparationError(
          "INPUT",
          "Operation key contains an isolated surrogate",
        );
      }
    } else if (code >= 0xdc00 && code <= 0xdfff) {
      throw new OperationPreparationError("INPUT", "Operation key contains an isolated surrogate");
    }
  }
}

/** Shared exact UTF-8 framing with the Go emitter. No path comes from unchecked input. */
export async function operationLookupFilename(
  kind: OperationLookupKind,
  key: string,
): Promise<string> {
  assertLookupKey(key);
  const bytes = new TextEncoder().encode(`${kind}\0${key}`);
  const digest = await globalThis.crypto.subtle.digest("SHA-256", bytes);
  const encoded = Array.from(new Uint8Array(digest), (byte) =>
    byte.toString(16).padStart(2, "0"),
  ).join("");
  return `${kind === "route" ? "r" : "o"}-${encoded.slice(0, 16)}/${encoded.slice(16)}.js`;
}

function referenceValue<Route extends string>(data: ReferenceData): OperationReference<Route> {
  const reference = Object.create(null) as object;
  Object.defineProperty(reference, referenceKey, { value: Object.freeze(data) });
  return Object.freeze(reference) as OperationReference<Route>;
}

/** Used only by generated static operation modules; selection still occurs once. */
export function staticOperationReference<Route extends string>(
  provider: OperationExecutionProvider & { readonly route: Route },
): OperationReference<Route> {
  return referenceValue({
    abi: operationLoaderABI,
    generation: provider.generation,
    kind: "route",
    key: provider.route,
    provider,
  });
}

/** Creates one generated document's small reference facade and code preparation boundary. */
export function createOperationLoader(configuration: OperationLoaderConfiguration) {
  const generation = configuration.generation;
  const publicRoutes =
    configuration.publicRoutes === undefined ? undefined : new Set(configuration.publicRoutes);
  const baseURL = new URL("./lookup/", configuration.baseURL);
  const importModule =
    configuration.importModule ?? ((url: string) => import(/* @vite-ignore */ url));
  const references = new Map<string, OperationReference>();
  const prepared = new WeakMap<object, PreparedData>();
  const loads = new Map<string, Promise<OperationExecutionProvider>>();
  const canonical = new Map<string, OperationExecutionProvider>();
  let clientModule: Promise<PreparedClientModule> | undefined;

  function validateProvider(value: unknown): OperationExecutionProvider {
    if (value === null || typeof value !== "object") {
      throw new OperationPreparationError(
        "IDENTITY",
        "Lookup did not return an execution provider",
      );
    }
    const provider = value as Partial<OperationExecutionProvider>;
    if (
      provider.abi !== operationLoaderABI ||
      provider.generation !== generation ||
      typeof provider.route !== "string" ||
      typeof provider.bind !== "function" ||
      (provider.operationID !== undefined && typeof provider.operationID !== "string")
    ) {
      throw new OperationPreparationError(
        "IDENTITY",
        "Execution provider identity or ABI mismatch",
      );
    }
    return provider as OperationExecutionProvider;
  }

  function canonicalize(provider: OperationExecutionProvider): OperationExecutionProvider {
    const previous = canonical.get(provider.route);
    if (previous !== undefined) {
      if (previous.operationID !== provider.operationID) {
        throw new OperationPreparationError(
          "IDENTITY",
          "Conflicting operation aliases for one route",
        );
      }
      return previous;
    }
    canonical.set(provider.route, provider);
    return provider;
  }

  function reference(kind: OperationLookupKind, key: string): OperationReference {
    assertLookupKey(key);
    const identity = `${kind}\0${key}`;
    let value = references.get(identity);
    if (value === undefined) {
      value = referenceValue({ abi: operationLoaderABI, generation, kind, key });
      references.set(identity, value);
    }
    return value;
  }

  function namespace(kind: OperationLookupKind): Readonly<Record<string, OperationReference>> {
    const unsupported = () => {
      throw new TypeError(
        "Reference namespaces have no runtime name list; import selective/all.js to enumerate names",
      );
    };
    const readonly = () => {
      throw new TypeError("Operation reference namespaces are readonly");
    };
    return new Proxy(Object.create(null) as Record<string, OperationReference>, {
      get(_target, key) {
        if (key === Symbol.toStringTag) return "OperationReferences";
        return typeof key === "string" ? reference(kind, key) : undefined;
      },
      ownKeys: unsupported,
      has: unsupported,
      getOwnPropertyDescriptor: unsupported,
      set: readonly,
      defineProperty: readonly,
      deleteProperty: readonly,
      setPrototypeOf: readonly,
      preventExtensions: readonly,
    });
  }

  function readReference(value: object): ReferenceData | undefined {
    const data: unknown = Reflect.get(value, referenceKey);
    if (data === undefined) return undefined;
    if (data === null || typeof data !== "object") {
      throw new OperationPreparationError("INPUT", "Invalid operation reference");
    }
    const candidate = data as Partial<ReferenceData>;
    if (
      candidate.abi !== operationLoaderABI ||
      candidate.generation !== generation ||
      (candidate.kind !== "operation" && candidate.kind !== "route") ||
      typeof candidate.key !== "string"
    ) {
      throw new OperationPreparationError(
        "IDENTITY",
        "Operation reference belongs to another generation or ABI",
      );
    }
    assertLookupKey(candidate.key);
    return candidate as ReferenceData;
  }

  function load(
    data: ReferenceData,
    loadProvider?: () => Promise<OperationExecutionProvider>,
  ): Promise<OperationExecutionProvider> {
    if (data.provider !== undefined) {
      const provider = validateProvider(data.provider);
      if (data.kind !== "route" || provider.route !== data.key) {
        throw new OperationPreparationError("IDENTITY", "Static reference route mismatch");
      }
      return Promise.resolve(canonicalize(provider));
    }
    if (data.kind === "route") {
      const existing = canonical.get(data.key);
      if (existing !== undefined) return Promise.resolve(existing);
    }
    const identity = `${data.kind}\0${data.key}`;
    const existing = loads.get(identity);
    if (existing !== undefined) return existing;
    const pending = (async () => {
      if (loadProvider !== undefined) {
        let loaded: unknown;
        try {
          loaded = await loadProvider();
        } catch (cause) {
          throw new OperationPreparationError(
            "MODULE_LOAD",
            "Could not load the linked operation module",
            { cause },
          );
        }
        const provider = validateProvider(loaded);
        if (data.kind !== "route" || provider.route !== data.key) {
          throw new OperationPreparationError("IDENTITY", "Link selected a different operation");
        }
        return canonicalize(provider);
      }
      const filename = await operationLookupFilename(data.kind, data.key);
      let module: unknown;
      try {
        module = await importModule(new URL(filename, baseURL).href);
      } catch (cause) {
        throw new OperationPreparationError(
          "MODULE_LOAD",
          "Could not load the selected operation module",
          { cause },
        );
      }
      const entry = (module as { readonly entry?: Partial<OperationLookupEntry> } | null)?.entry;
      if (
        entry?.abi !== operationLoaderABI ||
        entry.generation !== generation ||
        entry.kind !== data.kind ||
        entry.key !== data.key
      ) {
        throw new OperationPreparationError(
          "IDENTITY",
          "Operation lookup identity or ABI mismatch",
        );
      }
      const provider = validateProvider(entry.provider);
      if ((data.kind === "route" ? provider.route : provider.operationID) !== data.key) {
        throw new OperationPreparationError("IDENTITY", "Lookup selected a different operation");
      }
      return canonicalize(provider);
    })();
    loads.set(identity, pending);
    void pending.catch(() => {
      if (loads.get(identity) === pending) loads.delete(identity);
    });
    return pending;
  }

  const resolve: OperationProviderResolver = (route, loadProvider) =>
    load({ abi: operationLoaderABI, generation, kind: "route", key: route }, loadProvider);

  async function loadOperations<const Selection>(
    selection: Selection & SelectionInput<Selection>,
  ): Promise<PreparedOperations<Selection>> {
    // No SDK module request starts until the entire selection has been collected.
    // Application getter exceptions intentionally retain their original identity.
    const selected = collectSelectionReferences(selection, readReference);
    for (const data of selected) {
      if (data.kind === "route" && publicRoutes !== undefined && !publicRoutes.has(data.key)) {
        throw new OperationPreparationError(
          "INPUT",
          "Operation is outside the generated public selection",
        );
      }
    }
    const providers = await Promise.all(selected.map((data) => load(data)));
    if (
      publicRoutes !== undefined &&
      providers.some((provider) => !publicRoutes.has(provider.route))
    ) {
      throw new OperationPreparationError(
        "INPUT",
        "Operation is outside the generated public selection",
      );
    }
    if (clientModule === undefined) {
      const pending = Promise.resolve().then(() => configuration.loadClient());
      clientModule = pending;
      void pending.catch(() => {
        if (clientModule === pending) clientModule = undefined;
      });
    }
    let client: PreparedClientModule;
    try {
      client = await clientModule;
    } catch (cause) {
      throw new OperationPreparationError(
        "MODULE_LOAD",
        "Could not load the client implementation",
        { cause },
      );
    }
    if (typeof client.createSelectedClient !== "function") {
      throw new OperationPreparationError("IDENTITY", "Client implementation ABI mismatch");
    }
    const handle = Object.freeze(Object.create(null)) as PreparedOperations<Selection>;
    prepared.set(handle, { providers: Object.freeze([...new Set(providers)]), client });
    return handle;
  }

  function createClient<Selection>(
    options: ClientOptions & { readonly operations: PreparedOperations<Selection> },
  ): object {
    const data = prepared.get(options.operations);
    if (data === undefined)
      throw new OperationPreparationError(
        "INPUT",
        "Prepare operations with this generated SDK before creating its client",
      );
    return data.client.createSelectedClient(options, data.providers, resolve);
  }

  return {
    operations: namespace("operation"),
    routes: namespace("route"),
    loadOperations,
    createClient,
  };
}
