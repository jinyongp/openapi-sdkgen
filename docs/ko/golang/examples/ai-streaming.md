# AI 응답 스트리밍


생성된 클라이언트로 질문을 보내고 모델이 답변하는 대로 출력합니다.
서버는 `github.com/openai/openai-go/v3`로 OpenAI를 호출하고 텍스트 SSE를 전달합니다.
[전체 예제 ZIP](/downloads/golang-ai-streaming-app.zip)에 서버와 클라이언트의 Go 모듈,
실행 방법이 들어 있습니다. 서버에 API 키와 모델 ID를 설정해야 합니다.

## 스트림 읽기 {#consume}

[Generation API 명세](/examples/golang-ai-stream.json)에서 SDK를 생성하고 클라이언트를 준비합니다.
다음 코드는 함수 안에서 실행하며 `ctx`는 호출자가 제공하는 `context.Context`입니다.

```go
events := client.Operations.Generate(api.GenerateInput{
	Body: api.GenerateBody{Prompt: "Explain what an OpenAPI SDK does in one sentence."},
}).Stream(ctx)
defer events.Close()

for events.Next() {
	fmt.Print(events.Item().Data)
}
if err := events.Err(); err != nil {
	return err
}
fmt.Println()
```

텍스트가 도착할 때마다 이어서 출력합니다. 답변은 다음과 같은 형태입니다.

```text
An OpenAPI SDK turns your API contract into typed calls.
```

답변 내용과 텍스트가 나뉘는 단위는 모델에 따라 달라집니다.

이 명세의 SSE 이벤트는 `Data` 문자열을 반환합니다. 조기에 중단해도 스트림이 닫히도록
`defer events.Close()`를 유지하고, 반복이 끝나면 `Err()`를 확인하세요.

## AI 서버 실행 {#service}

`OPENAI_API_KEY`와 `OPENAI_MODEL`을 설정하고 예제의 `server/` 디렉터리에서
`go run .`을 실행합니다. 다른 터미널에서는 예제 루트에서 `go run .`으로 질문을 보냅니다.
SDK 생성, 의존성 설치와 공급자 주소 설정은 예제 README에 있습니다.
서버는 공식 OpenAI 라이브러리를 사용하는 별도 Go 모듈입니다.
클라이언트 모듈에는 외부 라이브러리가 필요하지 않고 공급자 인증 정보도 전달되지 않습니다.

실행 예제는 Ctrl+C로 취소할 수 있고 제한 시간은 2분입니다. 클라이언트 연결이 끊기면
서버도 모델 요청을 취소합니다. 모델 호출에 실패하면 텍스트 전송 전에는 HTTP 502를 반환하고,
전송을 시작한 뒤에는 응답 연결을 끊어 오류를 알립니다.

예제 서버는 로컬에서만 접속할 수 있습니다. 외부에 공개하려면 애플리케이션 인증을 추가하세요.
모델 호출에는 공급자 요금이 발생할 수 있습니다.
프레임 크기 제한과 이벤트 변환은 [스트리밍 API](../reference/streaming.md)를 참고하세요.
