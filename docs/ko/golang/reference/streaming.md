# 스트림 API

순차 응답은 요청한 만큼 소비합니다. 첫 `Next`에서 실행을 시작하며 항목을 소비하는 속도로
다음 항목 요청을 조절합니다. `MaxStreamFrameBytes`는 프레임 하나의 크기를 제한하며, 전체 응답이나
사용자 HTTP 전송이 보관하는 버퍼의 크기를 제한하지 않습니다. 호출 옵션은 클라이언트
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

## 수명과 소비 속도

`Stream[T]`는 지연 실행합니다. `Next()` 또는 `Response()`가 교환을 시작하며 `Response()`는
항목을 소비하지 않고 헤더를 읽습니다. `Next`는 한 번에 하나만 실행하세요. false를 반환한 뒤에는
`Err()`를 확인합니다. `Close()`는 실행을 중단하고 소유한 데이터 소스를 닫습니다. 명시적 닫기 자체는
순회 오류가 아니며 context 취소는 오류에서 확인할 수 있습니다. 소비자가 다음 항목을 요청하는
속도에 맞춰 읽습니다. 백그라운드 항목 큐는 약속하지 않습니다.

SSE·NDJSON·JSON sequence와 선언된 증분 미디어는 작업 계약을 따릅니다. 프레임 제한은
총 바이트 수나 사용자 HTTP 전송의 버퍼 크기를 제한하지 않습니다. 타입이 있는 오류는 실행 context를
유지합니다. 순차형 raw 출력 본문은 호출자가 닫습니다. 사용자 `MediaCodec`·`StreamCodec`은
지원하는 방향만 구현하며 없는 방향은 명시적 오류입니다. [파일과 Link](../guide/files-links-streams.md)를 확인하세요.
