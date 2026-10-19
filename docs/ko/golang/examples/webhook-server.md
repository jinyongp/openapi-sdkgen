# 웹훅 수신


생성된 `http.Handler`를 애플리케이션의 라우터에 연결합니다.
[전체 Go Todo 예제 ZIP](/downloads/golang-todo-app.zip)에는 웹훅 요청을 보내고 응답을
확인하는 코드와 명세가 들어 있습니다. 실행 방법은 포함된 README를 따르세요.

## 처리기 연결 {#handler}

[Todo 명세](/examples/golang-todo.json)를 `--with server`로 생성하고,
생성된 패키지의 `server/webhooks`를 가져옵니다. 다음 코드는 함수 안에서
기존 `http.ServeMux`인 `mux`에 처리기를 연결합니다.

```go
handler, err := webhooks.NewCreatedPostHandler("/hooks/created", webhooks.Options{},
	func(ctx context.Context, input webhooks.CreatedPostInput) (webhooks.CreatedPostResponse, error) {
		fmt.Println(input.Body.ID)
		return webhooks.NewCreatedPost204NoContentResponse(), nil
	})
if err != nil {
	return err
}
mux.Handle("/hooks/created", handler)
```

SDK는 입력을 검증한 뒤 처리기에 전달하고 응답을 인코딩합니다.
애플리케이션은 리스너, 수신 경로와 인증 정책을 관리합니다.

이 예제의 웹훅은 익명 요청을 허용합니다. 인증이 필요한 명세에는
`Options.Authenticate`를 제공하세요. 수신 크기 제한, 콜백과 오류 처리는
[서버 API](../reference/server-api.md)에 설명합니다.
