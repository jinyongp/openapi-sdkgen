# Go 타깃 구현 계획

> 2026-10-19 후속 결정: 생성 Go SDK의 외부 의존성 제거와 압축의 사용자 HTTP 계층 위임은
> [무의존성 전환 계획](../golang-zero-dependencies/plan.md)이 현재 기준이다. 이 문서의
> D09·D23·D25·D26 및 6.1절은 이전 구현의 결정 이력이며 새 구현 기준으로 사용하지 않는다.

작성일: 2026-10-17. 설계 결정 이후 2026-10-18 제품 구현을 시작했다. 현재 작업 상태와
실제 검증 범위는 [구현 기록](implementation.md)·[workstream 기록](workstream.md)을 따른다.

목표는 기준 TypeScript SDK가 제공하는 모든 작업을 Go SDK에서도 수행하게 하는 것이다.
HTTP 계약, 스키마 의미, 진단, 후속 호출, 스트림, 서버 확장과 생성 관리까지 포함한다.
언어별 표현은 달라도 제공하는 작업과 관찰 가능한 결과는 유지한다.

이 문서는 내부 구현 계획이다. 아직 제공하지 않는 Go 기능을 제품 문서의 지원 기능으로
소개하지 않는다. 아래 계약과 작업 단위를 구현 기준으로 사용한다. 측정하지 않은 제품
성능이나 완료하지 않은 기능 검증을 통과한 것으로 취급하지 않는다.

## 1. 기준과 완료 범위

기준 커밋은 `fe010a8`이다. 기능 기준은
[OpenAPI 기능 매니페스트](../../docs/openapi-feature-manifest.json),
[현재 클라이언트 API](../../docs/reference/client-api.md),
[스트리밍 API](../../docs/reference/streaming.md),
[서버 API](../../docs/reference/server-api.md),
[선택형 클라이언트](../../docs/guide/selective-client.md),
[독립 클라이언트](../../docs/guide/named-clients.md)다.

기능 매니페스트에는 302개 항목이 있다. 기본 상태는 `generated` 190개,
`metadata` 96개, `error` 11개, `omitted` 5개다. 조건부 상태도 계약의 일부다.
예를 들어 웹훅·콜백의 기본 상태는 오류지만 `server` 확장을 켜면 생성된다.
302라는 숫자는 지원 기능 수나 테스트 수가 아니다.

[기능별 구현 배정표](feature-parity.json)는 모든 ID를 작업 단위에 배정한다.
기존 명세의 필드·조건·증거를 복제해서 별도 정답을 만들지 않고, 원본 파일의 해시와
ID로 연결한다. 표의 배정은 구현 완료 표시가 아니다.

완료 범위에는 다음도 포함한다.

- 리소스·작업 식별자·경로 기반 호출, 타입·열거형·상태별 오류 접근.
- 전체 클라이언트, 정적 선택, 실행 중 계산한 선택, 독립 클라이언트, Link 내부 의존.
- 모든 현재 `x-*` 확장과 사용자 정의 전체 미디어·스트림 코덱.
- 웹훅·콜백 수신과 지원되는 정규화 스키마의 범용 수신 처리.
- `generate`, `inspect`, `--check`, `--incremental`, 진단 모드·형식,
  서버·메타데이터 확장, 설정 파일, npm으로 배포되는 생성기와 문서 playground.
- 대형 명세, 현재 제공자·holdout fixture, 실패·생략 범위 보존.

Go 지원을 공개할 때 위 범위를 모두 완료한다. 작업 단위는 개발 순서이며 부분 구현을
전체 기능 대응으로 배포하기 위한 단계가 아니다. 일반 HTTP 서버 전체 생성, OAuth 토큰
발급, 재접속·재시도 정책은 현재 제품과 같이 애플리케이션의 책임이다.

## 2. 확정한 결정

| 결정 | 내용 | 근거·검증 |
| --- | --- | --- |
| D01 | 정식 타깃 이름은 `golang`, 설정 절도 `[golang]`이다. 별칭은 추가하지 않는다. | 사용자가 지정한 타깃 이름과 일치시킨다. |
| D02 | 생성 SDK의 최소 Go 버전은 `1.27.0`이다. CI는 해당 minor의 최신 패치를 사용한다. | 현재 생성기의 `go.mod`와 맞춘다. 로컬 검증은 Go 1.27.1이다. |
| D03 | Go·Node 설치 없이 배포된 생성기 바이너리로 Go 소스를 생성할 수 있게 한다. | 포맷은 생성기 안의 `go/format`으로 처리한다. 검증·컴파일만 Go 도구가 필요하다. |
| D04 | 애플리케이션 소스와 생성 런타임을 출력한다. 사용자의 `go.mod`·`go.sum`은 수정하지 않는다. | 현재 소스 소유 모델과 관리 출력 경계를 유지한다. |
| D05 | 리소스·작업·경로 호출은 구체적인 요청 타입을 반환하고 `Do(ctx)`에서 실행한다. | 지원 기능만 메서드로 노출하고 전역 작업 레지스트리를 기본 호출에서 제외한다. |
| D06 | JSON `integer`와 `number`의 기본 Go 표현은 `float64`다. | TypeScript의 binary64 범위와 반올림을 보존한다. 숫자 시제품 9개로 확인한다. |
| D07 | 선택 필드는 `Optional[T]`, nullable은 `Nullable[T]`, 선택 nullable은 두 타입의 조합이다. | 누락·null·false·0·빈 문자열·빈 배열을 구분한 시제품을 통과했다. |
| D08 | JSON은 손실 없는 문자열·필드 존재 정보를 가진 공통 값 표현을 거쳐 검증한다. | 기본 구조체 디코딩의 대소문자 허용, 추가 필드 소실, surrogate 치환을 확인했다. |
| D09 | 정규식은 `github.com/go-quickjs/go-quickjs/jsregexp`의 `v0.27.0`, `Compile(pattern, "u")`를 사용한다. | 42개 Node 비교 사례, 동시 사용, 복잡도 제한 검증을 통과했다. |
| D10 | 기본 정규식 실행을 RE2로 자동 대체하지 않는다. | 문법·문자 클래스·앵커 의미 차이가 있다. 비교 결과는 [조사 기록](research.md)에 보존한다. |
| D11 | 클라이언트는 동시 사용 가능하다. 요청 객체는 값으로 구성하고 실행 상태는 호출마다 분리한다. | 공유 인증·버퍼·스키마 상태를 요청별 상태와 분리한다. 입력 소유권은 5절에 정한다. |
| D12 | 일반 호출의 본문은 SDK가 닫고, 순차형 Raw와 Stream의 본문은 반환된 핸들이 소유한다. | HTTP 연결 재사용·열린 SSE 취소를 시제품에서 확인했다. |
| D13 | 스트림은 지연 시작·단일 소비자·pull 방식이다. 기본 경로에 채널과 백그라운드 goroutine을 두지 않는다. | 현재 시작·소비 속도 계약을 Go의 반복자와 본문 수명으로 표현한다. |
| D14 | 기본·스트림·서버·메타데이터·동적 조회의 의존 경로를 분리한다. | 직접 메서드는 미사용 XML 함수를 제거했고 동적 map은 보존했다. |
| D15 | 공통 의미를 타깃 중립 계획으로 추출하고 TypeScript·Go가 함께 사용한다. | 기존 `schema/plan`은 언어 중립 의미를 이미 분리한다. TypeScript 문자열 방출기는 공유하지 않는다. |
| D16 | 생성 시 API 선택과 독립 클라이언트를 유지한다. 실행 중 선택은 별도 `selective` 패키지에서 제공한다. | 빌드·IDE 비용을 줄이면서 계산된 선택 기능도 보존한다. |
| D17 | 서버 수신 진입점은 `http.Handler`다. 서버 리스너와 인증 정책은 호스트가 제공한다. | Fetch 수신 계약을 표준 Go HTTP 경계로 옮긴다. |
| D18 | 기능 대응은 독립 기대값과 양쪽 실행 결과로 검증한다. | TypeScript 실행 결과만 정답으로 삼아 기존 결함을 복제하지 않는다. |
| D19 | 성능은 생성·소비자 빌드·실행·자원 수명으로 구분해서 측정한다. | 11절의 고정 워크로드와 지표를 사용한다. 일반 CI에 비율·소스 문자열 게이트를 추가하지 않는다. |
| D20 | 기존 TypeScript 공개 동작과 설정은 보존한다. Go 전용 설정은 타깃과 함께 검증한다. | 공통화·출력 fingerprint 변경은 기존 호환성 검사를 함께 통과해야 한다. |
| D21 | 복합·혼합 열거형은 불변 JSON 값 wrapper와 값 생성 함수로 표현한다. | Go 상수로 객체·배열·null을 표현할 수 없다. 4.5절의 값·동등성 계약을 적용한다. |
| D22 | Link는 원본 입력·대상 덮어쓰기·옵션을 분리하고 HTTP 오류와 상태별 대상도 지원한다. | 현재 LinkInvocation과 runtime expression 계약을 7.1절에 옮긴다. |
| D23 | 응답 압축은 SDK 경계에서 풀고 원래 응답 헤더를 보존한다. | 기본 Go Transport와 Fetch의 헤더 차이를 재현했다. 6.1절을 적용한다. |
| D24 | 코덱과 수신 인증에는 본문뿐 아니라 원래 HTTP 메타데이터와 취소 context를 전달한다. | 현재 사용자 hook으로 가능한 헤더 기반 처리와 거부 응답을 유지한다. |
| D25 | `URL`에 의존하는 format 검증은 `github.com/nlnwa/whatwg-url@v0.6.2`를 조건부 사용하고 `golang.org/x/net@v0.61.0`, `golang.org/x/text@v0.43.0`을 고정한다. | W02에서 확인한 URL 경계의 추가 결정이다. Node와 28개 URL·hostname·직렬화 사례를 대조했고 실제 의존 경로는 CGO를 사용하지 않았다. [추가 조사](research.md#6-url-format-경계의-추가-검증)를 따른다. |
| D26 | HTTP 서버 URL·origin도 D25의 고정 URL 파서로 정규화한다. 인증 쿼리는 독립적인 URLSearchParams 의미로 직렬화하며 라이브러리의 SearchParams API는 사용하지 않는다. | W04에서 IDNA·기본 포트·IPv4 표기의 origin 차이와 SearchParams의 구분자·더하기 처리 차이를 확인했다. HTTP 경로가 URL 의존성을 사용하며 format 검증의 별도 초기화 데이터는 계속 조건부로 제거된다. |
| D27 | multipart의 자유 형식 값은 `types.MultipartValue`, 공개 모델 변환 자료는 `types.MultipartData`로 표현한다. JSON 값과 바이너리 source 경로를 분리하며 source 유실·겹침을 오류로 처리한다. MediaContext는 표준 HTTP metadata snapshot과 빌린 본문을 사용한다. | W05 생성 소비자에서 합성·추가 속성·미디어별 투영, 코덱 5개 방향의 수명과 EOF trailer를 검증했다. 미디어별 직접 함수 계획과 pull reader로 기존 JSON 계약·패키지 경계·대형 업로드의 지연 읽기를 유지한다. |
| D28 | 순차 응답의 전체·항목 스키마와 파트 encoding을 별도 직접 계획으로 생성한다. multipart 응답의 바이너리는 `MultipartData`에 보존하며 작업별 sealed 결과의 `AsMultipart`·항목의 `AsBinary`로 조회한다. 기본 multipart 송신은 파일을 버퍼링하지 않는 pull 방식이다. | W06 소비자에서 서로 다른 schema/itemSchema, prefix/item 헤더, XML·중첩 바이너리, HTTP/1·HTTP/2 양방향 전송과 64 MiB source의 지연 읽기를 검증했다. 응답 바이트·실패 정리는 아래 스트림 계약과 [조사 기록](research.md#9-w06-순차-미디어의-검증-경계)에 고정한다. |
| D29 | 리소스·Link·페이지·확장의 구체 타입과 HTTP 응답 속성 profile을 고정한다. | 7.2절과 [조사 기록](research.md#10-w07-호출-표현과-응답-profile)을 따른다. 공유 의미 계획은 언어별 렌더러에 의존하지 않는다. |

D06은 의도적인 선택이다. Go `int64`로 일괄 생성하면 TypeScript가 표현하는
`1e100` 같은 정수를 받지 못하고, `9007199254740993` 같은 값에서 양쪽 반올림이
달라진다. Go SDK가 임의 정밀도 숫자나 `int64`를 기본으로 제공하는 별도 정책은 넣지
않는다. `integer`의 유한성·정수성·범위는 생성된 검증기가 검사한다. `int64` format
때문에 현재 TypeScript에 없는 범위 제약을 추가하지 않는다. 상태 코드·인덱스·바이트
크기처럼 SDK 자체가 정의하는 정수는 일반 Go 정수 타입을 사용한다.

D09의 의존성은 정규식 실행 또는 `format: regex` 처리가 필요한 생성물에만 포함한다.
애플리케이션은 생성된 의존성 안내에 따라 버전을 고정한다. 생성기는 검사한 import 경로와
버전을 안내하지만 모듈 명령을 자동 실행하지 않는다. JavaScript VM, Node, CGO와
런타임 외부 프로세스는 필요하지 않다. MIT 고지와 의존성 근거는 조사 기록에 남긴다.

## 3. 생성 명령·파일·패키지 계약

아래는 구현할 명령의 확정 형태다. 아직 현재 CLI에서 실행할 수 있는 명령이 아니다.

```toml
source = "./openapi.yaml"
target = "golang"
output = "./internal/api"
addons = ["server", "metadata"]

[golang]
package = "api"
import_path = "example.com/app/internal/api"

[clients.catalog.selection]
operations = ["listProducts", "getProduct"]
```

- `package` 기본값은 `api`다. 유효한 비공개 Go 패키지 식별자만 허용한다.
- `import_path`는 필수다. CLI 대응 옵션은 `--go-package`, `--go-import-path`다.
  `go.mod` 탐색이나 사용자 디렉터리 이름으로 추측하지 않는다.
- import 경로는 Go import 경로 문법으로 검사한다. 상대 경로·빈 segment·제어 문자·
  경로 역행은 거부한다. 출력 파일의 OS 경로 검사와 독립적으로 수행한다.
- `[typescript]`의 명시 설정과 `--typecheck`는 Go 타깃에서 기존처럼 거부한다.
  Go 설정을 TypeScript 타깃에 명시해도 오류로 처리한다.
- 생성 fingerprint에 Go 패키지 이름·import 경로를 저장한다. 변경 시 기존
  `--incremental` 계약으로 재생성한다. 이전 TypeScript manifest는 계속 읽는다.
- 선택·독립 클라이언트·확장 선택은 기존 공통 옵션을 사용한다.

생성 트리의 패키지 방향은 다음으로 고정한다.

```text
api/                         root client and public aliases
  types/                     Optional, Nullable, binary and stream contracts
  models/                    named DTOs, operation inputs and response variants
  operations/                typed operation-reference factories
  selective/                 prepared sets and selected client
  clients/catalog/           distinct client and aliases for its public API set
  server/webhooks/           http.Handler and typed webhook handlers
  server/callbacks/          host-bound callback handlers
  server/runtime/            normalized-schema inbound helpers
  metadata/                  optional original document and metadata
  internal/core/             request composition, errors and body leases
  internal/transport/        net/http adapter, compression and capabilities
  internal/security/         requirement/credential application
  internal/media/            JSON/text/binary/XML/form/multipart codecs
  internal/stream/           framing, protocols and adapters
  internal/wire/             canonical values and JSON/media primitives
  internal/schema/           selected schema programs and validation helpers
  internal/operations/       one shared implementation per operation
```

`models`·`types`는 클라이언트 진입점을 import하지 않는다. `internal/schema`는
`models`를 import하지 않고 공통 값 표현을 검사한다. 모델 코덱은 `internal/wire`를
사용한다. 루트·독립 클라이언트·서버는 필요한 작업 구현과 모델만 참조한다.
`metadata`는 호출 런타임이 참조하지 않는다. 패키지별 파일은 작업·스키마 단위로 나누되
작업마다 Go 패키지를 추가하지 않는다. 공유 작업 코드는 복제하지 않는다.
`internal/core`는 아래 실행 흐름을 조합하고 요청 수명을 소유한다. HTTP 전송·압축·인증·
미디어·프레이밍 알고리즘의 소유자는 위 별도 패키지다. `core`의 폴더 안에 모두 모으지 않는다.

모든 관리 파일은 `Code generated ... DO NOT EDIT.` 고지를 가진다. Go 소스는
`go/format`으로 포맷한다. 실패는 방출기 내부 오류로 보고하고 부분 출력을 게시하지 않는다.
결정적 정렬과 기존 원자적 게시·잠금·복구·사용자 파일 보존 규칙을 유지한다.

## 4. 모델·스키마·JSON 계약

### 4.1 공개 타입

| 스키마 | 생성 표현 |
| --- | --- |
| required, non-null | `T` |
| optional, non-null | `types.Optional[T]` |
| required, nullable | `types.Nullable[T]` |
| optional, nullable | `types.Optional[types.Nullable[T]]` |
| string | 기본 `string`; format은 검증하고 암묵적으로 `time.Time`으로 바꾸지 않는다. 미디어의 바이너리 투영은 아래 별도 계약을 사용한다. |
| integer·number | `float64`; 스키마 검증으로 계약을 제한한다. |
| array | `[]T`; 튜플은 위치별 필드와 잔여 항목을 가진 생성 타입이다. |
| object | 이름 있는 구조체와 선언된 경우 `AdditionalProperties` |
| enum·const | 동종 non-null 스칼라는 기반 타입·상수, 혼합·복합 값은 불변 wrapper·생성 함수. 4.5절 참고. |
| true·빈 스키마·불특정 값 | `types.Value`; JSON 값 전체를 표현한다. |
| false | 생성 타입은 유지하되 값 검증은 항상 실패한다. |

`Some(value)`, `NonNull(value)`, `Null[T]()` 생성 함수를 제공한다.
`Optional`의 제로값은 누락, `Nullable`의 제로값은 null이다. 상태 필드는 비공개다.
부모 객체 코덱이 선택 필드의 포함 여부를 결정한다. `omitempty`에 맡기지 않는다.
루트에서 누락 상태를 직렬화하면 오류다. 일반 Go 제로값은 존재하는 값으로 취급한다.
요청 모델의 required 필드를 구조체 리터럴에서 쓰지 않았다고 별도의 누락 상태를 만들지
않는다. 수신 디코더는 실제 필드 존재 여부를 별도로 검사한다.

non-null 배열·맵의 nil 값은 빈 배열·객체로 인코딩한다. null은 `Nullable`로만 표현한다.
`minItems`·`minProperties`는 그 결과에 그대로 적용한다. 입력·출력 모델은
`readOnly`·`writeOnly`에 따라 분리한다. 출력 DTO와 배열은 수정 가능하다.

### 4.2 합성·재귀·추가 속성

- 단순하고 충돌 없는 `allOf` 객체는 합쳐진 이름 있는 구조체로 생성한다. 검증은 모든
  원래 분기를 유지한다. flatten으로 의미를 보존할 수 없는 합성은 공통 값과 타입별
  접근자를 가진 생성 타입으로 표현한다.
- `oneOf`는 일치 분기가 정확히 하나일 때만 성공한다. discriminator는 후보 탐색을
  돕지만 나머지 제약을 우회하지 않는다. `anyOf`는 일치한 여러 분기를 유지한다.
- 합성 타입에는 생성된 분기 생성자와 `As...() (T, bool)` 접근자를 제공한다.
  잘못된 분기로 변환하지 않는다. 성공 분기만 평가 annotation을 전달한다.
- `unevaluated*`, `contains`, `dependent*`, `if/then/else`, `$dynamicRef`의
  평가 상태와 동적 scope는 검증 호출마다 분리한다.
- 재귀 DTO는 필요한 위치에서 포인터로 참조한다. nil 재귀 값과 스키마 null은
  구분하며, 인코딩 시 금지된 nil·실제 객체 참조 순환을 오류로 처리한다.
- 추가 속성은 원래 키를 보존한다. 알려진 필드와 `AdditionalProperties`에 같은 키를
  넣으면 인코딩 오류다. `patternProperties`와 `propertyNames`도 전체 키에 적용한다.

### 4.3 JSON 경계

생성된 기본 코덱은 `internal/wire.Value`로 JSON을 한 번 읽고 스키마를 검사한 뒤
DTO를 구성한다. 값 표현에는 원래 필드 존재와 대소문자가 보존된다. 중복 키는 현재
`JSON.parse`와 같이 마지막 값을 사용한다. 객체는 prototype을 가진 객체로 흉내 내지
않고 Go map으로 보관한다. `__proto__`도 일반 데이터 키다.

문자열 스캐너는 JSON escape를 직접 해석한다. 정상 surrogate pair는 하나의 문자로,
lone surrogate는 WTF-8 형태의 Go 문자열로 보존한다. 인코더는 lone surrogate를
`\uXXXX`로 돌려준다. 일반 `encoding/json.Unmarshal`에 문자열 변환을 위임하지
않는다. UTF-8 전송에서 잘못된 바이트의 처리, escape 오류, 문자열 길이는 TypeScript
전송·JSON 경계의 기대값으로 고정한다. 길이는 바이트 수나 UTF-16 unit 수 대신 현재
런타임처럼 코드 포인트 수로 검사한다. 보존된 lone surrogate 하나는 한 항목이다.

숫자는 유한 binary64로 읽는다. 무한대·NaN은 값 검증에서 거부한다. 출력의 음수 0은
0으로 정규화한다. `multipleOf`는 binary64의 최단 십진 표현을 유리수로 비교하여
현재 런타임 의미를 보존한다. 임의 정밀도 연산은 이 검사처럼 필요한 경로에 한정한다.
알려진 숫자 format에 없는 정밀도·범위 제한을 새로 추가하지 않는다.

타입별 직접 인코더와 생성된 스키마 프로그램을 사용한다. JSON을 다시 만들어 구조체에
재디코딩하지 않는다. 복잡한 스키마는 공유 helper로 처리한다. 완전한 스키마 문서와
파서를 API 호출마다 새로 만들지 않는다. 일반 미디어와 multipart 바이너리 투영은
동일 스키마여도 서로 다른 의미 계획으로 관리한다.

### 4.4 정규식

선택된 엔진의 `Compile(pattern, "u")`를 사용한다. 스키마의 고정 패턴은 생성 준비 때
문법을 검증하고, 생성 런타임에서는 해당 패턴을 처음 사용할 때 `sync.Once`로 컴파일한다.
패턴별 결과는 불변으로 공유한다. 패키지 초기화에서 모든 패턴을 컴파일하지 않는다.
동적 `format: regex` 문자열은 호출별로 컴파일하고 전역 무제한 캐시에 넣지 않는다.

엔진의 `ErrComplexity`는 불일치로 삼지 않고 검증 실행 실패로 전달한다.
정규식은 엔진의 유한 step budget을 따른다. `context`는 검사 전후와 재귀·항목 경계에서
확인한다. 진행 중인 동기 정규식을 즉시 중단한다는 시간 보장은 하지 않는다.
이 계약은 TypeScript의 동기 검증도 즉시 선점하지 못한다는 경계와 맞춘다.
지원되는 정규식을 RE2 제한 때문에 생성에서 생략하지 않는다.

Unicode 속성 표는 선택 버전이 내장한 Unicode 17.0.0으로 고정한다. 현재 프로젝트의
Node도 Unicode 17.0·ICU 78.3을 사용하며 새 script 표본으로 대조했다. 다른 JavaScript
런타임의 Unicode 버전 차이를 Go 도구 버전에 따라 자동 추종하지 않는다. 의존성 업데이트는
문법·속성 표의 동등성 검증을 다시 수행한다.

### 4.5 열거형·상수·불변 값

`types.Value`는 비공개 JSON tree를 가진 불변 값이다. 제로값은 JSON null이며 누락은
`Optional[Value]`로 구분한다. `ParseValue([]byte) (Value, error)`와
`ValueFrom[T any](T) (Value, error)`의 생성 코덱으로 구성한다. 후자는 package 함수이며 Go의
generic method를 요구하지 않는다. 공개 배열·객체 접근자는 복사본을 반환한다. 외부 map·
slice를 생성자에 넘기거나 접근자 결과를 수정해도 보관된 값은 변하지 않는다. `ValueFrom`
대상은 JSON 값·생성 DTO이며 binary source·reader·참조 순환·비유한 수는 오류다.

동종 string·boolean·number enum과 스칼라 const는 이름 있는 기반 타입과 Go 상수를
생성한다. nullable 스칼라 스키마는 기존 `Nullable` 투영을 적용한다. 서로 다른 JSON 종류가
섞이거나 객체·배열을 포함하면 이름 있는 `Literal` wrapper를 생성하고,
`LiteralNull()`, `LiteralStringA()`, `LiteralNumber1()`, `LiteralObject1()`처럼
스키마 순서·공개 명명 규칙으로 정한 생성 함수를 제공한다. 객체·배열 const도 같은 방식이다.
가변 전역 값이나 Go `const`에 객체·배열을 넣지 않는다.

모든 열거형은 `LiteralValues() []Literal`, `IsLiteral(Value) bool`,
`ParseLiteral(Value) (Literal, error)`, `Equal(Literal) bool`에 대응하는 이름 있는 API를
가진다. 스칼라의 Equal도 생성 타입의 메서드다. 값 목록은 매번 새 slice이고 복합 값은
불변 tree를 공유할 수 있다. 객체는 키 순서와 무관하게 동일한 정확한 키 집합·재귀 값을
비교하고 배열은 순서·길이를 비교한다. null·문자열·boolean·숫자를 서로 변환하지 않는다.
숫자는 binary64 값으로 비교하며 -0과 0은 같다. 다른 스키마 제약도 별도 검증한다.
스키마 identity가 다른 같은 모양 enum은 공개 타입을 합치지 않는다.

## 5. 호출·응답·오류·소유권

확정한 사용 형태는 다음과 같다. 예제는 구현할 API 계약이며 현재 생성 결과가 아니다.

```go
api, err := sdk.NewClient(sdk.ClientOptions{
    BaseURL: "https://api.example.test/v1",
})
if err != nil {
    return err
}

todo, err := api.Todos.ByID("todo-1").Get().Do(ctx)
created, err := api.Todos.Create(sdk.CreateTodoInput{
    Body: sdk.CreateTodoBody{Title: "Write documentation"},
}).Do(ctx)

result, err := api.Operations.GetTodo(sdk.GetTodoInput{
    Path: sdk.GetTodoPath{TodoID: "todo-1"},
}).Raw(ctx)
```

`NewClient`는 설정만 검사하고 HTTP 요청을 보내지 않는다. 리소스 선택자도 통신하지 않는다.
`Routes`에는 `GetTodosByID` 같은 구체적인 메서드를 생성하고, 원래의 정확한 메서드·경로는
문서와 작업 참조에 유지한다.

공개 이름은 원문 이름을 Go export 식별자로 정규화하고 안정된 경로 순서로 예약한다.
키워드·숫자 시작·예약 helper·정규화 충돌은 원래 경로의 SHA-256 접미사로 해결한다.
접미사는 8자리부터 시작해 충돌할 때 4자리씩 늘린다. 선택 집합에 따라 이름이 바뀌지
않도록 전체 문서의 공개 이름 계획을 먼저 계산한다. 원래 필드명은 wire 이름으로 보존한다.
원래 작업 식별자가 없거나 충돌해도 정확한 route 참조는 항상 유지한다. 원문 중복 ID의
기존 문서 오류를 임의 이름 변경으로 숨기지 않는다.

### 5.1 요청 객체

- 요청 객체와 `WithOptions`는 새 값을 반환한다. 옵션의 헤더·맵은 복사하고
  클라이언트 설정은 생성 때 복사한다. 공유 클라이언트 설정을 요청 중 변경하지 않는다.
- DTO·바이트·사용자 코덱·인증 제공자는 호스트 소유다. 실행 중 입력을 수정하지 않는다.
  SDK는 입력 DTO를 수정하지 않는다. 전체 입력의 깊은 복사를 기본 비용으로 추가하지 않는다.
- 버퍼형 요청은 입력을 수정하지 않는 조건으로 재사용·동시 실행할 수 있다.
  실행마다 취소·응답·검증 평가 상태를 새로 만든다.
- 바이너리는 `[]byte`와 `BinarySource`를 제공한다. source는 이름·미디어 타입·길이와
  `Open(ctx) (io.ReadCloser, error)`를 가진다. 외부 reader를 감싼 one-shot source는
  공유된 atomic 소비 상태로 두 번째 실행을 거부한다. 복사한 요청도 이 상태를 공유한다.
- 재실행 가능한 스트림 입력은 매번 새 `Source[T]`를 여는 factory를 사용한다.
  실패·취소 때 source와 pipe를 닫는다. 기본 SDK는 요청을 자동 재시도하지 않는다.

바이너리 입력은 비공개 tag를 가진 `types.Binary`로 통일한다.
`BinaryFromBytes([]byte) Binary`와 `BinaryFromSource(BinarySource) Binary`로 구성한다.
bytes 생성자는 깊은 복사 없이 호스트 바이트를 빌리므로 실행 중 수정하지 않는 규칙을
따른다. Binary의 제로값은 빈 bytes다. buffered 바이너리 응답은 소유한 `[]byte`를
반환하고 순차형 Raw는 읽지 않은 Body 핸들을 반환한다. JSON 안의 string·base64 값은
문자열로 유지한다. multipart 등 미디어 계획이 바이너리로 투영하는 필드만 Binary를 쓴다.

```go
type BinaryMetadata struct {
    Filename string
    ContentType string
    Size Optional[int64]
}
type BinarySource interface {
    Metadata() BinaryMetadata
    Open(context.Context) (io.ReadCloser, error)
}
type SourceFactory[T any] func(context.Context) (Source[T], error)
```

Size 누락은 알 수 없음, 0은 빈 본문, 음수는 오류다. Filename·ContentType의 빈 값은
미지정이며 파트 명세·호출 옵션의 기존 우선순위로 결정한다. `io.Reader`를 직접 받는
편의 함수 `BinaryFromReader(io.ReadCloser, BinaryMetadata) Binary`는 공유 atomic
상태를 가진 one-shot source를 만든다. Open에 성공한 reader만 SDK가 소유한다. Open이
reader와 오류를 함께 반환하면 reader도 닫아 누수를 막고 실패한다. 아직 열지 않은 source의
자원은 호스트가 관리한다. 길이가 알려진 경우 실제 전송 바이트와 일치하는지 검사한다.

### 5.2 출력과 Raw

단일 성공 본문 계약이면 `Do`는 그 DTO를 반환한다. 본문이 없으면 `NoContent`를 반환한다.
서로 다른 성공 본문이면 작업별 출력 union과 `As200JSON`·`As2XXJSON`·`AsDefaultJSON`
형태의 접근자를 생성한다. 미디어 접미사는 `JSON`, `XML`, `Text`, `Binary`를 사용하고,
그 외에는 정규화한 미디어 이름과 충돌 접미사를 사용한다.

응답 선택은 정확한 상태, 상태 범위, default와 미디어 우선순위를 현재 계획과 공유한다.
`2XX`가 덮는 범위에서 default 오류 본문을 성공 타입에 잘못 섞지 않는다. 정상 HTTP
상태는 `Do`·`Raw`·Stream·페이지 조회에서 동일한 규칙을 따른다.

`Raw`는 작업별 결과 타입으로 상태·선택 미디어·타입이 있는 헤더·본문과
`*http.Response`·요청 메타데이터를 제공한다. 일반 응답은 본문을 읽고 닫은 상태다.
순차형 응답의 Raw는 본문을 읽지 않고 핸들이 소유한다. 이때 `Close()` 또는 EOF가
자원과 요청별 cancel을 해제한다. 반환 직전에 `defer cancel()`로 본문을 끊지 않는다.
Raw 핸들의 `Close()`는 여러 번 호출해도 안전하다. 유한하지 않은 본문을 끝까지 drain해
연결을 재사용하려고 하지 않는다.

`x-envelope`는 일반 호출의 값만 지정한 투영으로 바꾸고 Raw의 전체 본문은 유지한다.
각 단말 호출은 새 HTTP 요청이다. 같은 요청 객체의 `Do`와 `Raw`가 응답을 공유하지 않는다.

union의 `As...` 결과는 보관된 불변 wire 값에서 만든 독립 DTO snapshot이다. 내부 map·
slice·포인터를 공유하지 않는다. 같은 분기의 반복 조회에도 수정 가능한 cache를 노출하지
않는다. 분기를 수정한 뒤 union 자체를
갱신하려면 생성된 `With...` 메서드로 새 값을 만든다. 이 메서드는 해당 분기의 원래 키만
갱신하고 무관한 추가 속성을 보존하며, 이전 분기 projection cache를 공유하지 않는다.
일반 DTO 필드는 직접 수정할 수 있다. 값을 수정해도 네트워크로 자동 저장하지 않는다.

### 5.3 오류

공통 `APIError`에는 코드·범주·작업·요청 정보와 `Unwrap()`을 제공한다. HTTP 오류는
작업별 생성 타입과 상태·미디어별 본문 접근자로 좁힌다. 취소·시간 제한은 기존
`REQUEST_ABORTED`·timeout 코드 계약을 유지하고 원인을 `errors.Is`로 확인할 수 있게 한다.
기존 인증 선택·인증 정보 코드도 유지한다. 전송·인코딩·디코딩·검증 오류를 HTTP 오류로
위장하지 않는다. body source 재소비와 검증 실행 한계는 별도 SDK 오류 코드로 구분한다.
진단·로그에 토큰·자격 증명·임의 본문을 추가하지 않는다.

현재 전송 코드의 `REQUEST_ENCODE_FAILED`, `NETWORK_ERROR`, `REQUEST_ABORTED`,
`REQUEST_TIMEOUT`, `RESPONSE_DECODE_FAILED`, `SECURITY_REQUIREMENT_REQUIRED`,
`SECURITY_REQUIREMENT_INVALID`, `SECURITY_CREDENTIALS_REQUIRED`,
`SECURITY_CREDENTIALS_INVALID`, `TRANSPORT_CAPABILITY_REQUIRED`를 그대로 사용한다.
새 Go 계약의 세부 원인은 `REQUEST_BODY_CONSUMED`, `VALIDATION_LIMIT_EXCEEDED`,
`OPERATION_NOT_PREPARED`, `STREAM_ALREADY_CONSUMED`, `REQUEST_OPTIONS_INVALID`다.
인코딩·디코딩 실패 안에서 발생했다면 바깥 단계 코드는 기존 값을 유지하고 `Unwrap()`의
타입이 있는 원인에서 세부 코드를 확인하게 한다. HTTP 오류의 서버 정의 코드는 덮어쓰지 않는다.

### 5.4 입력과 옵션의 호출 규칙

필수 입력 영역이 있으면 메서드는 구체 입력 하나를 받는다. 남은 입력이 전부 선택이면
`input ...GeneratedInput`을 받고 0개 또는 1개만 허용한다. 입력 영역이 전혀 없으면
무인자 메서드다. 리소스 선택자가 이미 바인딩한 경로 필드는 리소스 입력에서 제외한다.
옵션은 `request.WithOptions(RequestOptions{...})`로 전달한다. 입력과 옵션을 같은
variadic 인자에 섞지 않는다. 생성자에서 발견한 입력 개수·옵션 오류는 요청 객체에 보관하고
단말 호출에서 반환하며 HTTP 요청을 보내지 않는다.

`RequestOptions`는 `BaseURL`, `Accept`, `Headers`, `Authorization`, `Credentials`,
`CSRFToken`, `RequestID`, `Timeout`, `MultipartHeaders`, `MultipartContentTypes`,
`StreamCodec`, `MaxStreamFrameBytes`와 작업별 타입이 있는 `SecurityRequirement`를
제공한다. context는 옵션에 저장하지 않고 단말 호출에서 받는다. `Timeout`은
`Optional[time.Duration]`이며 누락은 클라이언트 설정 상속, 0 이하는 오류다.
`MaxStreamFrameBytes`도 누락과 잘못된 0을 구분한다. 클라이언트 timeout 기본은 제한 없음,
프레임 기본은 1 MiB다. 호출 context의 deadline과 SDK 제한 중 먼저 끝나는 제한이 적용된다.

`ClientOptions`에는 공통 옵션에 더해 `Origin`, `Server`, `HTTPClient`, `Transport`,
`SecurityProvider`, `Codecs`, `StreamCodecs`가 있다. 인증 제공자는
`func(context.Context, SecurityCredentialContext) (SecurityCredentials, error)`로 호출한다.
`SecurityCredentialContext`에는 원래 route·작업 ID·메서드·경로, 이미 선택된 보안 요구와
최종 요청 origin을 제공한다. 단일 보안 요구는 SDK가 선택하고 복수 대안은 호출자가
명시적으로 선택한다. 제공자는 이미 선택된 요구의 인증 정보만 반환한다. 직접 제공한
인증 정보의 충족 여부와 제공자 호출·충돌 규칙은 기존 보안 계획대로 처리한다.
호스트 제공자·코덱의 동시 사용 안전성은 호스트 계약이다. SDK가 제공자 전체를 전역 잠금으로
직렬화하지 않는다.

공통 `RequestOptions.SecurityRequirement`는 생성 작업별 선택 타입이 구현하는
`SecurityRequirement` 인터페이스를 받으며 실행 전에 해당 작업의 선언과 대조한다.
작업 객체의 `WithSecurityRequirement(GeneratedSecurityRequirement)`는 선택 타입을
메서드 인자로 한정한다. 생성한 상수의 값은 기존 scheme 기반 ID를 유지하되 전체 ID
점유를 검사해 `__2`가 포함된 scheme과 실제 `anonymous` scheme도 서로 구별한다.

## 6. HTTP·인증·미디어

기본 전송은 재사용하는 `*http.Client`와 Transport다. `ClientOptions.HTTPClient`로
주입하거나 명시적인 `Transport` 구현을 제공할 수 있다. 둘을 동시에 지정하면 오류다.
전송 계약은 `Do(*http.Request, TransportOptions) (*http.Response, error)`와
`Capabilities()`로 고정한다. request의 context를 전송과 인증·코덱에 전달한다.

`TransportOptions`에는 자격 증명 모드·origin·리다이렉트 허용 여부를 전달한다.
표준 어댑터는 호출별로 HTTP 클라이언트의 옵션을 구성하면서 연결 Transport는 공유한다.
공유 Jar·Transport의 필드를 요청 중 변경하지 않는다. 사용자 Transport는 선언한 능력과
이 옵션 계약을 지킨다. SDK 자체의 응용 계층 재시도는 없다. 주입한 전송의 재시도·리다이렉트
동작은 그 전송의 계약이며, one-shot 본문을 SDK가 재생하지 않는다.

| TypeScript 기능 | Go 동작 |
| --- | --- |
| Fetch·사용자 정의 전송 | 표준 HTTP 어댑터·명시적 Transport |
| AbortSignal·timeoutMS | `context.Context`, 요청별 `time.Duration` 제한 |
| credentials | `Omit`·`SameOrigin`·`Include`의 ambient cookie 정책 |
| Cookie 직접 전송·읽을 수 있는 헤더 | 표준 Go 어댑터가 명시적으로 제공하는 능력 |
| mutualTLS | 호스트가 인증서를 설정한 전송과 명시적 능력 선언 |
| 서버·origin·서버 변수 | 기존 우선순위·변수 제약 유지 |
| 보안 요구의 대안·복수 scheme | 기존 AND/OR 의미와 인증 충돌 검사 유지 |

자격 증명 모드 기본값은 `SameOrigin`이다. Go에는 브라우저의 ambient origin이 없으므로
same-origin cookie jar 사용은 `Origin`이 명시된 경우에만 허용한다. `Include`는
Jar를 사용하고 `Omit`는 사용하지 않는다. 명시적으로 전달한 명세상 cookie·인증 정보는
ambient cookie와 구분하고 기존 충돌 규칙으로 검사한다. mTLS·토큰 발급은 SDK가 추측하지
않는다. 자격 증명을 적용한 요청은 현재 런타임과 같이 리다이렉트를 거부한다.

헤더 이름은 HTTP 규칙대로 처리하지만 JSON 속성명은 정확히 구분한다. 경로·쿼리·
querystring·헤더·cookie의 style, explode, allowReserved, allowEmptyValue,
content 기반 인코딩과 매개변수 상속을 공통 의미 계획에서 처리한다. SDK 관리 헤더와
명세가 소유한 헤더의 충돌은 생성·호출 경계에서 기존과 같이 검사한다.

Go 표준 전송은 Fetch의 금지 메서드 규칙을 상속하지 않는다. 유효한 HTTP 토큰 메서드는
표현할 수 있다. CONNECT 터널 API 등 별도의 프로토콜 인터페이스는 추가하지 않는다.
Go에서 HTTP 요청으로 표현 가능한 기능에 TypeScript의 Fetch 생략 규칙을 기계적으로
적용하지 않는다. 이 타깃 차이는 배정표의 `compat.method.fetch-capability`에 명시한다.

JSON·텍스트·바이너리·폼·XML·멀티파트와 사용자 코덱을 모두 구현한다. XML은 namespace,
이름 투영, 속성·텍스트·nodeType, 합성 분기의 매핑을 유지한다. 단순 구조체 XML 태그에
전체 의미를 위임하지 않는다. 멀티파트의 파트별 헤더·미디어·바이너리 투영·비동기 source를
유지하고, 대형 파일을 통째로 메모리나 임시 파일에 읽는 기본 경로는 두지 않는다.
파트 프레임은 한 번 준비하고 pull reader로 순차 송신한다. 모든 파트의 길이를 알면
선읽기 없이 전체 길이를 계산하고, 모르는 길이나 합계 overflow는 미정 길이로 전송한다.
중단 시 진행 중인 Open의 context와 reader·source를 정리한다.

multipart DTO의 바이너리는 `types.Binary`이며 문자열은 `BinaryFromString`으로 만든다.
`Binary.Validate()`는 source를 열지 않고 nil·잘못된 metadata를 검사한다. source의 실제
Open·Read 오류는 소비 시 전파한다. 자유 형식 multipart는 `MultipartFromValue`·
`MultipartFromBinary`·`MultipartFromObject`·`MultipartFromArray`로 구성한다.
`MultipartValue`와 `MultipartData`는 JSON 값과 경로별 source 슬롯을 보존하며 모델의
`EncodeMedia`·`DecodeMedia`로 변환한다. 경로가 없거나 겹치거나 DTO로 보존할 수 없는
source는 오류다. JSON 직렬화로 source를 버리지 않는다. JSON 미디어의 같은 component는
기존 `types.Value` 계약을 유지한다. 유한 multipart 응답은 기존 TypeScript와 같이
사용자 Decode 코덱을 사용하며 입력의 바이너리 투영을 출력 DTO에 적용하지 않는다.

전체 미디어 코덱은 encode·decode·parameter encode/decode·inbound decode 함수를
각각 선택적으로 등록하는 구조체다. 모든 함수는 context와 선택 미디어 정보를 받는다.
미구현 방향을 호출하면 기존 미디어 계약의 encode/decode 오류다. 일반 레지스트리의
값 표현은 `types.Value`이고, 생성 DTO와의 변환은 해당 작업의 코덱이 담당한다.
바이트 입력·출력은 `BinarySource`·`io.ReadCloser`를 사용하며 본문 소유권은 SDK가 가진다.
매개변수 방향은 문자열을 사용한다. 동기 함수만으로도 사용할 수 있고 HTTP·파일 I/O가
필요한 구현은 전달받은 context를 사용한다.

정확한 공개 코덱 계약은 다음과 같다. `MediaContext.ContentType`은 매개변수를 포함한
선택 미디어이고 `Request`·`Response`는 복사한 헤더·URL·메서드·상태·프로토콜·trailer
정보와 본문을 가진다. 실제 Go 표준 타입을 사용해 사용자 처리에 필요한 정보를 빠뜨리지 않는다.

```go
type MediaContext struct {
    ContentType string
    Request *http.Request
    Response *http.Response
}
type MediaCodec struct {
    Encode func(context.Context, Value, MediaContext) (BinarySource, error)
    Decode func(context.Context, *http.Response, MediaContext) (Value, error)
    EncodeParameter func(context.Context, Value, MediaContext) (string, error)
    DecodeParameter func(context.Context, string, MediaContext) (Value, error)
    DecodeInbound func(context.Context, *http.Request, MediaContext) (Value, error)
}
```

함수 필드가 nil이면 그 방향은 미지원이다. 빈 본문은 빈 BinarySource로 표현하며 nil
source의 성공 반환은 인코딩 오류다. Encode의 source는 실제 전송 때 한 번 열고 SDK가
종료한다. Decode·DecodeInbound에 전달한 Body는 빌린 본문이며 callback은 소비할 수
있지만 보관·백그라운드 소비·직접 Close하지 않는다. SDK가 성공·오류·취소에서 정확히
한 번 닫는다. metadata는 hook용 snapshot이라 헤더 수정을 실제 요청·응답에 반영하지
않는다. 본문을 SDK에서 미리 문자열로 바꾸거나 읽은 뒤 callback을 호출하지 않는다.
반환 Value를 스키마 검사하고 DTO로 변환한다. hook 오류는 해당 단계의 encode/decode
오류로 감싸고 원인은 Unwrap으로 유지한다. parameter 방향은 HTTP 본문을 갖지 않는다.
EncodeParameter의 Request는 선택한 서버와 현재까지 준비한 경로·쿼리·헤더를 담은
작성 중 요청이다. 아직 처리하지 않은 경로 변수는 남을 수 있다. DecodeParameter에는
응답 metadata를 전달한다. Decode·DecodeInbound의 함수 인자와 MediaContext는 서로
독립된 metadata 컨테이너로 같은 빌린 Body를 읽는다. EOF에 확정된 trailer는 이 snapshot에
반영한다. hook이 snapshot을 수정해도 원본 HTTP 객체는 바뀌지 않는다.

생성 시 multipart 파트 질의 계획이 참조·대안·조건부·패턴·추가 속성·위치를 직접 함수로
낮추고 XML 계획은 XML 매핑만 담당한다. 요청 실행 중 스키마 문서를 해석하지 않는다.
XML의 임의 추가 속성 및 prefixItems 매핑은 기존 TypeScript의 제한을 유지한다.

### 6.1 압축 응답과 주입 전송

Fetch와 같이 본문은 해제한 바이트이고 `Content-Encoding`·`Content-Length` 헤더는
원래 응답 값을 유지한다. Raw·Link·미디어 코덱·선언된 응답 헤더 검증에 모두 적용한다.
표준 `http.Transport`의 자동 gzip 해제는 헤더를 지우므로 그대로 사용하지 않는다.
SDK 소유 Transport는 생성 때 한 번 Clone하고 `DisableCompression=true`로 고정한다.
명시적 Accept-Encoding이 없으면 HTTP에는 `gzip, deflate`, HTTPS에는
`br, gzip, deflate`를 넣고 Range 요청은 `identity`를 사용한다. 이것은 현재 Node Fetch
기본값이다. 헤더 값은 전송을 재현할 metadata에도 보존한다.

gzip·x-gzip은 표준 `compress/gzip`, zlib-wrapped·raw deflate는 앞부분을 보존하는
reader와 표준 `compress/zlib`·`compress/flate`로 해제한다. br은
`github.com/andybalholm/brotli@v1.2.6`, zstd는
`github.com/klauspost/compress/zstd@v1.20.3`을 사용한다. Node가 해제하는 zstd도
응답에서 지원하되 기본 Accept-Encoding 목록에는 추가하지 않는다. coding은 오른쪽부터
해제하고 하나라도 알 수 없는 coding이면 전체 체인을 그대로 전달한다. HEAD·204·304처럼
본문이 없는 응답은 해제하지 않는다. Go Response.ContentLength는 해제 후 알 수 없는
길이 -1로 두되 Header의 원래 길이는 남긴다. `Uncompressed=true`로 바이트 상태를 알린다.

압축 reader는 pull 방식이고 각 응답이 소유한다. zstd는 decoder concurrency 1로
설정하여 기본 응답 경로에서 read-ahead worker를 만들지 않는다. 닫기·취소·중간 디코딩
오류는 decoder와 원본 Body에 함께 전달한다. 프레임·서버 본문 제한은 해제 후 바이트에
적용하며 압축 크기로 우회할 수 없다. br·zstd 의존성은 기본 HTTP 어댑터의 비용이며
정규식의 조건부 의존성과 구분해서 설치 안내·라이선스·빌드 측정에 포함한다.

주입한 HTTPClient는 값으로 복사하고 원래 Jar·Timeout·CheckRedirect 설정을 보존하면서
호출 옵션을 적용한다. 연결 풀은 공유한다. SDK가 명시적 Accept-Encoding을 넣으므로
표준 Transport는 자동 해제를 하지 않아 사용자 Transport를 수정할 필요가 없다.
사용자 RoundTripper가 헤더를 제거하는 자동 해제를 한다면 HTTPClient 주입 대신
`Transport`를 사용하고 원본 헤더를 보존한 decoded response를 반환해야 한다.
`TransportCapabilities.ResponseBody`는 `Encoded` 또는 `DecodedWithOriginalHeaders`로
필수 선언한다. 앞의 경우 SDK가 해제하고 뒤의 경우 중복 해제하지 않는다. HTTPClient
경로에서 이미 `Uncompressed=true`인 응답을 받으면 원래 헤더 계약을 확인할 수 없으므로
`TRANSPORT_CAPABILITY_REQUIRED`로 거부한다. 조용히 손실된 헤더를 만들어내지 않는다.
주입한 HTTPClient.Timeout과 context·SDK timeout 중 먼저 끝나는 제한이 적용된다.

## 7. 스트림·페이지 조회·Link

`request.Stream(ctx)`는 통신하지 않는 `*Stream[T]`를 반환한다. `Next()` 또는
`Response()`가 최초 실행을 시작한다. `Response()`는 헤더가 도착하면 결과를 반환하고
데이터 소비권을 획득하지 않는다. `Next()`·`Item()`·`Err()`·`Close()`로 읽는다.
핸들은 `Source[T]` 계약을 구현하므로 다른 코덱·처리 단계에 연결할 수 있다.
단일 핸들의 동시 소비는 지원하지 않는다. 한 핸들에서 Next를 겹쳐 호출하면
`STREAM_ALREADY_CONSUMED`로 핸들을 실패 종료하고 진행 중인 읽기도 취소한다.
Item은 직전 Next가 true인 동안만 유효하며 다음 Next와 동시에 읽지 않는다.
Response는 반복·동시 호출할 수 있고 같은 metadata 또는 시작 오류를 반환한다.
Close는 어느 goroutine에서든 호출할 수 있는 멱등 해제다. 시작 전 Close는 통신하지
않고, 진행 중 읽기를 취소한다. 명시적 Close 자체는 Err를 만들지 않으며 이미 발생한
오류는 보존한다. 닫힌 핸들의 Next는 false다. 헤더 전에 닫혔다면 Response는
`REQUEST_ABORTED`와 닫힘 원인을 반환한다. 외부 context 취소·timeout은 Err에 남는다.
읽기 중 Close로 발생한 취소와 외부 취소를 원인으로 구분한다.

SSE·NDJSON·JSON-seq·순차 멀티파트를 청크 경계와 무관하게 파싱한다. SSE의 event,
data, id 지속·초기화, retry, BOM·줄바꿈 의미를 유지한다. 기본 SSE는 임의로 data를
JSON으로 바꾸지 않는다. `contentSchema`는 내부 내용을 검사해도 공개 문자열을 유지한다.
프레임 오류·항목 검증 오류·시간 제한·전송 오류를 구분한다.

프로토콜은 wire frame을 처리하고 어댑터는 애플리케이션 항목으로 변환한다. 전체 미디어
코덱과 스트림 코덱은 별도 계약이다. 요청별 코덱이 클라이언트 기본값보다 우선한다.
사용자 프로토콜에도 같은 최대 프레임 크기와 취소 context를 전달한다.

공개 반복자와 스트림 확장 계약의 형태를 고정한다.

```go
type Source[T any] interface {
    Next() bool
    Item() T
    Err() error
    Close() error
}

type StreamReader interface {
    Read(maxBytes int) ([]byte, error)
    Close() error
}

type StreamProtocol[Frame any] interface {
    Decode(context.Context, StreamReader, StreamContext) (Source[Frame], error)
    Encode(context.Context, Source[Frame], StreamContext) (io.ReadCloser, error)
}

type StreamAdapter[Frame, Item any] interface {
    Decode(context.Context, Source[Frame], StreamContext) (Source[Item], error)
    Encode(context.Context, Source[Item], StreamContext) (Source[Frame], error)
}
```

`StreamContext`에는 boundary 등 parameter를 포함한 실제 Content-Type과 최대 프레임
바이트가 들어간다. 레지스트리는 parameter를 제외한 미디어 이름으로 조회한다. 실제 context는
함수 인자다. `StreamReader.Read`는 요청한 상한을 넘기지 않고 EOF는 `io.EOF`로
표현한다. 돌려준 바이트는 다음 Read까지 유효하다. 기본 프로토콜은 프레임 누적 상한도
검사한다. typed `NewStreamCodec` 등록 함수가 프로토콜·어댑터의 타입 일치를 검사하고
레지스트리에 넣을 소거된 wrapper를 만든다. 내부 소거를 사용자 DTO의 `any` 반환으로
확대하지 않는다. 기본 송수신은 pull 방식이며 pipe·writer goroutine·선읽기와 무제한
queue를 만들지 않는다. 사용자 확장의 개별 프레임·집계 비용은 해당 확장이 관리한다.

현재 기본 최대 프레임은 1 MiB다. 제한은 어댑터 처리 전 wire frame·record·part의
바이트 수에 적용한다. 전체 값을 모으는 호출과 항목별 호출을 구분하고, 전체 값 처리까지
상수 메모리라고 주장하지 않는다. 사용자 어댑터의 명시적 집계 상태도 별도 비용이다.

요청 미디어가 itemSchema만 선언하면 body 타입은 `SourceFactory[Item]`이다. schema와
itemSchema가 함께 있으면 작업별 `GeneratedBody` tagged wrapper와
`NewCompleteBody(ValueType)`·`NewStreamBody(SourceFactory[Item])` 생성자를 제공한다.
schema만 있으면 기존 complete DTO 타입이다. 런타임 `any` 검사로 두 형태를 추측하지
않는다. nil factory·정해지지 않은 wrapper의 제로값은 인코딩 오류다. one-shot Source를
쓰는 편의 factory도 복사본 전체가 소비 상태를 공유한다. 서버의 incremental body는 이미
열린 `Source[Item]`이며, SDK가 요청 처리 lease를 소유한다. handler 오류·취소 또는
응답 송신 종료 때 닫는다. 응답 source가 수신 source를 변환해 쓰더라도 핸들러 반환만으로
조기 종료하지 않는다. response source는 EOF·오류·취소 때 함께 닫는다.

전체 응답은 `schema`의 배열·prefixItems와 `prefixEncoding`/`itemEncoding`을 사용한다.
증분 응답은 `itemSchema`와 `itemEncoding`만 사용한다. 두 스키마가 서로 다를 수 있으며
전체 값을 모으면서 증분 항목 스키마를 잘못 적용하지 않는다. 파트의 실제 Content-Type이
없으면 해당 encoding의 첫 contentType, 그것도 없으면 `text/plain`을 사용한다.
필수 파트 헤더와 값 검증은 native framing 단계에서 어댑터보다 먼저 수행한다.
사용자 protocol은 framing을 대체하고, adapter는 native 또는 사용자 frame을 변환한다.

JSON/SSE의 native Frame은 `types.Value`, multipart는 소유한 `Headers`·`Data`가 있는
`types.MultipartPart`다. JSON DTO의 공개 스키마 정체성은 유지한다. native multipart가
바이너리를 반환하면 JSON 값과 별도 바이트 슬롯을 가진 `MultipartData`로 보존한다.
이때 작업별 StreamItem과 전체 Output은 sealed wrapper이며 `AsMultipart`로 컨테이너를
조회한다. 최상위 바이너리 항목의 `AsBinary`와 `Binary.Bytes`는 복사본을 반환한다.
`Binary.Bytes`는 source를 열지 않고 버퍼 여부를 함께 반환한다. 바이너리를 포함하는 값을
JSON 전용 `AsStatus` 조회로 바꾸면 성공으로 처리하지 않는다. 중첩 값은 `At`/`Get`으로
같은 방식으로 조회한다. TypeScript ArrayBuffer와 같이 바이너리의 JSON Schema 검증용
필드는 빈 객체이며, 문자열·배열 스키마로 잘못 수용하지 않는다.

기본 incremental multipart 송신은 현재 파트만 준비하고 파일 source를 헤더 소비 뒤에
연다. complete multipart는 알려진 파트 metadata·헤더·custom media codec 결과를
전송 전에 검증하되 파일을 읽지 않는다. byte Frame을 요구하는 명시적 stream codec만
프레임 상한 안에서 파트를 버퍼링한다. 고정 길이는 읽기 없이 계산할 수 있을 때 보존한다.
기본 frame 상한은 파트 본문에 적용하고 바깥 스트림 헤더는 별도 8192바이트 상한을
적용한다. 이미 상한 안에 들어온 중첩 응답 payload에는 더 작은 헤더·frame 기본값을
추가하지 않는다. 현재 TypeScript의 중첩 유한 미디어 처리와 같은 경계다.

factory lease는 자신의 child context를 먼저 취소하고 source를 한 번 닫는다.
`Source.Next`는 단일 소비자이며 `Close`는 다른 goroutine에서 진행 중인 Next를 중단할
수 있어야 한다. SDK 정리로 생긴 취소가 정상 EOF나 앞선 실제 실패를 바꾸지 않는다.
시작·protocol·adapter 생성 실패와 정리 실패는 원인을 함께 보존한다. typed 응답은
네트워크의 bytes+EOF 뒤 남은 프레임까지 파싱·검증한 후 실행 수명을 종료한다.

`Paginate(ctx)`도 지연 시작한다. `Next`·`Item`·`Err`·`Close`로 개별 항목을 제공하고
현재 페이지와 다음 요청 상태를 보관한다. 임의의 과거 cursor·offset 반복을 종료하기 위한
방문 집합은 순회 동안 유지하며 Close에서 페이지·방문 집합·callback을 해제한다. cursor·offset·both의 입력·응답 경로,
같은 cursor 반복·빈 페이지·마지막 페이지 종료 규칙을 유지한다.

### 7.1 Link 호출

`api.Links.CreateTodo.GetTodo.Follow(source, invocation).Do(ctx)`로 후속 요청을 만든다.
Follow는 통신하지 않는다. `source`는 원본 작업 전용 sealed `CreateTodoLinkSource`이며
그 작업의 Raw 결과와 HTTP 응답을 가진 생성 오류 타입이 구현한다. `errors.As`로 얻은
HTTP 오류도 그대로 사용할 수 있다. 응답이 없는 네트워크·인증 오류는 source가 될 수 없다.
원본 상태·원래 헤더·URL·전체 decoded body를 사용하며 envelope projection은 적용하지
않는다. 순차형 Raw 본문을 Link가 암묵적으로 소모하지 않는다. response body 표현식이
필요한 경우에만 아직 materialize되지 않은 source를 후속 통신 전에 오류로 거부한다.

```go
type LinkInvocation[SourceInput, TargetOverride, Options any] struct {
    SourceInput Optional[SourceInput]
    Input Optional[TargetOverride]
    Options Optional[Options]
}
```

`SourceInput`은 원본의 구체 입력 타입이다. `$request.body`·path·query·header·cookie
표현식은 여기서 읽는다. 소비된 요청 본문을 다시 열거나 매 호출의 DTO를 응답에 보관하지
않는다. 호출자는 필요할 때 원래 입력을 제공한다. 없는 source 값은 undefined에 해당하는
누락 값으로 전파하고 대상 스키마가 필수로 요구하면 오류다. `$url`·`$statusCode`·
`$response.header`·`$response.body`는 응답에서 읽는다. JSON Pointer escaping·정확한
속성명·배열 index 검사를 현재 runtime expression과 공유한다.

TargetOverride는 별도 생성 타입이다. 각 매개변수 영역과 그 안의 각 필드를 Optional로
감싸 field 단위 덮어쓰기를 표현한다. body도 누락과 명시 값을 구분한다.
명시적인 0·false·빈 문자열·null도 유도한 값보다 우선한다. 부모 section은 한 단계 병합하고
body 내부 객체도 현재 TS merge와 같이 한 단계 병합한다. 그러므로 객체 body override는
필드를 Optional로 만든 별도 BodyOverride다. 배열·스칼라 body는 전체 교체한다.
입력·유도 값은 수정하지 않는다. nullable 객체 body는 null이면 전체 교체하고 non-null
객체일 때만 한 단계 병합한다. Options는 대상 작업 전용 옵션 타입이며 원본 옵션을
자동 상속하지 않는다. Link server와 클라이언트 기본 설정의 기존 우선순위를 유지한다.

같은 Link 이름이 상태마다 있으면 `.ByStatus201.Follow`·`.ByStatus409.Follow`·
`.ByStatus2XX.Follow`·`.ByStatusDefault.Follow`를 모두 생성한다. 이는 해당 상태
definition을 명시적으로 골라 실행하는 API이며 실제 source 상태를 다른 상태로 바꾸지
않는다. 기본 Follow는 원래 상태에 정확·범위·default 순으로 definition을 고른다.
대상이 하나면 구체 대상 요청을 반환한다. 서로 다른 대상이면 그룹 전용 LinkRequest와
출력 union을 반환한다. 그룹 invocation의 Input과 Options는 상태별 tagged wrapper이고
`For201(...)` 같은 생성자로 만든다. 선택된 definition과 다른 tag는 요청 전 오류다.
상태별 Follow에는 대상의 구체 override·options·출력 타입을 제공한다. 대상에서 필수로
요구하는 options는 상태별 메서드의 required invocation 타입에서 필수 필드로 두며,
그룹 Follow도 필수인 분기가 있으면 invocation 생략을 허용하지 않고 선택 분기의 충족을
검사한다. 외부 문서의 이미 읽은 참조 범위와 비공개 Link 대상도 그대로 호출 가능하다.

### 7.2 W07의 구체 표현과 검증 경계

리소스는 공개 작업만 구성한다. `Operations`는 internal 작업도 유지하고 hidden 작업은
노출하지 않는다. `Routes`는 구체 작업 입력을 그대로 받는다. 선택자는 경로 값을 복사해
보관하고 실행할 작업의 제약을 `Do(ctx)`에서 검사한다. 문자열·숫자·boolean 인자는
기본 Go 타입을 받고 복합 인자는 명명한 모델을 받는다. 같은 경로 위치에서 Go 입력
계약을 공유할 수 없는 작업은 선택자를 만들지 않고 구체 Routes로 제공한다. 전체 문서를
기준으로 이름을 예약하므로 후속 생성 선택이 공개 이름을 바꾸지 않는다. 문장부호만 있는
literal 경로도 Go 이름 fallback과 HTTP 메서드 terminal로 처리한다.

선택적 data envelope는 data가 있으면 투영하고 없으면 전체 envelope를 유지한다.
작업별 출력 union의 `As200JSON()`·`As200JSONEnvelope()`처럼 두 경우를 구분한다.
명시적인 false·0·null은 누락과 다르다. Raw와 Link는 투영 전 전체 응답을 보존한다.

HTTP 응답은 현재 TypeScript의 `unknownProperties: preserve` 계약을 따른다.
`additionalProperties: false`·`unevaluatedProperties: false`의 미선언 속성 거부만
완화하고 scalar·required·enum 등 나머지 검사는 유지한다. anyOf·oneOf·조건의 판정은
strict match를 먼저 사용한다. DTO는 선언한 출력 속성을 제공하며 Raw·Link에는 전체
값을 보존한다. standalone 모델과 요청 검증은 strict다. 응답에서 읽은 wrapper의 typed
getter와 직렬화는 응답 profile을 유지한다. contentSchema의 공개 문자열을 임의로
논리 객체로 바꾸지 않는다.

Link의 SourceInput은 표현식에 필요한 값을 읽기 위한 자료이며 원본 작업의 제약을 다시
적용하지 않는다. 대상 입력은 대상의 제약으로 검증한다. binary·multipart 파일·항목
factory는 JSON 모양과 분리한 내부 carrier로 전달하며 Follow에서 열지 않는다.
미디어 union의 Body는 전체 선택을 교체한다. contentType 또는 value만 교체할 때는
`WithBodyContentType`·`WithJSONBodyValue`·`WithStreamBodyValue` 같은 타입별 메서드를
사용한다. 전체 Body와 부분 교체를 동시에 지정하면 요청 전에 오류다. nullable 객체는
non-null 객체끼리만 한 단계 병합한다. 배열·스칼라·native source는 전체 교체한다.
기본 Go HTTP 전송에는 browser Fetch의 금지 헤더 규칙을 적용하지 않는다. 명세가 필수로
선언한 Origin 등의 헤더도 구체 입력으로 전달할 수 있다. browser가 채우는 값의 자동
공급을 약속하지 않는다. 컴포넌트 Link·컴포넌트 응답의 참조는 이 차이와 무관하게 유지한다.

페이지 항목은 전체 응답 검증 후 읽는다. 허용된 응답들의 항목 제약이 다르면 전체 항목
계약을 anyOf 모델로 보존하여 한 응답의 제약을 다른 응답 항목에 강제하지 않는다.
단일 계약은 직접 DTO를 유지한다. x-sort 입력은 Field·Direction의 구체 enum 쌍이며
선언된 wire 문자열로만 인코딩한다. 오류 code/category 상수·목록·정적 범주 판별과
code별 typed details 조회를 제공한다. APIError.Category는 HTTP 처리 단계 구분이고
서버 오류 범주는 별도 code 집합이다. Data·Details·Fields는 명시 조회로 제공하며 오류
문자열에 본문을 노출하지 않는다. GetRequestID는 대소문자가 다른 헤더 키도 결정적인
순서로 합친다.

## 8. 선택·독립 클라이언트·메타데이터

Go는 실행 파일을 링크할 때 도달하지 않는 코드·데이터를 제거한다. TypeScript의 번들러
트리 셰이킹과 목적은 비슷하지만 별도 번들러 설정은 필요하지 않다. imported package의
소스 파싱·타입 검사·컴파일 비용은 여전히 생기므로 생성 시 선택도 유지한다. 전체 registry·
init 함수·reflection이 함수와 타입을 참조하면 링크 제거가 제한된다. 따라서 정적 참조와
동적 조회를 분리하고 registry는 Lookup 경로에서만 지연 초기화한다. 기본 클라이언트와
정적 Prepare가 전역 목록을 참조하거나 init에서 만들지 않는다. 기본 HTTP 압축 디코더는
응답 처리 경로가 참조하는 필수 비용이고, 선택한 미디어만 남는다고 오해하지 않게 한다.

생성 시 선택은 기존 `[selection]`·`--operation`·`--route` 계약을 재사용한다.
잘못된 이름·숨겨진 API·빈 집합의 오류와 독립 클라이언트가 만드는 루트 합집합도 유지한다.
Link 대상은 내부 의존으로 생성하지만 직접 호출 공개 여부는 별도로 관리한다.

정적 호출은 전체 동적 map을 사용하지 않는다. `operations.ListTodos()` 같은 함수가
타입이 있는 작업 참조를 반환한다. 참조에는 원래 ID·route와 필요한 구현만 연결한다.
선택 목록은 `selective.Prepare(refs...)`로 합치고 중복 제거한다. 이 과정은 동기적이며
네트워크를 사용하지 않는다. `selective.NewClient(options, prepared)`와
`selective.Bind(client, typedRef)`로 타입이 정해진 요청 API를 얻는다.

실행 중 계산한 ID·route 선택은 `selective.LookupOperation`·`LookupRoute`와 전체
레지스트리에서 처리한다. 이 경로는 전체 목록의 비용을 부담한다. 임의 문자열로 조회한
결과의 입력·출력은 `types.Value`를 쓰되 스키마를 검사한다. 구체적인 작업 참조로 Bind하면
그 작업의 타입이 유지된다. 준비하지 않은 참조는 HTTP 요청 전에 거부한다. Go에서 실행 중
문자열에 따라 구조체 메서드 집합을 바꾼다고 약속하지 않는다.

독립 클라이언트는 선택된 API만 가진 별도 concrete receiver와 공개 타입 alias를 생성한다.
필요한 스키마·작업 코드는 같은 생성 트리에서 공유한다. 이름 있는 진입점만 import해도
루트 클라이언트·다른 클라이언트·전체 레지스트리를 끌어오지 않는다.

메타데이터는 `--with metadata`일 때만 생성한다. 원래 source document의 JSON과
알 수 없는 확장 필드를 보존한다. 호출 패키지는 메타데이터 패키지를 import하지 않는다.

## 9. 웹훅·콜백 수신

`--with server`가 없을 때 필요한 수신 계약의 기존 문서 오류를 유지한다. 확장이 있으면
타입이 있는 핸들러와 `http.Handler`를 생성한다. 핸들러는
`func(context.Context, GeneratedInput) (GeneratedResponse, error)` 형태다.
응답은 선언된 상태·미디어·헤더·본문을 생성된 variant로 구성한다. 비공개 variant 필드와
생성자를 사용해 모순되는 상태·본문을 만들지 않도록 돕고 송신 전 스키마도 검사한다.

웹훅의 실제 경로와 콜백 endpoint 연결은 호스트가 지정한다. callback expression을
호스트의 임의 URL에 자동 등록하거나 별도 listener를 시작하지 않는다. 인증 대안은
호스트 `Authenticate`에 전달한다. 익명 허용이면 인증 함수를 생략하고, 인증이 필요한데
함수가 없으면 현재와 같이 401을 반환한다. 핸들러 실행 전 수신 검증과 인증 경계를 유지한다.

인증 계약은 `func(context.Context, InboundRequestContext) (*InboundRejection, error)`다.
nil·nil은 계속 진행, rejection은 호스트가 정한 상태·헤더·BinarySource 본문으로 조기
응답한다. error는 기존 수신 오류 분류로 처리한다. context에는 `Request *http.Request`,
원래 OperationID·Method·Path, 정확한 OpenAPI 이름으로 그룹화한 Params, 원래 Security와
scheme별 SecurityCandidates를 제공한다. Params에는 path·query·querystring·header·cookie
모두 포함한다. candidate는 scheme·type·location·name·raw value를 보존한다. Request는
헤더 snapshot과 `http.NoBody`를 가진 metadata view다. context의
`CloneBody func(context.Context) (io.ReadCloser, error)`로 제한 내에 복제한
독립 reader를 얻어 서명 검증할 수 있고, SDK가 원본을 보존해 뒤의 DecodeInbound와
핸들러가 읽는다. 반환 reader는 호스트가 닫고 backing bytes는 읽기 전용이다.
clone 경로만 명시적으로 버퍼링하며 complete body에는 8 MiB 제한을
적용한다. 순차형 입력은 유한 복제할 수 없으므로 clone 요청을 오류로 거부한다. 스트림
본문 서명이 필요하면 호스트가 handler 앞에서 Body reader를 검증 wrapper로 제공한다.
인증 callback의 read가 본문을 소모해서 뒤의 코덱을 깨뜨리는 암묵적 동작은 만들지 않는다.
인증 함수 오류는 500, 인증 미설정은 401, body clone 초과는 413이다. 익명 허용인
작업에서는 인증 callback을 호출하지 않는다. Rejection은 status int·headers http.Header·
body BinarySource를 가지며 본문 없는 응답은 nil Body로 표현한다. SDK가 응답 source를
열고 종료하며, 잘못된 status·source 열기 실패는 헤더 송신 전 500으로 처리한다.

전체 수신 본문의 기본 제한은 8 MiB다. 실제 읽은 바이트로 검사하고 Content-Length는
조기 거부에만 사용한다. 초과하면 413이다. 항목별 스트림에는 전체 본문 제한 대신
1 MiB 기본 프레임 제한을 적용한다. 프레임 초과는 현재와 같은 400 경로로 처리한다.
수신 취소, 잘못된 입력, 사용자 핸들러 오류, 응답 검증·인코딩 실패를 구분한다.
스트림 응답의 헤더를 이미 보낸 뒤 실패하면 상태를 바꾸려 하지 않고 스트림을 종료한다.
수신 스트림을 읽으며 응답을 보내는 HTTP/1 경로는 헤더를 보내기 전에
`http.NewResponseController(writer).EnableFullDuplex()`를 호출한다. HTTP/2는 기본
동시 송수신을 사용한다. 호스트 writer가 HTTP/1 full duplex를 지원하지 않으면 헤더 송신
전 500과 내부 `TRANSPORT_CAPABILITY_REQUIRED` 원인으로 실패한다. 부분 응답을 보낸 뒤
지원 여부를 발견하지 않는다. W09에서 실제 HTTP/1·HTTP/2 송수신과 취소를 확인한다.

`server/runtime`은 정규화된 스키마와 정의 집합을 받는 범용 Decode·Validate·Respond
기능도 제공한다. 생성 시 사용자 어휘 확장은 이미 낮춰진 표준 스키마를 사용한다.
런타임 범용 검증이 원격 문서를 가져오거나 임의 확장 코드를 실행하지 않는다.
서버 패키지 때문에 클라이언트 전용 사용자가 서버 라우터를 참조하지 않게 한다.

## 10. 공통화·CLI·검증 도구 변경

[현재 generator 경계](../../internal/generator/generator.go)의 Prepare·Emit·EmitTo,
컴파일 진단 orchestration, 원자적 output publisher는 재사용한다. 스키마 의미 계획은
`internal/schema/plan`으로 옮기고 TypeScript는 동일 계획을 소비하게 한다.
이동과 Go 방출기를 한꺼번에 무검증 변경하지 않는다. TypeScript 회귀 검증이 통과한
공통 계획 위에 Go 계획을 연결한다. HTTP 의미·상태 선택·Link·수신 계약의 현재 작업이
공통 의미를 요구하는 범위만 IR·공통 helper로 추출한다.

Go는 `internal/target/golang`에 Generator와 준비 계획·명명·모델·작업·런타임
방출기를 둔다. 새 타깃을 공통 registry에 추가한다. 타깃별 공개 옵션과 검사 능력을
인터페이스로 선언한다. 공통 orchestration이 TypeScript 구체 타입을 참조하지 않게 한다.

`inspect`는 현재 `typescript` JSON 필드를 보존하고 `golang` 필드를 추가한다.
공통 작업 inspection 계약에는 노출 상태·생략 이유·언어별 호출 예시가 들어간다.
선택 출력은 원래 ID·route를 사용한다. `inspect --target golang`과 스키마 확장도 지원한다.
진단 코드의 target은 `golang`이며 공통 컴파일 규칙 코드는 유지한다.

`scripts/generate/check.sh`는 입력 snapshot·생성 이후 target verifier로 분기한다.
Go 검사기는 격리된 소비자 모듈을 만들고 생성 의존성을 명시적으로 고정해 컴파일·실행한다.
사용자 모듈은 수정하지 않는다. TypeScript 검증 절차는 그대로 보존한다.
Go 소비자 검증에는 TypeScript 설치가 필요하지 않다.

playground의 registry와 옵션 계약에 Go를 추가한다. Go 소스 생성만 브라우저 WASM에서
실행하며 브라우저에서 Go SDK를 컴파일·실행하지 않는다. 외부 파일·네트워크 접근이 없는
현재 입력 경계를 유지한다. import 경로는 playground의 명시 예시 값으로 전달한다.

기능 매니페스트는 ID를 유지한 채 타깃별 상태·조건·증거를 표현하도록 확장한다.
타깃이 생략된 기존 항목의 TypeScript 해석과 문서 생성 호환성을 보존한다.
Go가 미완성인 동안 타깃 목록에만 추가해서 기존 상태를 Go 지원으로 해석하지 않는다.

### 10.1 책임 소유자와 생성 파이프라인

공통 compiler는 OpenAPI가 뜻하는 바를 정하고, Go Prepare는 그 의미의 Go 표현을 정한다.
EmitTo는 고정된 계획을 소스로 바꾸고, output publisher는 파일의 안전한 게시를 소유한다.
이 경계는 [현재 Target 계약](../../internal/generator/generator.go)과
[현재 파이프라인](../../internal/generator/pipeline.go)을 유지하면서 확장한다.

```text
CLI/config -> compiler -> normalized IR + schema resources
                              |
                              v
                    golang.Prepare
                    names / contracts / closure / files
                              |
                              v
                    frozen SourcePlan
                       /           \
                  inspect        EmitTo
                                    |
                              go/format
                                    |
                              ArtifactSink
                                    |
                    output staging -> atomic publish
```

| 소유자 | 입력 → 출력 | 책임과 경계 |
| --- | --- | --- |
| CLI·공통 generator orchestration | 설정·명령 → compiler/target 호출·진단 보고 | 옵션·타깃·검사 명령 선택. 모델 명명·Go 코드 작성·HTTP 실행은 타깃/런타임에 위임 |
| compiler·IR | 문서·참조 정책 → 정규화된 작업·서버·보안·미디어·스키마 자원 | 버전 의미·참조·상속·원래 위치·공통 지원 판단. Go 식별자·import 경로를 저장하지 않음 |
| `internal/schema/plan` | 정규화 스키마·투영 정책 → 의미 노드·참조·필요 능력 | input/output·합성·평가·dynamic scope 의미. 언어 코드·모델 이름·출력 경로에 독립 |
| `golang.Prepare` | IR·공통 옵션·Go 옵션 → 불변 SourcePlan·진단 | Go 이름·DTO·호출 형태·스키마 프로그램·공개/내부 closure·파일/import/런타임 조합 확정 |
| `golang.EmitTo` | SourcePlan → 포맷한 Artifact | 선언·함수·배선·준비된 런타임 소스 출력. 새 스키마/Link 발견·지원 결정·사용자 파일 수정 없음 |
| output publisher | ArtifactSink 입력 → 관리 파일·manifest | 경로 보호·잠금·check/incremental·staging·원자 게시·복구. Go 의미와 코드 내용 판단 없음 |
| 생성 SDK | 구체 DTO·옵션·context → HTTP 결과·오류 | 준비된 계약 실행·값 검증·전송·본문 수명. 원격 OpenAPI를 다시 읽거나 생성기를 호출하지 않음 |
| 호스트 앱 | 사용자 정책·자원 → SDK에 제공하는 hook | 자격 증명 획득·서버 listener/route·인증 판단·사용자 코덱·명시 재시도 정책·모듈 의존성 관리 |

Prepare의 순서는 아래로 고정한다. 모두 한 번의 생성 호출이 소유하며 전역 mutable cache를
두지 않는다. 원래 위치와 진단 provenance는 각 계획에서 참조로 유지한다.

1. 전체 문서의 공개 이름·schema identity를 예약한다. 선택을 바꿔도 이름이 변하지 않는다.
2. 기존 visibility·선택·독립 클라이언트 집합과 타깃 진단을 적용한다. Link의 내부 대상과
   server addon의 수신 계약도 독립 root로 수집한다. 공개 집합과 실행 의존 집합을 구분한다.
3. 작업별 입력·출력·오류·헤더·보안·미디어·Link·페이지·stream 계약을 구체화한다.
4. 필요한 스키마를 투영별로 낮춘다. input/output·바이너리/XML 등 표현 정책·동적 scope를
   identity에 포함하고 순환 참조는 자원 그래프로 처리한다. 공개 DTO identity와 내부 검증
   알고리즘의 공유 identity를 별도로 관리한다.
5. 각 root가 요구하는 모델·검증 함수·런타임 기능의 도달 가능한 의존을 계산한다.
   동일 의미의 검증 프로그램은 공유하지만 추가 속성·평가 정책이 다른 계약은 합치지 않는다.
6. 파일 경로·package·선언·참조·import·런타임 모듈을 계획하고 충돌·import cycle·누락
   참조를 검사한다. 정렬을 고정한 SourcePlan을 freeze한다. 이후 의미를 추가하지 않는다.

SourcePlan은 이름·모델·스키마 프로그램·작업·공개 진입점·수신·런타임 closure·파일 계획을
소유한다. FilePlan은 출력 경로·package·정렬된 import·선언 계획을 갖는다. 전체 입력의
임의 map을 emitter마다 다시 순회하지 않는다. `inspect`는 같은 계획의 읽기 전용 view다.
독립 클라이언트와 선택 출력도 같은 작업·모델 계획을 참조하고 재분석하지 않는다.
예상 가능한 명세 문제는 Prepare의 구조화 진단이다. EmitTo의 포맷·누락된 binding·sink
실패는 내부/출력 오류다. 진단 모드 때문에 차단된 경우에는 방출 가능한 plan을 반환하지 않는다.

### 10.2 Go 방출기와 소스 소유 구조

새 타깃의 내부 책임은 다음 파일군으로 나눈다. 이는 구현할 구조이며 현재 파일은 아니다.
관련 계획과 방출기는 같은 `golang` 패키지의 비공개 타입·함수로 연결한다. 파일마다 새로운
패키지·interface를 만들지 않는다.

```text
internal/target/golang/
  generator.go               Target/StreamingTarget adapter
  plan.go                    SourcePlan and preparation order
  plan_names.go              stable public identifiers
  plan_models.go             DTO/projection/enum/codec plans
  plan_schema.go             Go validation program plans
  plan_operations.go         parameters/status/media/security/call plans
  plan_clients.go            roots/resources/refs/selection/named clients
  plan_server.go             webhook/callback/inbound plans
  plan_runtime.go            explicit runtime feature/dependency catalog
  plan_files.go              package/file/import ownership and checks
  emit.go                    ordered per-file rendering and sink ownership
  emit_models.go             DTO/enum/codec render views
  emit_schema.go             validation function render views
  emit_operations.go         concrete calls, Link and pagination render views
  emit_clients.go            public entry-point render views
  emit_server.go             inbound binding render views
  templates/                 declarations and file templates
  runtime/                   authored *.go.tmpl runtime modules
```

선언은 타입이 정해진 계획에서 `text/template`과 작은 출력 helper로 Go 소스를 만든다.
템플릿은 선언·참조·필요한 import를 출력하고 OpenAPI 의미를 판단하지 않는다.
식별자는 NamePlan, 문자열 리터럴은 `strconv.Quote`, import는 FilePlan을 사용한다.
범용 AST 코드 생성 기반은 추가하지 않는다. `go/format`은 파일마다 한 번 실행한다.

직접 작성하는 런타임 알고리즘은 `runtime/*.go.tmpl`을 유일한 정의로 embed한다.
동적 치환은 준비된 package/import와 명시적인 binding으로 제한한다.
새 파일을 두는 것만으로 모든 SDK에 복사하지 않고 명시적 런타임 목록의 root와 의존
closure로 출력한다. 생성 소스를 분석·문자열 검색해서 기능을 잘라내지 않는다.
생성된 JSON 검증은 기본 제약을 직접 검사하고 합성·regex·평가처럼 필요한 부분만
공통 helper를 호출한다. 호출마다 전체 스키마를 해석하는 경로로 되돌리지 않는다.
`server/runtime`의 임의 정규화 스키마용 helper만 기존의 명시적인 범용 경계를 유지한다.

작업·스키마 단위 파일로 나누고 같은 package 안에서 공유한다.
EmitTo는 파일 하나를 출력·포맷해서 sink에 넘긴 뒤 버퍼를 해제한다.
넘긴 byte slice를 재사용·수정하지 않는다. Emit은 같은 EmitTo의 수집 adapter여서 생성
로직이 두 개가 되지 않는다. sink/format 실패 때 publisher가 staging을 버리고 이전 출력을 보존한다.

### 10.3 생성 SDK의 의존 방향과 실행 책임

아래 화살표는 허용하는 import 방향이며 역방향은 금지한다.

```text
wire                       canonical JSON/value primitives; stdlib only
types -> wire              public value/option/source/transport contracts
schema -> wire             generated checks and selected regex helpers
stream -> types, wire      frames, protocol/adapters, pull iteration
media -> types, wire, schema, stream
security -> types          declared requirements and credential application
transport -> types         net/http, redirects, cookies, compression
core -> types, media, security, transport
models -> types, wire, schema
internal/operations -> models, core, schema, media, stream, types
root/clients/selective/operations -> internal/operations and needed lower layers
server/runtime -> models-free lower schema/media/stream/security layers
server/webhooks,callbacks -> server/runtime, models, schema, types
metadata -> stdlib          source document only; no incoming runtime dependency
```

공개 `types.Value`는 wire 표현을 감싸며 wire는 types를 import하지 않는다. `schema`는
모델을 알지 않고 wire 값만 검사한다. 모델 코덱은 검증 프로그램과 DTO 투영을 조합한다.
media는 본문 표현·파싱, stream은 프레임, transport는 HTTP 바이트 교환·해제를 소유한다.
security는 명세의 요구 충족과 인증 정보 적용을 소유하고 실제 권한 판단·토큰 발급은 호스트에
남긴다. core는 이들을 조합해 호출별 context·오류 단계·body lease를 관리한다.
서버 수신은 client core·외부 전송·클라이언트 작업을 import하지 않는다.

구체 작업은 Prepare에서 고른 요청·응답 코덱과 검증 함수를 실행 서비스에 직접 연결한다.
core가 모든 내장 코덱을 참조하는 전역 switch/registry를 갖고 실행 때 매번 탐색하지 않는다.
사용자 등록 코덱의 미디어 조회는 해당 확장 계약에 한정하고, 전체 작업의 동적 조회는
selective의 명시적인 Lookup 경로에 한정한다. 따라서 같은 core를 사용해도 단순 JSON
작업이 XML·stream·전체 작업 목록을 함수 참조로 붙잡지 않는다.

호출 흐름은 구체 작업 wrapper → 옵션 결합·입력 검증 → URL/헤더 구성·보안 선택 →
본문 source 열기·인코딩 → Transport → 압축 해제 → 상태/미디어 선택 → decode·검증·DTO
투영 → 결과 반환이다. 스트림 항목은 소비 시점에 검사한다. core가 일반 호출의 자원을
정리하고, 순차형 Raw/Stream은 열린 lease를 핸들에 넘긴다. 후속 cleanup을 호스트 hook의
성공 반환에 의존하지 않는다. 서버는 route/매개변수 → 인증 → 본문 decode·검증 → 호스트
핸들러 → 응답 검증·encode·송신 → 요청/응답 lease 종료로 흐른다.

기능이 추가되면 변경 위치도 이 책임을 따른다. 예를 들어 새로운 OpenAPI 제약의 의미는
공통 스키마 계획에, Go 검증 표현은 plan_schema/emit_schema에, 필요한 실행 helper는
schema 런타임에 둔다. XML 인코딩 수정은 media에, HTTP 연결·압축 수정은 transport에 둔다.
resource·operation ID·route 진입점은 같은 작업 구현을 호출하고 HTTP 실행을 복제하지 않는다.

W01~W02에서 계획 불변성·결정적 출력·import cycle·Emit/EmitTo 동등성과 생성 소비자
컴파일을 확인한다. W03~W09에서는 이 의존 방향을 실제 생성 패키지의 `go list -deps`와
컴파일·실행으로 확인한다. 명세별 closure와 server/metadata/named-client 단독 소비도
포함한다. 템플릿 문자열 존재 여부만으로 경계 준수를 판정하지 않는다.

## 11. 성능 설계와 측정 계약

성능은 기능·검증을 켠 상태에서 비교한다. 코드 생성과 검증을 끄거나 입력을 축소해서
동일 작업의 개선으로 보고하지 않는다. 기본 경로에 `unsafe`, CGO, 자체 goroutine pool,
무제한 전역 캐시를 추가하지 않는다. 공유 계획·참조·지연 컴파일로 반복 작업을 줄인다.
버퍼 풀은 현재 범위에서 도입하지 않는다. 우선 정확한 소유권과 제한된 생명 주기로 구현한다.

| 워크로드 | 고정 입력·변형 | 지표와 판정 |
| --- | --- | --- |
| 소형 JSON | 기본 Todo의 일반·Raw·오류 호출, 검증 켬 | ns/op, B/op, allocs/op; 같은 응답 계약의 기본 HTTP·JSON 비용과 SDK 추가 비용 분리 |
| 대형 JSON·합성 | 1 KiB·1 MiB·16 MiB 본문, 중첩·재귀·oneOf·추가 속성 | 시간·최대 메모리·GC; 검증·DTO 변환에서 재직렬화 중복 없음 |
| 특수 미디어 | XML namespace·분기, multipart 1 MiB·64 MiB 업로드 | 처리량·최대 메모리·파일 수명; 스트림형 source에서 전체 파일 선읽기 없음 |
| 압축 응답 | gzip·deflate·br·zstd의 1 KiB·1 MiB·16 MiB 해제 후 본문, 높은 압축률·체인 | 최초 바이트 지연·live heap·해제 처리량·닫기; decoder window 비용과 SDK 프레임 버퍼를 분리 |
| 스트림 | 1,000·100,000 항목, 같은 크기 프레임, 느린 소비자·분할 청크 | 첫 항목 지연·최대 live heap·소비 속도; 항목 누적에 비례하는 SDK 버퍼 없음 |
| 정규식 | ASCII·Unicode·lookaround·backreference·복잡도 초과 | 컴파일과 실행 분리, 할당·복잡도 오류; Node 의미와 일치 |
| 동시 호출 | 공유 클라이언트 1·8·64 동시 호출, 인증 제공자·코덱 포함 | 처리량·지연·경합·race·종료 뒤 자원 상태 |
| 대형 생성 | 기존 고정 제공자, 1,000·10,000 작업의 공유·재귀 스키마 | 생성 시간·peak RSS·파일/바이트 수; 선택 1·10 작업의 출력·준비 비용 분리 |
| 소비자 빌드 | 전체 생성의 단일 호출, 선택 생성, 독립 클라이언트, 동적 조회 | cold/warm 빌드·peak RSS·바이너리·심볼·의존 패키지 |

빌드 비교는 같은 도구 버전·플래그·타깃에서 한다. cold 빌드는 전용 빈 GOCACHE,
warm 빌드는 동일 캐시로 측정한다. 원본 바이너리와 `-ldflags='-s -w'` 결과는 구분한다.
직접 호출 바이너리에서 미사용 작업·XML·스트림·메타데이터·정규식의 심볼과 데이터 경로가
제거되는지 관찰한다. 단순 크기 차이만으로 제거 성공을 판단하지 않는다. 소스 전체가
작아졌다는 주장과 바이너리에서 제거됐다는 주장도 구분한다.

생성·빌드 측정은 최소 5회, Go microbenchmark는 최소 5개 sample로 기록한다.
실행·생성·빌드 결과에는 입력·생성기·생성물 hash와 도구 버전·옵션을 저장한다.
pprof는 실제 비용이 큰 경로를 설명할 때 사용한다. 측정은 선택 명령으로 제공하고
일반 CI에 고정 비율·코드 줄 수·소스 문자열 기준을 추가하지 않는다.
첫 릴리스 전에는 이 표의 측정 결과와 실패 경로 검증을 완료해야 한다. 언어 간 절대
속도 우위를 약속하지 않는다. 공개 수치는 기존 정책대로 GitHub Actions 측정과 run 링크를 사용한다.

## 12. 구현 작업 단위

각 작업의 검증은 구현 후 필요한 완료 증거다. 아래 표는 검증 통과 기록이 아니다.

| 작업 | 결과·주요 경로 | 선행 | 작업별 완료 증거 |
| --- | --- | --- | --- |
| W01 | Go registry·옵션·fingerprint·최소 JSON health SDK와 10절의 Prepare/FilePlan/EmitTo 경계. `internal/generator`, `cmd/openapi-sdkgen`, `internal/output`, `internal/target/golang` | 없음 | 외부 소비자 모듈에서 컴파일·호출, 결정적 출력·Emit/EmitTo 동등성·경계 검사, 옵션 오류, check·incremental과 TypeScript 출력 회귀 |
| W02 | 공통 스키마 계획과 Go 모델·JSON·검증. `internal/schema/plan`, 양쪽 타깃, Go wire/runtime | W01 | 모든 스키마 fixture, 4가지 필드 계약, 혼합·복합 enum/const·구조적 동등성·snapshot, 합성·재귀·Unicode·숫자·동적 참조·추가 속성, 기존 TS runtime·선언 검사 |
| W03 | 매개변수·상태·미디어 선택과 오류가 완성된 JSON/텍스트/바이너리 클라이언트 | W02 | 실제 로컬 서버의 URL·헤더·본문·상태·오류 기대값, 전체 압축 coding·원래 헤더·Raw·해제 실패·종료, 원본 응답 소유권과 취소 |
| W04 | 서버 선택·인증 요구·자격 증명·주입 전송 | W03 | AND/OR·익명·충돌·redirect·cookie 모드·mTLS 선언·인증 취소의 통합 검증 |
| W05 | XML·폼·multipart·전체 값 사용자 코덱 | W03 | 기존 wire fixture, XML 분기·namespace, 파트별 헤더·파일·미디어, 코덱의 HTTP 메타데이터·5개 방향·nil·실패·context·소유권, 실패한 source 정리 |
| W06 | 요청·응답 스트림과 프로토콜·어댑터 | W03, W05 | framing fixture, lazy 시작·헤더 관찰·단일 소비·상한·취소·중간 오류·양방향 전송 |
| W07 | 리소스·Link·페이지 조회·x 확장 | W04, W06 | 경로 바인딩·이름 충돌, envelope·sort·오류 범주, 외부 Link·sourceInput·부분 덮어쓰기·HTTP 오류·상태별 대상·필수 옵션·stream 대상·종료 규칙 |
| W08 | 선택 생성·정적/동적 준비·독립 클라이언트·metadata·Go inspect | W02, W07 | 공개 API 집합·타입·내부 Link, 출력 전환·사용자 파일 보존, 정적 경로의 의존 제거 |
| W09 | 웹훅·콜백·범용 수신 런타임 | W04, W05, W06 | 실제 net/http 요청의 인증 메타데이터·거부 응답·body clone·입출력·코덱·한계·stream flush, host 경로 연결·실패 정리 |
| W10 | 검증 명령·매니페스트·playground·영문/한글 문서·예제 | W08, W09 | 기능별 Go 증거 연결, TS 도구 없이 Go 소비자 검사, WASM 생성과 양쪽 docs 검사 |
| W11 | 전체 호환성·성능·플랫폼 검증과 출시 준비 | W10 | 13절의 통합 완료 기준과 11절 측정, 기존 TypeScript·배포 경로 회귀 없음 |
| W12 | 모든 구현과 통합 검증 후 전체 범위 최종 리뷰루프 | W01~W11 | 12.2절의 전체 영역·변경하지 않은 영향 호출자까지 검토, 발견 결함 수정·영향 검증 재실행, 남은 수정 대상 없음과 최종 증거 현재성 |

작업별 경량 검증은 해당 slice에서 실행한다. 대형 제공자·모든 compiler·전체 성능 검증은
W11에서 한 번 통합 실행하며 중간에 완료로 표시하지 않는다. 실제 실패나 공통 변경으로
증거가 무효화되면 영향을 받은 검증만 다시 실행한다.

### 12.1 현재 개발 속도를 반영한 작업량

현재 저장소의 실제 변경 단위를 기준으로, 같은 도구·AI 작업 방식과 바로 검토할 수 있는
조건에서 **집중 작업 10~17일**을 계획 범위로 잡는다. 근무시간을 기록한 자료가 없어
사람의 8시간 근무일이나 확정 납기와 같다고 보지 않는다. 일정은 구현 시작 시점과
검토·CI 대기 시간에 따라 달라진다. 범위의 근거는 [개발 이력 조사](research.md#5-개발-이력과-추정의-한계)다.

| 범위 | 집중 작업일 추정 | 주된 비용 |
| --- | ---: | --- |
| W01~W02 | 3~4.5 | 타깃 경계·명명·모델·wire·전체 스키마 의미·TypeScript 공통화 회귀 |
| W03~W06 | 3~5 | Go HTTP 수명·인증·압축·미디어·양방향 스트리밍 |
| W07~W09 | 2~3.5 | Link·페이지·선택·독립 클라이언트·서버 수신 |
| W10~W12 | 2~4 | CLI·playground·문서·제공자·성능·플랫폼 통합·최종 리뷰 |

상단 범위는 표의 합계이며 기능 일부만 구현하는 견적이 아니다. 프런트엔드용 번들 최적화를
다시 만드는 비용은 없지만, Go의 값 표현·압축·수명·전송·서버 처리는 새 구현이다.
302개 항목을 커밋 수로 나눠 일정을 정하지 않는다. W02와 W06 완료 때 실제 작업일과
통과한 범위를 기록해 남은 작업량을 다시 계산한다. 이 재산정은 기능이나 공개 계약을
구현 중에 선택한다는 뜻이 아니다.

### 12.2 작업별·최종 리뷰루프

W01~W11 각각의 완료 조건에는 구현 검증과 별도의 필수 리뷰루프를 포함한다.
각 task에 필수 리뷰 검증을 하나씩 연결하며 이 검증이 현재 코드 기준으로 통과하기 전에는
해당 작업을 완료하지 않는다. 리뷰를 끝으로 미루거나 테스트 통과만으로 대신하지 않는다.
각 작업에서 `index`의 해당 검토 경로와 `review-loop` 지침을 적용한다.

1. 해당 작업의 요구·공개 계약·책임 경계·의도적으로 정한 제한을 복구하고 변경 파일과
   영향을 받는 호출자·공유 helper·생성 결과·검증 도구를 검토 영역으로 나눈다.
2. 영역마다 미검토·검토 완료·재검토 필요 상태를 기록한다. 유한한 변경 범위를 모두
   살피고, 실제 도달하는 위반인지 코드·관찰 결과로 확인한다. 선호나 미채택 기능을
   결함으로 바꿔 계획 범위를 확장하지 않는다.
3. 확인한 결함을 원인에서 수정한다. 바뀐 경계와 변경하지 않은 영향 호출자도 다시
   확인하고, 새 수정으로 무효화된 테스트·증거를 해당 범위에서 다시 실행한다.
4. 미검토 영역과 수정 대상이 없고 필요한 검증이 최종 코드에 적용될 때까지 반복한다.
   고정된 반복 횟수나 다른 영역 하나의 통과만으로 끝내지 않는다.
5. 검토 범위·발견 사항·수정·재검토·실제 명령/결과·코드 기준·미검증 한계를 증거로 남긴다.
   후속 작업이 이미 검토한 동작을 바꾸면 영향을 받은 작업의 리뷰·검증도 다시 연다.

W11은 전체 기능·성능·플랫폼의 통합 검증과 해당 작업 자체의 리뷰루프를 소유한다.
**W12는 W01~W11 완료 뒤 별도 task로 수행하는 전체 범위 최종 리뷰루프**다. 요구/기능
302개·조건 5개·보충 계약 18개, D01~D24, compiler/Prepare/방출/publisher 경계,
모델·JSON·enum·regex, 호출·오류·HTTP·보안·미디어·압축·소유권, 스트림·페이지·Link,
선택·독립 클라이언트·metadata·server, CLI·playground·EN/KO 문서·배포 준비,
성능·플랫폼·검증 도구·증거 현재성을 모두 검토한다.

W12에서 결함을 고치면 영향 받은 개별 작업의 검토와 통합 검증도 재확인한다. 변하지 않은
유효한 증거는 재사용하고, 변경·실패·새 우려로 무효화된 범위만 재실행한다. 기존 작업의
완료/검증이 stale이면 갱신한 뒤 최종 완료한다. W12의 필수 리뷰 검증과 workstream의
최종 완료 검증까지 현재 기준으로 충족해야 workstream을 닫는다. 계획에 리뷰 검증을
등록한 것과 실제 리뷰를 수행해 통과한 것은 별도로 기록한다.

## 13. 통합 완료 기준

1. 배정표의 모든 ID와 조건이 Go 증거에 연결된다. 기존 규칙의 문서 차단·작업 생략·
   보조 기능 생략 범위를 유지하며 Go 전송 차이는 명시적으로 구분한다.
2. 매니페스트 밖의 CLI·선택·독립 클라이언트·확장·playground·수신 런타임 계약까지
   사용자 작업별 통합 시나리오로 통과한다.
3. 같은 fixture의 입력을 TypeScript·Go에서 실행하고 독립 기대값과 비교한다.
   XML prefix·헤더 대소문자·JSON 키 순서는 의미로 비교하되, 직렬화 규칙상 순서나
   원본 값이 중요한 항목은 정규화로 차이를 숨기지 않는다.
4. 유효·무효·경계 입력을 모두 확인한다. 이름 충돌·잘못된 옵션·부분 출력 실패·
   종료·취소·본문 소비 실패를 포함한다. 무작위 스냅샷을 정답으로 삼지 않는다.
5. 실제 생성 Go 소스를 호스트 모듈 내부 경로와 별도 모듈에서 각각 컴파일한다.
   루트·독립 클라이언트·서버·메타데이터를 개별 소비하고 명시된 의존만 필요하다.
6. 생성 소스는 Go 1.27 최신 패치에서 실행 검증하고 Linux·macOS·Windows의
   amd64·arm64로 `CGO_ENABLED=0` 교차 빌드한다. wasm은 생성기 playground 빌드로
   확인한다. 교차 빌드와 해당 OS에서 실행한 결과를 구분해서 보고한다.
7. 일반 CI는 Go 타깃·생성 SDK 검증을 소유한 suite에서 한 번 실행한다. race는
   공유 클라이언트·스키마·코덱·stream cleanup을 직접 포함한다. 기존 제한된 race
   명령만 통과하고 Go runtime 전체를 검사했다고 보고하지 않는다.
8. 기존 제공자·holdout·3.2 cohort의 생성·방출 API 집합을 비교한다. 기존에 거부하는
   입력을 성공으로 바꾸기 위해 fixture를 수정하지 않는다. Go에서 생긴 새 누락은
   해결한 뒤 완료한다. TypeScript의 알려진 지원 한계는 제품 범위와 구분한다.
9. 11절 워크로드를 측정하고 원시 결과를 보관한다. 측정 결과는 기능 대응과 별도로
   보고한다. 검증 생략으로 처리량을 올리는 경로는 허용하지 않는다.
10. 영문·한글 사용 문서는 구현된 Go API·설치 조건·기능 차이를 설명한다. 코드 예제는
    양쪽 언어 문서에서 같은 영문 예제를 사용한다. TypeScript·Node·CGO 설치가
    Go 생성·소비 경로에 필요하지 않음을 독립 환경에서 확인한다.
11. W01~W11 각각의 리뷰루프와 W12 전체 최종 리뷰루프가 실제 수행되고, 발견한 결함과
    영향 경로를 수정·재검증한 최종 코드 기준의 증거가 남는다. 미검토·재검토 필요 영역이나
    남은 수정 대상·stale 검증이 있으면 작업/전체 완료로 처리하지 않는다.

## 14. 계획 검증과 구현 검증의 구분

[조사 기록](research.md)과 [시제품](probes/)은 엔진·표현·소유권 설계의 근거다.
실행 결과 요약은 [검증 기록](evidence.json)에 보관한다. 42개 정규식 사례를 통과했다는
사실을 ECMAScript 전체 적합성이나 302개 기능의 구현 완료로 확대하지 않는다.

실행 전 계획·배정표·조사 기록·시제품·검증 도구·관련 TypeScript 계약의 해시를 고정한다.
각 실행은 독립 디렉터리에 명령·exit code·stdout/stderr 해시·도구 버전을 저장한다.
실행 종료 때 같은 소스인지 확인한 뒤에만 검증 기록을 원자적으로 갱신한다. 기존 로그를
새 소스로 재인증하는 명령은 제공하지 않는다. 검증기는 실제 로그를 다시 읽어 요약과
비교한다. 로그가 없는 새 checkout은 [재실행 절차](research.md#4-검증-재실행과-해석)를
먼저 수행한다. [리뷰 기록](review.md)에 전체 점검 범위와 수정·재검증 결과를 남긴다.

설계 정책은 이 문서로 고정한다. 구현에서는 이 계약을 충족하는 알고리즘·코드를 작성하고
지정한 증거를 확보한다. 발견된 반례는 계약에 맞게 고친다. 반례 때문에 명세 지원 범위나
공개 계약을 바꿔야 한다면 계획 변경으로 명시하고 관련 계약·배정표·검증을 함께 갱신한다.
테스트 전 결과를 보장하거나 예상하지 못한 명세 충돌이 절대 없다고 주장하지 않는다.
