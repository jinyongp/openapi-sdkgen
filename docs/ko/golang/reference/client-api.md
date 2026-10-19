# 클라이언트 API

## `NewClient`와 `ClientOptions` {#clientoptions}

`api.NewClient(options)`는 `(*api.Client, error)`를 반환하며 요청을 보내지 않습니다.
`BaseURL`은 인증 정보·query·fragment가 없는 HTTP(S) 절대 URL입니다. 생략하면 해당하는
OpenAPI 서버를 사용합니다. 작업 범위가 경로 범위보다, 경로 범위가 루트 범위보다 우선합니다.

| 옵션 | 용도 |
| --- | --- |
| `Server`, `Origin`, `BaseURL` | 범위별 OpenAPI 서버 선택과 URL 해석 |
| `HTTPClient` 또는 `Transport` | 호스트 HTTP 구현. 함께 지정할 수 없음 |
| `SecurityProvider` | 선언된 보안 요구와 인증 방식 식별자에 맞는 자격 증명 |
| `Headers`, `Accept`, `Authorization` | 기본 헤더와 미디어 선호 |
| `Credentials`, `CSRFToken`, `RequestID` | 선언된 인증·요청 정책 |
| `Timeout`, `MaxStreamFrameBytes` | 실행 시간 제한과 프레임별 크기 제한 |
| `MultipartHeaders`, `MultipartContentTypes` | 선언된 multipart 파트 설정 |
| `Codecs`, `StreamCodecs` | 사용자 미디어·스트림 어댑터 |

선택적 요청 설정에는 `types.Some(value)`를 사용합니다. `WithOptions(types.RequestOptions{...})`는
설정을 덮어쓴 호출을 만듭니다. 옵션 컨테이너는 복사하지만 DTO와 바이트 버퍼는 애플리케이션이
소유합니다. 실행 중에는 빌려준 데이터를 바꾸지 마세요. 공유 provider·codec·transport는 동시
호출을 지원해야 합니다. [인증과 전송](../guide/transport.md)을 확인하세요.

## 작업·경로·리소스

`client.Operations.CreateTodo(input)`과 생성된 `Routes` 메서드는 타입이 있는 호출을 만듭니다.
`client.Todos.ByID("todo-1").Get()` 같은 리소스 선택자는 경로 값을 미리 연결합니다.
이름은 명세를 따르며 `inspect --target golang`으로 확인할 수 있습니다.
`Do(ctx)`는 성공 출력을, `Raw(ctx)`는 전체 디코딩 응답·상태·헤더를 반환합니다.
실행할 때마다 별도 요청을 보냅니다. 선언된 응답 접근자는 `(value, bool)`을 반환합니다.
입력·인코딩·전송·HTTP·디코딩 실패는 오류로 반환합니다.

## 오류 {#errors}

`errors.As`로 `*types.APIError`나 생성된 작업별 HTTP 오류를 확인하세요.
안정된 도우미에는 `IsAPIError`, `GetErrorCode`, `IsErrorCode`, `GetRequestID`가 있습니다.
취소와 기한 초과는 원인을 유지하면서 SDK 코드로 변환합니다. 메타데이터 스냅샷은
실시간 본문 소유권을 hook에 넘기지 않습니다. [호출과 오류](../guide/client.md)에서 시작하세요.

## Link와 페이지 순회

`client.Links.CreateTodo.GetTodo.Follow(raw)`는 원본 응답으로 Link 대상을 준비하고 실행 전에
입력을 검증합니다. `x-pagination`을 선언하면 `Paginate(ctx)`가 생성됩니다. 모드가 `both`면
생성된 cursor/offset 상수를 전달하세요. 순회에는 `Next`·`Item`·`Err`·`Close`를 사용합니다.
취소하면 이후 요청을 중단합니다. [확장](./extensions.md)을 확인하세요.

## OpenAPI 메타데이터 {#openapi-metadata}

`--with metadata`를 켜면 생성된 `metadata` 패키지를 import할 수 있습니다.
`Version`·`VersionLine`은 OpenAPI 버전을 나타내고, `Document()`는 원래 진입 명세를
독립된 `json.RawMessage` 복사본으로 반환합니다. 알 수 없는 `x-*` 필드는 데이터로 남으며
SDK 동작을 바꾸지 않습니다. 이 확장은 모델·클라이언트 런타임과 별개이며 API 호출에 필수는 아닙니다.
