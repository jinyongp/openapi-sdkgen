# 웹훅과 콜백



`--with server`로 수신 처리기를 생성합니다. 애플리케이션의 라우터와 리스너에 연결해 사용하세요.
[Go Todo 예제 ZIP](/downloads/golang-todo-app.zip)에서 전체 실행 코드를 볼 수 있습니다.

## 처리기 등록

생성된 `server/webhooks`를 가져온 뒤 기존 `http.ServeMux`에 연결합니다.

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

JSON 본문 `{"id":"todo-1","title":"Ship the SDK"}`를 받으면 핸들러는 `todo-1`을 출력하고 본문 없이 `204 No Content`로 응답합니다. 입력이 명세에 맞지 않으면 이 출력 전에 요청을 거절합니다.

## 인증과 요청 제한

인증이 필요한 명세에는 `Options.Authenticate`를 제공합니다. 인증 함수를 제공하지 않으면
해당 요청은 `401`로 거부됩니다. 익명 요청을 허용한 작업에서는 이 함수를 호출하지 않습니다.

전체 본문은 기본 8 MiB, 증분 프레임은 기본 1 MiB로 제한합니다.
`MaxBodyBytes`와 `MaxStreamFrameBytes`로 바꿀 수 있습니다.

콜백 수신, 동시 송수신, 오류 처리와 동적 스키마 검증은 [서버 API](../reference/server-api.md)를 참고하세요.
