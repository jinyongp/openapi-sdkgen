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

## 생성된 소스의 의존성 설치

```sh
go get github.com/andybalholm/brotli@v1.2.6 \
  github.com/klauspost/compress@v1.20.3 \
  github.com/nlnwa/whatwg-url@v0.6.2 \
  github.com/go-quickjs/go-quickjs@v0.27.0
go mod tidy
```

이 예제는 외부 모듈 4개를 직접 사용합니다. 별도의 SDK 런타임 모듈은 설치하지 않지만,
생성된 Go 소스에 외부 의존성이 없는 것은 아닙니다.

| 기능 | Go 소스가 사용하는 모듈 | TypeScript에서 사용하는 기능 |
| --- | --- | --- |
| Brotli 응답 압축 해제 | `andybalholm/brotli` | 실행 환경의 Fetch 응답 압축 해제 |
| Zstandard 응답 압축 해제 | `klauspost/compress/zstd` | 실행 환경이 지원하는 Fetch 응답 압축 해제 |
| URL 해석·정규화 | `nlnwa/whatwg-url` | 내장 `URL` |
| JavaScript 정규식과 호환되는 검증 | `go-quickjs/go-quickjs/jsregexp` | 내장 `RegExp` |

일반 클라이언트를 생성해도 현재 전송 코드는 압축 해제와 URL 모듈을 import합니다.
정규식 모듈은 선택한 스키마의 정규식 검증 또는 `server` 확장의 검증 코드가 필요할 때 포함됩니다.
Go 표준 라이브러리의 `net/url`·`regexp`는 여기서 보장하는 URL·정규식 동작과 같지 않습니다.

`whatwg-url`이 사용하는 `bitset`, `golang.org/x/net`, `golang.org/x/text`는 간접 의존성입니다.
Go가 함께 설치하므로 설치 명령에 따로 나열할 필요는 없습니다. `go mod tidy`는 생성된 코드에서
사용하지 않는 모듈을 제거합니다. TypeScript 생성 소스에는 별도 npm 런타임 의존성이 없으며,
대응 기능은 JavaScript 실행 환경이 제공합니다.

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
