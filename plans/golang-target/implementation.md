# Go 타깃 구현 기록

기준 계획은 [plan.md](plan.md)의 W01–W12다. 2026-10-19 모든 구현·작업별 리뷰·전체 최종
리뷰와27개 필수 검증을 완료했고 workstream은 done/current다. [최종 완료 증거](completion.json)와
[전체 리뷰](w12-review.md)가 현재 상태를 설명한다. 아래 단계별 기록은 당시 이력을 보존하며,
설계 probe 통과는 제품 구현 검증으로 계산하지 않는다.

## W01 — 타깃 경계와 최소 JSON 호출

구현한 경로:

- `golang` registry, `--go-package`, `--go-import-path`, `[golang]` TOML 설정.
  package 기본값 `api`, import root 명시 필수, CLI 우선, 다른 타깃 옵션 혼용 거부.
- Go 옵션을 manifest fingerprint에 저장하고 package/import root 변경을 감지.
  기존 TypeScript manifest는 Go 옵션 없이 읽는다.
- Prepare에서 전체 작업 이름 예약, 순서 고정, 입력과 분리된 파일 계획,
  명시적 런타임 의존 closure, 패키지/import 누락·cycle 검사.
- 파일별 template + `go/format`, `Emit`은 `EmitTo`를 수집하며 sink에 넘긴 byte를 재사용하지 않음.
- root client → operation → core/transport → schema/wire/types/models의 최소 JSON 호출.
  외부 소비자 모듈의 `httptest` 서버 호출, Raw 본문 소유권, context 취소 검증.
- 기존 출력 publisher를 사용하므로 생성 중 포맷 오류에도 staging을 rollback하고
  소비자의 `go.mod`, `go.sum`, 사용자 파일을 보존.

리뷰 영역과 결과:

| 영역 | 점검 및 보완 |
| --- | --- |
| 옵션과 영향 호출자 | config/CLI 우선순위, 명시 옵션 혼용, help와 config 매핑 기대값 갱신 |
| fingerprint와 구형 manifest | package/import 변경 감지, TypeScript identity 호환 검증 |
| Prepare와 명명 | 입력 수정 이후 byte 불변, 재준비 결정성, 정규화 이름 충돌·순서 검증 |
| 파일과 import | 런타임·전체 생성 package graph의 누락/cycle 거부 보강 |
| Emit와 출력 소유권 | Emit/EmitTo 동등성, 재호출 artifact 독립성, sink 오류 즉시 중단 |
| 포맷과 publication | 일부 파일 staging 후 포맷 실패 시 기존 manifest와 파일 유지 검증 |
| 소비자 실행 | 별도 모듈에서 compile 및 health Do/Raw/context 취소 검증 |

이 단계의 wire/schema/model은 최소 JSON 경계다. 전체 스키마 의미와 typed DTO는 W02,
요청 입력·보안·응답 오류는 W03 이후의 필수 작업이며 현재 완료로 취급하지 않는다.
입력·선택 등 아직 연결하지 않은 계약은 생성 진단으로 차단한다. 공개 문서나 릴리스에서
전체 Go 지원을 선언하지 않는다.

검증 명령:

```sh
devtools run -- go test ./internal/generator ./internal/output ./internal/target/golang ./cmd/openapi-sdkgen -count=1
devtools run -- go test -race ./internal/generator ./internal/output ./internal/target/golang -count=1
devtools run -- go vet ./internal/generator ./internal/output ./internal/target/golang ./cmd/openapi-sdkgen
```

위 세 명령은 모두 통과했다. 첫 명령의 네 패키지 회귀, 두 번째 명령의 세 패키지
race 검사, 마지막 명령의 정적 검사를 확인했다. `git diff --check`도 통과했다.
소비자 테스트는 별도 모듈의 일반 실행이며, 이 기록의 race 결과는 생성기와 출력 경계에
해당한다. 생성 소비자 전체의 race·교차 플랫폼 검증은 후속 통합 작업에 남아 있다.

### W02 확장 리뷰에서 찾은 W01 명명 검증 누락

W01의 최초 구현과 충돌 검사는 순번 접미사를 사용했다. 이는 계획 5절의 원본 경로
SHA-256 접미사 규칙과 달랐다. W02 확장 리뷰에서 확인해 8자리부터 4자리씩 늘리는
규칙으로 고쳤고, 공개 타입뿐 아니라 파생 codec/helper·필드·enum·생성자 이름도
예약하도록 보강했다. `ID`·`URL` 등의 initialism과 대문자 구분이 없는 Unicode 이름의
공개 접근성도 보강했다. 최초 W01 코드 snapshot은 당시 증거로 보존하며 수정하지 않는다.
2026-10-18 W01을 stale 완료 상태에서 재claim했다. 옵션·CLI/config·fingerprint·
명명/파일/import·Emit/sink·staging rollback·최소 소비자 경계를 다시 검토했고, 위의
세 검증 명령과 `git diff --check`가 다시 통과했다. 추가로 대문자 package 거부,
Windows 예약 경로·축약 경로 거부, 유효한 dot-leading segment 허용, 빈 `[golang]`
설정의 명시성도 검사했다. 현재 기준은 `w01-refresh-code.json`이며 최초 snapshot과
검증 이력은 그대로 보존한다.

## W02 — 구현·최종 검증 완료

현재 구현한 경로:

- 공통 스키마 계획을 `internal/schema/plan`으로 이동. TypeScript는 같은 lowering과
  참조 이름 정규화를 사용한다. 선택한 기존 스키마·선언·runtime 회귀가 통과했다.
- 불변 JSON tree와 반복형 parser. 정확한 키·중복 키·WTF-8 lone surrogate,
  UTF-8 전송 치환, binary64 숫자·코드 포인트 길이를 검증한다.
- `Optional`, `Nullable`, `ValueFrom`과 컨테이너 snapshot·순환·비유한 수 거부.
- 스키마별 직접 검증 함수. 타입·숫자·길이·객체·배열·const/enum·합성·조건·의존·
  평가 상태·재귀/동적 참조·패턴/속성명 검증이 연결됐다. 평가 집합은 필요할 때 할당한다.
- 정규식 엔진은 고정한 순수 Go JavaScript 엔진을 조건부 import한다. 패턴 문법은
  Prepare에서 검사하고 런타임은 `sync.Once`로 컴파일한다.
- DTO의 입력/출력 투영, 네 필드 존재 계약, 직접 코덱, 필요한 재귀 포인터,
  타입이 있는 추가 속성, 단순 `allOf` 구조체, scalar/composite literal API,
  union 생성자·독립 `As` snapshot·불변 `With` 갱신을 연결했다.
- 모델과 스키마는 계약별 파일로 준비하고 파일마다 포맷한다. emitter는 새 의미를
  발견하지 않으며 모델·스키마 런타임 상태는 각 호출에 속한다.
- 19개 format의 assertion·annotation 구분과 날짜·시간·IP·WHATWG URL 경계를 연결했다.
  TypeScript의 기존 날짜/시간·끝 줄바꿈 처리 오류를 Go의 독립 기대값으로 복제하지 않았다.
- discriminator의 명시 mapping/defaultMapping과 실제 분기 검증을 연결했다.
  nullable enum·튜플/rest·주석형 enum·member/component 폐기 표시와 GoDoc도 생성한다.
- `contentSchema`의 JSON/text/base64 및 XML 검증을 연결했다. XML은 wire의 표준 라이브러리
  파서와 schema별 직접 디코더를 사용하며 `schema -> media` 의존을 만들지 않는다.
  namespace·속성·wrapped 배열·CDATA·합성/조건·재귀/dynamic scope와 원래 문자열 보존을 검사했다.
  미디어 루트의 모순된 선언 무시는 중첩 필드까지 전파하지 않는다.

실행한 제품 검증에는 Go 생성 소비자의 race 검사, 54개 독립 스키마 사례,
JSON 문법·Unicode·숫자 사례, DTO·투영·재귀·literal·union snapshot 검증이 포함된다.
작업 중 반복한 각 명령은 그 시점의 코드에 대한 결과이며, W02 전체의 최종 검증·리뷰
통과를 뜻하지 않는다.

확인하고 수정한 결함:

- `$ref`와 함께 선언한 dynamic anchor가 참조 실행 뒤 등록되어 중첩 객체의 외부 제약이
  누락됐다. 함수 진입 시 먼저 등록하도록 고치고 성공·위반 사례를 재검증했다.
- W01의 순번 충돌 접미사가 확정한 hash 규칙과 달랐다. 원본 경로를 보존하는 이름
  계획과 파생 식별자 예약을 추가했다.
- 루트 배열을 기본 Go slice alias로 두면 표준 JSON 인코딩에서 nil이 null이 됐다.
  루트 스칼라·배열의 생성 codec 메서드를 통해 계약을 유지하도록 보강했다.
- GoDoc과 선언 사이의 빈 줄 때문에 IDE 문서 연결이 끊겼다. 실제 생성 소스를 AST로
  읽어 type/상수/factory/필드의 문서·폐기 표시가 연결되는지 검사하고 수정했다.
- XML 구문 불일치가 일반 실행 오류로 흘러 `anyOf`의 다른 정상 분기를 막았다.
  내용의 형식 불일치와 context/regex 실행 오류를 구분하도록 보강했다.
- XML 속성의 실제 탭/개행과 문자 참조로 들어온 탭을 구분해 정규화하고 CDATA는 보존했다.
- 패키지 초기화가 미사용 enum/const와 URL 기준값을 바이너리에 남겼다. 처음 사용할 때
  한 번만 초기화하도록 바꾸고 실제 세 소비자 바이너리의 데이터 제거/보존을 대조했다.
- Go 문자열의 정상 surrogate pair와 객체 키를 정규화하고, UTF-8/WTF-8 밖의 문자열과
  정규화 후 map/추가 속성 키 충돌은 비JSON 값·충돌 오류로 거부한다.
- 같은 `allOf` 필드가 동일한 검증 노드로 합쳐져도 annotation의 폐기 표시는 보존한다.
- XML 표현 충돌은 원래 위치의 `GO_XML_REPRESENTATION` target 진단으로 반환한다.

직접 검증 63개와 저장된 공식 JSON Schema fixture 64개(18개 그룹)는 실제 Go 생성 모듈에서 통과했다.
정규식 42개 Node 대조·동시 사용·Unicode 17·복잡도 초과·취소도 생성 SDK에서 통과했다.
정규식 소비자는 별도의 `CGO_ENABLED=0` 실행도 통과했다. 기존 정상 fixture 18개의
스키마 root 230개를 Go 모듈로 생성·컴파일했다. 이는 HTTP 작업 흐름의 완료 증거가 아니다.
공통화된 TypeScript의 `go test ./... -count=1` 전체 회귀는 통과했다(TypeScript 512.479초).
이후 Go 전용 XML 보강에 대해서는 Go target 전체·generator/output/CLI 검증, race, vet를
다시 실행해 통과했다. 생성 소비자 모듈은 각 테스트에서 별도로 race 검사를 실행한다.

W02 최종 기준은 `w02-final-code.json`(SHA-256
`9e9c5cebad0dcb1c11e9750be2faf7f18347bd6607fa1087fd532332e3b74ef9`)이다.
그 앞의 `w02-code.json`도 당시 검증 기준으로 보존한다. 두 기준의 제품 코드는 같고
최종본에는 테스트 파일 포맷 보완을 반영했다. 도구 버전은 `w02-tools.json`에 기록한다.

최종 결과:

- `go test ./internal/generator ./internal/output ./internal/target/golang ./cmd/openapi-sdkgen -count=1`: 모두 통과. Go target 49.093초, CLI 1.500초.
- `go test -race ./internal/generator ./internal/output ./internal/target/golang -count=1`: 모두 통과. Go target 54.671초.
- `go vet ./...`, `fmt-check:dev`, `mod-tidy-check:dev`, `git diff --check`: 통과.
- 공통 annotation/reference의 TypeScript 영향 호출자 재검증: 0.578초, 통과.
- 전체 TypeScript 회귀 이후 TS/common schema 소스 hash가 바뀌지 않았음을 확인했다.
- 최종 포맷의 D25 대조 모듈 28개·race 재검증: 1.033초, 통과.

명령은 모두 프로젝트 환경의 `devtools run`으로 실행했다. 필수 구현/리뷰 검증의 code
basis를 먼저 등록한 뒤 최종 검증을 실행했고, 기록 직전에 1,128개 source hash와
누락/추가 파일을 다시 확인한다. 상세 영역·수정·재검토와 한계는 [W02 리뷰](w02-review.md)를 따른다.
W02의 두 필수 결과를 기록해 task를 완료했다(revision `1466`). 구현 결과는
`20556e88-c674-4907-9617-a0e16a731b09`, 리뷰 결과는
`dbb24489-0236-4763-9f19-a2c5bd693919`다. 전체 Go 기능 대응은 W03–W12까지 남아 있다.

URL format 경계는 D25로 구체화했다. 별도 [검증 모듈](format-probe/url_test.go)의
28개 Node 대조와 race 검사는 dependency 선택 근거다. 제품 format 구현은 실제 생성
모듈의 별도 assertion·annotation·동시 사용 검증으로 확인했다.

### W02 이후 작업량 재산정

W01 최초 claim은 2026-10-18 02:39:44 KST, W02 claim은 02:50:28 KST다.
최종 검증은 같은 날 05시대에 끝났다. 이는 자동 작업의 경과 시간이며 사람의 집중 근무시간이나
8시간 근무일을 측정한 자료가 아니다. 기존 추정에서 완료한 W01/W02를 제외한
W03–W12의 범위 합은 집중 작업 7–12.5일이다. 이 수치를 경과 시간 비율로 곧바로 축소하지
않는다. HTTP·압축·양방향 스트림·보안·수신과 전체 통합은 아직 통과하지 않았고, XML 하위
디코더 같은 재사용 산출물이 실제 후속 비용을 얼마나 줄이는지는 W06 완료 때 다시 확인한다.

## W03 — 구현·리뷰 완료

작업 run은 `fcff144b-03da-4248-90a3-1e9494e60ac0`이다. 요청 입력과 호출,
응답·오류·압축·본문 수명 및 해당 생성 구조를 구현했다. 인증·서버 선택을 비롯한
W04–W12는 후속 필수 작업으로 남는다.

- 필수/선택/무입력 메서드와 Path/Query/Header/Cookie/Querystring/Body DTO,
  복사하는 `WithOptions`, 명시한 빈 값과 상속을 구분하는 옵션을 연결했다.
- HTTP style/explode/content/allowReserved, 원래 wire 이름·schema pointer와 문서,
  JSON/text/binary 및 구체·범위 미디어, 사용자 본문·매개변수 코덱을 처리한다.
  일반 범위의 `MediaValue[T]`는 검증된 값과 소유한 bytes를 구분한다.
- 정확한 상태·범위·default를 미디어별로 선택한다. 단일 성공 DTO와 다중 성공 union,
  유한 Raw snapshot, 타입 헤더·HTTP 오류·서버 코드와 원인 체인을 제공한다.
  오류의 기본 문자열/JSON에는 요청 URL·본문·credential을 포함하지 않는다.
- binary source의 공유 one-shot, nil/typed nil, 음수/불일치 길이, reader+error,
  읽기 실패와 Close 소유권을 처리한다. 외부 취소와 SDK 자체 정리를 구분한다.
- gzip/x-gzip·zlib/raw deflate·br·zstd·역순 체인·identity/unknown, 지연 초기화와
  무본문 응답을 처리한다. 순차형 Raw는 읽지 않은 lease를 넘기며 EOF/Close/취소에 정리한다.
- 주입 client의 설정·pool·Jar를 보존하고 요청별 cookie/redirect 정책을 적용한다.
  native Content-Length/Transfer-Encoding/Trailer를 연결하며 표현할 수 없는 조합은 거부한다.
  Encoded/DecodedWithOriginalHeaders 능력을 구분하고 손실된 암묵적 해제를 거부한다.
- 작업·입력 파일을 분리하고 전체 작업 registry는 생성하지 않는다. 공개 계약 root와
  factory를 선예약하며 원래 pointer 기반 이름으로 중첩 모델·미디어·검증 함수 충돌을 막는다.
  실제 패키지 그래프에서 core/media/models/schema/wire의 의존 방향을 확인했다.
- 공유 HTTP 상태·미디어·참조·헤더·버전별 binary 의미는 TypeScript와 재사용한다.
  compiler의 작업 생략 소유권을 검증하고 Fetch 전용 제약은 Go에 적용하지 않는다.

리뷰의 영역별 범위와 확인·수정한 결함은 [w03-review.md](w03-review.md)에 기록했다.
초기 후보 `w03-code.json`은 이름 공간 수정 전의 역사적 기록이며 완료 근거가 아니다.
최종 후보 `w03-final-code.json`의 1176개 소스 해시를 확인했다. 파일 자체 SHA-256은
`eae77a33eedc7bc050c6bb77c7d288bcfbbf0f95ab2dfa6e27a05c56c1336eef`다.

최종 후보 검증 결과:

- Go target/generator/output/CLI 전체 회귀 통과: Go target 95.316초, CLI 1.540초.
- Go target/generator/output race 통과: Go target 104.298초. 생성 SDK 소비자도 각각
  race로 실행하며 압축 HTTP와 정규식 소비자는 CGO=0도 확인한다.
- `go vet ./...`, `fmt-check:dev`, `mod-tidy-check:dev`, `git diff --check` 통과.
- 공유 HTTP 상태·헤더·참조 추출 뒤 TypeScript 전체 회귀 519.775초 통과.
  그 후 공유 binary helper를 위임한 최종 소스에서 상태 도달성·공개 타입·버전별 binary·
  PDF/ZIP 디코딩 호출자를 다시 실행해 11.625초로 통과했다. 앞선 전체 실행을
  후속 변경까지 포함한 전체 실행으로 간주하지 않는다.

검증에는 기존 schema/DTO/literal/union/content/XML/format, 공식 fixture,
실제 바이너리의 미사용 데이터 제거, rollback, 실제 HTTP/TLS 전송과 동시 호출을 포함한다.
W05 전체 미디어, W06 타입 스트림, W07 리소스/Link/페이지, W08 선택·metadata,
W09 수신, W10 문서·도구, W11 전체 동등성·성능·플랫폼, W12 최종 리뷰는 아직 완료하지 않았다.

## W04 — 구현·리뷰 완료

서버·인증 기능과 W03 전송 경계를 연결했다. 현재 run은
`096abaaf-3aca-46e3-b1b0-3052c74e5521`이다. D26을 계획/spec/W04 정의에 반영한
revision은 `1479`다. 이 변경으로 앞선 작업의 완료 서명이 오래된 상태가 되어
작업별로 현재 정의·소스·독립 검사·영향 리뷰를 대조한 후 근거를 갱신한다.

- root/path/operation 서버 목록·pointer 기반 ID·변수의 기본값/enum·명시 선택,
  요청 BaseURL→클라이언트 BaseURL→서버 순서와 상대 URL의 Origin 해석을 구현했다.
  IDNA·기본 포트·IPv4 표기는 고정 WHATWG 파서로 정규화한다.
- 작업별 보안 선택 타입·상수와 `WithSecurityRequirement`를 생성한다. 공통 옵션의
  선택 인터페이스도 실행 전에 해당 작업의 선언과 대조한다. 단일 자동 선택·복수 명시 선택,
  AND/OR·익명·루트 보안 상속·명시한 빈 security·3.2 URI 참조를 처리한다.
- 직접 Authorization/CSRF, Include의 실제 Jar 능력, mTLS 선언 능력이 충족하면
  제공자를 생략한다. 부족하면 최종 origin·작업·선택된 요구와 metadata를 제공한다.
  누락·잘못된 종류·선택 외 scheme·기존 header/query/cookie 충돌과 동일 직접 값을 구분한다.
- apiKey header/query/cookie, UTF-8 Basic, Bearer, 임의 HTTP scheme, OAuth2/OIDC,
  mTLS를 처리한다. 흐름·요청 scope·discovery URL·bearerFormat·deprecated metadata를 유지한다.
- 제공자에 실행 context와 deadline을 전달하고 전역 잠금·SDK 재시도는 추가하지 않는다.
  제공자에 넘기는 requirement/scope 컨테이너를 복사한다. 인증을 적용한 요청은 redirect를 거부한다.
- 요청 본문 source는 인증 후에 열며 Host는 최종 헤더에서 native 필드로 옮긴다.
  기존 HTTPClient 복사·pool/Jar 공유·쿠키 정책·압축 능력·본문 lease 소유권을 유지한다.
- 보안 대안 ID의 scheme 접미사/익명 이름 충돌을 공통 HTTP 계획에서 해결해 양 타깃이
  사용한다. 라이브러리 SearchParams에서 재현한 구분자·더하기 결함은 독립 form 직렬화로 피한다.

리뷰 범위와 수정 내역은 [W04 리뷰](w04-review.md), URL/쿼리 결정은
[조사 7절](research.md#7-w04의-http-url인증-쿼리-경계)에 기록했다.
최종 `w04-final-code.json`은 1186개 소스를 고정하며 SHA-256은
`36a7abdc3bc5affa862b2b3abc5d0a7e8dd56a2f9b7ece5be2d29e7304f38b33`다.

최종 소스 검증 결과:

- Go target/generator/output/CLI 전체 회귀: Go target 103.649초, CLI 1.525초 통과.
- Go target/generator/output race: Go target 114.523초 통과. 생성 SDK 소비자는 각각
  race로 실행하고 기존 정규식·압축 소비자의 CGO=0 검사도 통과했다.
- TypeScript의 인증 전체·공개 선택 타입·URI·서버 우선순위 관련 회귀: 5.421초 통과.
  최종 공통 helper 변경 뒤 TypeScript 전체 실행을 완료했다고 주장하지 않는다.
- `go vet ./...`, `fmt-check:dev`, `mod-tidy-check:dev`, `git diff --check` 통과.
- W01의 독립 재검사 12개 최상위 사례가 모두 통과했다(Go target 2.172초,
  generator 0.003초, output 0.004초, CLI 0.051초).
- W02의 독립 재검사 14개 최상위/34개 실제 사례가 50.641초로 통과했다.
- W03의 독립 재검사 16개 최상위/19개 실제 사례가 37.622초로 통과했다.

작업별 실행 명령·통과 사례·패키지 결과·원본 로그 해시는 `w01-d26-check.json`,
`w02-d26-check.json`, `w03-d26-check.json`에 각각 보존한다. 해당 작업별 영향 리뷰는
`w01-d26-review.md`, `w02-d26-review.md`, `w03-d26-review.md`다. 원래 완료 기록을 덮어쓰지 않는다.
W05–W12의 나머지 미디어·타입 스트림·리소스·선택·수신·도구·전체 기능 대응·성능·최종 리뷰는 남아 있다.

W01–W03의 독립 재검증을 현재 정의로 기록했고 완료 상태도 current다.
revision은 W01 `1486`, W02 `1493`, W03 `1500`이다. W04 context 동기화는 `1501`,
W04 완료는 `1506`이다. W04 구현·리뷰 record는 각각
`e0b5a0d3-846d-421e-a562-6118ba37621b`, `3ffefc7e-9baf-45e1-842a-726ba8c665a0`다.

## W05 — 구현·최종 검증 완료

W05를 claim했고 context가 유효하다(run `935d9198-03b0-41c1-b584-a044240c4592`,
revision `1507`). TypeScript의 multipart 입력 투영 326줄을 공통 HTTP 계획으로 옮겼다.
기존 TypeScript 진입점은 공통 함수를 호출한다. JSON에서의 원래 문자열 스키마와
multipart에서의 `schema/plan.BinaryInput` 투영을 구분하며 합성·참조·nullable·
부분별 encoding·배열/위치별 encoding 규칙은 그대로 유지한다.

기존 multipart 입력·합성/참조·위치별/순차형 바이너리·실제 encoding wire 회귀가
TypeScript에서 47.943초로 통과했다. 이는 Go multipart 구현이나 W05 전체 통과의 근거가 아니다.
이후 XML HTTP 요청·응답을 기존 직접 스키마 프로그램에 연결했다. 단일/복수 미디어,
일반 미디어 범위의 실제 XML, 참조·분기·namespace·CDATA·속성·비 BMP 이름을 처리한다.
일반 XML 파싱은 표준 파서를 한 번 사용한다. 유효한 XML 이름을 표준 파서가 거부하는
경우에만 이름 토큰을 충돌 없는 별칭으로 바꿔 재시도하며 본문·속성 값은 보존한다.
URL 인코딩 폼은 배열/객체 explode, 필드별 JSON과 문자열 인코딩을 적용한다.

multipart 입력의 `Binary` source는 JSON 검증 투영과 별도의 경로별 슬롯으로 보존한다.
JSON의 불변 `Value` 계약을 확장하지 않는다. DTO·nullable·tuple·합성 뷰의 인코딩,
디코딩과 `With`에서 source를 유지하며 모델 검증은 파일을 열지 않는다. 원시 문자열은
문자열 assertion을 검사하고 바이트/source에는 해당 문자열 assertion을 적용하지 않는다.
타입을 생략한 JSON Schema 분기는 임의 JSON 값도 허용하므로 객체 DTO라고 가정하지 않는다.

생성 소비자 검사는 각각 race를 사용한다. XML·폼·multipart DTO 관련 회귀 그룹이
12.084초로 통과했다. 앞선 XML namespace·이름·기존 content XML 그룹은 12.301초,
일반 미디어 범위·XML·폼 그룹은 12.512초로 통과했다. 복합 multipart 모델의 독립
검사는 1.929초로 통과했다. 이 결과는 아직 W05 전체 통과 근거가 아니다.

multipart HTTP 송신과 파트 순차 reader를 연결했다. 각 파일은 해당 파트가 읽힐 때
열고 EOF·실패·취소·Close에서 정리한다. 전체 파일 버퍼나 임시 파일을 사용하지 않는다.
64 MiB 합성 source, blocked reader 취소, Open 실패, 후속 source 미개방과 헤더 주입
거부를 생성 소비자의 race 실행으로 검사했다. 실제 HTTP에서 파일명·콘텐츠 타입·파트
순서·반복 배열과 단일/복수 미디어 본문을 검사했다. 이는 최대 RSS 측정 결과는 아니다.

선언한 파트 헤더의 필수 여부·스키마와 명시 미디어의 허용 범위를 전송 전에 검사한다.
위치별 `prefixEncoding`·`itemEncoding`과 중첩 multipart를 연결했으며 중첩 파트에는
외부 옵션을 다시 적용하지 않는다. 공통 투영에서 중첩 바이너리와 prefix의 itemEncoding
상속 누락을 재현해 고쳤다. JSON·XML·사용자 코덱 파트와 그 source 소유권도 검사했다.
객체인 사용자 코덱 파트가 JSON 직렬화로 빠지는 우선순위 결함을 수정했다.

자유 형식 본문과 추가 속성에는 불변 `types.MultipartValue`를 사용한다. JSON/바이너리/
객체/배열 생성자로 구성하고 JSON 직렬화는 바이너리 source가 있을 때 거부한다.
컨테이너와 wire 슬롯의 입출력 복사, 원래 JSON 컴포넌트의 `types.Value` 계약과 잘못된
문자열의 오류 전파를 검사했다. multipart 루트 materialization은 Go 미디어 occurrence에만
요청하며 기존 TypeScript 참조 이름과 JSON occurrence를 유지한다.

XML 요청 매개변수와 응답 content 헤더도 직접 스키마 프로그램을 사용한다. 사용자
코덱의 parameter 방향에는 선택한 서버·현재까지 준비한 경로/쿼리·헤더의 snapshot과
본문 없는 Request를 전달한다. 완성 전 URL이므로 아직 처리하지 않은 경로 변수는 남는다.
response Decode의 인자와 MediaContext.Response는 같은 빌린 Body를 읽을 수 있고
metadata 컨테이너는 서로와 실제 응답에서 분리한다. 본문의 최종 Close 소유자는 SDK다.

추가 검사: 실제 HTTP/XML parameter/header/custom codec/multipart 그룹 8.010초,
자유 형식·합성 multipart 그룹 4.013초, JSON/XML/custom 파트 검사 2.106초 통과.
공통 투영 독립 검사는 0.003초 통과했다. 영향 받는 TypeScript multipart 네 검사는
50.249초로 모두 통과했으나 같은 명령의 공통 패키지는 테스트 코드의 IR map 타입 오류로
실패했다. 공통 테스트 코드를 수정해 위 독립 명령으로 다시 통과했으며 전체 혼합 명령을
통과로 기록하지 않는다. Go target 전체 129.969초와 generator/output/CLI가 통과한 실행은
자유 형식 MultipartValue·XML 파트 후속 변경 이전/도중의 검사라 최종 근거로 재사용하지 않는다.

후속 리뷰에서 XML 파트 질의를 미디어당 직접 함수로 공유했다. 선언·패턴·추가 속성,
prefix/items, 참조·대안·조건부 fallback을 실제 파트 이름과 반복 항목 위치로 선택한다.
전체 본문의 oneOf 배타성을 한 필드에 다시 적용하지 않는다. SDK에는 스키마 문서를
보관하거나 해석하는 경로를 넣지 않았다. 미선언 XML 파트와 explode 배열의 실제 wire
검사가 2.069초로 통과했고, multipart 기존 회귀 그룹은 14.461초로 통과했다.

기본 encoding을 하위에서 무한 확장한 생성 단계 결함을 회귀 검사에서 재현했다.
실패 실행은 741.836초에 종료됐고 완료 근거로 사용하지 않는다. 명시된 중첩 multipart만
하위 계획을 생성하도록 수정한 뒤, 상한을 둔 회귀가 14.463초로 통과했다. 기본 XML
선택과 숫자 필드명/배열 위치 분리의 독립 검사는 2.025초로 통과했다.

MediaCodec의 nil/typed-nil source, Open/Read 실패, 부분 Decode 후 실패·스키마 오류·
취소, DecodeParameter의 HTTP snapshot과 반환 값 검증을 생성 race 소비자로 확인했다.
custom content 헤더만 있는 작업에서 미사용 schema import가 생기는 결함을 고쳤다.
해당 검사와 XML 헤더 검사는 4.000초, 선언 헤더·XML 이름/namespace 회귀를 포함한
검사는 6.244초로 통과했다. DecodeInbound 공통 helper의 빌린 본문·metadata 복사·
오류·취소 검사는 package 경계 검사/기존 custom media와 함께 6.026초로 통과했다.
handler가 없는 SDK에는 inbound helper를 내보내지 않으며 실제 통합은 W09에서 검사한다.
HTTP snapshot의 공통 구현은 media에 두고 core가 위임해 의존 방향을 유지한다.

XML anyOf/조건부/동적 참조의 native HTTP roundtrip도 추가했다. 동적 참조에 붙은
지역 xml metadata가 무시되어 자식이 소실되던 결함을 수정했고, content XML과 이름/
namespace를 포함한 그룹이 6.558초로 통과했다. 비 BMP 이름 alias 충돌, Unicode prefix,
본문·속성·CDATA 보존, DTD·잘못된 이름·중복 expanded 속성·namespace 오류도 검사했다.

기존 TypeScript의 FormData/수동 named multipart 차이를 원본에서 확인해 적용했다.
일반 모드의 파일/JSON/명시 미디어 파트에는 기본 파일명 blob을 주고, 미디어를 선언하지
않은 문자열 파트의 Content-Type은 생략한다. 명시한 빈 MultipartHeaders map도 수동
모드이므로 Clone/Merge가 nil 여부를 보존한다. named multipart는 form-data 객체를,
positional 선언은 배열을 요구한다. 스키마의 기본 파트 미디어와 바이너리 contentMediaType
우선순위를 적용한 뒤 named wire/잘못된 모드와 source 수명 검사가 4.219초로 통과했다.

최종 리뷰에서는 nil 파트 source가 조기 HTTP 응답으로 성공할 수 있는 경로를 Open 없는
Binary.Validate로 차단했다. 합성 바이너리의 실제 유효 분기와 contentMediaType은
직접 파트 질의에서 선택한다. source 슬롯의 잘못된 pointer·겹침·누락과 DTO의 표현 불가를
거부하며, 공개 codec 자료는 types.MultipartData로 명명했다. response에는 입력 binary
투영을 적용하지 않는다. source Close의 실제 완료도 취소/명시 Close 경합에서 기다린다.

EOF에서 확정된 trailer는 독립된 HTTP metadata snapshot에 복사한다. 실제 plain/gzip
응답으로 확인했다. multipart 프레임은 한 번 준비하고 모든 파트 길이를 알 때 선읽기 없이
전체 Content-Length를 계산한다. 64 MiB 합성 업로드에서 계산 길이와 실제 읽은 길이를
대조했다. 파트 source가 읽히기 전 HTTP가 조기 종료되는 경우도 정확히 정리한다.

옵션으로 선택한 중첩 multipart가 nullable 객체의 기본 미디어를 잃는 문제를 마지막에
재현해 수정했다. 별도 무한 하위 encoding 확장 없이 직접 생성한 root 질의와 반복 위치
경로를 재사용한다. 선언 필드·반복 배열·미선언 patternProperties의 HTTP 검사가
2.134초로 통과했고 multipart/XML 그룹이 59.049초로 통과했다.

최종 기준은 1228개 소스의 `w05-reviewed-code.json`이다. SHA-256은
`4adb9c923fa8c128578151012bbb058e23699bd80ae293e8e7e0b723236caf3d`다.
후보 `w05-final-code.json`과 후보 검사들은 역사 기록으로 보존한다.

```sh
devtools run -- go test -race ./internal/target/golang ./internal/generator ./internal/output ./cmd/openapi-sdkgen ./internal/http/plan ./internal/schema/plan -count=1 -json -timeout=6m
devtools run -- go vet ./...
devtools run fmt-check:dev
devtools run mod-tidy-check:dev
git diff --check
```

최종 명령이 모두 통과했다. Go target 184.659초, CLI 7.441초이며 전체 통과 이벤트는
383건이다. 각 생성 소비자도 race로 검사하고 기존 regex/compression 소비자의 CGO=0
실행을 유지했다. 영향받는 TypeScript multipart/XML 55건 61.830초와 공통 XML/encoding
8건은 마지막 수정 전후 해당 소스 해시가 같아 재사용한다. 원본 로그 해시·실제 명령·결과는
`w05-reviewed-*-check.json`, 도구는 `w05-tools.json`에 보존했다.

배정한 baseline 기능 15개와 보충 계약의 실제 통과 사례는 `w05-feature-evidence.json`,
전체 영역의 수정·호출자 재검토·현재성은 `w05-review.md`에 기록했다. W01–W04의 D27
영향은 별도 단위별 검사와 리뷰로 재확인했다. W05의 미검토 영역이나 확인된 미수정 결함은 없다.
타입 스트림은 W06, 실제 inbound handler는 W09, 전체 동등성·성능·플랫폼은 W11,
전체 최종 리뷰는 W12에서 완료한다. 전체 TypeScript 동등성을 W05 완료로 계산하지 않는다.

## W06 — 완료

`types.Stream[T]`가 지연 시작·헤더 조회·단일 소비·취소·Close를 관리한다. 계약과 codec
registry, handle을 각각 나눴으며 `NewStreamCodec`의 generics가 protocol/adapter의
Frame 일치를 검사한다. 레지스트리 안에서만 타입을 소거하고 공개 결과는 `Source[T]`다.
기본 Frame은 JSON/SSE의 `Value`, multipart의 소유한 header/data `MultipartPart`다.

reader는 호출별 byte 상한을 적용하고 background prefetch를 하지 않는다. incremental
UTF-8 decoder는 최대 3바이트를 넘겨받아 BOM·잘못된 prefix를 처리한다. NDJSON·JSON-seq·
SSE·순차 multipart parser/encoder를 추가했다. 프레임·헤더 상한과 lazy pull, source 종료를
독립 생성 race 소비자로 검사했다. JSON 1천/10만 항목에서 내부 버퍼 누적이 없음을
관찰했으나 최대 RSS나 W11의 전체 성능 결과는 아니다.

응답 Stream과 schema-only/both의 전체 수집, media/status별 item union과 Raw를
연결했다. bytes+EOF 뒤 파서에 남은 데이터를 검증할 때까지 typed 실행 수명을 유지한다.
전체 응답의 schema/prefix encoding과 증분 itemSchema/itemEncoding을 별도 직접
함수로 계획한다. 파트 header·default media·XML·nested를 연결했고 바이너리는
MultipartData와 sealed AsMultipart/AsBinary에 보존한다. Bytes 조회는 복사본이다.

요청은 SourceFactory와 complete/stream tagged wrapper를 사용한다. 기본 native MIME는
현재 파트만 여는 pull reader이며 complete MIME는 metadata를 미리 검사하고 파일은
읽지 않는다. 실제 HTTP/1·HTTP/2에서 업로드가 끝나기 전 첫 응답을 받았다. 64 MiB
파일은 첫 header Read까지 Open/Read가 0이며 최종 Close가 한 번이다.

factory와 protocol/adapter는 단계별 child context를 먼저 취소하고 한 번 닫는다.
생성 실패의 원인·정리 오류를 함께 보존하면서 상위 EOF 뒤 마지막 item/closing bytes도
허용한다. 수정 전 네 생성 방향의 취소 순서 실패를 재현했고 수정 후 실제 송수신까지
통과했다. D28과 공유 caller의 영향은 W01–W05 전용 검사·리뷰로 별도 재검증했다.

최종 소스는 `w06-final-code.json`의 1259개 파일, SHA-256
`33600463c88cccdb6ee07b624c0126416210d7e7c1db5836869262760ea7b847`다.
전체 후보 race 391개 통과 이벤트·Go target 225.535초, TypeScript 107개·83.233초,
공통 XML/encoding 8개가 통과했다. 마지막 codec/수명 테스트 두 파일만 달라져 모든
관련 stream/Raw/의존 경계 11개를 최종 소스에서 race로 다시 검사해 28.300초 통과했다.
vet·format·module·diff도 최종 상태에서 통과했다. 나머지 소스의 동일 해시를 확인해
전체 후보 검사의 유효한 영역을 재사용했다.

18개 배정 기능과 보충 계약의 실제 근거는 `w06-feature-evidence.json`, 전체 리뷰와
수정·재검토·제한은 `w06-review.md`에 있다. W07–W12는 여전히 필요한 후속 작업이다.

devtools revision `1593`에서 구현·리뷰를 별도 기록하고 현재 정의 기준 완료했다.
구현 record는 `005516cc-237b-4240-a44d-f20e7773f1ac`, 리뷰 record는
`344ee1db-818b-474a-94aa-8d2d1e1fdf7e`다.

## W07·W08 완료와 후속 범위

W07은 D29의 리소스·Link·페이지·확장 계약과 8개 영역 리뷰를 완료했다.
revision `1646`의 구현·리뷰 record는 각각
`b31396c1-1b1f-4304-bf2e-665e783865a2`와
`ac5eda71-98e9-4aeb-b43f-ebd870141c00`다. 당시 최종 1,307개 파일과
6개 배정 기능·3개 보충 계약의 근거는 [W07 기능 근거](w07-feature-evidence.json)와
[W07 리뷰](w07-review.md)에 있다. D29에서 W01–W06도 각 단위의 새 검사와 독립 리뷰로
현재 정의 기준 갱신했다.

W08은 revision `1656`에서 구현·리뷰를 별도로 기록해 완료했다. 생성 선택과 Link
내부 의존, 타입이 있는 정적 Ref·Prepare·Bind, 실행 중 ID/route 조회와 지연 레지스트리,
독립 concrete client·타입 alias, 원문 metadata·Go inspect·출력 전환을 제공한다.
동적 호출도 Binary·항목 factory를 JSON 모양과 분리한 NativeInput으로 전달하며
source는 실행할 때 연다. 구체 API·소유권은 [W08 결정](w08-decisions.md)에 있다.
이 내용은 D29 계획 8절을 구체화해 W08 task에 기록했으며(revision `1650`), 전체 계획과
선행 작업 정의를 바꾸지 않았다. W01–W07의 current 완료 상태도 확인했다.

리뷰에서 독립 클라이언트 사이의 경로 인자 충돌, Content-Type 매개변수 유실,
nil sealed-interface 임베딩, 리소스 생략 strict 옵션과 중복 경로 매개변수,
metadata 확장 없는 annotation·deprecated 문서 누락을 보완했다. 새 패키지 의존성
경계와 Go 지원에 맞춘 도움말 기대값도 갱신했다. 독립 리소스 결함은 수정 전 실제 생성
소비자의 컴파일 실패와 수정 후 scalar·복합 객체 HTTP 소비자로 확인했다.

최종 소스는 `w08-verified-code.json`의 1,336개 파일이며 SHA-256은
`7ffccd36305e34c249d070602db3341868a50faecc3737ea02563f3ca17f4b68`다.
전체 Go/CLI/공통 후보 race434 이벤트가 통과했고, 뒤에 annotation 관련 10개 파일을
보완한 최종 상태에서 영향 Go31·TS158 이벤트와 vet·format·module·diff가 통과했다.
전체 TypeScript 패키지 race도 564.110초 통과했다. 당시 결합 명령의 CLI 도움말 기대값
실패는 별도 수정·전체 CLI 재검증으로 해결했으며 결합 명령 자체를 통과로 표시하지 않는다.
마지막 소스에서 전체 후보 검사를 다시 실행했다고 하지 않는다.

최종 상태에서 정적·계산한 선택·생성 선택을 같은 실행 결과로 비교하고 nm·바이너리·
생성 파일과 cold/warm 각 3회, 총 18개 빌드 표본을 기록했다. 정적 선택은 레지스트리와
미선택 실행을 제거하며 생성 선택은 불필요 패키지의 소스 컴파일도 제거한다. 작은 fixture의
cold 시간 구간은 겹쳐 속도 우열을 주장하지 않는다. 이는 내부 로컬 측정이고 W11의
대형 명세·실행 성능·RSS·플랫폼 및 공개 CI benchmark를 대신하지 않는다.

96개 배정 기능·5개 보충 계약과 최종 검사·측정 근거는
[W08 기능 근거](w08-feature-evidence.json), 전체 범위와 수정·한계는
[W08 리뷰](w08-review.md)에 있다. 구현 record는
`dad78801-03b1-435c-9f3f-b2a99580d4b7`, 리뷰 record는
`9c382a81-3777-468e-b2be-6037cda7be3b`다. W09 수신, W10 문서·도구,
W11 전체 대응·성능·플랫폼과 W12 전체 최종 리뷰는 계속 필수 작업이다.


## W09 — 구현·리뷰 완료

수신 webhook·callback의 원래 소유권/표현식을 유지한 host-bound typed http.Handler와
응답 variant, parameter decode/Authenticate/CloneBody, complete·stream 미디어와 수명,
범용 normalized-schema Decode/Validate/Respond를 구현했다. 수신은 client core/transport/
operations에 의존하지 않는다. 직접 생성 타입 경로와 범용 스키마 경계는 분리했다.

리뷰에서 HTTP1 입력 전 헤더 대기 및 조기 오류 drain 교착, 부분 응답 실패의 정상 EOF,
연결 취소 후 막힌 응답 read, raw 응답 헤더 검증 우회, form 미지 속성 삭제, MIME 기본값과
XML 대안 덮어쓰기, multipart 버퍼의 Bytes 뷰 회귀를 재현하고 원인에서 수정했다.

최종1383개 소스의 hash는 df8c5cd75ecc097cf9250c0c1dedfdd05e9df4c8fc688f6717c31843cd068ef3다.
Go 타깃·공통 계층·CLI race461개 통과 이벤트(동일 이름 중복 제외455), Go395.449초와
CLI9.346초, vet/fmt/module/diff가 통과했다. 이전 전체 패키지 race도 통과했고 그 뒤 서버
헤더 변경의 수신/XML/MIME 영향 검사를 재실행했다. TypeScript 도구 전체 typecheck/test와
제공자 native/declaration·selection 문서 예제가 통과했다. 실제 HTTP1/2의 typed NDJSON·
MIME·custom protocol/adapter·slow consumer·취소·partial abort와 독립 CLI component
callback을 확인했다. 로컬 경과 시간은 성능 비교나 공개 benchmark가 아니다.

[W09 리뷰](w09-review.md)의8개 영역을 모두 검토했고 [기능 증거](w09-feature-evidence.json)에
배정5개 기능과3개 보충 계약 및 원시 로그/최종 source 해시를 연결했다. 구현 검증 record는
`dc272e14-de95-433e-a4a3-0781b263a4f1`, 별도 리뷰 검증 record는
`606cf881-d0ae-4418-81e5-a13942198fe9`다. task는 revision1668에서 current/done이다.
W10 도구·문서·playground, W11 전체 동등성·성능·플랫폼, W12 전체 최종 리뷰와 통합 검증은
계속 필수 작업이다. commit/push/출시는 수행하지 않았다.


## W10 — 구현·리뷰 완료

snapshot을 한 번 획득한 뒤 target별 verifier로 분기한다. Go는 사용자 모듈을 변경하지 않고
private 복사본의 실제 import를 바탕으로 의존 버전을 고정해 compile/run한다. Node/TypeScript
없는 실행, TOML package/import, compiler17 전파·정리와 기존 TypeScript profile/호출자가 통과했다.
Go suite/race/선택 coverage·성능 명령을 SSOT에 연결했고 CI의 suite 소유권을 유지했다.

Playground에는 Go target/import/package·server/metadata·Go 문법·대상별 ZIP을 연결했다.
실제 WASM에서 두 target과 모든 ZIP 파일 경로/바이트를 검사했다. EN/KO 브라우저에서 설정·
서버 명세 URL·오류·다운로드·새로고침을 확인했고, 예제 URL이 Go 설정을 잃던 문제와 SSR 초기
조작 경계를 보완했다. 문서 서버는 준비 확인 뒤 사용하고 stopped로 정리했다.

EN/KO Go 가이드와 실행 예제를 추가했다. 코드 블록은 locale 간 동일하다. 로컬 HTTP 예제는
타입 입력·Raw·Link·리소스·스트림·정적 선택·웹훅을 실행하며 private readonly module에서도
통과했다. PATH가 완전히 빈 환경의 native generator는 Go server SDK144파일을 생성했다.
npm source pack의 실제 경로와 Releaseway 소유 launcher 경계도 통과했다. 배포하지 않았다.

기존302개 feature ID와 무target TypeScript의 모든 필드·5조건은 JSON 구조상 그대로 유지했다.
Go profile의 상태·조건·필요 addon·failure scope/effect와 실제 테스트 경로를 추가했고 validator로
확인했다.18보충 계약의 단위별 근거를 연결하되 W11 전체 성능/플랫폼 통합은 대기로 구분했다.
W10 소유 보충 계약은 source-delivery와 playground-and-distribution의2개다.

최종1402개 소스 hash는14f373a2b238361053707821989e3e251abad8cb452a69ce0daadc6b23306458다.
영향 Go race206이벤트와 vet/fmt/module/diff·docs build/30문서도구검사·실제WASM/ZIP·예제·verifier·
성능 명령 smoke의11개 고정 소스 검사가 통과했다. Go CI 전체와 TypeScript tools CI도 통과했다.
명령 smoke를 성능 결론으로 취급하지 않는다. [W10 리뷰](w10-review.md)의8개 영역이 완료됐고
[기능/배포 증거](w10-feature-evidence.json)에 원시 로그와 source/binary/browser 경로 hash를 보존했다.

구현 record는 `8086da50-d95a-4075-9639-8a073ce9397b`, 별도 리뷰 record는
`d0d8dd09-1c5a-40e9-99b3-f482e79e17e0`다. task는 revision1675에서 current/done이다.
W11 전체 동등성·성능·플랫폼·출시 준비와 W12 전체 최종 리뷰/통합 검증은 계속 필수 작업이다.
commit/push/출시는 하지 않았다.

## W11 — 통합 검증 진행 중

Go 1.27.2에서 W11을 수행하고 있다. 동일한 TypeScript catalog의19개 fixture를
각 addon 설정으로 양쪽 타깃에 준비·방출하고, Go는 실제 전체 생성 패키지를 race로 실행했다.
방출 경로·숨김 경로·원본 inbound 정체성 비교도 통과했다. 이는 fixture 범위의 증거이며
302개 기능 전체의 런타임 동등성 통과를 뜻하지 않는다.

원본 SHA256/git blob을 검증한 modern·production32·holdout 및 제공자 코호트 감사에서
34건이 통과한 뒤 Cloudflare가300초를 넘었다. CPU/heap 진단은 Go 모델 계획에서
재귀 객체의 계속된 펼침을 확인했다. 실패 진단에서 live heap은 약10GiB였으며
해당 실행은 통과·일반 성능 표본으로 기록하지 않았다. 활성 객체 DTO를 포인터로 연결하는
수정을 검증하고 있다. 재귀 allOf·참조 제약을 유지하는 독립 생성 SDK 검사는 통과했다.
수정 뒤 동일 원본의 Go 준비 단계는 완료되며, 전체 방출과 나머지 코호트는 확인 중이다.

보안 감사의 OSV 입력이 root go.mod만 포함하던 누락도 수정했다. 새로운 Go 예제의
중첩 모듈까지 소스 의존성 목록에 포함하며, 감사 명령의 clean/findings/incomplete
상태 검사는 통과했다. 실제 취약점 DB 검사와 플랫폼·성능·compiler 통합은 아직 남아 있다.

W11 리뷰와 W12 전체 리뷰는 완료되지 않았다. 상세 원시 로그는 private
`/tmp/sdkgen-go-implementation/w11-*`에 보관한다.

### W11 중간 통합 결과

동일 원본 small-json fixture의 실제 TypeScript/Go 실행을 독립 기대값으로 검사했다.
일반·Raw·Unicode 경로·명시 Accept·잘못된 출력/JSON·선언 오류·금지 원시 인증 헤더를
대조했다. Go가 일반 호출에 Accept를 자동 삽입하던 차이를 원인에서 수정했고
미지정/명시 옵션과 실제 반환 값·오류·전송 횟수를 확인했다. 빈 문자열 경로는 이
fixture의 schema가 허용하므로 잘못된 실패 기대값을 제거했으며 입력은 바꾸지 않았다.

런타임46조건 각5표본, 네 소비자 각각5회 생성/cold/warm과 실제 실행117건,
CGO0 여섯 플랫폼 교차 빌드·native Linux 실행·실제 WASM10건이 통과했다.
[성능 기록](w11-performance.md)에 raw 경로와 심볼/의존성 해석·남은 범위를 적었다.
개발 도구의 VCS 조회 오류는 -buildvcs=false로 수정하고 실패 로그를 보존했다.
라이선스는 [고정 모듈 목록](w11-dependency-licenses.json)에 하위 라이선스 파일까지
해시를 남겼다. 실제 취약점 DB 검사는 아직 통과로 기록하지 않았다.

최종39건 원본 감사가 진행 중이며 Graph는 준비 완료 후 전체 방출을 검증 중이다.
비교 도구 리뷰에서 선언되지 않은 추가 경로·중복 경로/수신 계약과 Fetch 진단 식별 검사가
약했던 부분을 보강했다. 기존 실행 결과도 강화한 조건으로 검증해야 한다.
1k/10k·제공자 반복 생성, 전체 CI/compiler/docs/보안/출시 교차 빌드, W11 완료 리뷰와
W12는 남아 있다. 중간 측정·교차 빌드만으로 전체 구현 완료를 선언하지 않는다.


## W11 — 통합 구현·리뷰 완료

위 W11 중간 기록은 실행 당시 이력이다. 최종1415파일 소스의 SHA256은
`1e7c3b09d00dab78f12159ce6082fc1bf4f50eb02d7fd048d865729ef6e187c5`다.
302개 기능·5조건·18보충 계약과 기존 TS 보존을 실제 증거에 연결했다. catalog19건은
정상18건의 전체 생성 패키지 race 소비자와 의도한 오류1건으로 구분한다. 원본39건은
전체 방출·API/inbound 비교를 통과했고 제공자7개×선택1/10은 실제 격리 소비자14건이 통과했다.

공개 typed Stream 검증을 켠 runtime46조건×5표본, 실제소비자 cold/warm·심볼·의존117명령,
1k/10k 생성33명령, 플랫폼10명령, 원본 제공자105표본/107명령을 완료했다. 전체276명령과
원시 hash·시간 범위·RSS/liveheap·수명·실패 이력은 [성능 기록](w11-performance.md)에 있다.
모든 Go/TS runtime/tools/4compiler CI, docs·WASM/ZIP·예제·native emptyPATH·npm source pack·
release6교차 빌드와 실제 root/SDK 취약점 DB·의존 라이선스 검사가 통과했다.

재귀 allOf·일반/스트림 Accept·감사 inventory/비교/선택·측정 경계·가이드 옵션 결함을 고치고
영향 경로를 다시 확인했다. [W11 리뷰](w11-review.md)의8영역과 [기능 증거](w11-feature-evidence.json),
[원래 소스/실행을 보존한 적용성](w11-verified-applicability.json)을 연결했다. 실패한 첫 typed
측정 helper 중복 compile도 보존하며, 최종 새 소스에서 runtime과 영향 문서 검사를 재실행했다.

W11은 revision1691에서 done/current다. 구현 record는 `aa4df9e5-0c5d-4462-a282-e3642a77cac4`,
별도 리뷰 record는 `8eaf5468-7404-4d77-9cd0-808a686a783d`다. I01~I03도 revision1697까지
현재 pass를 기록했으며, W12 최종 리뷰는 별도 claim/basis 이후 수행한다.
거대한 full 제공자 전체 compile이나 다른 OS의 실제 실행으로 검증을 확대 해석하지 않는다.
commit/push/출시는 하지 않았다.


## W12 — 전체 최종 리뷰·workstream 완료

W11과 I01~I03 이후 W12를 claim하고 302/5/18·D01~D29와 전체12영역의 연결을 다시
검토했다. 공통 의미/기존 TS 호출자, Prepare/방출/publisher, 모델/실행/미디어/수신/선택과
CLI/playground/문서·배포·성능/플랫폼·증거 현재성을 확인했다. 제품 소스 변경은 없고
추가 수정 대상·미검토 영역·미결 설계도 없다. 선행25개 current-basis pass와 소스/원시 hash,
최종 diff/template 형식을 감사했다. [W12 리뷰](w12-review.md)와 [최종 감사](w12-final-evidence.json)에
범위·재사용·실제 검증·한계를 보존했다.

W12 리뷰 record는 `1d3af9e6-1603-4740-aca4-3b43c2e7cfd7`, 완료 revision은1701이다.
종료용 workstream basis가 W12 완료 상태를 반영하도록 I01~I03의 같은 소스 근거를 갱신하고,
I04와 실제 close를 통과했다. 최종 revision1710은12작업 done/current·27필수검증 충족이다.
[완료 기록](completion.json)에 최종 integration record와 close 응답 hash를 남겼다.
커밋·푸시·PR·출시는 수행하지 않았다.
