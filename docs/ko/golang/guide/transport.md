# 인증과 전송

<span id="보안과-전송"></span>

`ClientOptions.SecurityProvider`는 선언된 보안 스킴의 식별자에 맞는 인증 정보를 반환합니다.
직접 인증 정보와 범위별 서버 선택에는 생성 타입을 사용하며, 제공자는 대안과 스킴 메타데이터를
확인할 수 있습니다. `HTTPClient`와 `Transport`는 둘 중 하나를 사용합니다.

## 응답 압축 제어하기

압축 협상과 해제는 사용자 `http.Client` 또는 `http.RoundTripper`가 담당합니다.
SDK는 `Accept-Encoding`을 추가하거나 `http.Transport.DisableCompression`을 바꾸지 않습니다.
Go 기본 HTTP 전송은 gzip을 협상하고 해제할 수 있습니다. 기본 동작을 바꾸려면 다음처럼 설정하세요.

```go
transport := http.DefaultTransport.(*http.Transport).Clone()
transport.DisableCompression = true
client, err := api.NewClient(api.ClientOptions{
	BaseURL:    "https://api.example.com",
	HTTPClient: &http.Client{Transport: transport},
})
```

Brotli·Zstandard 등은 사용자 전송에서 설정하세요. HTTP 계층이 해제할 수 있는 인코딩만 요청해야
합니다. JSON·XML·스트림 검증에는 해제한 바이트를 반환하거나, 반환된 표현을 처리할 미디어 코덱을
제공하세요. SDK는 HTTP 계층이 반환한 본문과 메타데이터를 사용하며 실행이 끝나면 본문을 닫습니다.

HTTP 압축 해제로 제거된 헤더는 SDK가 복원하지 않습니다. 필수 응답 헤더는 실제로 반환된 헤더를
기준으로 검사합니다. 바이너리와 순차 `Raw`는 반환된 바이트를 노출하며, JSON 등 구조화된 `Raw`
응답은 기존 미디어 디코딩과 스키마 검증을 거칩니다. [스트림 수명](../reference/streaming.md)도 확인하세요.

## 취소와 연결 재사용

시간 제한과 취소는 실행 context로 제어합니다. HTTP/1 연결을 재사용하려면 응답을 소비하거나
닫아야 합니다. 사용자 전송과 훅은 동시 호출을 지원해야 합니다. Go는 유효한 HTTP 토큰
메서드를 허용하고 브라우저 Fetch의 금지 헤더·메서드 규칙을 적용하지 않습니다.
별도 CONNECT 터널 API는 제공하지 않습니다.

[클라이언트 옵션](../reference/client-api.md)도 확인하세요.
