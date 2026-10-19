# 필요한 API 선택

[Todo 명세](/examples/golang-todo.json)와 [모듈 설정](./getting-started.md)을 사용합니다.
코드는 `ctx context.Context`를 사용할 수 있고 `error`를 반환하는 함수 안에서 실행하세요.

정적 참조로 선택해도 구체적인 작업 타입을 유지합니다.

```go
ref := operations.GetTodo()
prepared, err := selective.Prepare(ref)
if err != nil {
    return err
}
client, err := selective.NewClient(selective.ClientOptions{BaseURL: "https://api.example.com"}, prepared)
if err != nil {
    return err
}
operation, err := selective.Bind(client, ref)
if err != nil {
    return err
}
todo, err := operation.Call(api.GetTodoInput{Path: api.GetTodoPath{ID: "todo-1"}}).Do(ctx)
```

생성 루트 아래의 `operations`와 `selective`를 import하세요. Go 링커는 도달하지 않는 코드를
제거하므로 JavaScript 번들러나 트리 셰이킹 설정이 필요하지 않습니다. 미사용 메서드가
제거돼도 import한 패키지의 초기화 코드는 남을 수 있습니다. 생성 시 선택하면 사용하지
않는 생성 런타임 패키지도 제외할 수 있습니다. 다만 컴파일러는 import한 패키지를 검사합니다. 생성 소스와 컴파일 작업을 줄이려면 `--operation getTodo` 또는
`--route 'GET /todos/{id}'`로 생성 범위를 선택하세요. Link 실행에 필요한 작업은 내부에 남깁니다.
TOML의 이름 있는 클라이언트는 독립 진입 패키지를 가집니다. 실행 중 `LookupOperation`이나
`LookupRoute`로 선택하면 레지스트리를 지연 초기화하며 `types.Value` 또는 `types.NativeInput`을
받는 동적 작업을 반환합니다. 일정한 바이너리 크기나 빌드 속도 개선을 보장하지 않습니다.

생성 범위를 선택하면 출력 소스는 줄어들지만, 입력 문서를 파싱하고 분석하는 작업은
여전히 필요합니다. 따라서 선택한 작업 수에 비례해 생성 시 메모리 사용량이 줄어드는
것은 아닙니다. 큰 명세를 비교할 때는 같은 입력과 선택 조건으로 생성 비용과 SDK
컴파일 비용을 따로 측정하세요.

## 선택 생성 {#generation}

```sh
openapi-sdkgen generate --input ./openapi.json --target golang \
  --go-import-path example.com/consumer/internal/api --output ./internal/api \
  --operation getTodo
```

루트는 선택한 공개 API만 노출합니다. 필요한 Link 대상은 내부에서 사용할 수 있도록 남습니다.
없는 API나 숨긴 API를 선택하면 오류이며, 선택해도 원래 입력의 파싱은 생략하지 않습니다.
고정된 선택을 여러 개 두려면 [이름 있는 클라이언트](./named-clients.md)를 확인하세요.
