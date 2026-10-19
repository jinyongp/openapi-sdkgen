# 첫 Go SDK 만들기

<span id="go-sdk-생성과-사용"></span>
<span id="raw-응답과-link"></span>
<span id="문제-해결"></span>
<span id="보안과-전송"></span>
<span id="사용자-모듈에-생성하기"></span>
<span id="선택-방식과-컴파일-비용"></span>
<span id="스트림·파일·사용자-코덱"></span>
<span id="웹훅과-콜백"></span>
<span id="타입이-있는-호출과-값의-존재-여부"></span>
<span id="생성된-소스의-의존성-설치"></span>

OpenAPI 명세에서 Go 소스를 생성하고 모듈에서 불러옵니다. 생성된 코드를 빌드하려면
Go 1.27 이상이 필요합니다. 실행 가능한 전체 코드는 [Go Todo 예제 ZIP](/downloads/golang-todo-app.zip)에 있습니다.

## CLI 설치 {#install}

```sh
brew install jinyongp/tap/openapi-sdkgen
```

npm으로 설치하려면 `npm install -g openapi-sdkgen`을 사용하세요.
[실행 파일](https://github.com/jinyongp/openapi-sdkgen/releases)을 직접 받아도 됩니다.

## SDK 생성 {#generate}

기존 Go 모듈에서 [Todo 명세](/examples/golang-todo.json)를 `openapi.json`으로 저장합니다.
이 명세에는 웹훅이 있어 `--with server`를 함께 지정합니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.json \
  --target golang \
  --output ./internal/api \
  --with server \
  --incremental
```

출력 위치에서 가장 가까운 `go.mod`를 찾아 import 경로를 자동으로 정합니다.
예를 들어 모듈이 `example.com/consumer`라면 `example.com/consumer/internal/api`가 됩니다.
명세가 바뀌면 같은 명령으로 갱신할 수 있습니다.

## 애플리케이션에서 호출 {#call}

생성된 패키지를 `api "example.com/consumer/internal/api"`로 가져옵니다.
다음은 함수 안에서 요청을 보내는 코드입니다. `ctx`는 호출자가 제공하는 `context.Context`이며,
`baseURL`에는 실제 API 서버 주소를 지정하세요.

```go
client, err := api.NewClient(api.ClientOptions{BaseURL: "http://localhost:8080"})
if err != nil {
	return err
}

todo, err := client.Operations.CreateTodo(api.CreateTodoInput{
	Body: api.CreateTodoBody{Title: "Ship the SDK"},
}).Do(ctx)
if err != nil {
	return err
}
fmt.Printf("Created %s: %s\n", todo.ID, todo.Title)
```

Todo 예제 서버를 사용하면 다음처럼 출력됩니다.

```text
Created todo-1: Ship the SDK
```

`todo`에서 생성된 항목의 `ID`, `Title`을 읽을 수 있습니다.
서버를 준비하지 않고 실행해보려면 위의 예제 ZIP을 사용하세요.
로컬 서버, 모듈 설정, 생성 명령과 실행 방법이 포함되어 있습니다.

[호출과 오류](./client.md), [모델](./models.md), [재생성](./generate.md)을 이어서 확인하세요.
