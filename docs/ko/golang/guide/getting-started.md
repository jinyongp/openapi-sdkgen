# 첫 Go SDK 만들기

<span id="go-sdk-생성과-사용"></span>
<span id="사용자-모듈에-생성하기"></span>


실행 파일로 소스를 생성한 뒤 **Go 1.27 이상**인 사용자 모듈에서 컴파일하고 실행합니다.
Homebrew·npm으로 CLI를 설치하거나 [Releases](https://github.com/jinyongp/openapi-sdkgen/releases)에서
실행 파일을 내려받으세요.

```sh
brew install jinyongp/tap/openapi-sdkgen
# Alternative: npm install -g openapi-sdkgen
openapi-sdkgen --version
go mod init example.com/consumer
```

[Todo 명세](/examples/golang-todo.json)를 `openapi.json`으로 저장하세요.
웹훅이 포함된 명세이므로 `server` 확장을 켭니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.json \
  --target golang \
  --go-import-path example.com/consumer/internal/api \
  --output ./internal/api \
  --with server \
  --incremental
```

`--go-import-path`는 모듈과 출력 디렉터리에 맞춰 지정해야 합니다. 루트 패키지 기본 이름은
`api`입니다. 생성기는 `go.mod`를 쓰지 않으며 의존성은 애플리케이션이 관리합니다.
실행 파일로 생성할 때는 Go·Node.js가 필요하지 않습니다. npm 실행기는 Node.js가 필요합니다.

<span id="생성된-소스의-의존성-설치"></span>

## 로컬 모의 서버에 첫 요청 보내기

아래 코드를 `main.go`로 저장하세요. 로컬 서버에서 타입이 있는 호출을 한 번 실행하고 서버를 닫습니다.

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "net/http/httptest"
    "time"

    api "example.com/consumer/internal/api"
)

func main() {
    if err := run(); err != nil {
        panic(err)
    }
}

func run() error {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        fmt.Fprint(w, `{"id":"todo-1","title":"Ship the SDK"}`)
    }))
    defer server.Close()
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    client, err := api.NewClient(api.ClientOptions{BaseURL: server.URL})
    if err != nil {
        return err
    }
    todo, err := client.Operations.CreateTodo(api.CreateTodoInput{
        Body: api.CreateTodoBody{Title: "Ship the SDK"},
    }).Do(ctx)
    if err != nil {
        return err
    }
    fmt.Println(todo.ID, todo.Title)
    return nil
}
```

```sh
go run .
```

```text
todo-1 Ship the SDK
```

[호출과 오류](./client.md), [모델](./models.md), [재생성](./generate.md)을 이어서 확인하세요.
[전체 Go 예제](../examples/index.md)에는 스트림과 웹훅도 포함돼 있습니다.
