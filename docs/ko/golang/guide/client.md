# API 호출과 오류

생성 패키지를 `api`로 import한 다음 클라이언트와 호출을 구성합니다.
아래 코드는 `ctx context.Context`를 사용할 수 있고 `error`를 반환하는 함수 안에서 실행합니다.

```go
client, err := api.NewClient(api.ClientOptions{BaseURL: "https://api.example.com"})
if err != nil {
	return err
}
call := client.Operations.CreateTodo(api.CreateTodoInput{
	Body: api.CreateTodoBody{Title: "Ship the SDK"},
})
todo, err := call.Do(ctx)
if err != nil {
	return err
}
fmt.Println(todo.ID, todo.Title)
```

`Operations`는 작업별 API를, `Routes`는 HTTP 메서드·경로별 진입점을 제공합니다.
리소스 메서드는 `client.Todos.ByID("todo-1").Get().Do(ctx)`처럼 경로 값을 미리 연결합니다.
클라이언트·선택자·호출을 구성할 때는 통신하지 않습니다. `Do`, `Raw`, 스트림 실행마다
자기 context를 사용합니다. 입력 데이터 소스를 다시 열 수 있으면 같은 호출을 재실행할 수 있습니다.



## Raw 응답과 Link

`Raw`에는 상태·헤더·전체 디코딩 값이 남습니다. `As201JSON()` 같은 응답 접근자는
타입이 있는 값과 성공 여부를 반환합니다. 원래 Raw 응답으로 Link를 따라가면 대상 입력을
만들고 검증합니다. 실제 요청은 이후 실행할 때 전송합니다.

```go
raw, err := call.Raw(ctx)
if err != nil {
	return err
}
fmt.Println(raw.Status, raw.Headers.Get("Content-Type"))
todo, err := client.Links.CreateTodo.GetTodo.Follow(raw).Do(ctx)
if err != nil {
	return err
}
fmt.Println(todo.ID)
```

같은 교환에서는 `Raw`와 `Do` 중 하나를 실행하세요. 두 메서드는 각각 별도 요청을 보냅니다.
타입 없는 순차형 `Raw`의 `Body`는 읽은 뒤 닫아야 합니다. 스트림은 소진·취소·명시적 닫기에서
본문과 데이터 소스를 정리합니다. 중간에 소비를 멈추면 반드시 닫으세요.

## 오류 처리

SDK 패키지 경로를 빠뜨리면 생성 오류입니다. 빠진 의존성은 사용자 모듈에서 설치해야 하며
생성기가 설치하지 않습니다. 증분 갱신에는 출력 디렉터리의 관리 매니페스트를 함께 유지하세요.
`errors.As`와 안정된 오류 코드로 `types.APIError`를 검사합니다. 오류 문자열에는 비공개 요청·응답
데이터를 넣지 않습니다.

```go
var failure *types.APIError
if errors.As(err, &failure) {
	fmt.Println(failure.Code)
}
```
