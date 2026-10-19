# AI 응답 스트리밍


SSE 이벤트를 하나씩 읽고 처리합니다. [전체 예제 ZIP](/downloads/golang-ai-streaming-app.zip)에는
명세, Go 모듈, 로컬 서버와 실행 코드가 들어 있습니다. 모델이나 API 키 없이 실행할 수 있습니다.

## 스트림 읽기 {#consume}

[Generation API 명세](/examples/golang-ai-stream.json)에서 SDK를 생성하고 클라이언트를 준비합니다.
다음 코드는 함수 안에서 실행하며 `ctx`는 호출자가 제공하는 `context.Context`입니다.

```go
events := client.Operations.Generate(api.GenerateInput{
	Body: api.GenerateBody{Prompt: "Summarize the release notes."},
}).Stream(ctx)
defer events.Close()

for events.Next() {
	fmt.Println(events.Item().Data)
}
if err := events.Err(); err != nil {
	return err
}
```

내려받은 예제는 정해진 SSE 응답을 로컬에서 보내며 다음처럼 출력됩니다.

```text
hello
```

실제 서비스에서는 서버가 보내는 값과 항목 수에 따라 결과가 달라집니다.

이 명세의 SSE 이벤트는 `Data` 문자열을 반환합니다. 조기에 중단해도 스트림이 닫히도록
`defer events.Close()`를 유지하고, 반복이 끝나면 `Err()`를 확인하세요.

실제 AI 서비스에 연결하려면 클라이언트의 서버 주소를 바꾸세요. 모델과 공급자 설정은 서버에서 관리합니다.
취소, 프레임 크기 제한과 JSON 이벤트 변환은 [스트리밍 API](../reference/streaming.md)를 참고하세요.
