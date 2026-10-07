<span id="generated-client로-ai-streaming-api-사용"></span>

# 생성된 클라이언트로 AI 스트리밍 API 사용

이 예제는 AI 서비스를 구현하는 서버와 생성된 SDK를 사용하는 애플리케이션을
서로 다른 코드베이스로 나눕니다.

```text
ai-service/
  openapi.yaml
  src/model.ts
  src/server.ts

sdk-consumer/
  src/generated/api/
  src/client.ts
```

**서버**가 AI SDK와 모델 제공자를 소유합니다. OpenAPI 3.2로 설명된 일반
HTTP/SSE API를 공개합니다. **사용 애플리케이션**은 그 명세에서 클라이언트를 생성하며
AI SDK 패키지에 의존하지 않습니다.

| 코드베이스 | 소유 범위 | 주요 의존성 |
| --- | --- | --- |
| `ai-service` | 모델·제공자 설정, HTTP 엔드포인트, OpenAPI 명세 | `ai`, 모델 제공자 패키지, 서버 프레임워크·실행 환경 |
| `sdk-consumer` | 생성된 SDK, 애플리케이션 동작 | openapi-sdkgen 출력물. AI SDK와 모델 제공자 의존성 없음 |

<span id="_1-server-streaming-contract-공개"></span>

## 1. 서버: 스트리밍 명세 공개

서버 저장소가 OpenAPI 문서를 소유합니다.

```yaml
openapi: 3.2.0
info:
  title: Generation API
  version: 1.0.0

paths:
  /generate:
    post:
      operationId: generate
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [prompt]
              properties:
                prompt:
                  type: string
      responses:
        "200":
          description: Generation events
          content:
            text/event-stream:
              itemSchema:
                type: object
                required: [data]
                properties:
                  data:
                    type: string
                    contentMediaType: application/json
                    contentSchema:
                      type: object
                      required: [type, text]
                      properties:
                        type:
                          const: text-delta
                        text:
                          type: string
```

`itemSchema`는 표준 SSE 이벤트 객체를 설명합니다. 문자열 `data`에 들어 있는
새 텍스트 조각을 나타내는 `text-delta` 데이터는 클라이언트가 이벤트를 반환하기 전에
`contentSchema`로 검증합니다.

<span id="_2-server-api-내부에서-ai-sdk-사용"></span>

## 2. 서버: AI 구현 연결

서버는 AI SDK를 내부 구현에 사용할 수 있으며 SDK 사용 애플리케이션에 해당 의존성을
노출하지 않습니다. `src/model.ts`는 애플리케이션이 관리하는 모델 제공자
설정입니다.

이 절은 서버 연동 코드입니다. `ai`와 모델 제공자 패키지를 설치하고
`src/model.ts`에서 설정한 `model`을 내보내야 합니다. HTTP 서버에서 `/generate`의
`POST` 요청을 `handleGenerate`로 연결하세요. 클라이언트만 확인할 때는 4절의 모의 응답을 사용합니다.

ESM 프로젝트(`"type": "module"`)를 사용합니다. Node 서버에는 `@types/node`,
AI SDK 타입 의존성에는 `@types/json-schema`가 필요할 수 있습니다. 인증 정보와
모델 설정은 선택한 제공자의 설치 안내를 따르세요.

```ts
// ai-service/src/server.ts
import { streamText } from "ai";
import { model } from "./model.js";

export async function handleGenerate(request: Request) {
  let input: unknown;
  try {
    input = await request.json();
  } catch {
    return new Response("Invalid JSON", { status: 400 });
  }
  if (typeof input !== "object" || input === null ||
      !("prompt" in input) || typeof input.prompt !== "string") {
    return new Response("Expected a prompt string", { status: 400 });
  }
  const result = streamText({
    model, prompt: input.prompt, abortSignal: request.signal,
    onError: ({ error }) => { console.error(error); },
  });
  const encoder = new TextEncoder();

  const body = new ReadableStream<Uint8Array>({
    async start(controller) {
      try {
        for await (const text of result.textStream) {
          const event = { type: "text-delta", text };
          controller.enqueue(
            encoder.encode(`data: ${JSON.stringify(event)}\n\n`),
          );
        }
        controller.close();
      } catch (error: unknown) {
        controller.error(error);
      }
    },
  });

  return new Response(body, {
    headers: {
      "content-type": "text/event-stream",
      "cache-control": "no-cache",
    },
  });
}
```

AI SDK의 `streamText()`는 새로 생성된 텍스트 조각을 `textStream`으로 제공합니다.
비동기 반복 가능한 객체와 웹 스트림으로 사용할 수 있습니다. 이 예제는 텍스트
이벤트만 공개합니다. 도구 호출, 출처, 추론 같은 이벤트도 필요하면 서버에서
추가 이벤트 형식을 정의하고 `itemSchema`에 선언하세요. 모델 제공자의 전송 형식과
별개로 애플리케이션에서 사용할 공개 이벤트 형식을 정할 수 있습니다.

서버에서 사용하는 스트리밍 API는 AI SDK
[`streamText` 레퍼런스](https://ai-sdk.dev/docs/reference/ai-sdk-core/stream-text)를
참고하세요.

<span id="_3-consumer-client-생성"></span>

## 3. 사용 애플리케이션: 클라이언트 생성

사용 애플리케이션 저장소는 OpenAPI 명세와 openapi-sdkgen만 필요합니다.

별도 소비자 프로젝트에서 [시작하기](../guide/getting-started.md)의 설치·ESM
설정을 준비하고 1절의 명세를 `openapi.yaml`로 저장하세요. 기존 출력이 있으면
생성 명령에 `--incremental`을 추가합니다.

```sh
pnpm exec openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

생성된 클라이언트는 `generate` API의 각 이벤트에 응답 타입을 적용합니다.
`data`를 문자열로 유지하면서 선언한 내부 JSON 구조를 검증합니다.

<span id="_4-consumer-typed-sse-item-사용"></span>

## 4. 사용 애플리케이션: 타입이 지정된 SSE 항목 사용

```ts
// sdk-consumer/src/client.ts
import { createClient } from "./generated/api/index.js";

interface TextDelta {
  readonly type: "text-delta";
  readonly text: string;
}

const api = createClient({
  baseURL: "https://api.example.test",
  fetch: async () => new Response(
    'data: {"type":"text-delta","text":"Hello"}\n\n' +
    'data: {"type":"text-delta","text":"world"}\n\n',
    { headers: { "content-type": "text/event-stream" } },
  ),
});

const stream = api.$operations.generate.stream({
  body: {
    prompt: "Summarize the release notes.",
  },
});

for await (const event of stream) {
  const payload = JSON.parse(event.data) as TextDelta;
  console.log(payload.text);
}
```

프로젝트의 기존 빌드 도구로 실행하거나, 설치된 TypeScript 컴파일러로 아래와 같이
확인할 수 있습니다. SDK 생성에는 컴파일러 설치가 필요하지 않습니다.

```sh
pnpm exec tsc --strict --target ES2022 --module NodeNext --moduleResolution NodeNext \
  --lib ES2022,DOM,DOM.Iterable --outDir dist src/client.ts
node dist/client.js
```

출력은 두 줄의 `Hello`, `world`입니다. 모의 SSE 응답으로 생성·타입 검사·스트림
소비를 확인하며 AI 제공자를 호출하지 않습니다. 실제 서비스를 호출하려면 `fetch`를
제거하고 `baseURL`을 서버 주소로 바꾸세요. 서버 프레임워크와 모델 설정은 별도로 필요합니다.



<span id="sse-기본-mapping"></span>

## SSE 기본 변환

기본 SSE 프로토콜은 표준 `data`, `event`, `id`, `retry` 필드를 반환합니다.
`itemSchema`는 이벤트 객체를 검증합니다. 이 예제의 `contentSchema`는 `data`의
내부 JSON도 검증하며 공개 `data` 값은 문자열로 유지됩니다.

JSON 데이터 직접 반환, 이름에 따른 이벤트 분기, 종료 표시, 프레임 묶기처럼
애플리케이션별 변환이 필요하면 `StreamAdapter<ServerSentEvent, Item>`를 사용합니다.
SDK는 어댑터 결과에 애플리케이션 스키마를 적용합니다. 스트리밍 레퍼런스에서
명시적 JSON 호환 어댑터 예제를 확인할 수 있습니다.


프로토콜/어댑터 계약은 [스트리밍 API](../reference/streaming.md), 버전별 기능은
[OpenAPI 지원 범위](../reference/capabilities.md)를 참고하세요.
