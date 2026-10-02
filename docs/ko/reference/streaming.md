# 스트리밍 API

생성된 순차형 미디어 API를 빠르게 찾기 위한 레퍼런스입니다. 실제 사용 흐름은
[인증·전송·스트림](../guide/transport.md)에서 설명합니다.

<span id="openapi-version-support"></span>

## OpenAPI 버전 지원

openapi-sdkgen은 OpenAPI 3.0.x, 3.1.x, 3.2.x를 읽습니다. 순차형 미디어는
각 OpenAPI 버전이 표현할 수 있는 필드에 따라 동작 범위가 달라집니다.

| OpenAPI | 전체 데이터를 모은 값 | 항목별 스트림 |
| --- | --- | --- |
| 3.0.x | 일반 미디어 타입 객체의 `schema`로 기본 순차형 콘텐츠 타입의 전체 값을 표현할 수 있습니다. | 표준 OpenAPI 필드로는 표현할 수 없습니다. |
| 3.1.x | 3.0.x와 같은 전체 값 동작을 사용하며 3.1 JSON Schema 모델을 적용합니다. | 표준 OpenAPI 필드로는 표현할 수 없습니다. |
| 3.2.x | `schema`는 계속 전체 값을 표현합니다. | `itemSchema`가 타입이 지정된 항목별 입력·출력을 활성화합니다. `prefixEncoding`, `itemEncoding`은 순서가 있는 멀티파트와 스트리밍 멀티파트 처리를 추가합니다. |

3.2 전용 필드는
[OpenAPI 미디어 타입 객체](https://spec.openapis.org/oas/v3.2.0.html#media-type-object)에
정의되어 있습니다.

기본 순차형 응답은 `schema`와 `itemSchema`가 없어도 생성되며,
전체 데이터를 모으는 호출의 반환 타입은 `unknown`입니다. 본문은 해당 미디어 코덱으로
해석하므로 프레임 구분 규칙이나 JSON 형식이 잘못되면 디코딩 오류가 발생합니다. `.stream()`은
`itemSchema`가 있을 때 제공됩니다. 순차형 미디어의 `.raw()`는 본문을
소비하지 않고 `data`에 `undefined`를 반환합니다. `schema`만 선언한 응답도 같습니다.

기본 순차형 프레임 처리는 다음 미디어를 처리합니다.

- `text/event-stream`: Server-Sent Events
- NDJSON / JSON Lines 미디어 타입
- `application/json-seq` 및 `+json-seq`
- OpenAPI 3.2 순서가 있는 멀티파트·스트리밍 멀티파트

<span id="response-stream"></span>

## 응답 스트림

응답 미디어 타입 객체에 OpenAPI 3.2 `itemSchema`가 있으면 생성된
`$operations`, `$routes`, 리소스 메서드에 `.stream(...)`이 추가됩니다.

```ts
const stream = api.$operations.watchTodos.stream({
  query: { cursor: "0" },
});
```

반환 타입은 `OperationStream<T>`입니다.

### `OperationStream`

```ts
interface OperationStream<Item> extends AsyncIterable<Item> {
  readonly response: Promise<StreamResponseMetadata>;
  abort(reason?: unknown): void;
  toReadableStream(): ReadableStream<Item>;
}
```

요청은 응답 메타데이터 또는 스트림 순회를 처음 요청할 때 시작됩니다.
하나의 `OperationStream`은 한 곳에서만 소비할 수 있습니다.
`abort()`, 웹의 `ReadableStream` 취소, 외부 `AbortSignal`, 시간 제한은
HTTP 본문을 취소합니다.

`stream.response`의 `Promise`는 응답 헤더가 도착하면 완료됩니다.

```ts
const metadata = await stream.response;
metadata.status;
metadata.headers;
metadata.contentType;
metadata.request;
```

원본 Fetch 응답 본문을 소비하지 않은 상태로 사용해야 한다면 별도의
`.raw()` 요청을 사용합니다.

<span id="streaming-request-body"></span>

## 스트리밍 요청 본문

OpenAPI 3.2 요청 본문에 `itemSchema`가 있으면 `StreamSource<T>`를
입력으로 받을 수 있습니다.

```ts
type StreamSource<T> = AsyncIterable<T> | ReadableStream<T>;
```

미디어 타입 객체에 `schema`만 있으면 전체 데이터 값을
전달합니다. `schema`와 `itemSchema`가 함께 있으면 전체 값과
`StreamSource<T>`를 모두 사용할 수 있습니다.

<span id="built-in-protocol-frame"></span>

## 기본 프로토콜 프레임

### `ServerSentEvent`

기본 SSE 파서는 표준 이벤트 필드를 반환합니다.

```ts
interface ServerSentEvent {
  readonly event?: string;
  readonly data: string;
  readonly id?: string;
  readonly retry?: number;
}
```

기본 SSE의 기본값은 응답·요청·서버 수신 모두 이벤트 객체입니다.
JSON이 들어 있어도 `data`는 문자열로 유지됩니다. 파서는 이벤트 사이에 마지막
이벤트의 `id`를 유지하고 빈 `id:` 필드가 오면 초기화합니다.

항목별 스트림은 이벤트 객체를 `itemSchema`로 선언합니다. 전체 데이터를 모으는 호출의
`schema`는 이벤트 배열로 선언합니다. `data`에 `contentMediaType: application/json`과
`contentSchema`를 지정하면 내부 JSON을 검증하면서 공개 `data` 값은 문자열로 유지합니다.

SSE 재접속과 재전송 정책은 애플리케이션에서 관리합니다.

<span id="protocol과-adapter"></span>

## 프로토콜과 어댑터

### `StreamAdapter`

`StreamAdapter`는 프로토콜 프레임을 애플리케이션 항목으로 변환하고 요청 항목을
다시 프로토콜 프레임으로 변환합니다.

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

응답과 서버의 수신 스트림에서는 어댑터 결과를 `itemSchema`로 검증하고
생성된 타입에 맞게 변환합니다. 스트리밍 요청 본문에서는 생성된 항목
값을 `itemSchema`로 인코딩한 다음 어댑터가 실행됩니다.

애플리케이션별 변환이 필요할 때 어댑터를 사용합니다. SSE에서는 JSON 데이터
변환, 이름에 따른 이벤트 분기, 종료 표시, 프레임 묶기 등이 해당합니다.
어댑터에는 `ServerSentEvent`가 전달되고, SDK는 변환 결과에 애플리케이션 항목
스키마를 적용합니다.

JSON 애플리케이션 항목을 선언한 SDK 명세는 다음 어댑터로 해당 변환을 선택합니다.

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

이 어댑터를 사용하면 SDK가 JSON 데이터를 직접 반환합니다. 표준 SSE 이벤트 스키마는
프레임 객체를 설명합니다. 이벤트 출력을 유지하면서 내부 JSON을 제한하려면
`data`의 `contentSchema`를 사용합니다.

### `StreamProtocol`

사용자 정의 순차형 미디어의 프레임 분리는 `StreamProtocol`이 담당합니다.

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

`StreamReader.read(maxBytes)`는 설정된 프레임 크기 제한을 넘을 수 없으며
`StreamReader.cancel(reason?)`로 데이터 읽기를 취소할 수 있습니다.

### `StreamCodec`

```ts
interface StreamCodec<Frame = unknown, Item = unknown> {
  readonly protocol?: StreamProtocol<Frame>;
  readonly adapter?: StreamAdapter<Frame, Item>;
}
```

코덱은 프레임 처리를 교체하거나 애플리케이션 어댑터를 추가하거나 두 작업을 함께
수행할 수 있습니다. 기본 SSE에서 `adapter`를 생략하면 이벤트 객체를
사용합니다. 어댑터는 애플리케이션 값을 변환하고 사용자 정의 프로토콜은 자체 프레임을 제공합니다.

## 설정

### `ClientOptions.streamCodecs`

한 클라이언트의 미디어 타입별 기본 설정입니다. 이벤트 객체를 그대로 사용하면
설정이 필요 없습니다. JSON 데이터를 직접 받으려면 명시적 어댑터를 등록합니다.

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

키는 정규화된 미디어 타입입니다.

### 재생성 마이그레이션

이 변경은 생성기 v9.0.0에 포함됐습니다. 새로 생성한 SSE 클라이언트는
이벤트 객체를 기본값으로 사용합니다. 기존 JSON 데이터 스키마를 유지하려면 위의
`jsonSSEAdapter`를 등록합니다. 표준 이벤트 스키마로 전환하면 `data`를 직접 파싱합니다.
요청도 이벤트 객체를 기본값으로 받고 같은 어댑터로 JSON 데이터 인코딩을 유지할 수 있습니다.

순차형 `.raw()`는 `schema`만 선언한 응답도 본문을 소비하지 않고 `data`에
`undefined`를 반환합니다. 원문은 `raw.response`로 읽고, 해석된 값이 필요하면
전체 데이터를 모으는 일반 호출을 사용합니다. 기존 생성 코드는 재생성하기 전까지 동작을 유지합니다.

### `RequestOptions.streamCodec`

한 번의 호출에서 선택된 요청 또는 응답 미디어 타입의 코덱을
대체합니다.

```ts
const stream = api.$operations.generate.stream(input, {
  streamCodec: providerCodec,
});
```

요청별 코덱이 클라이언트의 미디어 타입 기본값보다 우선합니다.

### `maxStreamFrameBytes`

`ClientOptions.maxStreamFrameBytes`는 클라이언트 기본값을 설정하고,
`RequestOptions.maxStreamFrameBytes`는 한 호출에서 이를 대체합니다.

제한은 애플리케이션 값으로 변환하기 전에 전송 프레임, 레코드, 멀티파트의 각 부분 하나에
적용됩니다. 사용자 정의 `StreamProtocol`에는 같은 값이
`StreamContext.maxFrameBytes`로 전달됩니다.

생성된 웹훅/콜백 서버 API도 수신 스트림에 같은 옵션을
제공합니다. [생성된 서버 API](./server-api.md)를 참고하세요.

<span id="타입-helper"></span>

## 제공되는 타입

생성된 진입점은 다음 타입을 제공합니다.

| 타입 | 용도 |
| --- | --- |
| `OperationStream<T>` | 필요할 때 시작되는 단일 소비자 응답 스트림 |
| `StreamResponseMetadata` | 상태 코드, 헤더, 콘텐츠 타입, 요청 메타데이터 |
| `StreamSource<T>` | 항목별 요청 데이터 공급 |
| `ServerSentEvent` | 사용자 정의 어댑터에 전달되는 기본 SSE 프로토콜의 원본 프레임 |
| `StreamReader` | 사용자 정의 프로토콜에서 제한된 크기로 데이터를 읽는 객체 |
| `StreamContext` | 콘텐츠 타입, 프레임 크기 제한, 중단 신호 |
| `StreamProtocol<Frame>` | 프레임 분리 |
| `StreamAdapter<Frame, Item>` | 애플리케이션 값으로 변환 |
| `StreamCodec<Frame, Item>` | 프로토콜/어댑터 설정 |
| `RouteStreamItem<Route>` | 정확한 경로의 항목 타입 |
| `OperationStreamItem<Source>` | API 식별자로 선택한 항목 타입 |

생성된 SDK가 제공하는 전체 타입은
[생성된 TypeScript 타입](./typescript-types.md)을 참고하세요.
