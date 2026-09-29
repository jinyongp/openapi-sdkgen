/** Compile-time identity carried by generated operation references. */
declare const operationReferenceIdentity: unique symbol;

/** A reference selects an operation without importing its execution module. */
export interface OperationReference<Route extends string = string> {
  readonly [operationReferenceIdentity]: Route;
}

/** Supported value composition; generic input checking also preserves class fields. */
export type OperationSelection =
  | OperationReference
  | readonly OperationSelection[]
  | { readonly [key: string]: OperationSelection };

type SelectionKeys<Value> = Extract<keyof Value, string | number>;
type IsAny<Value> = 0 extends 1 & Value ? true : false;
type SelectionFunction = (...args: never[]) => unknown;
type SelectionConstructor = abstract new (...args: never[]) => unknown;

/**
 * Checks leaves without requiring an index signature on a feature object.
 * Optional properties remain optional. Symbols are outside group collection.
 */
export type SelectionInput<Value> =
  IsAny<Value> extends true
    ? Value
    : Value extends OperationReference
      ? Value
      : Value extends SelectionFunction | SelectionConstructor | PromiseLike<unknown>
        ? never
        : Value extends readonly unknown[]
          ? { [Key in keyof Value]: SelectionInput<Value[Key]> }
          : Value extends object
            ? {
                [Key in keyof Value]: Key extends string | number
                  ? SelectionInput<Value[Key]>
                  : Value[Key];
              }
            : never;

/** Every route that could occur, restricted to the generated document's route universe. */
export type PossibleSelection<Value, Routes extends string = string> =
  IsAny<Value> extends true
    ? Routes
    : Value extends OperationReference<infer Route>
      ? Route & Routes
      : Value extends SelectionFunction | SelectionConstructor | PromiseLike<unknown>
        ? never
        : Value extends readonly unknown[]
          ? PossibleSelection<Value[number], Routes>
          : Value extends object
            ? {
                [Key in SelectionKeys<Value>]-?: PossibleSelection<Value[Key], Routes>;
              }[SelectionKeys<Value>]
            : never;

type EverySelectionBranch<Value, Route extends string> =
  IsAny<Value> extends true
    ? false
    : [Value] extends [never]
      ? false
      : [SelectionBranchHas<Value, Route>] extends [true]
        ? true
        : false;

// Inspect required prefixes and suffixes, not just fixed-length tuples. A
// selection [A, ...dynamic, B] still guarantees A and B even when length is number.
type ArraySelectionHas<
  Value extends readonly unknown[],
  Route extends string,
> = Value extends readonly [infer First, ...infer Rest]
  ? true extends EverySelectionBranch<First, Route> | ArraySelectionHas<Rest, Route>
    ? true
    : false
  : Value extends readonly [...infer Rest, infer Last]
    ? true extends EverySelectionBranch<Last, Route> | ArraySelectionHas<Rest, Route>
      ? true
      : false
    : false;

type SelectionBranchHas<Value, Route extends string> =
  Value extends OperationReference<infer Selected>
    ? [Selected] extends [Route]
      ? true
      : false
    : Value extends SelectionFunction | SelectionConstructor | PromiseLike<unknown>
      ? false
      : Value extends readonly unknown[]
        ? ArraySelectionHas<Value, Route>
        : Value extends object
          ? true extends {
              [Key in SelectionKeys<Value>]-?: Record<never, never> extends Pick<Value, Key>
                ? false
                : EverySelectionBranch<Value[Key], Route>;
            }[SelectionKeys<Value>]
            ? true
            : false
          : false;

/**
 * Routes present in every possible selection branch. The route universe is
 * supplied by generated declarations; an unconstrained string cannot prove presence.
 */
export type GuaranteedSelection<Value, Routes extends string> = string extends Routes
  ? never
  : {
      [Route in PossibleSelection<Value, Routes>]: EverySelectionBranch<Value, Route> extends true
        ? Route
        : never;
    }[PossibleSelection<Value, Routes>];

/** Keeps original callable signatures, making only uncertain membership optional. */
export type SelectedOperationCalls<Value, Calls extends object> = {
  readonly [Route in GuaranteedSelection<Value, Extract<keyof Calls, string>>]: Calls[Route];
} & {
  readonly [
    Route in Exclude<
      PossibleSelection<Value, Extract<keyof Calls, string>>,
      GuaranteedSelection<Value, Extract<keyof Calls, string>>
    >
  ]?: Calls[Route];
};
