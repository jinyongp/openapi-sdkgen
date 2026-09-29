// Compile-only contract checks, also copied into strict source and d.ts-only consumers.
import type {
  GuaranteedSelection,
  OperationReference,
  OperationSelection,
  PossibleSelection,
  SelectedOperationCalls,
  SelectionInput,
} from "../fixtures/generated/lifecycle/internal/runtime/selection-types.js";
import type { BaseCall as Echo } from "../fixtures/generated/lifecycle/internal/operations/inline/post.js";
import type {
  BaseCall as Events,
  Stream,
} from "../fixtures/generated/lifecycle/internal/operations/events/get.js";

type Routes = "GET /a" | "POST /b" | "GET /c";
type A = OperationReference<"GET /a">;
type B = OperationReference<"POST /b">;
type C = OperationReference<"GET /c">;
// Untyped external values must not manufacture guaranteed operation membership.
type UncheckedJSON = ReturnType<typeof JSON.parse>;
type Required<Value> = GuaranteedSelection<Value, Routes>;
type Possible<Value> = PossibleSelection<Value, Routes>;
type Equal<Left, Right> =
  (<Value>() => Value extends Left ? 1 : 2) extends <Value>() => Value extends Right ? 1 : 2
    ? true
    : false;
type Assert<Value extends true> = Value;
declare const outside: unique symbol;
declare const a: A, b: B, c: C;
declare function preserve<const Value>(value: Value & SelectionInput<Value>): Value;

function getterFeature() {
  return {
    get reads() {
      return [a] as const;
    },
    writes: { one: b },
  };
}
class Feature {
  readonly read = a;
}

type Inline = ReturnType<typeof inlineFeature>;
function inlineFeature() {
  return preserve([a, { nested: [b, c] }]);
}

// Each tuple entry is checked by tsc. There is no runtime assertion standing in for typechecking.
export type SelectionTypeLaws = [
  Assert<Equal<Possible<A>, "GET /a">>,
  Assert<Equal<Required<A>, "GET /a">>,
  Assert<Equal<Required<readonly [A, B]>, "GET /a" | "POST /b">>,
  Assert<Equal<Required<A[]>, never>>,
  Assert<Equal<Possible<readonly (A | B)[]>, "GET /a" | "POST /b">>,
  Assert<Equal<Required<OperationReference<"GET /a" | "POST /b">>, never>>,
  Assert<Equal<Required<{ one: A; nested: { two: readonly [B] } }>, "GET /a" | "POST /b">>,
  Assert<Equal<Required<{ maybe?: A }>, never>>,
  Assert<Equal<Possible<{ maybe?: A }>, "GET /a">>,
  Assert<Equal<Required<readonly []>, never>>,
  Assert<Equal<Possible<Record<never, never>>, never>>,
  Assert<Equal<Required<readonly [A, B] | readonly [A, C]>, "GET /a">>,
  Assert<Equal<Required<{ left: A } | { right: A }>, "GET /a">>,
  Assert<Equal<Required<{ left: A } | { right: B }>, never>>,
  Assert<Equal<Required<{ fixed: A; maybe?: B; dynamic: C[] }>, "GET /a">>,
  Assert<Equal<Required<Readonly<Record<string, A>>>, never>>,
  Assert<Equal<Possible<{ one: A; [outside]: B }>, "GET /a">>,
  Assert<Equal<Required<{ 7: A }>, "GET /a">>,
  Assert<Equal<Required<Feature>, "GET /a">>,
  Assert<Equal<Required<ReturnType<typeof getterFeature>>, "GET /a" | "POST /b">>,
  Assert<Equal<SelectionInput<() => A>, never>>,
  Assert<Equal<SelectionInput<Promise<A>>, never>>,
  Assert<Equal<SelectionInput<undefined>, never>>,
  Assert<Equal<SelectionInput<null>, never>>,
  Assert<Equal<SelectionInput<number>, never>>,
  Assert<Equal<Required<readonly [A, ...B[]]>, "GET /a">>,
  Assert<Equal<Required<readonly [...B[], A]>, "GET /a">>,
  Assert<Equal<Required<readonly [A, ...B[], C]>, "GET /a" | "GET /c">>,
  Assert<Equal<Required<readonly [A, B?]>, "GET /a">>,
  Assert<Equal<Required<readonly [A?, ...B[]]>, never>>,
  Assert<Equal<Required<Inline>, Routes>>,
  Assert<Equal<Required<OperationReference<string>>, never>>,
  Assert<Equal<Possible<OperationReference<string>>, Routes>>,
  Assert<Equal<Required<readonly [A, ...OperationReference<string>[]]>, "GET /a">>,
  Assert<Equal<Required<UncheckedJSON>, never>>,
  Assert<Equal<Possible<UncheckedJSON>, Routes>>,
  Assert<Equal<Required<readonly [A, UncheckedJSON]>, "GET /a">>,
  Assert<Equal<Required<unknown>, never>>,
  Assert<Equal<Required<never>, never>>,
  Assert<Equal<Required<OperationReference<never>>, never>>,
  Assert<Equal<Possible<OperationReference<never>>, never>>,
  Assert<Equal<GuaranteedSelection<A, string>, never>>,
  Assert<Equal<Required<{ fixed: A; choice: B | C }>, "GET /a">>,
  Assert<Equal<Required<{ first: A | B; second: B | C }>, never>>,
  Assert<Equal<Required<{ first: A | B; second: B }>, "POST /b">>,
  Assert<Equal<Required<{ left: A; right: B } | { left: B; right: A }>, "GET /a" | "POST /b">>,
  Assert<Equal<Required<{ maybe?: A } | { maybe: A }>, never>>,
  Assert<Equal<Required<{ required: A | undefined }>, never>>,
  Assert<Equal<Required<OperationReference<`GET /${string}`>>, never>>,
  Assert<Equal<Required<{ broad: OperationReference<`GET /${string}`>; fixed: B }>, "POST /b">>,
  Assert<Equal<Required<OperationReference<"GET /a" | "GET /outside">>, never>>,
  Assert<Equal<Required<{ impossible: OperationReference<never>; choice: A | B }>, never>>,
  Assert<Equal<SelectionInput<OperationSelection>, OperationSelection>>,
  Assert<Equal<Required<OperationSelection>, never>>,
  Assert<Equal<Possible<OperationSelection>, Routes>>,
  Assert<Equal<Required<readonly [A, OperationSelection]>, "GET /a">>,
  Assert<Equal<Possible<readonly [A, OperationSelection]>, Routes>>,
  Assert<Equal<Required<{ fixed: A; feature: OperationSelection }>, "GET /a">>,
  Assert<Equal<Possible<Readonly<Record<string, OperationSelection>>>, Routes>>,
  Assert<Equal<Required<Readonly<Record<string, OperationSelection>>>, never>>,
  Assert<Equal<Required<readonly OperationSelection[]>, never>>,
  Assert<Equal<Possible<readonly OperationSelection[]>, Routes>>,
  Assert<Equal<SelectionInput<OperationReference | number>, OperationReference>>,
  Assert<Equal<Possible<unknown>, never>>,
];

type Calls = { "POST /inline": Echo; "GET /events": Events & { readonly stream: Stream } };
type EchoReference = OperationReference<"POST /inline">;
type EventReference = OperationReference<"GET /events">;
type Fixed = SelectedOperationCalls<readonly [EchoReference], Calls>;
type Dynamic = SelectedOperationCalls<readonly EventReference[], Calls>;
type Either = SelectedOperationCalls<readonly [EchoReference] | readonly [EventReference], Calls>;
export type SelectedCallableLaws = [
  Assert<Equal<Fixed["POST /inline"], Echo>>,
  Assert<Equal<keyof Fixed, "POST /inline">>,
  Assert<Equal<Dynamic["GET /events"], Calls["GET /events"] | undefined>>,
  Assert<Equal<keyof SelectedOperationCalls<readonly [], Calls>, never>>,
  Assert<Equal<Either["POST /inline"], Echo | undefined>>,
  Assert<Equal<Either["GET /events"], Calls["GET /events"] | undefined>>,
];

declare const fixed: Fixed, dynamic: Dynamic, either: Either;
function consumerWitness() {
  preserve(new Feature());
  preserve(getterFeature());
  preserve({ default: [a], nested: { more: [b] } });
  preserve({ [outside]: () => "not a collected field", selected: a });
  // @ts-expect-error Present undefined is not an omitted optional field.
  preserve({ present: undefined });
  // @ts-expect-error The API does not invoke configuration functions.
  preserve(() => a);
  // @ts-expect-error Async configuration must be awaited by the caller.
  preserve(Promise.resolve(a));
  // @ts-expect-error Metadata is not silently discarded from a selection group.
  preserve({ selected: a, metadata: "not an operation" });
  void fixed["POST /inline"]({ body: { value: 1 } });
  void fixed["POST /inline"].raw({ body: { value: 1 } });
  // @ts-expect-error Required selection must retain the original numeric body contract.
  void fixed["POST /inline"]({ body: { value: "wrong" } });
  // @ts-expect-error The unselected route is absent, not an arbitrary callable.
  fixed["GET /events"];
  // @ts-expect-error A dynamic array may be empty.
  dynamic["GET /events"].stream();
  const stream = dynamic["GET /events"]?.stream();
  void stream;
  // @ts-expect-error A union of disjoint selections guarantees neither operation.
  void either["POST /inline"]({ body: { value: 1 } });
  void either["POST /inline"]?.({ body: { value: 1 } });
}
void consumerWitness;
