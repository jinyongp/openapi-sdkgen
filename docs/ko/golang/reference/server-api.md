# 서버 API

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

아래 정규화 스키마 예제에서는 생성된 `server/runtime` 패키지를 `runtime`으로, 생성된 `types` 패키지를 `types`로 import하세요.

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

## 호스트 연결

생성된 처리기는 `http.Handler`를 구현합니다. 호스트가 선택한 경로에 연결하세요.
TLS·리스너 종료·미들웨어·콜백 등록·HTTP 프로토콜 기능은 호스트가 담당합니다.
`--with server`는 타입이 있는 수신 계약을 생성하며 서버를 시작하지 않습니다.
[웹훅 예제](../examples/webhook-server.md)에서 전체 로컬 실행을 확인하세요.
