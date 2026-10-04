import type { RequestOptions } from "../http/request.js";
/** Query controls for cursor-based pagination. */
export type CursorPaginationInput = {
  /** Opaque cursor returned by the previous page. Omit for the first page. */
  readonly cursor?: string | undefined;
  /** Maximum number of items requested for one page. */
  readonly limit?: number | undefined;
  /** Offset pagination is unavailable in cursor mode. */
  readonly offset?: never;
};

/** Query controls for offset-based pagination. */
export type OffsetPaginationInput = {
  /** Zero-based index of the first requested item. */
  readonly offset?: number | undefined;
  /** Maximum number of items requested for one page. */
  readonly limit?: number | undefined;
  /** Cursor pagination is unavailable in offset mode. */
  readonly cursor?: never;
};

/** Query controls for an operation supporting either cursor or offset pagination. */
export type BothPaginationInput = CursorPaginationInput | OffsetPaginationInput;

/** Pagination strategy declared by an OpenAPI operation. */
export type PaginationProfile = "cursor" | "offset" | "both";

type QueryInput<Input> = Input extends { readonly query: infer Query } ? Query : never;
type WithoutQueryControl<Query, Name extends string> = Name extends ""
  ? Query
  : Omit<Query, Name> & { readonly [Key in Name]?: never };

/** Validated correlation between exact query controls and response-body locations. */
export type PaginationPlan<
  Profile extends PaginationProfile = PaginationProfile,
  CursorName extends string = string,
  OffsetName extends string = string,
> = {
  readonly mode: Profile;
  readonly request: {
    readonly cursor?: CursorName;
    readonly offset?: OffsetName;
    readonly limit?: string;
  };
  readonly response: {
    readonly items: readonly string[];
    readonly nextCursor?: readonly string[];
    readonly offset?: readonly string[];
    readonly limit?: readonly string[];
    readonly total?: readonly string[];
  };
};

/**
 * Input accepted by a generated pagination helper.
 *
 * Operations supporting both strategies require `mode`; cursor and offset fields
 * remain mutually exclusive in every profile.
 */
export type PaginateInput<
  Input,
  Profile extends PaginationProfile,
  CursorName extends string = "cursor",
  OffsetName extends string = "offset",
> = Profile extends "both"
  ?
      | (Omit<Input, "query"> & {
          readonly mode: "cursor";
          readonly query: WithoutQueryControl<QueryInput<Input>, OffsetName>;
        })
      | (Omit<Input, "query"> & {
          readonly mode: "offset";
          readonly query: WithoutQueryControl<QueryInput<Input>, CursorName>;
        })
  : Input & { readonly mode?: never };

/** Requires pagination options only when the operation contract requires them. */
export type PaginationOptions<Options, Required extends boolean> = Required extends true
  ? [options: Options]
  : [options?: Options];

/** Function that fetches one typed page for a generated pagination helper. */
export type PageRequest<
  Input,
  Page,
  Options extends RequestOptions = RequestOptions,
  OptionsRequired extends boolean = false,
> = (input: Input, ...options: PaginationOptions<Options, OptionsRequired>) => Promise<Page>;
