# Streaming API

Code on this page illustrates API shapes. Operation names, parameters, media,
and security come from your own contract. The small Todo contract in
[Getting started](../guide/getting-started.md) does not include every feature below.


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

A built-in sequential response without `schema` or `itemSchema` still generates
a buffered call returning `unknown`. The built-in media codec decodes the body;
malformed framing or JSON remains a decode error. `.stream()` requires
`itemSchema`. For sequential media, `.raw()` leaves the response body unconsumed
and returns `undefined` as `data`, including schema-only responses.

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
metadata.headers;
metadata.contentType;
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
  readonly event?: string;
  readonly data: string;
  readonly id?: string;
  readonly retry?: number;
}
```

Built-in SSE uses this Event object as its default application value for
responses, requests, and inbound server streams. `data` remains a string,
including when it contains JSON. The parser preserves last-event-id state and
resets it when the wire stream sends an empty `id:` field.

Declare `itemSchema` as an Event object for incremental calls. For buffered
calls, declare `schema` as an array of Event objects. A `data` property with
`contentMediaType: application/json` and `contentSchema` validates the embedded
JSON without changing the public `data` string.

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

Use an adapter for application-specific mapping, such as JSON payloads,
named-event routing, terminal markers, or frame aggregation. An SSE adapter
receives `ServerSentEvent` frames. Its output is the value the SDK validates
against the configured application item schema.

For an SDK contract that describes JSON application items, opt into that mapping:

```ts
import type { ServerSentEvent, StreamAdapter } from "./generated/api";

const jsonSSEAdapter: StreamAdapter<ServerSentEvent, unknown> = {
  async *decode(frames) {
    for await (const frame of frames) yield JSON.parse(frame.data);
  },
  async *encode(items) {
    for await (const item of items) {
      const data = JSON.stringify(item);
      if (data === undefined) throw new TypeError("Item must be JSON-serializable");
      yield { data };
    }
  },
};
```

This compatibility adapter changes the SDK application value. A normative SSE
Event schema describes the frame object; use `contentSchema` on `data` to
constrain embedded JSON while retaining Event output.

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

A codec may replace framing, add an application adapter, or do both. For
built-in SSE, omitting `adapter` returns Event objects. Providing an adapter
replaces that application mapping. A custom protocol supplies its own frames.

## Configuration

### ClientOptions.streamCodecs

Set media-type defaults for every operation on one client. Event objects need no
entry. Configure an adapter when the application consumes JSON payloads directly:

```ts
const api = createClient({
  baseURL,
  streamCodecs: {
    "text/event-stream": {
      adapter: jsonSSEAdapter,
    },
  },
});
```

Keys are normalized media types.

<span id="regeneration-migration"></span>

### SSE values and raw responses

SSE clients use Event objects by default. To consume JSON payloads directly,
configure `jsonSSEAdapter` above or parse the Event object's `data` explicitly.
Request callers send Event objects by default; the same adapter encodes JSON payloads.

Sequential `.raw()` calls, including schema-only responses, leave the body
unconsumed and expose `undefined` in `data`. Read `raw.response` for bytes, or use
the ordinary buffered call for the decoded value.

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
| `ServerSentEvent` | raw built-in SSE frame exposed to custom adapters |
| `StreamReader` | bounded reader for custom protocols |
| `StreamContext` | content type, frame limit, abort signal |
| `StreamProtocol<Frame>` | byte framing |
| `StreamAdapter<Frame, Item>` | application semantic mapping |
| `StreamCodec<Frame, Item>` | protocol/adapter configuration |
| `RouteStreamItem<Route>` | item type for one exact route |
| `OperationStreamItem<Source>` | item type selected by operation identity |

For the broader generated type surface, see
[Generated TypeScript types](./typescript-types.md).
