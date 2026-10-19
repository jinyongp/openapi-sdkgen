# Go SDK 생성과 사용

타입이 있는 Go 소스를 생성하고 `context.Context`로 API를 호출하며, 웹훅과 콜백을
사용자 `net/http` 서버에 연결할 수 있습니다. 생성한 SDK를 사용하려면 **Go 1.27 이상**이
필요합니다. 내려받은 생성기 실행 파일은 Go·Node.js·TypeScript 없이도 소스를 생성합니다.
npm 실행기는 Node.js가 필요하지만 생성한 Go SDK에는 필요하지 않습니다.

[Go Todo 예제](https://github.com/jinyongp/openapi-sdkgen/tree/main/examples/golang-todo-app)는
로컬 서버에서 Raw·Link·리소스·스트림·정적 선택·웹훅을 실행합니다.
`created: 201`, `linked: todo-1 Ship the SDK` 등의 결과를 확인할 수 있습니다.
Go 지원은 현재 소스에 포함돼 있으며 다음 릴리스에서 제공될 예정입니다.

## 사용자 모듈에 생성하기

기존 Go 모듈에서 시작하거나 `go mod init example.com/consumer`를 실행하세요.
[예제 명세](/examples/golang-todo.json)를 `openapi.json`으로 저장합니다.
이 예제는 웹훅이 있으므로 `server` 확장을 사용합니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.json \
  --target golang \
  --go-import-path example.com/consumer/internal/api \
  --output ./internal/api \
  --with server \
  --incremental
```

`--go-import-path`는 모듈 내부 디렉터리까지 포함한 생성 패키지의 전체 경로입니다.
출력 파일 경로와 별개로 반드시 지정해야 합니다. `--go-package`는 루트 패키지 이름이며
기본값은 `api`입니다. 생성기는 `go.mod`나 `go.sum`을 쓰지 않습니다. 의존성 버전은 사용자
모듈이 관리합니다. `--incremental`은 관리 대상 파일을 갱신하면서 사용자 파일을 보존합니다.

예제의 생성 소스는 다음 버전의 의존성을 사용합니다.

```sh
go get github.com/andybalholm/brotli@v1.2.6 \
  github.com/klauspost/compress@v1.20.3 \
  github.com/nlnwa/whatwg-url@v0.6.2 \
  github.com/go-quickjs/go-quickjs@v0.27.0 \
  github.com/bits-and-blooms/bitset@v1.20.0 \
  golang.org/x/net@v0.61.0 golang.org/x/text@v0.43.0
go mod tidy
```

HTTP 클라이언트는 압축 처리와 URL 파서를 공유합니다. ECMAScript 정규식과 일부 형식
검사는 생성한 스키마 계층에서 `go-quickjs`를 사용합니다. 실제 생성 기능이 import하는
의존성만 필요하며 별도 SDK 런타임 모듈을 설치하지 않습니다.

## 타입이 있는 호출과 값의 존재 여부

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

## 스트림·파일·사용자 코덱

순차 응답은 요청한 만큼 소비합니다. 첫 `Next`에서 실행을 시작하며 항목을 소비하는 속도로
다음 항목 요청을 조절합니다. `MaxStreamFrameBytes`는 프레임 하나의 크기를 제한하며, 전체 응답이나
압축 해제기가 이전 데이터를 보관하는 버퍼의 크기를 제한하지 않습니다. 호출 옵션은 클라이언트
기본값보다 우선하며, 두 곳 모두 `Accept`를 지정하지 않았을 때만 SDK가 스트림 미디어를 선택합니다.
반복문 뒤에는 반드시 `Err`를 확인하세요.

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

유한 파일은 `types.Binary`로 표현합니다. `BinaryFromBytes`, `BinaryFromString`, 또는
메타데이터와 `Open(context.Context)`를 제공하는 `BinarySource`를 사용하세요. 실행 전에
데이터 소스를 열지 않으며 성공적으로 연 읽기 객체는 SDK가 소유하고 닫습니다.
`BinaryFromReader`로 만든 일회용 읽기 객체는 재실행할 수 없습니다. 바이트 입력은 버퍼를 빌리므로
실행 중 변경하지 마세요. 접근자가 암묵적으로 데이터 소스를 열지는 않습니다.

항목별 입력에는 `types.SourceFactory[T]`를 사용합니다. 실행마다 새 데이터 소스를 제공하거나
`types.SourceOnce`로 일회용임을 명시하세요. SDK는 context를 취소한 뒤 소유 데이터 소스를 닫습니다.
사용자 미디어는 `types.MediaCodec`, 프레임과 항목 변환은 `types.NewStreamCodec(protocol, adapter)`로
연결합니다. nil인 방향은 지원하지 않습니다. 코덱은 HTTP 메타데이터 복사본을 받고 본문을 빌려 쓰며
최종 Close는 SDK가 담당합니다. 훅의 공유 상태는 동시 호출에 안전해야 합니다.
열린 미디어의 `types.MediaValue[T]`는 `AsValue`, `AsBinary`, `AsMultipart`로 검증된 JSON 형태의 값,
버퍼 바이트, 파일 슬롯을 구분합니다.

## 선택 방식과 컴파일 비용

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
않는 런타임 의존성을 import하는 일도 줄일 수 있습니다. 다만 컴파일러는 import한 패키지를 검사합니다. 생성 소스와 컴파일 작업을 줄이려면 `--operation getTodo` 또는
`--route 'GET /todos/{id}'`로 생성 범위를 선택하세요. Link 실행에 필요한 작업은 내부에 남깁니다.
TOML의 이름 있는 클라이언트는 독립 진입 패키지를 가집니다. 실행 중 `LookupOperation`이나
`LookupRoute`로 선택하면 레지스트리를 지연 초기화하며 `types.Value` 또는 `types.NativeInput`을
받는 동적 작업을 반환합니다. 일정한 바이너리 크기나 빌드 속도 개선을 보장하지 않습니다.

생성 범위를 선택하면 출력 소스는 줄어들지만, 입력 문서를 파싱하고 분석하는 작업은
여전히 필요합니다. 따라서 선택한 작업 수에 비례해 생성 시 메모리 사용량이 줄어드는
것은 아닙니다. 큰 명세를 비교할 때는 같은 입력과 선택 조건으로 생성 비용과 SDK
컴파일 비용을 따로 측정하세요.

## 보안과 전송

`ClientOptions.SecurityProvider`는 선언된 보안 스킴의 식별자에 맞는 인증 정보를 반환합니다.
직접 인증 정보와 범위별 서버 선택에는 생성 타입을 사용하며, 제공자는 대안과 스킴 메타데이터를
확인할 수 있습니다. `HTTPClient`와 `Transport`는 둘 중 하나를 사용합니다.
사용자 전송은 응답이 압축 상태인지, 원래 헤더를 유지한 해제 상태인지 선언해야 합니다.
SDK는 응답 압축을 해제하면서 원래 헤더를 보존합니다. 내부에서 이미 압축을 풀었다면
압축 상태를 선언하지 마세요.

시간 제한과 취소는 실행 context로 제어합니다. HTTP/1 연결을 재사용하려면 응답을 소비하거나
닫아야 합니다. 사용자 전송과 훅은 동시 호출을 지원해야 합니다. Go는 유효한 HTTP 토큰
메서드를 허용하고 브라우저 Fetch의 금지 헤더·메서드 규칙을 적용하지 않습니다.
별도 CONNECT 터널 API는 제공하지 않습니다.

## 웹훅과 콜백

생성 루트 아래의 `server/webhooks` 또는 `server/callbacks`를 import하세요.
호스트가 정한 경로에 타입 핸들러를 연결합니다. 리스너와 콜백 URL 등록은 사용자가 담당합니다.

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

인증이 필요하면 `Options.Authenticate`를 제공합니다. 매개변수를 디코딩한 뒤 이 함수를 호출하며,
원래 보안 요구·후보, 다섯 매개변수 그룹, context, `http.NoBody`를 가진 HTTP 메타데이터 복사본을
전달합니다. nil 거부 값은 계속 진행하고 거부 값은 지정한 상태·헤더·데이터 소스로 응답합니다.
익명 대안이면 인증 함수를 호출하지 않습니다. 인증 미설정은 401, 함수 오류는 500입니다.
`CloneBody(ctx)`는 제한 안에서 전체 본문의 독립 읽기 객체를 반환하며 사용자가 닫습니다.
항목별 입력은 복제할 수 없습니다.

전체 본문은 기본 8 MiB를 넘으면 413, 증분 프레임은 기본 1 MiB를 넘으면 400입니다.
스트림 합계에는 전체 본문 제한을 적용하지 않습니다. `MaxBodyBytes`와 `MaxStreamFrameBytes`로
제한을 조정하세요. HTTP/1 동시 송수신에는 `EnableFullDuplex`를 지원하는 응답 작성기가 필요하고,
스트림은 flush도 지원해야 합니다. HTTP/2는 기본 동시 송수신을 사용합니다.
헤더를 보낸 뒤 실패하면 응답을 중단하고 `OnError`로 알리며 이미 보낸 상태를 바꿀 수 없습니다.
입력 데이터 소스를 응답에 사용하면 송신 종료까지 유지해야 하며 이후 SDK가 정리합니다.

`server/runtime.CompileSchema`는 정규화 스키마와 정의 map을 받는 별도 범용 경로입니다.
`types`, `reference`, `constValue` 같은 정규화 키워드를 받으며 원래 JSON Schema의 `type`,
`$ref`는 받지 않습니다. URL을 가져오거나 스키마 확장을 실행하지 않습니다.

```go
schemaJSON := []byte(`{"types":["object"],"properties":{"count":{"types":["integer"]}},"required":["count"]}`)
normalized, err := types.ParseValue(schemaJSON)
if err != nil {
    return err
}
contract, err := runtime.CompileSchema(normalized, nil)
if err != nil {
    return err
}
value, err := contract.Decode(ctx, []byte(`{"count":2}`))
if err != nil {
    return err
}
return contract.Validate(ctx, value)
```

## 문제 해결

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

XML은 문서화된 네임스페이스·속성·CDATA·배열 감싸기 및 타입 합성 경로를 지원합니다.
임의 추가 속성과 위치별 배열 항목의 XML 매핑은 기존 제한을 유지합니다. XML로 표현할 수
없는 스키마에는 JSON이나 명시적 코덱을 선택하세요. 전체 import와 로컬 서버, 실행 가능한
오류 처리·정리는 [Go Todo 예제](https://github.com/jinyongp/openapi-sdkgen/tree/main/examples/golang-todo-app)에서 확인할 수 있습니다.
