# 모델과 검증

아래 예제는 [첫 Go SDK](./getting-started.md)를 이어서 사용합니다. 생성 루트의
`example.com/consumer/internal/api/types`를 `types`로 import하세요. `api`와 `client`는 그 SDK를 가리킵니다.

<span id="타입이-있는-호출과-값의-존재-여부"></span>


필수 멤버는 생성 타입을 직접 사용합니다. 선택 멤버의 `types.Optional[T]`에서
`types.Some(value)`는 값이 있음을, 제로 값은 부재를 뜻합니다. null을 허용하는 값은
`types.Nullable[T]`로 표현하므로 null과 부재를 구분합니다. `types.Value`는 불변 JSON
트리이며 파일이나 실행 중인 스트림을 담지 않습니다. 입력은 HTTP 전에 검사하고 응답에서는
보존 대상인 미선언 속성도 유지합니다. 독립 모델 코덱과 서버 응답 인코딩은 해당 계약을 검사합니다.

```go
input := api.CreateTodoInput{Body: api.CreateTodoBody{Title: "Ship the SDK"}}
call := client.Operations.CreateTodo(input)
call = call.WithOptions(types.RequestOptions{RequestID: types.Some("example")})
```

이 코드는 요청 ID가 `example`인 호출을 준비합니다. 아직 HTTP 요청을 보내거나 Todo를 반환하지 않습니다. `call.Do(ctx)`를 실행했을 때의 반환값은 [API 호출](./client.md)에서 확인할 수 있습니다.

코덱·합성 타입·열거값·컴포넌트 투영은 [타입 참조](../reference/types.md)를 확인하세요.
