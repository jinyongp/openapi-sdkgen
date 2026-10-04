import { collectSelectionReferences } from "./selection.js";
import type { OperationReference, SelectionInput } from "./selection-types.js";
import type { ClientOptions } from "../http/configuration.js";
import type { RequestContext } from "../http/http-types.js";
import type { ResourcePathBinder } from "./resource-binding-support.js";

/** Version of the generated lookup/provider protocol, not an authentication token. */
export const operationLoaderABI: 1 = 1;
const referenceKey: unique symbol = Symbol.for("openapi-sdkgen.operation-reference.v1");
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
  /** Exact binding emitted with this provider's generation identity. */
  readonly bindResource?: ResourcePathBinder;
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
interface OperationLoaderOptions {
  readonly generation: string;
  readonly loadClient: () => Promise<PreparedClientModule>;
  /** Public roots of a generation selection; Link dependencies resolve privately. */
  readonly publicRoutes?: readonly string[];
  /** Non-public routes in the generated execution union, including named-only APIs. */
  readonly privateRoutes?: readonly string[];
}

/** A generated relative importer or an absolute lookup base supplied by internal callers. */
export type OperationLoaderConfiguration =
  | AbsoluteOperationLoaderConfiguration
  | RelativeOperationLoaderConfiguration;

type OperationModuleImporter = (url: string) => Promise<unknown>;

interface AbsoluteOperationLoaderConfiguration extends OperationLoaderOptions {
  readonly baseURL: URL;
  readonly importModule?: OperationModuleImporter;
}

interface RelativeOperationLoaderConfiguration extends OperationLoaderOptions {
  readonly baseURL?: undefined;
  readonly importModule: OperationModuleImporter;
}

interface OperationLookupModule {
  readonly entry?: Partial<OperationLookupEntry>;
}

interface RoutedExecutionProvider<Route extends string> extends OperationExecutionProvider {
  readonly route: Route;
}

/** Client settings combined with code prepared by this loader. */
export interface SelectedClientOptions<Selection> extends ClientOptions {
  readonly operations: PreparedOperations<Selection>;
}

/** Reference lookup and preparation boundary shared by generated entry points. */
export interface OperationLoader {
  readonly operations: Readonly<Record<string, OperationReference>>;
  readonly routes: Readonly<Record<string, OperationReference>>;
  loadOperations<const Selection>(
    selection: Selection & SelectionInput<Selection>,
  ): Promise<PreparedOperations<Selection>>;
  createClient<Selection>(options: SelectedClientOptions<Selection>): object;
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
  override readonly name: "OperationPreparationError" = "OperationPreparationError";
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
  for (let index: number = 0; index < key.length; index++) {
    const code: number = key.charCodeAt(index);
    if (code >= 0xd800 && code <= 0xdbff) {
      const next: number = key.charCodeAt(++index);
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
  const bytes: Uint8Array<ArrayBuffer> = new TextEncoder().encode(`${kind}\0${key}`);
  const digest: ArrayBuffer = await globalThis.crypto.subtle.digest("SHA-256", bytes);
  const encoded: string = Array.from(new Uint8Array(digest), (byte: number): string =>
    byte.toString(16).padStart(2, "0"),
  ).join("");
  return `${kind === "route" ? "r" : "o"}-${encoded.slice(0, 16)}/${encoded.slice(16)}.js`;
}

function referenceValue<Route extends string>(data: ReferenceData): OperationReference<Route> {
  const reference: object = Object.create(null) as object;
  Object.defineProperty(reference, referenceKey, { value: Object.freeze(data) });
  return Object.freeze(reference) as OperationReference<Route>;
}

/** Used only by generated static operation modules; selection still occurs once. */
export function staticOperationReference<Route extends string>(
  provider: RoutedExecutionProvider<Route>,
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
export function createOperationLoader(
  configuration: OperationLoaderConfiguration,
): OperationLoader {
  const generation: string = configuration.generation;
  const publicRoutes: ReadonlySet<string> | undefined =
    configuration.publicRoutes === undefined ? undefined : new Set(configuration.publicRoutes);
  const privateRoutes: ReadonlySet<string> | undefined =
    configuration.privateRoutes === undefined ? undefined : new Set(configuration.privateRoutes);
  const baseURL: URL | undefined =
    configuration.baseURL === undefined ? undefined : new URL("./lookup/", configuration.baseURL);
  const importModule: OperationModuleImporter =
    configuration.importModule ??
    ((url: string): Promise<unknown> => import(/* @vite-ignore */ url));
  const references: Map<string, OperationReference> = new Map();
  const prepared: WeakMap<object, PreparedData> = new WeakMap();
  const loads: Map<string, Promise<OperationExecutionProvider>> = new Map();
  const canonical: Map<string, OperationExecutionProvider> = new Map();
  let clientModule: Promise<PreparedClientModule> | undefined;

  function isPublicRoute(route: string): boolean {
    return (
      (publicRoutes === undefined || publicRoutes.has(route)) &&
      (privateRoutes === undefined || !privateRoutes.has(route))
    );
  }

  function validateProvider(value: unknown): OperationExecutionProvider {
    if (value === null || typeof value !== "object") {
      throw new OperationPreparationError(
        "IDENTITY",
        "Lookup did not return an execution provider",
      );
    }
    const provider: Partial<OperationExecutionProvider> =
      value as Partial<OperationExecutionProvider>;
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
    const previous: OperationExecutionProvider | undefined = canonical.get(provider.route);
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
    const identity: string = `${kind}\0${key}`;
    let value: OperationReference | undefined = references.get(identity);
    if (value === undefined) {
      value = referenceValue({ abi: operationLoaderABI, generation, kind, key });
      references.set(identity, value);
    }
    return value;
  }

  function namespace(kind: OperationLookupKind): Readonly<Record<string, OperationReference>> {
    const unsupported: () => never = (): never => {
      throw new TypeError(
        "Reference namespaces have no runtime name list; import selective/all.js to enumerate names",
      );
    };
    const readonly: () => never = (): never => {
      throw new TypeError("Operation reference namespaces are readonly");
    };
    return new Proxy(Object.create(null) as Record<string, OperationReference>, {
      get(
        _target: Record<string, OperationReference>,
        key: string | symbol,
      ): string | OperationReference | undefined {
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
    const candidate: Partial<ReferenceData> = data as Partial<ReferenceData>;
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
      const provider: OperationExecutionProvider = validateProvider(data.provider);
      if (data.kind !== "route" || provider.route !== data.key) {
        throw new OperationPreparationError("IDENTITY", "Static reference route mismatch");
      }
      return Promise.resolve(canonicalize(provider));
    }
    if (data.kind === "route") {
      const existing: OperationExecutionProvider | undefined = canonical.get(data.key);
      if (existing !== undefined) return Promise.resolve(existing);
    }
    const identity: string = `${data.kind}\0${data.key}`;
    const existing: Promise<OperationExecutionProvider> | undefined = loads.get(identity);
    if (existing !== undefined) return existing;
    const pending: Promise<OperationExecutionProvider> =
      (async (): Promise<OperationExecutionProvider> => {
        if (loadProvider !== undefined) {
          let loaded: unknown;
          try {
            loaded = await loadProvider();
          } catch (cause: unknown) {
            throw new OperationPreparationError(
              "MODULE_LOAD",
              "Could not load the linked operation module",
              { cause },
            );
          }
          const provider: OperationExecutionProvider = validateProvider(loaded);
          if (data.kind !== "route" || provider.route !== data.key) {
            throw new OperationPreparationError("IDENTITY", "Link selected a different operation");
          }
          return canonicalize(provider);
        }
        const filename: string = await operationLookupFilename(data.kind, data.key);
        let module: unknown;
        try {
          module = await importModule(
            baseURL === undefined ? `./lookup/${filename}` : new URL(filename, baseURL).href,
          );
        } catch (cause: unknown) {
          throw new OperationPreparationError(
            "MODULE_LOAD",
            "Could not load the selected operation module",
            { cause },
          );
        }
        const entry: Partial<OperationLookupEntry> | undefined = (
          module as OperationLookupModule | null
        )?.entry;
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
        const provider: OperationExecutionProvider = validateProvider(entry.provider);
        if ((data.kind === "route" ? provider.route : provider.operationID) !== data.key) {
          throw new OperationPreparationError("IDENTITY", "Lookup selected a different operation");
        }
        return canonicalize(provider);
      })();
    loads.set(identity, pending);
    void pending.catch((): void => {
      if (loads.get(identity) === pending) loads.delete(identity);
    });
    return pending;
  }

  const resolve: OperationProviderResolver = (
    route: string,
    loadProvider?: () => Promise<OperationExecutionProvider>,
  ): Promise<OperationExecutionProvider> =>
    load({ abi: operationLoaderABI, generation, kind: "route", key: route }, loadProvider);

  async function loadOperations<const Selection>(
    selection: Selection & SelectionInput<Selection>,
  ): Promise<PreparedOperations<Selection>> {
    // No SDK module request starts until the entire selection has been collected.
    // Application getter exceptions intentionally retain their original identity.
    const selected: readonly ReferenceData[] = collectSelectionReferences(selection, readReference);
    for (const data of selected) {
      if (data.kind === "route" && !isPublicRoute(data.key)) {
        throw new OperationPreparationError(
          "INPUT",
          "Operation is outside the generated public selection",
        );
      }
    }
    const providers: OperationExecutionProvider[] = await Promise.all(
      selected.map((data: ReferenceData): Promise<OperationExecutionProvider> => load(data)),
    );
    if (
      providers.some(
        (provider: OperationExecutionProvider): boolean => !isPublicRoute(provider.route),
      )
    ) {
      throw new OperationPreparationError(
        "INPUT",
        "Operation is outside the generated public selection",
      );
    }
    if (clientModule === undefined) {
      const pending: Promise<PreparedClientModule> = Promise.resolve().then(
        (): Promise<PreparedClientModule> => configuration.loadClient(),
      );
      clientModule = pending;
      void pending.catch((): void => {
        if (clientModule === pending) clientModule = undefined;
      });
    }
    let client: PreparedClientModule;
    try {
      client = await clientModule;
    } catch (cause: unknown) {
      throw new OperationPreparationError(
        "MODULE_LOAD",
        "Could not load the client implementation",
        { cause },
      );
    }
    if (typeof client.createSelectedClient !== "function") {
      throw new OperationPreparationError("IDENTITY", "Client implementation ABI mismatch");
    }
    const handle: PreparedOperations<Selection> = Object.freeze(
      Object.create(null),
    ) as PreparedOperations<Selection>;
    prepared.set(handle, { providers: Object.freeze([...new Set(providers)]), client });
    return handle;
  }

  function createClient<Selection>(options: SelectedClientOptions<Selection>): object {
    const data: PreparedData | undefined = prepared.get(options.operations);
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
