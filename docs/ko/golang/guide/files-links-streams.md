# 파일, Link, 스트림

<span id="스트림·파일·사용자-코덱"></span>

[Todo 예제 ZIP](/downloads/golang-todo-app.zip)에서 Link와 스트림을 실행할 수 있습니다.
아래 코드는 `ctx context.Context`를 받는 함수 안에서 사용합니다.

## 응답에서 후속 API 호출

생성 응답의 Link를 따라 항목을 조회합니다. `call`은 `CreateTodo` 호출 객체입니다.

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

Todo 예제 서버를 사용하면 다음처럼 출력됩니다.

```text
201 application/json
todo-1
```

`Raw`와 `Do`는 각각 요청을 보냅니다. 한 번 받은 응답을 사용하려면 `Raw` 결과를 Link에 전달하세요.

## 스트림 읽기

```go
events := client.Operations.WatchTodos().Stream(ctx)
defer events.Close()
for events.Next() {
	fmt.Println(events.Item().Title)
}
if err := events.Err(); err != nil {
	return err
}
```

Todo 예제 서버를 사용하면 다음처럼 출력됩니다.

```text
Ship the SDK
```

첫 `Next()`에서 요청을 시작합니다. 조기에 중단할 때도 `Close()`를 호출하고,
반복이 끝나면 `Err()`를 확인하세요. `MaxStreamFrameBytes`는 전체 응답이 아닌 프레임 하나의 제한입니다.

## 파일과 사용자 정의 미디어

파일 입력은 `types.BinaryFromBytes`, `BinaryFromString`, `BinarySource`로 준비합니다.
SDK가 실행 중 연 읽기 객체를 닫습니다. 일회용 읽기 객체는 같은 요청에서 다시 사용할 수 없습니다.

Multipart 입력은 이름이 있는 필드나 순서가 정해진 파트로 선언해야 합니다.
전체 multipart 본문을 바이너리 문자열로 선언하는 방식은 지원하지 않으며,
생성 시 해당 스키마 위치를 오류로 안내합니다.

스트림 입력, 코덱과 버퍼 소유권의 세부 조건은 [스트리밍 API](../reference/streaming.md)를 참고하세요.
