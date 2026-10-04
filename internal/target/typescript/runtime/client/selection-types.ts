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

// A fully widened selection already permits every reference and guarantees none.
// Stop at that boundary instead of recursively expanding OperationSelection.
// The validity check keeps unions with unsupported leaves subject to validation.
type IsUnboundedSelection<Value> =
  OperationReference extends Extract<Value, OperationReference>
    ? [Value] extends [OperationSelection]
      ? true
      : false
    : false;

/**
 * Checks leaves without requiring an index signature on a feature object.
 * Optional properties remain optional. Symbols are outside group collection.
 */
export type SelectionInput<Value> =
  IsAny<Value> extends true
    ? Value
    : IsUnboundedSelection<Value> extends true
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
    : IsUnboundedSelection<Value> extends true
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

type IsUnion<Value, Whole = Value> = Value extends Whole
  ? [Whole] extends [Value]
    ? false
    : true
  : never;

type ExactReferenceRoute<Route extends string, Routes extends string> =
  IsAny<Route> extends true
    ? never
    : string extends Route
      ? never
      : [Route] extends [never]
        ? never
        : [Route] extends [Routes]
          ? true extends IsUnion<Route>
            ? never
            : Route
          : never;

// Fixed prefixes and suffixes contribute guarantees; an arbitrary array can be empty.
type ArrayGuaranteed<
  Value extends readonly unknown[],
  Routes extends string,
> = Value extends readonly [infer First, ...infer Rest]
  ? AllGuaranteed<First, Routes> | ArrayGuaranteed<Rest, Routes>
  : Value extends readonly [...infer Rest, infer Last]
    ? ArrayGuaranteed<Rest, Routes> | AllGuaranteed<Last, Routes>
    : never;

type BranchGuaranteed<Value, Routes extends string> =
  Value extends OperationReference<infer Route>
    ? ExactReferenceRoute<Route, Routes>
    : Value extends SelectionFunction | SelectionConstructor | PromiseLike<unknown>
      ? never
      : Value extends readonly unknown[]
        ? ArrayGuaranteed<Value, Routes>
        : Value extends object
          ? {
              [Key in SelectionKeys<Value>]-?: Record<never, never> extends Pick<Value, Key>
                ? never
                : AllGuaranteed<Value[Key], Routes>;
            }[SelectionKeys<Value>]
          : never;

// keyof a union contains only keys shared by every branch. Build each branch
// once instead of testing every route against every required feature key.
// The inferred intermediate also defers recursive mapped-key constraints.
type GuaranteedBranchMaps<Value, Universe extends string> = Value extends unknown
  ? BranchGuaranteed<Value, Universe> extends infer Routes
    ? { readonly [Route in Routes & string]: true }
    : never
  : never;

type AllGuaranteed<Value, Routes extends string> =
  IsAny<Value> extends true
    ? never
    : [Value] extends [never]
      ? never
      : IsUnboundedSelection<Value> extends true
        ? never
        : keyof GuaranteedBranchMaps<Value, Routes> & string;

/** Routes present in every possible selection branch, within the generated universe. */
export type GuaranteedSelection<Value, Routes extends string> = string extends Routes
  ? never
  : AllGuaranteed<Value, Routes> & Routes;

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
