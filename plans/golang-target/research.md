# Go 타깃 설계의 조사·검증 근거

2026-10-17에 현재 소스와 공식 자료를 조사하고 별도 Go 모듈에서 시제품을 실행했다.
제품의 Go 타깃을 구현한 결과는 아니다. 최종 구현 계약은 [계획](plan.md)에 있다.

## 1. 정규식 엔진 결정

현재 TypeScript 검증기는 `new RegExp(pattern, "u")`를 사용한다.
[문자열 패턴](../../internal/target/typescript/runtime/schema/assertions/wire-string-pattern.ts),
[객체 속성 패턴](../../internal/target/typescript/runtime/schema/assertions/wire-pattern.ts),
[regex format](../../internal/target/typescript/runtime/schema/assertions/wire-format-regex.ts)가
같은 의미를 요구한다. Go에서 pattern을 보존만 하고 실행 검사를 생략하는 것은 대응이 아니다.

동일한 [42개 입력](probes/regex-cases.json)을 현재 프로젝트 Node에서 실행하고
문법 허용 여부와 boolean 매칭 결과를 비교했다. lookaround·참조·그룹 순서·미참여 그룹,
Unicode code point·surrogate pair·script·script extension·문자 클래스·줄바꿈·잘못된
escape·다른 엔진 전용 문법을 포함한다.

| 후보 | 고정한 버전 | Node와 다른 사례 | 결정 |
| --- | --- | ---: | --- |
| Go `regexp` | Go 1.27.1 | 29/42 | 기본 경로로 채택하지 않는다. |
| `regexp2`, ECMAScript·Unicode·원래 capture 순서 | v2.9.1 | 7/42 | 직접 사용하지 않는다. |
| Goja `RegExp` | v0.0.0-20261016120441-313b6b662e53 | 3/42 | 전체 VM 의존과 문법 차이 때문에 채택하지 않는다. |
| `go-quickjs/jsregexp`, `Compile(pattern, "u")` | v0.27.0 | 0/42 | 생성기·생성 SDK의 정규식 실행 경로로 채택한다. |

`regexp2`에서는 surrogate pair 패턴과 일부 잘못된 Unicode 문법에서 차이가 났다.
Goja에서는 `\a`, 잘못된 10진 참조, 불완전 quantifier의 문법 허용이 달랐다.
정확한 사례와 비교 코드는 [시제품](probes/regex_test.go)에 보존한다.

선택한 패키지는 ECMAScript 전용 정규식 API를 제공하고, 전체 JS 엔진 없이 import할 수 있다.
실제 `go list -deps`의 외부 패키지는 같은 모듈의 `jsregexp`, `internal/regexp`,
`internal/wtf8` 세 개였다. CGO 파일은 없었고 JS VM·Intl 패키지는 이 의존 경로에 없었다.
모듈 전체의 `go.mod` 요구와 실제 import·링크 범위는 구분한다.

공유 패턴을 16개 goroutine에서 실행하고 race 검사를 통과했다. 병적인 패턴에서는
`ErrComplexity`를 받았고 같은 컴파일 결과로 다음 정상 입력을 다시 처리했다.
원시 패키지는 유한 step budget을 사용하므로 즉시 context 선점과 같은 의미로 설명하지 않는다.

Unicode 버전도 별도로 확인했다. 현재 Node v24.21.0은 Unicode 17.0·ICU 78.3을
사용하고 선택 패키지는 Unicode 17.0.0 표를 내장한다. Unicode 17에서 추가된 Tai Yo
script 문자를 양쪽에서 검사했다. Go 표준 라이브러리의 표 버전으로 정규식 의미를
암묵적으로 바꾸지 않도록 선택 패키지 버전과 속성 표를 함께 고정한다.

ASCII 패턴의 간단한 실행 측정도 3회 수행했다. 표준 엔진과 선택 엔진 모두 이 입력에서는
0 B/op·0 allocs/op였다. 이 결과는 Unicode·복잡한 분기·SDK 전체 성능을 보장하지 않는다.
`CompileUTF8`도 측정했지만 문자열 보존 경계를 일관되게 유지하기 위해 기본 API는
`Compile(..., "u")`로 확정했다. fast path를 선택하는 추가 정책은 넣지 않는다.

공식 근거:

- [Go regexp: RE2 문법과 실행 특성](https://pkg.go.dev/regexp)
- [ECMAScript RegExp 명세](https://tc39.es/ecma262/multipage/text-processing.html#sec-regexp-regular-expression-objects)
- [선택 버전의 jsregexp API 소스](https://github.com/go-quickjs/go-quickjs/blob/v0.27.0/jsregexp/jsregexp.go)
- [선택 버전의 jsregexp 문서](https://github.com/go-quickjs/go-quickjs/blob/v0.27.0/jsregexp/README.md)
- [선택 버전의 모듈 계약](https://github.com/go-quickjs/go-quickjs/blob/v0.27.0/go.mod)
- [선택 버전 MIT 라이선스](https://github.com/go-quickjs/go-quickjs/blob/v0.27.0/LICENSE)
- [regexp2 ECMAScript API·한계](https://github.com/dlclark/regexp2#ecmascript-compatibility-mode)

의존성 검토에서는 라이선스·필요 import·CGO·동시 사용·복잡도 실패 경로와 Go 최소 버전을
확인했다. Go 취약점 데이터베이스의 module index에서 선택 모듈의 등록 항목은 없었다.
이 조회는 취약점이 없다는 보증이나 전체 의존성 보안 감사가 아니다. 구현 시 버전·checksum을
고정하고 현재 프로젝트의 의존성 검사도 수행한다. 계획 검증 모듈의 대체 후보들은
제품 의존성에 추가하지 않았다.

## 2. 값·숫자·JSON 결정

[값 경계 시제품](probes/contracts_test.go)에서 다음을 실행했다.

- 선택 nullable boolean의 누락·null·false·true를 서로 다른 JSON으로 출력했다.
- 빈 문자열·0·빈 배열이 누락되지 않는 것을 확인했다.
- 기본 구조체 디코딩이 `ID`를 `id` 필드에 매칭하는 것을 재현했다.
- 정확한 raw key 조회와 추가 속성의 원시 값 보존 경로를 확인했다.
- 기본 JSON 디코더는 lone surrogate를 U+FFFD로 바꾸고 Node는 보존하는 것을 확인했다.
- nil 배열은 기본 JSON에서 null, 빈 배열은 `[]`이 되는 것을 확인했다.
- JSON 중복 키는 마지막 값이 되는 현재 경계를 재현했다.
- 9개 binary64 숫자의 출력이 `JSON.stringify`와 같은 것을 확인했다.
- `0.3`과 `0.1`의 부동소수점 나머지 대신 최단 십진 표현의 유리수 검사가 필요한
  반례를 재현했다.

따라서 부모 객체가 선택 여부를 제어하는 모델, 스키마에 맞는 nil collection 정규화,
문자열·키·존재 정보를 보존하는 wire decoder로 결정했다. raw map만으로 lone surrogate와
키의 모든 문자열 의미까지 해결됐다고 주장하지 않는다. 실제 스캐너·모델 생성·스키마
프로그램 구현과 전체 fixture 검증은 W02의 산출물이다.

Go 버전은 현재 프로젝트와 같은 1.27 minor로 정했다. 생성 코드에서 실험적 JSON API에
의존하지 않는다. `encoding/json`은 메타데이터 등 의미가 맞는 경계에서 사용할 수 있지만,
기본 구조체 변환을 전체 스키마 계약으로 삼지 않는다. binary64 선택은 편의상 `int64`로
범위를 좁히는 결정을 없애고 TypeScript가 표현하는 정수 입력 전체를 유지한다.

공식 근거:

- [Go JSON의 이름 매칭·수치·문자열 동작](https://pkg.go.dev/encoding/json)
- [Go 1.27의 JSON 변경과 실험 API 경계](https://go.dev/doc/go1.27)
- [현재 스키마 계획](../../internal/schema/plan/node.go)
- [현재 문자열 길이 검증](../../internal/target/typescript/runtime/schema/assertions/wire-string.ts)
- [현재 multipleOf 검증](../../internal/target/typescript/runtime/schema/assertions/wire-multiple-of.ts)

## 3. HTTP 수명·도달성 결정

[HTTP 시제품](probes/http_test.go)에서 공유 클라이언트의 연속 3회 요청이 한 연결을
사용했고, 열린 SSE의 context 취소가 서버까지 전달됐다. 전체 Go SDK의 timeout·오류
분류·source cleanup을 구현하거나 증명한 것은 아니다. 이 계약의 실현 가능성을 확인했다.

[링커 시제품](probes/linker_test.go)은 같은 모듈의 JSON·XML 메서드를 두 경로에서
빌드했다. JSON 직접 호출에서는 XML marker 심볼이 없었고 실행 중 키를 고르는 map에서는
남았다. 패키지·메서드를 만들었다는 이유만으로 모든 실행 코드가 보존되는 것은 아니다.
초기화와 함수 참조로 전체 의존을 연결하면 제거 범위가 줄어든다는 근거로 사용한다.
실제 SDK의 바이너리 절감량은 아직 측정하지 않았다.

공식 근거:

- [net/http 클라이언트 재사용·응답 본문 수명](https://pkg.go.dev/net/http#Client)
- [Go 링커 도달성 처리](https://go.dev/src/cmd/link/internal/ld/deadcode.go)
- [reflect 메서드 조회와 메서드 보존](https://pkg.go.dev/reflect#Value.MethodByName)
- [Go 진단 도구](https://go.dev/doc/diagnostics)

## 4. 검증 재실행과 해석

프로젝트 환경을 선택한 상태에서 다음 명령으로 시제품을 실행한다.

```sh
devtools run -- bash plans/golang-target/probes/run.sh
```

시제품은 별도 Go 모듈이다. 제품 `go.mod`·`go.sum`과 사용자 수정 파일은 변경하지 않는다.
첫 실행은 고정된 후보 모듈을 다운로드할 수 있다. Node는 TypeScript 쪽 결과를 비교하는
조사 도구이며 생성된 Go SDK의 실행 의존성이 아니다.

실행은 전체 시제품, race, `CGO_ENABLED=0` 전체 시제품, 표적 microbenchmark,
실제 정규식·압축 import 집합, `go vet`를
확인한다. 로그는 실행마다 `.tmp/golang-target-plan/run-*`에 분리한다. 실행 전에 기준
소스 해시를 고정하고 명령·exit code·stdout/stderr 해시를 저장한다. 종료 때 소스 해시를
다시 비교하고 성공한 결과만 [검증 요약](evidence.json)에 원자적으로 기록한다.
테스트 목록은 실제 `go test -list`로 찾고 일반·race·CGO 비활성 실행에서 모든 최상위 테스트가
실행됐는지 확인한다. skip·누락·실패·변조 로그·예전 소스의 로그는 거부한다. Node의
Unicode·ICU를 포함한 버전과 정규식 비교 수치는 실행 출력에서 읽는다. 이전 기록의
성공 상태를 물려받지 않는다. [도구 회귀 검사](test_evidence.py)는 소스 변경 뒤 이전
로그의 재사용·실패한 vet·누락/skip·로그 변경·불완전 benchmark·동시 실행 분리를 다룬다.
저장된 요약의 재검증도 원시 로그를 요구하므로 새 checkout에서는 위 명령부터 실행한다.
이 기록은 서명된 외부 증명이 아니라 로컬 실행과 소스의 일치 확인이다. 로컬 microbenchmark를 공개 제품
성능 수치로 사용하지 않는다.

계획에는 공개 API·의존성·플랫폼·기본값·소유권·오류·검증 방식에 대한 미결 선택을 남기지
않았다. 미래 구현의 모든 입력을 이미 실행했다는 뜻은 아니다. 전체 동등성과 성능은
계획의 작업별·통합 완료 기준으로 따로 검증한다.

## 5. 개발 이력과 추정의 한계

기준 HEAD의 Git 이력에서 기능 단위와 후속 수정 범위를 조사했다. 커밋 시각은 관찰값이고
집중 작업 시간은 기록되지 않아 아래 범위를 시간당 생산성으로 환산하지 않는다.

| 관찰 구간·커밋 | 확인한 변경 | 계획에 적용한 판단 |
| --- | --- | --- |
| 2026-10-02, `a462e92`~`868bb4c` | 생성 선택·Link 비공개 의존·선택 검사·독립 클라이언트·출력 전환이 같은 날의 여러 기능 커밋으로 완성됐다. | CLI·선택 모델을 처음부터 다시 설계하는 일반적인 SDK 프로젝트의 기간을 그대로 적용하지 않는다. |
| 2026-10-05, `0f6f715`, `ed43501` | 보안 참조 처리와 필요한 능력별 런타임 생성이 추가됐다. | 공통 compiler·fixture·출력 인프라를 재사용하고 Go 전송·런타임에 비용을 배정한다. |
| 2026-10-07, `db51f1f`~`9b6c2dc` | 별칭 소유·수신 스키마 회귀 검사·import 수정·중복 렌더링 개선이 이어졌다. | 기능 방출 직후를 완료로 보지 않고 소비자·성능·회귀 검증 시간을 따로 잡는다. |

`git log`에서 10월 2일에는 90개, 10월 5일에는 20개, 10월 7일에는 20개 커밋이 있었다.
커밋 분할 방식이 달라 이 수를 기능 처리량으로 사용하지 않았다. 작업을 빠르게 이어가는
현재 방식은 관찰되지만, TypeScript 코드를 Go로 그대로 옮길 수 있다는 근거는 아니다.
새로운 wire 값·JSON·Go 본문 소유권·압축·서버와 전체 검증을 포함해 계획 12.1절의
10~17 집중 작업일로 추정했다. 이는 Git에서 측정한 소요 시간이 아니라 이력과 작업
분해에 근거한 추정이다. W02·W06 실제 완료를 기준으로 재산정한다.

## 6. 전체 리뷰에서 보완한 계약과 근거

### 혼합 열거형·값 소유권

[현재 enum 방출기](../../internal/target/typescript/enums.go)와
[스키마 실행 검사](../../internal/target/typescript/schema_program_runtime_test.go)는
null·문자열·숫자·객체가 섞인 enum을 다룬다. Go 상수는 객체·배열·null을 표현하지
못하므로 스칼라 상수만으로 같은 작업을 제공할 수 없다. 계획 4.5절의 불변 wrapper·
생성 함수·구조적 동등성으로 결정했다. [API 시제품](probes/api_contracts_test.go)은
혼합 값과 객체 키 순서·배열 순서·사용자가 수정한 projection의 독립성을 확인한다.
시제품의 표준 JSON 사용은 이 유한 fixture의 검증용이며 제품 WTF-8 decoder를 대신하지 않는다.

### Link·사용자 hook

[LinkInvocation](../../internal/target/typescript/runtime/client/links-types.ts)은
sourceInput·input·options를 구분한다. [Link 평가기](../../internal/target/typescript/runtime/client/links.ts)는
`$request` 식을 sourceInput에서 읽고 HTTP 응답을 가진 APIError도 받는다.
[방출기](../../internal/target/typescript/links.go)는 같은 이름의 기본 dispatch와 상태별
호출을 모두 생성한다. 계획 7.1절에서 이 계약과 서로 다른 상태별 대상 타입을 확정했다.
시제품은 typed invocation·원본 입력·덮어쓰기·HTTP 오류·명시 상태 dispatch를 확인한다.
실제 runtime expression 전체·tagged group·생성 API 검증은 W07에서 수행한다.

[전체 미디어 코덱](../../internal/target/typescript/runtime/media/media-codec-types.ts)의
decode·decodeInbound는 Response·Request를 받는다. 본문만 넘기면 헤더 기반 복호화 같은
기존 사용자 작업을 잃는다. 계획 6절은 5개 함수의 정확한 인자·반환형·소유권을 고정했다.
[수신 인증 context](../../internal/target/typescript/runtime/server/runtime-types.ts)의
요청·매개변수·보안 후보·거부 응답도 계획 9절에서 보존한다. 시제품에서 HTTP 메타데이터·
취소 context·단일 본문 종료·명시적 body clone 이후 원본 보존을 확인했다.

### 압축 응답

기본 Go gzip 해제는 Content-Encoding·Content-Length를 지우지만 현재 Node Fetch는
본문을 해제해도 원래 헤더를 유지했다. 같은 서버를 양쪽에서 호출해 이 차이를 재현했다.
Node v24.21.0에 내장된 Undici 7.29.1의 실제 코드를 읽어 기본 Accept-Encoding과
gzip·x-gzip·deflate·br·zstd·체인·알 수 없는 coding 처리를 조사했다.
[고정 Node 소스](https://github.com/nodejs/node/blob/v24.21.0/deps/undici/src/lib/web/fetch/index.js)와
[표준 Transport 계약](https://pkg.go.dev/net/http#Transport)이 근거다.

[압축 시제품](probes/compression_test.go)은 같은 실제 HTTP 응답에 대해 Node Fetch와
Go 해제 경로의 본문·원래 헤더를 비교한다. gzip·x-gzip·zlib deflate·raw deflate·br·zstd·
gzip/br 체인·알 수 없는 coding의 8개 fixture, bodyless 응답과 원본 본문 종료를 포함한다.
모든 malformed 압축·네트워크 종료·리다이렉트 조합을 검증한 것은 아니다. 실제 SDK에서
Raw·Link·선언 헤더·사용자 코덱·프레임 제한까지 연결하는 검증은 W03~W06의 완료 조건이다.

Go 표준 공개 라이브러리만으로 br·zstd를 처리할 수 없어 아래 두 모듈로 고정했다.
자체 압축 포맷 구현이나 외부 실행 파일은 도입하지 않는다.

| 모듈 | 선택 버전·최소 Go | 채택 근거·비용 |
| --- | --- | --- |
| `github.com/andybalholm/brotli` | v1.2.6, Go 1.22 | MIT, reader API, CGO 없는 br 해제. 기본 HTTP 어댑터에 추가 의존성·바이너리 비용이 생긴다. |
| `github.com/klauspost/compress/zstd` | v1.20.3, Go 1.25 | BSD-3-Clause 및 포함 코드 고지, reader API, concurrency 1이면 비동기 decode를 사용하지 않는다. 모듈 전체를 SDK에서 import하지 않는다. |

고정 버전의 go.mod·LICENSE와 실제 import 집합을 확인했다.
[Brotli 모듈 계약](https://github.com/andybalholm/brotli/blob/v1.2.6/go.mod),
[Brotli 라이선스](https://github.com/andybalholm/brotli/blob/v1.2.6/LICENSE),
[zstd 모듈 계약](https://github.com/klauspost/compress/blob/v1.20.3/go.mod),
[zstd 라이선스](https://github.com/klauspost/compress/blob/v1.20.3/LICENSE),
[decoder 동시성 계약](https://github.com/klauspost/compress/blob/v1.20.3/zstd/decoder_options.go)을
기준으로 설치 안내와 라이선스 고지를 생성한다. Brotli 모듈의 randomstring 요구는 테스트용이며
실제 runtime import와 구분한다. 시제품의 `go list -deps` 결과로 두 decoder와 정규식의
import graph·CGO 여부를 함께 기록한다. 제품에는 아직 의존성을 추가하지 않았다.
Go의 표준 net 패키지는 기본 환경에서 선택적으로 CGO DNS resolver를 사용하므로,
CGO 불필요 여부는 `CGO_ENABLED=0`으로 목록과 전체 실행을 따로 확인한다. race 실행에
필요한 CGO와 생성 SDK의 필수 의존성을 구분한다.

2026-10-17 Go 취약점 module index에는 Brotli 항목이 없었고 compress에는
[GO-2026-5841](https://vuln.go.dev/ID/GO-2026-5841.json)이 있었다. 해당 OSV 레코드는
`s2.NewDict`의 1.16.0 이상~1.18.7 미만을 대상으로 한다. 선택한 1.20.3은 수정 버전
이후이며 사용 경로도 zstd다. 이 특정 기록의 비해당 판단이며 전체 안전성 보장은 아니다.
W11에서 고정된 실제 SDK 의존 집합에 프로젝트의 최신 취약점 검사를 적용한다.

## 6. URL format 경계의 추가 검증

2026-10-18 W02 구현 중 `uri`, `uri-reference`, `iri`, `iri-reference`,
`idn-hostname`, `ipv6`의 기존 검증이 JavaScript `URL`에 의존함을 확인했다.
`net/url`만으로 대체하면 WHATWG URL의 host 검사·IDNA·상대 참조 동작을 보존할 수 없다.
이 경계는 최초 계획에서 구체적인 Go 구현을 정하지 못했던 부분이므로 별도로 검증했다.

[whatwg-url](https://github.com/nlnwa/whatwg-url)은 기본 parser를 WHATWG 구현으로
제공하며 Apache-2.0으로 배포한다. README의 적합성 기준은 2023-05-24이므로 현재
Node 전체 적합성을 주장하는 근거로 사용하지 않았다.
[고정 버전의 모듈 정의](https://github.com/nlnwa/whatwg-url/blob/v0.6.2/go.mod)와
Go 모듈 레지스트리에서 `v0.6.2`, Go 1.16 이상을 확인했다. 전이 의존성은 현재
프로젝트의 Unicode 17 기준과 맞춰 `x/net v0.61.0`, `x/text v0.43.0`을 고정한다.
`bitset v1.20.0`도 소비자 격리 모듈에서 고정된다.

[격리 모듈](format-probe/url_test.go)에서 28개 입력의 parse 성공 여부·hostname·직렬화를
현재 프로젝트 Node와 대조해 일치했다. 상대 경로, 잘못된 percent escape, IPv4 변형,
IPv6 오류, port 범위, 역슬래시, IDN·emoji·Unicode 17 도메인, file·opaque URL을 포함한다.
`go test -race`도 통과했다. 이는 이 표본의 관찰이며 WHATWG 전체 적합성의 증명은 아니다.
실제 `whatwg-url/url`의 외부 패키지 의존 12개를 조회했고 모두 CGO 파일이 없었다.

선택한 네 모듈과 고정 버전을 OSV API로 조회했으며 일치하는 advisory는 없었다.
이 조회는 전체 보안 감사가 아니며 W11의 실제 소비자 의존 집합 검사를 대체하지 않는다.
생성기는 이 라이브러리를 호출하지 않는다. W02 시점에는 해당 format을 검사하는 생성
SDK만 조건부 import했다. W04의 D26에서 HTTP URL 정규화에도 재사용하기로 했으므로
현재 HTTP SDK에도 필요하다. 사용자의 `go.mod`·`go.sum`을 변경하지 않는 D04는 그대로 유지한다.

후속 구현에서는 format마다 필요한 helper만 배선하고 Node 비교와 독립 기대값을 함께
사용한다. 예를 들어 날짜의 `0001-01-01`은
[공식 JSON Schema 사례](https://github.com/json-schema-org/JSON-Schema-Test-Suite/blob/main/tests/draft2020-12/optional/format/date.json)에서
유효하다. TypeScript의 `Date.UTC`가 0–99년을 1900년대로 바꾸는 현상을 Go에 그대로
복제해서는 안 된다. format 구현·제품 검증 자체는 아직 W02 완료 증거가 아니다.

## 7. W04의 HTTP URL·인증 쿼리 경계

`net/url`의 표기만 비교하면 Unicode host와 punycode, 기본 포트, IPv4의 기존 표기를
같은 origin으로 정규화하지 못한다. 기존 D25 파서와 버전을 HTTP 서버 URL·origin에도
재사용한다(D26). 생성 SDK에서 IDNA·기본 포트·IPv4·역슬래시·dot segment,
상대 서버의 Origin 해석을 독립 기대값으로 검증한다. format의 별도 초기화 데이터 제거는
기존 실제 바이너리 검사를 유지하며 HTTP 파서의 필수 비용과 구분한다.

같은 라이브러리의 SearchParams API는 이 결정을 적용할 대상이 아니다. 고정 소스의
`SearchParams.init`은 percent 해제 뒤 `+`를 공백으로 바꾸고, `QueryEscape`는 URL의
query 집합을 사용한다. 실제 인증 소비자에서 `a+b&c`가 그대로 구분자를 포함해 전송되는
실패를 확인했다. 단순 URL 파싱의 기존 28개 대조 결과로 이 API까지 적합하다고 판단하지 않는다.

인증 쿼리는 순서를 유지한 pair를 각각 해제한 뒤 URLSearchParams의 form 집합으로
직렬화한다. 기존 pair·중복·빈 값과 `+`, `&`, `~`, `*`, 세미콜론을 별도로 검사한다.
URL 파서는 표준 HTTP 타입으로 변환할 때만 사용하며 SearchParams API는 호출하지 않는다.
토큰·URL·본문은 오류의 기본 출력이나 JSON 근거에 저장하지 않는다.

추가로 보안 대안 ID를 scheme 이름으로 합칠 때 `key`의 두 번째 대안과 실제 `key__2`,
익명 대안과 실제 `anonymous` scheme이 충돌할 수 있었다. 전체 점유 ID를 확인하는
공통 HTTP 계획 함수로 양쪽 타깃을 연결했다. 기존의 유일한 ID는 유지한다.
작업별 선택 타입과 전용 메서드가 Go 공개 경계의 구체 표현이며 공통 옵션 경계에서도
실행 전에 유효한 선택인지 검사한다. 호스트 인증 제공자와 토큰 발급·재시도의 책임은 바꾸지 않는다.

## 8. W05 미디어 표현과 소유권 확정

D27은 W05 구현 중 확인한 공개 타입·metadata 소유권·생성 책임의 구체 표현이다.
`MultipartValue`와 공개 `MultipartData`는 strict JSON 값과 source 슬롯을 분리한다.
실제 생성 소비자에서 nullable·tuple·합성·자유 형식·추가 속성의 source 보존, 겹친 경로와
표현 불가능한 source의 오류를 확인했다. 입력 multipart의 바이너리 투영을 유한 응답이나
JSON component에 전파하면 잘못된 공개 타입·미사용 import를 만들므로 occurrence 방향으로 구분한다.

기존 `internal/target/typescript/runtime/media/xml/xml-codec.ts`는 임의 추가 속성과
prefixItems 기반 XML 매핑을 사용하지 않는다. 이를 새 Go 기능 요구로 확대하지 않는다.
유한 폼·multipart 응답도 기존 전체 값 코덱의 Decode 계약을 유지한다. 반면 multipart의
encoding 위치 선택은 HTTP 규칙이므로 prefixItems와 별도로 유지한다.

multipart 헤더 프레임은 한 번 준비하고 현재 파트만 여는 pull reader를 선택했다.
64 MiB 합성 source의 선읽기 없음과 계산 길이, 조기 HTTP 응답·Open/Read 실패·취소·
Close의 정리를 생성 모듈 race 검사로 확인했다. 이 검사는 최대 RSS 측정 결과는 아니다.
소비 중 EOF에만 trailer가 확정되는 실제 HTTP와 gzip 응답도 코덱 snapshot에서 검증했다.
작업 전체의 최종 검증과 W06/W09의 실제 stream·handler 통합은 별도 완료 조건이다.

## 9. W06 순차 미디어의 검증 경계

기존 `runtime/http/http-stream-core.ts`는 증분 호출에 itemSchema/itemEncoding을,
`runtime/http/response/http-response-services.ts`와 `runtime/media/http-multipart-response.ts`는
전체 호출에 schema.items/prefixItems와 prefixEncoding/itemEncoding을 적용한다.
서로 다른 두 스키마를 선언한 실제 Go HTTP 소비자에서 전체 호출은 성공하고 증분 검증은
실패함을 확인했다. 전체 수집에 증분 DTO 디코더를 재사용하는 초기 구현을 수정했다.

기존 TypeScript 실행 소비자 `TestMultipartBinaryOutputSchemaCompatibility`로 바이너리
파트가 ArrayBuffer이며 true·빈 object 스키마를 통과하고 string·binary encoding string·
array 스키마에서 RESPONSE_DECODE_FAILED가 됨을 고정했다. Go는 같은 검증용 빈 객체와
별도 버퍼 슬롯을 사용한다. 일반 JSON DTO의 타입을 바이너리 타입으로 바꾸지 않는다.
generated StreamItem/Output의 미디어 조회로 소실 없이 제공하고 바이트는 복사한다.
prefix 파트 헤더·default media·XML과 중첩 바이너리도 별도 생성 race 소비자로 확인했다.

요청 consumer는 실제 HTTP/1·HTTP/2에서 첫 응답 항목을 받은 후 다음 업로드 항목을
허용하는 barrier로 full duplex를 검증한다. 64 MiB 합성 파일의 첫 헤더 read까지 파일
Open/Read가 모두 0이고 최종 Close가 한 번임을 검사한다. 전체 RSS 측정으로 해석하지 않는다.
complete 파트의 custom codec을 늦추면 조기 204가 nil/typed-nil/잘못된 metadata를
우회하는 회귀가 생겼으므로 metadata 사전 검증과 본문 지연 읽기를 분리했다.

파트 selector는 스키마와 encoding을 빌드 시 캐시하고 직접 함수로 출력한다. 헤더 lowering은
요청과 응답이 공유하고 XML은 기존 mapper가 담당한다. 재귀 배열도 캐시된 selector를
호출하며 런타임 OpenAPI 해석기나 전역 operation registry를 도입하지 않는다.
이는 D28의 구체 표현이다. W06의 최종 공유 회귀와 별도 리뷰, W11의 전체 기능·성능 검증은
이 조사 사례와 별도로 완료해야 한다.

재귀 Node(anyOf 배열/객체)의 중첩 바이너리를 양쪽 실제 소비자로 실행해 통과했다.
native response framing은 해당 sequence 자체의 items/prefixItems를 읽는다. 요청용
복합 필드 투영을 응답의 다음 중첩 단계마다 펼치면 재귀 anyOf가 무한히 늘어난다.
응답 selector는 기존 native codec의 물리적 facet 경계를 유지하고, 전체 schema의
ref·조합·재귀 assertion은 별도 검증 프로그램에서 검사한다. 이 경계를 요청 part query에
전파하지 않는다. 중첩 응답은 이미 상한 안에 잡힌 바깥 파트 payload가 한계이며,
새로 더 작은 frame·header 기본값을 적용하지 않는다.

## 10. W07 호출 표현과 응답 profile

[D29 검증 기록](w07-decisions.md)에 실제 Go/TypeScript 소비자 비교와 구체 API의
결정 근거를 보존한다. 계획 7.2절이 구현 계약이다. 주요 관찰은 HTTP 응답의 미선언
속성 보존, 복수 응답 페이지 항목의 제약 차이, native Link body의 지연 source 전달,
컴포넌트 Link·응답 참조, browser 헤더 위임과 Go 명시 입력의 차이다.
새 SourceInput은 원본 제약을 다시 강제하지 않으며 대상 실행 경계가 검증을 소유한다.
작업별 최종 전체 회귀·리뷰와 W11·W12의 전체 대응 검증은 별도 완료 조건이다.
