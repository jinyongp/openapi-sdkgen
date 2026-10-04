import { isRecord } from "./runtime-support.js";
import type { RequestOptions } from "./request.js";

import type {
  PageRequest,
  PaginateInput,
  PaginationOptions,
  PaginationPlan,
  PaginationProfile,
} from "./pagination-types.js";
/** Forwards the canonical type contracts without importing their implementation. */
export type * from "./pagination-types.js";

/**
 * Creates a lazy async iterator over all items returned by a paginated operation.
 *
 * The iterator preserves the original filters and sort order. Cursor pagination
 * advances only `cursor`; offset pagination advances only `offset`. No request is
 * sent until iteration begins, and iteration stops when the server signals the end.
 *
 * @param requestPage Generated function that fetches one page.
 * @param profile Pagination strategy declared by the operation.
 * @returns Function producing an {@link AsyncIterable} of decoded items.
 */
export function createPaginator<
  Item,
  Input,
  Page,
  Profile extends PaginationProfile = PaginationProfile,
  CursorName extends string = string,
  OffsetName extends string = string,
  Options extends RequestOptions = RequestOptions,
  OptionsRequired extends boolean = false,
>(
  requestPage: PageRequest<Input, Page, Options, OptionsRequired>,
  plan: PaginationPlan<Profile, CursorName, OffsetName>,
): (
  input: PaginateInput<Input, Profile, CursorName, OffsetName>,
  ...options: PaginationOptions<Options, OptionsRequired>
) => AsyncIterable<Item> {
  return (
    input: PaginateInput<Input, Profile, CursorName, OffsetName>,
    ...options: PaginationOptions<Options, OptionsRequired>
  ): PaginatorIterator<Item> => ({
    async *[Symbol.asyncIterator](): AsyncGenerator<Awaited<Item>, void, unknown> {
      const root: Record<string, unknown> = isRecord(input) ? { ...input } : {};
      const requestedMode: unknown = root["mode"];
      delete root["mode"];
      const mode: "cursor" | "offset" = resolvePaginationMode(plan.mode, requestedMode);
      const query: Record<string, unknown> = isRecord(root["query"]) ? { ...root["query"] } : {};
      const cursorName: CursorName | undefined = plan.request.cursor;
      const offsetName: OffsetName | undefined = plan.request.offset;
      const limitName: string | undefined = plan.request.limit;
      if (mode === "cursor" && offsetName !== undefined && query[offsetName] !== undefined) {
        throw new TypeError(`cursor pagination does not accept ${offsetName}`);
      }
      if (mode === "offset" && cursorName !== undefined && query[cursorName] !== undefined) {
        throw new TypeError(`offset pagination does not accept ${cursorName}`);
      }
      root["query"] = query;
      const seenCursors: Set<string> = new Set<string>();
      if (cursorName !== undefined && typeof query[cursorName] === "string") {
        seenCursors.add(query[cursorName]);
      }
      const seenOffsets: Set<number> = new Set<number>();
      if (offsetName !== undefined && typeof query[offsetName] === "number") {
        seenOffsets.add(query[offsetName]);
      }
      for (;;) {
        const page: Awaited<Page> = await requestPage(
          { ...root, query: { ...query } } as Input,
          ...options,
        );
        const items: readonly unknown[] = pageItems(page, plan.response.items);
        for (const item of items) yield item as Item;
        if (mode === "cursor") {
          const nextCursor: unknown = paginationValue(page, plan.response.nextCursor);
          if (typeof nextCursor !== "string" || nextCursor === "" || seenCursors.has(nextCursor)) {
            return;
          }
          seenCursors.add(nextCursor);
          if (cursorName === undefined) return;
          query[cursorName] = nextCursor;
          continue;
        }
        if (offsetName === undefined) return;
        const requestedOffset: number = numberValue(query[offsetName], undefined, 0);
        const currentOffset: number = numberValue(
          paginationValue(page, plan.response.offset),
          query[offsetName],
          0,
        );
        const limit: number = numberValue(
          paginationValue(page, plan.response.limit),
          limitName === undefined ? undefined : query[limitName],
          items.length,
        );
        const totalValue: unknown = paginationValue(page, plan.response.total);
        const total: number | undefined = typeof totalValue === "number" ? totalValue : undefined;
        const nextOffset: number = currentOffset + limit;
        if (
          limit <= 0 ||
          items.length === 0 ||
          items.length < limit ||
          nextOffset <= requestedOffset ||
          seenOffsets.has(nextOffset) ||
          (total !== undefined && nextOffset >= total)
        )
          return;
        seenOffsets.add(nextOffset);
        query[offsetName] = nextOffset;
      }
    },
  });
}

function resolvePaginationMode(
  profile: PaginationProfile,
  requested: unknown,
): "cursor" | "offset" {
  if (profile === "both") {
    if (requested !== "cursor" && requested !== "offset") {
      throw new TypeError('Pagination profile "both" requires mode "cursor" or "offset"');
    }
    return requested;
  }
  if (requested !== undefined && requested !== profile) {
    throw new TypeError(`Pagination mode ${String(requested)} does not match ${profile}`);
  }
  return profile;
}

function pageItems(page: unknown, pointer: readonly string[]): readonly unknown[] {
  const value: unknown = paginationValue(page, pointer);
  return Array.isArray(value) ? value : [];
}

function paginationValue(page: unknown, pointer: readonly string[] | undefined): unknown {
  if (pointer === undefined) return undefined;
  let current: unknown = page;
  for (const token of pointer) {
    if ((typeof current !== "object" && typeof current !== "function") || current === null) {
      return undefined;
    }
    if (!Object.prototype.hasOwnProperty.call(current, token)) {
      return undefined;
    }
    current = (current as Record<string, unknown>)[token];
  }
  return current;
}

function numberValue(primary: unknown, secondary: unknown, fallback: number): number {
  if (typeof primary === "number" && Number.isFinite(primary)) return primary;
  if (typeof secondary === "number" && Number.isFinite(secondary)) return secondary;
  return fallback;
}

type PaginatorIterator<Item> = {
  [Symbol.asyncIterator](): AsyncGenerator<Awaited<Item>, void, unknown>;
};
