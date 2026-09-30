import { describe, expect, it, vi } from "vitest";

import {
  bindPathOperation,
  bindStreamOperation,
  type InputOperationCall,
  type RequestFunction,
} from "../../../internal/target/typescript/runtime/internal/callables.js";
import type { OperationDefinition } from "../../../internal/target/typescript/runtime/internal/operation.js";
import type {
  OperationStream,
  RawResponse,
  RequestOptions,
} from "../../../internal/target/typescript/runtime/internal/request.js";

const operation: OperationDefinition = {
  route: "GET /items/{id}",
  method: "GET",
  path: "/items/{id}",
  envelope: "",
};

function streamHandle<Item>(): OperationStream<Item> {
  return {
    response: Promise.resolve({
      status: 200,
      headers: new Headers(),
      request: {},
    }),
    abort() {},
    toReadableStream() {
      return new ReadableStream<Item>();
    },
    async *[Symbol.asyncIterator]() {},
  };
}

describe("stream callable binding", () => {
  it("dispatches no-input, required-input and optional-input stream calls", () => {
    const stream = vi.fn(() => streamHandle<unknown>());
    const request = Object.assign(async () => undefined, {
      raw: async () => ({}) as RawResponse<unknown>,
      stream,
    }) as unknown as RequestFunction;

    const noInput = bindStreamOperation<never, unknown>(
      request,
      operation,
      false,
      false,
      "application/x-ndjson",
    );
    noInput();
    expect(stream).toHaveBeenLastCalledWith(operation, undefined, {
      accept: "application/x-ndjson",
    });

    const required = bindStreamOperation<{ query: { page: number } }, unknown>(
      request,
      operation,
      true,
      false,
      "application/x-ndjson",
    );
    const requiredInput = { query: { page: 2 } };
    required(requiredInput, { accept: "text/event-stream" });
    expect(stream).toHaveBeenLastCalledWith(operation, requiredInput, {
      accept: "text/event-stream",
    });

    const optional = bindStreamOperation<{ query?: { page?: number } }, unknown>(
      request,
      operation,
      true,
      true,
      "application/x-ndjson",
    );
    optional({ timeoutMS: 25 });
    expect(stream).toHaveBeenLastCalledWith(operation, undefined, {
      timeoutMS: 25,
      accept: "application/x-ndjson",
    });

    const optionalInput = { query: { page: 3 } };
    optional(optionalInput);
    expect(stream).toHaveBeenLastCalledWith(operation, optionalInput, {
      accept: "application/x-ndjson",
    });
  });

  it("merges resource path parameters into decoded, raw and stream calls", async () => {
    type FullInput = {
      readonly path: { readonly id: string };
      readonly query?: { readonly q?: string };
    };
    type Input = {
      readonly query?: { readonly q?: string };
    };

    const decoded = vi.fn(async (input: FullInput, _options?: RequestOptions) => input);
    const raw = vi.fn(async (input: FullInput, _options?: RequestOptions) => ({
      status: 200,
      headers: {},
      contentType: undefined,
      data: input,
      request: {},
      response: new Response(null),
    }));
    const stream = vi.fn((_input: FullInput, _options?: RequestOptions) => streamHandle<unknown>());
    const links = { follow: vi.fn() };
    const paginate = vi.fn();

    const source = Object.assign(decoded, {
      raw,
      stream,
      links,
      paginate,
    }) as unknown as InputOperationCall<
      FullInput,
      FullInput,
      RequestOptions,
      RawResponse<FullInput>
    > & {
      readonly stream: (input: FullInput, options?: RequestOptions) => OperationStream<unknown>;
      readonly links: typeof links;
      readonly paginate: typeof paginate;
    };

    const bound = bindPathOperation<FullInput, Input, FullInput>(
      source,
      { id: "item/1" },
      true,
      true,
    ) as ReturnType<typeof bindPathOperation<FullInput, Input, FullInput>> & {
      readonly stream: (input?: Input, options?: RequestOptions) => OperationStream<unknown>;
      readonly links: typeof links;
      readonly paginate: typeof paginate;
    };

    await bound({ query: { q: "decoded" } });
    expect(decoded).toHaveBeenLastCalledWith({
      query: { q: "decoded" },
      path: { id: "item/1" },
    });

    const optionalRaw = bound.raw as unknown as (
      options?: RequestOptions,
    ) => Promise<RawResponse<FullInput>>;
    await optionalRaw({ timeoutMS: 10 });
    expect(raw).toHaveBeenLastCalledWith({ path: { id: "item/1" } }, { timeoutMS: 10 });

    bound.stream({ query: { q: "stream" } }, { timeoutMS: 20 });
    expect(stream).toHaveBeenLastCalledWith(
      { query: { q: "stream" }, path: { id: "item/1" } },
      { timeoutMS: 20 },
    );
    expect(bound.links).toBe(links);
    expect(bound.paginate).toBe(paginate);
  });
});
