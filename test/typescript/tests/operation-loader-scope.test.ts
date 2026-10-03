import { describe, expect, it } from "vitest";
import {
  createOperationLoader,
  operationLookupFilename,
  staticOperationReference,
  type OperationExecutionProvider,
  type OperationLoader,
  type OperationLoaderConfiguration,
  type OperationLookupEntry,
  type OperationProviderResolver,
  type PreparedClientModule,
} from "../../../internal/target/typescript/runtime/internal/operation-loader.js";
import {
  createOperationLoader as emittedLoader,
  staticOperationReference as emittedStatic,
} from "../fixtures/generated/lifecycle/internal/runtime/operation-loader.js";

type RoutePolicy = Pick<OperationLoaderConfiguration, "publicRoutes" | "privateRoutes">;
type FixtureClientArguments = Parameters<PreparedClientModule["createSelectedClient"]>;

interface FixtureClient {
  readonly providers: FixtureClientArguments[1];
  readonly resolve: FixtureClientArguments[2];
}

interface LoaderFixture {
  readonly loader: OperationLoader;
  readonly requests: string[];
  readonly clientLoads: () => number;
}

interface ScopeCase {
  readonly name: string;
  readonly policy: RoutePolicy;
}

const generation: string = "scope-fixture";
function provider(route: string, operationID: string): OperationExecutionProvider {
  return {
    abi: 1,
    generation,
    route,
    operationID,
    bind: (): object => ({}),
  };
}
const a: OperationExecutionProvider = provider("GET /a", "a");
const b: OperationExecutionProvider = provider("POST /b", "b");
const policies: readonly ScopeCase[] = [
  { name: "public roots", policy: { publicRoutes: [a.route] } },
  { name: "private dependencies", policy: { privateRoutes: [b.route] } },
  {
    name: "intersecting scopes",
    policy: { publicRoutes: [a.route, b.route], privateRoutes: [b.route] },
  },
];

async function setup(
  create: typeof createOperationLoader,
  policy: RoutePolicy,
): Promise<LoaderFixture> {
  const requests: string[] = [];
  const entries: Map<string, OperationLookupEntry> = new Map();
  for (const value of [a, b]) {
    for (const [kind, key] of [
      ["route", value.route],
      ["operation", value.operationID!],
    ] as const) {
      entries.set(await operationLookupFilename(kind, key), {
        abi: 1,
        generation,
        kind,
        key,
        provider: value,
      });
    }
  }
  let clientLoads: number = 0;
  const loader: OperationLoader = create({
    ...policy,
    generation,
    baseURL: new URL("https://assets.test/selective/"),
    importModule: async (url: string): Promise<unknown> => {
      requests.push(url);
      const entry: OperationLookupEntry | undefined = entries.get(
        new URL(url).pathname.split("/lookup/")[1]!,
      );
      if (entry === undefined) throw new Error("missing lookup");
      return { entry };
    },
    loadClient: async (): Promise<PreparedClientModule> => {
      clientLoads++;
      return {
        createSelectedClient: (
          _options: FixtureClientArguments[0],
          providers: FixtureClientArguments[1],
          resolve: FixtureClientArguments[2],
        ): FixtureClient => ({ providers, resolve }),
      };
    },
  });
  return { loader, requests, clientLoads: (): number => clientLoads };
}

for (const [label, create, staticRef] of [
  ["template", createOperationLoader, staticOperationReference],
  [
    "emitted",
    emittedLoader as unknown as typeof createOperationLoader,
    emittedStatic as unknown as typeof staticOperationReference,
  ],
] as const) {
  describe(`${label} loader scope`, (): void => {
    for (const test of policies) {
      it(`${test.name} rejects private route/static references before imports`, async (): Promise<void> => {
        const fixture: LoaderFixture = await setup(create, test.policy);
        for (const reference of [fixture.loader.routes[b.route]!, staticRef(b)]) {
          await expect(fixture.loader.loadOperations(reference)).rejects.toMatchObject({
            stage: "INPUT",
          });
        }
        expect(fixture.requests).toEqual([]);
        expect(fixture.clientLoads()).toBe(0);
      });

      it(`${test.name} rejects operation-ID references after lookup`, async (): Promise<void> => {
        const fixture: LoaderFixture = await setup(create, test.policy);
        await expect(
          fixture.loader.loadOperations(fixture.loader.operations.b!),
        ).rejects.toMatchObject({
          stage: "INPUT",
        });
        expect(fixture.requests).toHaveLength(1);
        expect(fixture.clientLoads()).toBe(0);
      });

      it(`${test.name} lets a public operation resolve private Link code`, async (): Promise<void> => {
        const fixture: LoaderFixture = await setup(create, test.policy);
        const prepared: Awaited<ReturnType<OperationLoader["loadOperations"]>> =
          await fixture.loader.loadOperations(staticRef(a));
        const client: FixtureClient = fixture.loader.createClient({
          operations: prepared,
        }) as FixtureClient;
        expect(client.providers).toEqual([a]);
        expect(fixture.requests).toEqual([]);
        expect(
          await client.resolve(b.route, async (): Promise<OperationExecutionProvider> => b),
        ).toBe(b);
        await expect(
          fixture.loader.loadOperations(fixture.loader.routes[b.route]!),
        ).rejects.toMatchObject({
          stage: "INPUT",
        });
        expect(client.providers).toEqual([a]);
        expect(fixture.requests).toEqual([]);
      });
    }

    it("prepares all static roots with no scope lists", async (): Promise<void> => {
      const fixture: LoaderFixture = await setup(create, {});
      const prepared: Awaited<ReturnType<OperationLoader["loadOperations"]>> =
        await fixture.loader.loadOperations([staticRef(a), staticRef(b)]);
      const client: FixtureClient = fixture.loader.createClient({
        operations: prepared,
      }) as FixtureClient;
      expect(client.providers).toEqual([a, b]);
      expect(fixture.requests).toEqual([]);
    });
  });
}
