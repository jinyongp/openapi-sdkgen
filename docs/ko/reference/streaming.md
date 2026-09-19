# 스트리밍 API

생성된 sequential-media API를 빠르게 찾기 위한 레퍼런스입니다. 실제 사용 흐름은
[인증·전송·스트림](../guide/transport.md)에서 설명합니다.

<span id="openapi-version-support"></span>

## OpenAPI 버전 지원

openapi-sdkgen은 OpenAPI 3.0.x, 3.1.x, 3.2.x를 읽습니다. Sequential media는
각 OpenAPI 버전이 표현할 수 있는 필드에 따라 동작 범위가 달라집니다.

| OpenAPI | Complete sequential value | Incremental stream |
| --- | --- | --- |
| 3.0.x | 일반 Media Type Object의 `schema`로 built-in sequential content type의 complete value를 표현할 수 있습니다. | 표준 OpenAPI 필드로는 표현할 수 없습니다. |
| 3.1.x | 3.0.x와 같은 complete-value 동작을 사용하며 3.1 JSON Schema 모델을 적용합니다. | 표준 OpenAPI 필드로는 표현할 수 없습니다. |
| 3.2.x | `schema`는 계속 complete value를 표현합니다. | `itemSchema`가 typed incremental input/output을 활성화합니다. `prefixEncoding`, `itemEncoding`은 positional/streaming multipart semantics를 추가합니다. |

3.2 전용 필드는
[OpenAPI Media Type Object](https://spec.openapis.org/oas/v3.2.0.html#media-type-object)에
정의되어 있습니다.

Built-in sequential framing은 다음 media를 처리합니다.

- `text/event-stream`: Server-Sent Events
- NDJSON / JSON Lines media type
- `application/json-seq` 및 `+json-seq`
- OpenAPI 3.2 positional/streaming multipart

## Response stream

Response Media Type Object에 OpenAPI 3.2 `itemSchema`가 있으면 생성된
operation, exact route, resource method에 `.stream(...)`이 추가됩니다.

```ts
const stream = api.$operations.watchTodos.stream({
  query: { cursor: "0" },
});
```

반환 타입은 `OperationStream<T>`입니다.

### OperationStream

```ts
interface OperationStream<Item> extends AsyncIterable<Item> {
  readonly response: Promise<StreamResponseMetadata>;
  abort(reason?: unknown): void;
  toReadableStream(): ReadableStream<Item>;
}
```

요청은 response metadata 또는 stream 순회를 처음 요청할 때 시작됩니다.
하나의 `OperationStream`에는 하나의 consumer만 사용할 수 있습니다.
`abort()`, Web `ReadableStream` cancel, 외부 `AbortSignal`, timeout은
underlying HTTP body를 취소합니다.

`stream.response`는 response header가 도착하면 resolve됩니다.

```ts
const metadata = await stream.response;
metadata.status;
metadata.contentType;
metadata.headers;
metadata.request;
```

원본 Fetch response body를 소비하지 않은 상태로 사용해야 한다면 별도의
`.raw()` 요청을 사용합니다.

## Streaming request body

OpenAPI 3.2 request body에 `itemSchema`가 있으면 `StreamSource<T>`를
입력으로 받을 수 있습니다.

```ts
type StreamSource<T> = AsyncIterable<T> | ReadableStream<T>;
```

Media Type Object에 `schema`만 있으면 complete application value를
전달합니다. `schema`와 `itemSchema`가 함께 있으면 complete value와
`StreamSource<T>`를 모두 사용할 수 있습니다.

## Built-in protocol frame

### ServerSentEvent

Built-in SSE parser는 표준 event-stream 필드를 반환합니다.

```ts
interface ServerSentEvent {
  readonly data: string;
  readonly event?: string;
  readonly id?: string;
  readonly retry?: number;
}
```

`data`는 protocol frame 단계에서 문자열로 유지됩니다. Built-in SSE를
custom adapter나 protocol 없이 사용하면 openapi-sdkgen이 일반적인
JSON-in-`data` mapping을 기본으로 적용합니다. Response와 inbound stream에서는
`event.data`를 JSON으로 parsing하고, request item은 JSON 문자열을 SSE
`data`에 넣습니다. Parser는 SSE의 last-event-id state를 event 사이에
유지하고 wire stream의 빈 `id:` 필드에서 reset합니다.

Generated client는 SSE reconnect 또는 replay를 자동 수행하지 않습니다.

## Protocol과 adapter

### StreamAdapter

`StreamAdapter`는 protocol frame을 application item으로 변환하고 request item을
다시 protocol frame으로 변환합니다.

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

Response와 inbound server stream에서는 adapter 결과가 선언된 `itemSchema`
validation과 projection을 거칩니다. Streaming request body에서는 generated item
value를 `itemSchema`로 encode한 다음 adapter가 실행됩니다.

Built-in application mapping으로 처리할 수 없는 semantics가 있을 때 adapter를
사용합니다. SSE에서는 non-JSON `data`, named event routing, terminal marker,
frame aggregation 같은 경우가 해당합니다. Explicit adapter에는 raw
`ServerSentEvent` frame이 전달되며 기본 JSON mapping을 대체합니다.

### StreamProtocol

사용자 정의 sequential media의 byte framing은 `StreamProtocol`이 담당합니다.

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

`StreamReader.read(maxBytes)`는 설정된 frame limit를 넘을 수 없으며
`StreamReader.cancel(reason?)`로 source를 취소할 수 있습니다.

### StreamCodec

```ts
interface StreamCodec<Frame = unknown, Item = unknown> {
  readonly protocol?: StreamProtocol<Frame>;
  readonly adapter?: StreamAdapter<Frame, Item>;
}
```

Codec은 framing을 교체하거나 application adapter를 추가하거나 두 작업을 함께
수행할 수 있습니다. Built-in SSE에서 `adapter`를 생략하면 기본
JSON-in-`data` mapping을 사용합니다. Adapter를 지정하면 이 기본 mapping을
대체합니다. Custom protocol을 지정한 경우에는 SSE JSON adapter가 암묵적으로
적용되지 않습니다.

## 설정

### ClientOptions.streamCodecs

한 client의 media type별 기본 설정입니다. 일반적인 JSON SSE에는
`streamCodecs` 설정이 필요하지 않으며, application-specific semantics가 있을
때만 설정합니다.

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

Key는 normalized media type입니다.

### RequestOptions.streamCodec

한 번의 호출에서 선택된 request 또는 response media type의 codec을
override합니다.

```ts
const stream = api.$operations.generate.stream(input, {
  streamCodec: providerCodec,
});
```

Request-level codec이 client의 media-type 기본값보다 우선합니다.

### maxStreamFrameBytes

`ClientOptions.maxStreamFrameBytes`는 client 기본값을 설정하고,
`RequestOptions.maxStreamFrameBytes`는 한 호출에서 이를 override합니다.

제한은 application adaptation 전에 wire frame, record, multipart part 하나에
적용됩니다. 사용자 정의 `StreamProtocol`에는 같은 값이
`StreamContext.maxFrameBytes`로 전달됩니다.

Generated Webhook/Callback server API도 inbound stream에 같은 옵션을
제공합니다. [생성된 서버 API](./server-api.md)를 참고하세요.

## 타입 helper

생성된 진입점은 다음 타입을 제공합니다.

| 타입 | 용도 |
| --- | --- |
| `OperationStream<T>` | lazy 단일 소비자 response stream |
| `StreamResponseMetadata` | status, header, content type, request metadata |
| `StreamSource<T>` | incremental request source |
| `ServerSentEvent` | custom adapter에 전달되는 raw built-in SSE frame |
| `StreamReader` | 사용자 정의 protocol용 bounded reader |
| `StreamContext` | content type, frame limit, abort signal |
| `StreamProtocol<Frame>` | byte framing |
| `StreamAdapter<Frame, Item>` | application semantic mapping |
| `StreamCodec<Frame, Item>` | protocol/adapter 설정 |
| `RouteStreamItem<Route>` | exact route의 item 타입 |
| `OperationStreamItem<Source>` | operation identity로 선택한 item 타입 |

전체 generated type surface는
[생성된 TypeScript 타입](./typescript-types.md)을 참고하세요.
