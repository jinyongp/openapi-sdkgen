# Streaming API

This page is the lookup reference for generated sequential-media APIs. For a
task-oriented walkthrough, see
[Authentication, transport, and streams](../guide/transport.md).

## OpenAPI version support

openapi-sdkgen reads OpenAPI 3.0.x, 3.1.x, and 3.2.x. Sequential media behave
differently depending on which fields the OpenAPI version can express.

| OpenAPI | Complete sequential value | Incremental stream |
| --- | --- | --- |
| 3.0.x | A normal Media Type Object `schema` can describe the complete value for built-in sequential content types. | Not expressible with standard OpenAPI fields. |
| 3.1.x | Same as 3.0.x, using the 3.1 JSON Schema model. | Not expressible with standard OpenAPI fields. |
| 3.2.x | `schema` continues to describe the complete value. | `itemSchema` enables typed incremental input/output. `prefixEncoding` and `itemEncoding` add positional and streaming multipart semantics. |

The 3.2 fields are defined by the
[OpenAPI Media Type Object](https://spec.openapis.org/oas/v3.2.0.html#media-type-object).

Built-in sequential framing covers:

- `text/event-stream` as Server-Sent Events;
- NDJSON / JSON Lines media types;
- `application/json-seq` and `+json-seq`;
- OpenAPI 3.2 positional or streaming multipart.

## Response streams

When a response Media Type Object declares OpenAPI 3.2 `itemSchema`, generated
operation, exact-route, and resource methods expose `.stream(...)`.

```ts
const stream = api.$operations.watchTodos.stream({
  query: { cursor: "0" },
});
```

The return type is `OperationStream<T>`.

### OperationStream

```ts
interface OperationStream<Item> extends AsyncIterable<Item> {
  readonly response: Promise<StreamResponseMetadata>;
  abort(reason?: unknown): void;
  toReadableStream(): ReadableStream<Item>;
}
```

The request starts lazily when response metadata or stream consumption is
requested. One `OperationStream` has one consumer. Calling `abort()`, cancelling
the Web `ReadableStream`, an external `AbortSignal`, or a timeout cancels the
underlying HTTP body.

`stream.response` resolves after response headers arrive:

```ts
const metadata = await stream.response;
metadata.status;
metadata.contentType;
metadata.headers;
metadata.request;
```

Use a separate `.raw()` request when the application needs the original,
unconsumed Fetch response body.

## Streaming request bodies

OpenAPI 3.2 request bodies with `itemSchema` accept `StreamSource<T>`:

```ts
type StreamSource<T> = AsyncIterable<T> | ReadableStream<T>;
```

A Media Type Object with only `schema` accepts the complete application value.
When both `schema` and `itemSchema` are present, the generated request type
accepts either the complete value or a `StreamSource<T>`.

## Built-in protocol frames

### ServerSentEvent

The built-in SSE parser yields standard event-stream fields:

```ts
interface ServerSentEvent {
  readonly data: string;
  readonly event?: string;
  readonly id?: string;
  readonly retry?: number;
}
```

`data` remains a string. JSON decoding, provider sentinels, event aggregation,
and domain-specific semantics belong in an application adapter. The parser
preserves the SSE last-event-id state and resets it when the wire stream sends an
empty `id:` field.

Generated clients do not reconnect or replay SSE automatically.

## Protocols and adapters

### StreamAdapter

A `StreamAdapter` maps protocol frames to application items and maps request
items back to protocol frames:

```ts
interface StreamAdapter<Frame, Item> {
  decode(
    frames: AsyncIterable<Frame>,
    context: StreamContext,
  ): AsyncIterable<Item>;

  encode(
    items: AsyncIterable<Item>,
    context: StreamContext,
  ): AsyncIterable<Frame>;
}
```

For responses and inbound server streams, adapter output is validated and
projected through the declared `itemSchema`. For streaming request bodies, the
generated item value is encoded through `itemSchema` before the adapter runs.

Use an adapter when the byte framing is already supported but the application
wire events need another semantic layer, such as JSON carried in SSE `data`.

### StreamProtocol

A `StreamProtocol` owns byte framing for a custom sequential media type:

```ts
interface StreamProtocol<Frame> {
  decode(
    reader: StreamReader,
    context: StreamContext,
  ): AsyncIterable<Frame>;

  encode(
    frames: AsyncIterable<Frame>,
    context: StreamContext,
  ): ReadableStream<Uint8Array> | Promise<ReadableStream<Uint8Array>>;
}
```

`StreamReader.read(maxBytes)` is bounded by the configured frame limit and
supports cancellation through `StreamReader.cancel(reason?)`.

### StreamCodec

```ts
interface StreamCodec<Frame = unknown, Item = unknown> {
  readonly protocol?: StreamProtocol<Frame>;
  readonly adapter?: StreamAdapter<Frame, Item>;
}
```

A codec may replace framing, add an application adapter, or do both.

## Configuration

### ClientOptions.streamCodecs

Set media-type defaults for every operation on one client:

```ts
const api = createClient({
  baseURL,
  streamCodecs: {
    "text/event-stream": {
      adapter: eventAdapter,
    },
  },
});
```

Keys are normalized media types.

### RequestOptions.streamCodec

Override the selected request or response media type for one call:

```ts
const stream = api.$operations.generate.stream(input, {
  streamCodec: providerCodec,
});
```

The request-level codec takes precedence over the client media-type default.

### maxStreamFrameBytes

`ClientOptions.maxStreamFrameBytes` sets the client default.
`RequestOptions.maxStreamFrameBytes` overrides it for one call.

The limit applies to one wire frame, record, or multipart part before
application adaptation. A custom `StreamProtocol` receives the same value as
`StreamContext.maxFrameBytes`.

Generated Webhook and Callback server APIs expose the same option for inbound
streams. See [Generated server API](./server-api.md).

## Type helpers

The generated entry point exports:

| Type | Purpose |
| --- | --- |
| `OperationStream<T>` | lazy single-consumer response stream |
| `StreamResponseMetadata` | status, headers, content type, request metadata |
| `StreamSource<T>` | incremental request source |
| `ServerSentEvent` | built-in SSE frame |
| `StreamReader` | bounded reader for custom protocols |
| `StreamContext` | content type, frame limit, abort signal |
| `StreamProtocol<Frame>` | byte framing |
| `StreamAdapter<Frame, Item>` | application semantic mapping |
| `StreamCodec<Frame, Item>` | protocol/adapter configuration |
| `RouteStreamItem<Route>` | item type for one exact route |
| `OperationStreamItem<Source>` | item type selected by operation identity |

For the broader generated type surface, see
[Generated TypeScript types](./typescript-types.md).
