# AI 응답 스트리밍

<span id="_1-server-streaming-contract-공개"></span>
<span id="_1-서버-스트리밍-명세-공개"></span>
<span id="_2-server-api-내부에서-ai-sdk-사용"></span>
<span id="_2-서버-ai-구현-연결"></span>
<span id="_3-consumer-client-생성"></span>
<span id="_3-사용-애플리케이션-클라이언트-생성"></span>
<span id="_4-consumer-typed-sse-item-사용"></span>
<span id="_4-사용-애플리케이션-타입이-지정된-sse-항목-사용"></span>
<span id="generated-client로-ai-streaming-api-사용"></span>
<span id="sse-기본-mapping"></span>
<span id="sse-기본-변환"></span>
<span id="생성된-클라이언트로-ai-스트리밍-api-사용"></span>

서버가 OpenAPI로 선언한 SSE 응답을 생성된 클라이언트로 읽습니다.
[전체 예제 ZIP](/downloads/typescript-ai-streaming-app.zip)에는 명세, 클라이언트,
빌드 설정과 로컬 실행 방법이 들어 있습니다. 모델이나 API 키 없이 실행할 수 있습니다.

## 스트림 읽기 {#consume}

예제의 `openapi.yaml`로 SDK를 생성한 뒤, `baseURL`을 서비스 주소로 지정합니다.

```ts
import { createClient } from "./generated/api/index.js";

const api = createClient({ baseURL: "https://api.example.test" });
const stream = api.$operations.generate.stream({
  body: { prompt: "Summarize the release notes." },
});

for await (const event of stream) {
  const payload = JSON.parse(event.data);
  console.log(payload.text);
}
```

내려받은 예제는 정해진 SSE 응답을 로컬에서 보내며 다음처럼 출력됩니다.

```text
Hello
world
```

실제 서비스에서는 서버가 보내는 값과 항목 수에 따라 결과가 달라집니다.

SSE의 `data`는 문자열입니다. 이 명세에서는 그 안에 JSON 텍스트가 있으므로
해석한 뒤 `text`를 사용합니다. `itemSchema`는 이벤트 구조를,
`contentSchema`는 `data` 안의 JSON 구조를 검증합니다.

## 실제 AI 서비스 연결 {#service}

모델과 공급자 설정은 서버에 둡니다. 클라이언트는 서버가 공개한 HTTP 계약만 사용합니다.
전체 예제의 `integration/server.ts`에서 AI SDK 응답을 SSE로 보내는 코드를 볼 수 있습니다.
해당 서버 코드를 사용할 때는 모델 설정과 HTTP 호스트를 별도로 연결하세요.

## 취소와 이벤트 변환 {#mapping}

요청의 `signal`·`timeoutMS` 또는 `stream.abort()`로 수신을 중단할 수 있습니다.
JSON 변환이나 이벤트별 처리를 공통화하려면
[스트림 어댑터](../reference/streaming.md#streamadapter)를 사용하세요.
프레임 구조와 수명 관리는 [스트리밍 API](../reference/streaming.md)에 설명합니다.
