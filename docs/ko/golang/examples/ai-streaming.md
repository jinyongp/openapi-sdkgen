# SSE로 AI 스트림 받기

서버가 모델 제공자의 의존성을 관리하고 HTTP 계약을 공개합니다. Go 소비자는 생성된 SDK와
Go 표준 라이브러리만 사용하며 모델 제공자를 import하지 않습니다.
[Generation API 명세](/examples/golang-ai-stream.json)를 `openapi.json`으로 저장하고
[모듈 설정](../guide/getting-started.md)을 한 뒤 생성하세요.

```sh
openapi-sdkgen generate --input ./openapi.json --target golang \
  --go-import-path example.com/consumer/internal/api --output ./internal/api
```

아래 전체 `main.go`는 로컬 SSE 모의 서버를 사용합니다. 실제 provider를 연결할 때는 처리기를
서버 쪽 AI 구현으로 바꾸세요. 선언된 SSE 항목에는 필수 문자열 `data`가 있습니다.

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
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: hello\n\n")
		w.(http.Flusher).Flush()
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := api.NewClient(api.ClientOptions{BaseURL: server.URL})
	if err != nil {
		return err
	}
	events := client.Operations.Generate(api.GenerateInput{
		Body: api.GenerateBody{Prompt: "Say hello"},
	}).Stream(ctx)
	defer events.Close()
	for events.Next() {
		fmt.Println(events.Item().Data)
	}
	if err := events.Err(); err != nil {
		return err
	}
	return nil
}
```

```sh
go run .
```

```text
hello
```

중간에 소비를 멈출 때도 `defer events.Close()`를 유지하고 순회 뒤 `Err()`를 확인하세요.
context 취소는 교환을 중단합니다. `MaxStreamFrameBytes`는 전체 모델 출력이 아니라 한 프레임을
제한합니다. 구조화된 SSE 데이터 검증에는 `contentMediaType`·`contentSchema`를 추가하세요.
사용자 어댑터와 입력 스트림은 [스트림 API](../reference/streaming.md)를 확인하세요.
