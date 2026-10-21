// Expectations come from the pinned Graph paths and component schemas.
// __SDK_ROOT__ is replaced only in the generated checking fixture.
import type {
  Client, ComponentInput, ComponentOutput, OperationInput, OperationOutput,
  OperationBody, OperationQuery, OperationParameter, RouteQuery,
} from "__SDK_ROOT__";

type Equal<A, B> = [A] extends [B] ? [B] extends [A] ? true : false : false;
type Assert<T extends true> = T;
export type UserDisplayName = Assert<Equal<ComponentInput<"microsoft.graph.user">["displayName"], string | null | undefined>>;
export type TopParameter = Assert<Equal<OperationParameter<"users.user.ListUser", "query", "$top">, number>>;
export type UnionTop = Assert<Equal<Exclude<OperationQuery<"users.user.ListUser" | "groups.group.ListGroup">["$top"], undefined>, number>>;
export type NeverQuery = Assert<Equal<RouteQuery<never>, never>>;

export const userInput: ComponentInput<"microsoft.graph.user"> = { displayName: "Ada" };
export const userOutput: ComponentOutput<"microsoft.graph.user"> = { displayName: "Ada" };
export const createBody: OperationBody<"users.user.CreateUser"> = userInput;
export const createRequest: OperationInput<"users.user.CreateUser"> = { body: createBody };
export const listQuery: RouteQuery<"GET /users"> = { $top: 2 };
// @ts-expect-error $top is a number
export const invalidListQuery: RouteQuery<"GET /users"> = { $top: "2" };
export const listResponse: OperationOutput<"users.user.ListUser"> = { value: [userOutput] };

export async function consume(client: Client): Promise<string | null | undefined> {
  const response: OperationOutput<"users.user.ListUser"> = await client.$operations["users.user.ListUser"]({ query: listQuery });
  return response.value?.[0]?.displayName;
}
export function mutateOutput(): void { userOutput.displayName = "Grace"; }
export function rejectInvalidInputs(): void {
  // @ts-expect-error input projection remains readonly
  userInput.displayName = "Grace";
  // @ts-expect-error the schema requires string/null, not a numeric value
  userOutput.displayName = 123;
}
// @ts-expect-error GET /users has no request body section
export type MissingBody = OperationBody<"users.user.ListUser">;
// @ts-expect-error unknown query parameter name must be rejected
export type MissingParameter = OperationParameter<"users.user.ListUser", "query", "__sdkgen_absent__">;
