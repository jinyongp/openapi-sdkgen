# 파일·Link·스트림

<span id="스트림·파일·사용자-코덱"></span>


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

## 스트림과 파일의 소유권

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

프레이밍·취소·코덱은 [스트림 API](../reference/streaming.md)를 확인하세요.
