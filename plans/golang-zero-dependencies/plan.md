# 외부 의존성이 없는 Go SDK 구현 계획

작성: 2026-10-19. 기준: `72e679c`.
실행 상태와 검증 범위: [구현 기록](implementation.md), [최종 리뷰](w11-review.md) 및
workstream의 실제 완료·검증 기록을 따른다. 아래 기준 상태는 계획 작성 시점의 조사 이력이다.
사용자 결정: 생성 SDK의 외부 의존성을 모두 제거하고 필요한 검증은 제품이 자체 지원한다.
압축 협상·해제는 사용자 HTTP 전송 계층이 제어한다. 작업마다 리뷰→수정→재검증→커밋하고
마지막에 전체 통합 리뷰를 수행한다. 이 문서는 구현 명령이나 배포 실행 기록이 아니다.

## 1. 범위와 완료 계약

- 모든 Go 생성 모드(전체·선택·독립·동적 조회·server·metadata)의 생성 패키지는
  표준 라이브러리와 같은 생성 모듈 내부 패키지만 import한다. `require`, `replace`,
  vendor, CGO, 외부 바이너리, 런타임 다운로드가 필요하지 않는다.
- 클라이언트 호출·모델·오류·선택·스트림·Link·웹훅·콜백·범용 서버 검증을 유지한다.
  `pattern`, `patternProperties`, `format: regex`, 내장 format 검증도 생략하지 않는다.
- 현재 TypeScript의 `new RegExp(source, "u")`와 Go 검증의 의미 대응을 유지한다.
  미지원 패턴의 조용한 무시, RE2 대체, 사용자 검증기 필수화를 해결책으로 쓰지 않는다.
- Go 생성 경로의 정규식 사전 검증도 같은 자체 엔진으로 전환한다. 생성기의 직접
  `go-quickjs` 의존은 제거한다. OpenAPI/YAML/TOML 입력 처리 등 생성기 전체 의존성 제거는 범위 밖이다.
- 사용자 선택으로 애플리케이션에 설치한 HTTP 압축 라이브러리는 생성 SDK의 의존성과 구분한다.
  제품 예제·정상 검증·배포 smoke는 이 라이브러리 없이 실행한다.
- TypeScript 제품 동작은 유지한다. Go의 압축·원본 헤더 계약 변경은 아래 N02에서 명시한다.
- Go 출시와 문서·Playground·배포 smoke를 같은 출시 기준으로 준비한다. 게시 실행은 별도다.

기존 계획의 D09·D23·D25·D26과 6.1절은 이 문서가 대체한다. D10(의미가 다른 RE2 자동 대체 금지),
스키마·코덱·스트림·본문 수명 계약은 유지한다. 기존 연구와 완료 기록은 당시 구현의 이력이다.

## 2. 작성 시점에 조사한 기준 상태

| 소비 지점 | 기준 구현의 책임·의존 | 변경 영향 |
| --- | --- | --- |
| `internal/target/golang/plan_schema.go` | `jsregexp.Compile`로 생성 시 패턴 확인 | 자체 엔진과 생성 SDK에 같은 parser 필요 |
| `runtime/regex.go.tmpl`, `runtime/server-schema.go.tmpl` | 조건부 검증, 실행 중 받은 스키마의 패턴 컴파일 | 생성 시 컴파일만으로 대체 불가 |
| `plan_formats.go`, `plan_server_formats.go` | 일부 format이 lookaround를 포함한 정규식 사용 | 사용자 pattern이 없어도 자체 엔진 필요 |
| `runtime/transport-url.go.tmpl`, `runtime/format-url.go.tmpl` | WHATWG 파싱·IDNA·format 검증 | 요청 URL과 인증 origin, 비 HTTP format 모두 보존 |
| `runtime/transport-http.go.tmpl` | 압축 비활성화·Accept-Encoding 삽입·자동 해제 응답 거부 | 사용자 전송 정책을 존중하도록 변경 |
| `runtime/transport-body.go.tmpl` | gzip/deflate/br/zstd 해제와 Body 수명 관리가 결합 | 해제를 제거하면서 수명 관리는 보존 |
| `plan_files.go` | transport-body가 기본 root, regex/format은 조건부 | 새 패키지 도달 가능성과 import 재계획 |

기준 구현의 직접 의존은 brotli, compress/zstd, whatwg-url, go-quickjs/jsregexp였다.
whatwg-url의 bitset·x/net·x/text 간접 의존도 함께 사라진다.
기준 fixture 42개는 회귀 표본이며 정규식 전체 적합성 증거가 아니다.
기존 URL 28개 표본 역시 WHATWG 전체 적합성 증거가 아니다.

## 3. 확정 결정

| ID | 결정 |
| --- | --- |
| N01 | 자체 구현은 `internal/goruntime/`의 표준 Go 소스를 단일 원본으로 둔다. 표준 라이브러리만 사용한다. |
| N02 | 모든 압축은 HTTP 계층 소유다. SDK가 Accept-Encoding·DisableCompression을 자동 설정하거나 압축을 해제하지 않는다. |
| N03 | SDK의 Raw·코덱·Link·헤더 검증은 HTTP 계층이 반환한 Body·Header·Trailer를 사용한다. 사라진 원본 헤더를 복원·보장하지 않는다. |
| N04 | 사용자 HTTPClient의 자동 gzip 해제 및 `Uncompressed=true`를 허용한다. ResponseBodyMode와 Capabilities.ResponseBody 및 관련 enum/별칭은 제거한다. 다른 capability는 유지한다. |
| N05 | 정규식은 ECMAScript 2026의 Unicode `u` 의미를 기준으로 직접 구현한다. 별도 JavaScript VM, 정규식 치환식, regexp fallback은 사용하지 않는다. |
| N06 | 정규식의 공개 사용 목적은 컴파일 성공 여부와 boolean 검색이다. 내부 캡처·역참조는 구현하되 JS 객체 API·replace·split·g/y/v 모드를 새로 제공하지 않는다. |
| N07 | URL은 WHATWG parsing/serialization과 Unicode 17 IDNA 처리의 자체 구현을 공유한다. HTTP 요청 표현으로 바꾸는 마지막 경계에서만 net/url을 사용한다. |
| N08 | 정규식 속성·IDNA·정규화 표는 Unicode 17.0.0 공식 자료에서 재현 생성한다. Go unicode 표 버전에 의미를 맡기지 않는다. |
| N09 | 외부 엔진 코드를 vendor·복사해서 의존성 제거를 주장하지 않는다. 표준의 알고리즘을 자체 구현하고 공식 데이터·테스트의 출처와 고지를 보존한다. |
| N10 | 정적 스키마는 생성 시 패턴을 컴파일하고 결정적 프로그램 자료를 출력한다. 동적 서버 스키마와 regex format은 런타임 compiler를 포함한다. |
| N11 | Unicode 자료의 출력은 필요한 기능으로 나누되, 동적 패턴을 받는 경로에는 필요한 전체 parser·속성 조회 표를 포함한다. 정확성을 위해 필요한 표를 제거하지 않는다. |
| N12 | 새 옵션이나 선택적 외부 엔진 플러그인은 추가하지 않는다. 일반 HTTP 확장 지점은 기존 http.Client/RoundTripper다. |

### 3.1 HTTP 응답 경계

`HTTPClient/Transport → 반환된 http.Response → BodyLease → 상태·헤더·미디어 선택 → DTO/Raw/Stream`.
SDK는 client를 값으로 복사해 기존 timeout·credentials·redirect 정책을 적용하고 연결 풀은 공유한다.
압축을 이유로 Transport를 복제·교체하거나 사용자의 설정·헤더를 수정하지 않는다.
기존 응답의 `Header`, `ContentLength`, `Uncompressed`를 압축 목적으로 다시 쓰지 않는다.

`BodyLease`는 decode 계층 없이 닫기 1회·취소·EOF·에러 경로의 소유권을 담당한다.
HEAD·본문 없는 상태·Raw 및 Stream 소유권 이전·중간 실패 정리를 보존한다.
제한은 SDK가 실제로 읽는 바이트에 적용한다. HTTP 계층이 해제하면 해제 후 바이트가 대상이다.
압축된 Body가 그대로 오면 binary·순차형 Raw는 해당 바이트를 노출한다. JSON 등 일반 Raw와 typed
호출은 기존 미디어 코덱의 decode·검증 결과를 따른다. Raw를 검증 생략 API로 바꾸지 않는다.
Content-Encoding만 보고 거부하지 않는다. 사용자 코덱이 해당 본문을 처리할 수도 있기 때문이다.

기본 net/http 동작에 따라 gzip 자동 해제 시 Content-Encoding/Content-Length가 없어질 수 있다.
선언된 응답 헤더가 필수라면 현재 받은 헤더를 기준으로 기존 검증 오류를 반환한다.
원래 헤더가 필요한 사용자는 자신의 전송 계층에서 본문과 헤더를 구성한다.
SDK는 원본 헤더 보존을 compression capability로 강제하지 않는다.

### 3.2 정규식 parser·compiler·VM

단일 원본은 `internal/goruntime/ecmaregexp/`다. 주 진입은 `Compile(source string)`과
`(*Program).Match(ctx context.Context, value string) (bool, error)`다.
compiler는 group 이름·순서·전방 숫자 참조를 수집한 뒤 검증한다. scanner/AST/프로그램은
JavaScript UTF-16·`u` code point 의미를 명시하며 기존 wire 문자열의 고립 surrogate를 보존한다.

포함 범위: 분기·반복·greedy/lazy·캡처/이름 캡처·전후방 positive/negative assertion·가변 길이
lookbehind·숫자/이름 역참조·미참여 capture·반복 capture 초기화·빈 매치 진전·앵커·word boundary·
문자 클래스/escape·Unicode 속성·허용되는 이름 중복과 scoped modifier 등 `u` 문법 전체다.
문자열 API를 재현하지 않아도 내부 capture와 lookbehind 역방향 평가 순서는 필요하다.
`\s`, `.`, `$`, surrogate·속성 alias의 정확성은 독립 기대값으로 검증한다.

VM은 명시적 instruction·backtrack/capture stack을 사용하는 반복 실행기로 구현한다.
프로그램은 불변이고 cursor·capture·작업량은 Match 호출마다 소유한다. 공유 mutable matcher와
전역 무제한 캐시는 두지 않는다. lookaround와 역참조의 문자 비교도 실행 비용에 포함한다.
기존 엔진의 100,000,000 작업 budget을 초기 상한으로 유지하며 검색 시작 위치와 assertion 전체가
하나의 Match 예산을 공유한다. 1,024회 작업마다 context를 검사하고 긴 문자 비교도 같은 경로를 거친다.
중첩 한계는 기존 100,000을 기준으로 명시적 stack에서 검사한다. 잘못된 패턴과 자원 한계는 구분한다.

컴파일 syntax 실패는 기존 생성 진단/동적 스키마 컴파일 실패다. 실행 한계는 자체 `ErrComplexity`를
`ValidationLimitExceeded`에 감싸고 일반 불일치로 바꾸지 않는다. 취소는 context 오류로 전달한다.
anyOf/oneOf/not 등 합성 검증을 통과할 때도 실행 실패를 불일치로 삼키지 않는다.
이전 외부 ErrComplexity의 errors.Is identity는 내부 엔진 sentinel로 바뀌며 SDK 오류 코드는 유지한다.

### 3.3 Unicode·URL

`internal/goruntime/unicode17/`는 ECMAScript 속성·IDNA mapping·NFC 정규화 자료와 함수를 제공한다.
속성 alias·Script_Extensions·binary property·새 script·Unicode case folding·IDNA joining type,
canonical decomposition/reordering/composition,
Hangul 정규화를 다룬다. 생성 도구는 `scripts/goruntime-data/`에 두고 계획의 unicode-inputs.json을
검증한 입력만 사용한다. 정상 빌드·생성은 네트워크를 사용하지 않는다. 표와 출처/라이선스는 저장한다.

`internal/goruntime/urlstd/`는 base resolution, special/non-special/opaque/file URL, 사용자 정보·port·
percent encoding, IPv4 변형·IPv6·IDNA host와 serialization을 담당한다.
비 HTTP scheme은 URI/IRI format에 필요하고 실제 요청 단계만 HTTP(S)로 제한한다.
IDNA는 WHATWG가 호출하는 UTS #46 옵션을 그대로 적용한다. STD3·transitional·Bidi·Joiner·
hyphen/DNS-length 검사를 일괄 추측하지 않고 고정한 URL 표준의 domain-to-ASCII 절차대로 구분한다.

URL·origin·credential host 비교는 같은 canonical host·scheme·effective port 함수를 사용한다.
비정상 URL을 관대하게 net/url로 다시 해석하는 fallback은 두지 않는다.
기존 독립 query 직렬화 구현은 보존하고 URLSearchParams 의미를 URL parser에 재위임하지 않는다.

### 3.4 코드 생성 구조와 데이터 흐름

```txt
compiler/IR → schema/plan → Go planner → runtime 요구 목록 → Go 방출기 → publisher
                                 ↓                 ↑
                    goruntime compiler      같은 goruntime Go 원본
```

`internal/goruntime/assets.go`의 명시적 embed 목록은 runtime 소스와 데이터만 포함한다.
방출기는 Go AST import 선언의 `openapi-sdkgen/internal/goruntime/...`만
`<go-import-path>/internal/runtime/...`로 재작성한다. 본문 문자열 치환이나 test 파일 복사는 하지 않는다.
파일별 요구 목록과 package 의존 DAG를 `plan_files.go`와 연결한다.

정적 pattern 자료는 버전이 붙은 instruction/상수 자료다. 외부 입력 bytecode를 받는 공개 API는 없다.
출력 코드·자료를 generator fingerprint에 포함하고 사용자 파일 보존·incremental·check 결정성을 유지한다.
기존 schema helper는 ecma 엔진 오류를 wire 오류로 변환한다. 공통 schema/plan은 Go 엔진을 import하지 않는다.
URL engine은 transport와 format이 공유하고, Unicode table은 기능별 파일로 분리한다.
동적 server가 없는 정적 경로에서 parser가 제거되는지 실제 링크 결과로 확인한다.

## 4. 검증 원천과 조사 상태

| 원천 | 이번에 고정한 기준 | 사용 범위 |
| --- | --- | --- |
| ECMAScript | [2026 고정판](https://tc39.es/ecma262/2026/multipage/text-processing.html#sec-patterns) | RegExp `u` parser/Matcher 의미; API 전체 구현으로 확대하지 않음 |
| Test262 | `d53ebc8b1931d9f1541e43d59650776e4b766248` | RegExp syntax·boolean 매칭에 관련된 fixture와 예상 결과 |
| WHATWG URL | `fde3f74f063341a28d437216f75735b8b40128f5` | URL parser·serializer·domain-to-ASCII 절차 |
| WPT | `6cd3f1c39e2aec3d58f140aa0ff2991d79eb9991` | URL 파싱·hostname·직렬화·IDNA 표본 |
| Unicode | 17.0.0, [입력 해시](unicode-inputs.json) 15개 | 속성·NFC·IDNA mapping/test |
| 기존 제품 | regex-cases.json 42개, format-probe URL 28개, Go/TS runtime tests | 회귀 기준 및 새 누락 탐지 |

[URL 표준](https://url.spec.whatwg.org/), [UTS #46](https://www.unicode.org/reports/tr46/),
[net/http 압축 계약](https://pkg.go.dev/net/http#Transport)을 확인했다.
Test262/WPT commit은 공식 저장소 API로 확인했고 Unicode 15개 원문을 내려받아 SHA256을 계산했다.
이는 조사·pinning 결과다. 새 알고리즘 실행이나 전체 테스트 적합성 통과를 뜻하지 않는다.
fixture 전문·라이선스·필터 목록은 W01에서 고정 commit으로 가져와 보존한다.

Test262 전체 JS 테스트를 Go에 그대로 실행하지 않는다. 컴파일 허용/거부·boolean 결과를 추출하는
adapter를 만들고 사용 사례와 제외 사유를 전수 manifest로 남긴다. JS 객체 API만의 테스트는 범위 밖이며,
구현하지 못한 문법을 제외 목록으로 옮겨 통과시키지 않는다. Node 비교는 보조 수단이고 정상 CI에 JS 실행을
SDK 런타임 요구로 추가하지 않는다. Unicode 버전 차이가 있는 oracle은 버전을 기록하고 고정 기대값을 쓴다.

## 5. 작업 단위와 단위별 리뷰·커밋

각 단위는 결과물을 만든 뒤 지정 영역 리뷰→확인된 결함 수정→영향 검사 재실행→단위 커밋 순서다.
단위별 실행 상태와 검증 결과는 workstream과 `implementation.md`에 기록한다. 의존 순서는 표의 선행 단위가 실제 자료·구현을 제공하는 경우만 지정한다.

| 단위 | 선행 | 결과·주요 경로 | 완료 증거와 리뷰 범위 | 커밋 |
| --- | --- | --- | --- | --- |
| W01 | 없음 | 규격/fixture pin·현재 동작·생성/빌드/메모리 baseline. `testdata/goruntime`, `scripts/goruntime-data`, 검증 manifest | 42/28 회귀와 Test262/WPT 적용 범위, 전송 계약 변경 목록, 재현 가능한 baseline. 규격 누락·라이선스·oracle 독립성 리뷰 | `test(golang): pin dependency-free runtime contracts` |
| W02 | W01 | 압축을 HTTP 계층에 위임. transport-http/body, types-transport, core/execution, root aliases | 기본 gzip·사용자 해제·해제 안 된 Raw·명시 Accept-Encoding·Range·헤더 필수 검증·custom codec. shared client 정책/연결 풀/닫기/취소/race 리뷰. br/zstd 생산용 import 제거 | `refactor(golang): delegate compression to HTTP transport` |
| W03 | W01 | Unicode17 데이터 생성기·정규화/속성/IDNA 자료. `internal/goruntime/unicode17` | 15개 hash, 재생성 diff 없음, NormalizationTest/IDNA 데이터, alias/script/emoji·surrogate. 데이터 접근 비용·출처·버전 리뷰 | `feat(golang): add owned Unicode runtime data` |
| W04 | W01,W03 | ecma scanner/parser/compiler, 구문 오류·프로그램 자료 계약 | syntax-positive/negative·참조·중첩·잘못된 escape·문법 manifest. 재귀 stack/정수 overflow·결정성 리뷰 | `feat(golang): compile ECMAScript Unicode patterns` |
| W05 | W04 | boolean matcher와 capture/assertion/Unicode 실행·context/budget | 42개 및 적용 Test262·독립 경계값·fuzz. 반복·역방향 lookbehind·미참여 capture·오류 전파·동시성·성능 리뷰 | `feat(golang): execute bounded ECMAScript patterns` |
| W06 | W03 | urlstd parsing/IDNA/serialization·origin 함수 | 28개 및 관련 WPT·UTS46 사례, HTTP/nonHTTP/file/opaque·default port·IPv4/6·Bidi/Joiner. 인증 origin·중복 해석·불변 데이터 리뷰 | `feat(golang): add owned URL and IDNA runtime` |
| W07 | W05,W06 | 단일 원본 방출·static program·dynamic compiler 연결. assets, plan_files/schema/formats/server, runtime wrappers | 같은 source를 생성기/생성 SDK에서 실행, 정적·동적·server format·patternProperties·regex format 동일 결과. DAG·import·테이블 누락·오류 단계·생성 결정성 리뷰 | `feat(golang): emit dependency-free validation runtime` |
| W08 | W02,W07 | 생성기 jsregexp 제거, 격리 소비자·fixture·Go example·모듈·라이선스 정리 | 새 소비자에 go.mod만 두고 GOWORK=off/GOPROXY=off/GOSUMDB=off/GOTOOLCHAIN=local/CGO_ENABLED=0, 외부 module cache 없이 모든 모드 compile/run. go list 실제 그래프·import와 require/replace 없음 확인. test helper 핀 누락 우회 금지 리뷰 | `refactor(golang): remove external SDK runtime modules` |
| W09 | W08 | docs/ENKO·README·Go 예제·문서 실행기·Playground 도움말·배포 smoke·측정 도구 갱신 | 의존 설치 명령 없음, 전송 헤더 변화/압축 위임 설명, 직접 소비·WASM 생성 후 Go 컴파일·native/npm smoke. 이전 fixed go.mod 복사·test import로 외부 의존 숨김 없는지 리뷰 | `docs(golang): document standard-library-only SDKs` |
| W10 | W08,W09 | 성능 비교와 플랫폼 통합 검증, 실제 자료 보존 | 아래 성능 matrix·6개 플랫폼 cross-build·네이티브 실행·WASM build/생성, Go/TS/문서 영향 검사. 잘못된 측정·조건부 표 제거·컴파일 비용·지원 모드 누락 리뷰 | `test(golang): verify runtime parity and resource costs` |
| W11 | W10 | 모든 변경의 최종 리뷰 루프와 인계 기록 | 책임/경계·정규식·URL/Unicode·전송/수명·선택/방출·제품 전체 기능·문서/출시·성능/무의존 검증 전 영역, 수정 후 영향 영역 재검증, 남은 결함 없음. 실제 실행 결과와 pending 분리 | `test(golang): finalize zero-dependency runtime verification` |

W04/W05/W07은 완성 엔진을 연결하는 연속 단위다. 중간 commit은 출시 가능한 기능 완료를 의미하지 않는다.
정규식·URL 엔진 중 하나라도 기존 지원을 충족하지 못하면 범위를 축소하지 않고 해당 단위에서 보완한다.
각 리뷰는 책임·공개 계약·실패/수명·동시성·생성 도달 범위를 pending/reviewed/needs-recheck로
기록하고 전부 reviewed일 때 끝낸다. 발견 사항·수정·재실행 근거는 해당 단위 기록에 남긴다.

## 6. 성능·수명 검증

외부 module이 없어져도 자체 데이터·엔진의 소스·바이너리·실행 비용은 남는다.
기존 `scripts/perf/golang` 도구를 확장하고 기준/후보는 같은 입력·도구·옵션에서 5회씩 순차 측정한다.
평균만으로 숨기지 않고 원시 값·실패·환경·commit/input/output hash를 보존한다.

| 영역 | 시나리오 | 기록 |
| --- | --- | --- |
| 정규식 | ASCII·Unicode·lookaround·역참조·중첩·실패·1KiB/1MiB 입력·병적 패턴 | compile/match 분리, ns/op·B/op·allocs/op·취소·budget 도달·재사용 |
| URL·Unicode | 일반 URL·긴 IDN·상대 경로·IPv4/6·거부 입력·NFC | parsing/IDNA/normalization 비용·할당·정규화 결과 |
| 생성 | 작은 JSON·regex-heavy·server·고정 제공자·1/10/full 선택 | 시간·peak RSS·출력 파일/바이트·도달 runtime/table |
| 소비자 빌드 | 단일 호출·선택·독립·계산된 동적 조회·server | 5회 cold/warm 시간·RSS·binary·실제 deps·심볼·미사용 parser/table |
| HTTP·stream | 작은 요청·대형 업로드·SSE/multipart·취소·느린 소비자 | 최초 바이트·처리량·할당·live heap·body close·goroutine/race |
| 압축 경계 | 표준 gzip·정적 압축 응답 fixture·사용자 해제 전송 | HTTP 계층 비용과 SDK 비용 분리; SDK br/zstd 해제 benchmark 제거 |

br/zstd 응답은 고정된 압축 바이트 fixture와 그에 대응하는 알려진 평문을 이용한다.
SDK 검증을 위해 외부 decoder를 정상 소비자 모듈에 다시 넣지 않는다. wrapper가 반환하는 평문·헤더의
전달, encoded Raw의 일치 여부로 SDK 책임을 검증한다. 실제 외부 decoder 정확성은 해당 사용자 계층 범위다.

고정 비율 CI 성능 gate는 추가하지 않는다. 계획한 측정 누락·무한/폭발적 자원 사용·잘못된 결과는 실패다.
측정상 중대한 비용 증가가 나오면 원인·tradeoff·수정 결과를 기록하고 보완한다. 근거 없는 개선 수치는 쓰지 않는다.
공개 수치는 기존 정책대로 GitHub Actions에서 측정한 run에만 연결한다.

## 7. 통합 검사와 출시 전환

기존 SSOT: `devtools run golang:verify`, `golang:verify race`, `ci:dev go`,
`typescript:verify`, `go:example`, `build:docs`, `verify:docs`, `playground:verify`.
기존 release 검증·cross-build·npm/native smoke를 그대로 확장한다. 새 goruntime 패키지를
일반 Go CI·race·fuzz 대상에 명시적으로 포함한다. 기존 Go target 한 패키지 검사만으로 완료하지 않는다.
fixture 변경은 새 계약의 독립 기대값으로 수정하며 이전 검증 보호를 일괄 삭제하지 않는다.

- 생성 모드 matrix 전체에서 실제 go list -deps -json의 Standard/Module 정보를 검사한다.
  문자열에 점이 없다는 추측으로 표준 패키지 여부를 판단하지 않는다.
- 기존 압축 capability 공개 API 제거에 따라 consumer·root reexport·inspect/metadata·문서·smoke
  호출부를 함께 바꾼다. 진단/내부 compiler 버전과 incremental fingerprint도 갱신한다.
- Go 첫 출시 전에 통합한다. 이미 배포된 버전이 발견되면 N02/N04와 외부 오류 sentinel 변경을
  migration/release note의 breaking change로 기재하고 배포 버전 정책을 따른다.
- 기존 workstream 완료 기록은 과거 사실로 유지한다. 새 workstream 등록은 별도 등록 요청 시 이 W01–W11을 사용한다.
- 복구는 출시 전 후보 변경 단위를 되돌리고 기존 검증된 릴리스 소스를 사용한다. 생성된 사용자 소스나
  go.mod를 자동 수정·삭제하지 않는다. 소비자 go mod tidy는 사용자가 실행한다.

## 8. 계획 리뷰 결과와 남은 검증

| 리뷰 영역 | 보완한 사항 | 상태 |
| --- | --- | --- |
| 요구/경계 | generator 전체 무의존과 생성 SDK 무의존 구분, 압축 위임을 명시적 계약 변경으로 기록 | 검토 완료 |
| 정규식 | 런타임 서버 compiler·format 내부 패턴·UTF16/고립 surrogate·역방향 assertion·complexity 전파 추가 | 검토 완료 |
| Unicode/URL | NFC·case folding·IDNA joining type 자료와 hash, nonHTTP format, 인증 origin 동일 구현, query 별도 의미 보존 | 검토 완료 |
| 생성/선택 | source 단일 원본·AST import 재작성·static/dynamic compiler 분리·표 도달 가능성 추가 | 검토 완료 |
| 전송/수명 | 원본 헤더 보장 제거 영향·필수 헤더 실패·Raw encoded·custom codec·close/cancel 보존 명시 | 검토 완료 |
| 증거/출시 | 외부 module cache 없는 소비자, 측정 도구의 decoder 의존 제거, release/docs/WASM·플랫폼 포함 | 검토 완료 |

책임·경계 리뷰와 세부 알고리즘·검증 리뷰에서 위 누락을 보완하고 작업 단위·완료 기준에 반영했다.
후속 재검토에서 scoped modifier용 CaseFolding과 IDNA Joiner용 DerivedJoiningType 입력을 추가했다.
제품 방향·책임·알고리즘·데이터 버전·오류·출시 경계의 미결 선택은 없다.
계획 작성 시점에는 새 엔진의 적합성·성능이 미검증이었다. W01–W11의 실제 검증 결과와
출시 후에 실행할 검사는 구현·최종 리뷰 기록에서 구분한다. 계획 검토 자체를 제품 검증으로 취급하지 않는다.
