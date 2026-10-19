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

생성된 클라이언트로 질문을 보내고 모델이 답변하는 대로 출력합니다.
서버는 `ai`와 `@ai-sdk/openai`로 OpenAI를 호출하고 응답을 SSE로 전달합니다.
[전체 예제 ZIP](/downloads/typescript-ai-streaming-app.zip)에 서버, 클라이언트와
실행 방법이 들어 있습니다. 서버에 API 키와 모델 ID를 설정해야 합니다.

## 스트림 읽기 {#consume}

내려받은 예제의 서버를 실행한 뒤 호출합니다.

```ts
import { createClient } from "./generated/api/index.js";

const api = createClient({ baseURL: "http://127.0.0.1:18793" });
const stream = api.$operations.generate.stream({
  body: { prompt: "Explain what an OpenAPI SDK does in one sentence." },
});

for await (const event of stream) {
  const payload = JSON.parse(event.data);
  process.stdout.write(payload.text);
}
process.stdout.write("\n");
```

텍스트가 도착할 때마다 이어서 출력합니다. 답변은 다음과 같은 형태입니다.

```text
An OpenAPI SDK turns your API contract into typed calls.
```

답변 내용과 텍스트가 나뉘는 단위는 모델에 따라 달라집니다.

SSE의 `data`는 문자열입니다. 이 명세에서는 그 안에 JSON 텍스트가 있으므로
해석한 뒤 `text`를 사용합니다. `itemSchema`는 이벤트 구조를,
`contentSchema`는 `data` 안의 JSON 구조를 검증합니다.

## 실제 AI 서비스 연결 {#service}

`OPENAI_API_KEY`와 `OPENAI_MODEL`을 설정하고 `pnpm server`를 실행한 다음,
다른 터미널에서 `pnpm start`로 질문을 보냅니다. 설치 방법과 호환 공급자 주소 설정은
예제 README에 있습니다. 서버의 `src/server.ts`는 `provider.chat(modelID)`를 지정해
`streamText`를 호출하고, 텍스트 조각을 JSON SSE 이벤트로 전달합니다.
클라이언트는 이 HTTP 스트림을 받으며 모델 공급자를 직접 호출하거나 API 키를 전달받지 않습니다.

예제 서버는 로컬에서만 접속할 수 있습니다. 외부에 공개하려면 애플리케이션 인증을 추가하세요.
모델 호출에는 공급자 요금이 발생할 수 있습니다.

## 취소와 이벤트 변환 {#mapping}

요청의 `signal`·`timeoutMS` 또는 `stream.abort()`로 수신을 중단할 수 있습니다.
실행 예제는 Ctrl+C로 취소할 수 있고 제한 시간은 2분입니다. 클라이언트 연결이 끊기면
서버도 모델 요청을 취소합니다. 모델 호출에 실패하면 텍스트 전송 전에는 HTTP 502를 반환하고,
전송을 시작한 뒤에는 응답 연결을 끊어 오류를 알립니다.
JSON 변환이나 이벤트별 처리를 공통화하려면
[스트림 어댑터](../reference/streaming.md#streamadapter)를 사용하세요.
프레임 구조와 수명 관리는 [스트리밍 API](../reference/streaming.md)에 설명합니다.
