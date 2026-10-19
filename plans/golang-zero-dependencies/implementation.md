# Go 무의존성 구현 기록

현재 실행: [계획](plan.md), [devtools ID](workstream.json).
계획 단위별로 리뷰·재검증·커밋한다. W01–W09를 구현·검증했고 성능·플랫폼·최종 통합 검증을 이어간다.

## W01 — 기준과 적합성 입력

- 제품 기준 `cab3cb8`(계획만 추가, 제품은 `72e679c`와 동일).
- 고정 Test262에서 정규식 후보 2,119개(리터럴 구문 240개 포함)의 원본과 harness를 확보했다.
  fixture manifest는 path·SHA256·features·flags를 보존한다. adapter 상태는 pending이며
  통과 결과나 제외 판정으로 취급하지 않는다. 원본과 라이선스로 이후 판정을 감사할 수 있다.
- WPT URL·IDNA·percent encoding 원본 JSON을 보존했다. setter/DOM API를 구현 범위로 확대하지 않는다.
- 변경 전 `golang:verify test -run TestGenerated(RegexNodeOracleAndExecutionErrors|Format|HTTPTransportIntegration|Compression) -count=1` 통과.
- `golang:perf runtime`의 생성·컴파일·강제 GC 수명·benchmark 5회·동시성 profile·race 통과.
  원시 자료 `.tmp/golang-zero-baseline-runtime`에 보존했다. 비교 후보는 아직 없다.
- 소비자 4가지 방식의 cold/warm 각 5회와 실행·의존·심볼 검사가 모두 통과했다(117회 invocation).
- 1천/1만 작업 × 전체/1개/10개 선택 생성 각 5회와 소비자 컴파일이 통과했다(33회 invocation).
- 원시 결과·basis·stdout/stderr·GNU time metrics와 해시는 `baseline/{runtime,builds,generation}`에
  모두 보존했다. 생성 바이너리·profile·작업 디렉터리는 대응 `.tmp/golang-zero-baseline-*`에 남겼다.
- HTTP 변경 계약: 압축·Accept-Encoding·자동 해제 정책은 HTTP 계층 소유, 필수 헤더 검증은
  실제 반환된 헤더 기준, Raw는 실제 반환 바이트 기준, 본문 수명·취소는 SDK가 계속 관리한다.

리뷰: 입력 pin·다운로드 경계·licensing·manifest 상태·기존 회귀 확인 완료.
baseline 결과와 SHA, 적용 범위·독립 기대값·재현 경로를 재확인했다.
새 엔진용 upstream adapter는 W04–W07에서 구현·실행할 대상이며 현재 상태는 pending이다.

W01 최종 확인: 본문 수명 포함 명시적 회귀 5개 집합 재실행 통과. 모든 baseline 실행(9+117+33)의
exit code와 raw 출력 해시를 검사했다. 생성 artifact hash map은 중복 제거한 `artifact-trees.json.gz`에
보관하고 각 실행에서 참조한다. 원래 결과의 모든 map을 복원·대조해 일치함을 확인했다.
W01 범위의 남은 결함은 없다. 제품 엔진 적합성은 이후 단위의 pending 검사로 유지한다.

## W03 — 자체 Unicode17 자료와 기본 함수

단일 Go 원본 `internal/goruntime/unicode17`에 property·simple case folding·NFC와 IDNA mapping/
joining/bidi 조회를 구현했다. 표준 라이브러리만 import하며 외부 엔진 코드를 복사하지 않았다.
공식 입력 15개를 SHA256으로 확인해 결정적으로 Go 표를 생성하고 라이선스·고지·원문 fixture를 보존했다.
441개 property, 867개 alias, 23,045개 range를 제공한다. runtime은 map 초기화 대신 고정 배열을 사용한다.

공식 NormalizationTest의 NFC 전체 열, simple folding, IDNA mapping/Joiner의 범위 끝,
Unicode17 새 script·alias 대소문자·script extensions·서로게이트·속성 경계 검사 통과.
Node Unicode17과 전체 alias/범위·빈 구간 137,353개를 대조해 차이0개였다.
보조 oracle이 Hrkt/Katakana_Or_Hiragana를 실제 Script 값으로 받아들이는 오류를 발견했다.
Script/Script_Extensions에 실제 배정된 값만 alias로 채택하도록 고치고 재검사했다.
필수 공식 fixture 검사는 Node 없이도 실행하며 oracle만 선택적으로 skip한다.

일반·race 검사 통과, NFC 멱등성 fuzz 311,848회에 실패 없음(마지막 alias-only 보완 이전).
마지막 데이터 재생성의 byte 비교도 4개 파일 모두 일치했다. 오류·불변 view·기본값·출력 결정성·데이터
도달성 구조를 리뷰했고 고친 alias 영역과 전체 공식 검증을 재검토했다. 세부 해시는 `w03-evidence.json`.
정규식·URL 엔진 자체의 적합성이나 URL의 UTS46 정책을 완료했다고 주장하지 않는다.

## W02 — 압축을 사용자 HTTP 계층에 위임

SDK의 압축 decoder·기본 Accept-Encoding 삽입·DisableCompression 변경과 이미 해제된 응답의
거부를 제거했다. `ResponseBodyMode`/capability 선언도 제거했고 모든 생성 소비자 호출부를 갱신했다.
일반/순차 본문 lease는 지연 읽기·닫기1회·취소·오류·연결 재사용을 유지한다.
고정 br/zstd/gzip/deflate 바이트 fixture로 binary Raw와 custom codec의 전달을 확인하며 정상 테스트
소비자에서 외부 압축 라이브러리를 설치하지 않는다. gzip 자동 해제는 net/http가 수행한다.

리뷰 중 일반 JSON Raw도 기존 미디어 decode·schema 검증을 수행함을 확인했다. 이를 생략하는 API로
확대하지 않고 계획의 Raw 설명을 명확하게 했다. Binary/순차 Raw는 반환 바이트, JSON Raw는 기존
검증 계약을 유지한다. 필수 헤더가 HTTP 계층에서 사라지면 실제 헤더를 기준으로 오류다.

집중 검사에서 HTTP 설정·명시 encoding/Range·연결 풀·자동 gzip·원래 헤더 유지 사용자 transport·
opaque binary Raw·custom codec·필수 헤더 실패·close/cancel/race 통과.
전체 Go 타깃 회귀를 두 번 실행했고 이전 SDK 해제를 전제로 하던 순차 Raw·XML·stream 헤더 기대값을
찾아 새 계약에 맞췄다. 두 번째 전체 검사에서 stream metadata assertion 1개만 실패했으며,
제품 코드는 그대로 두고 해당 기대값을 수정했다. 최종 코드의 HTTP/압축 lease/순차 Raw/XML/실제 stream
영향 집합 6개를 재실행해 모두 통과했다. 나머지 전체 검사 결과는 동일 제품 코드에서의 결과를 재사용한다.
전체 단일 green run은 W11에서 다시 확인한다. 실패 기록을 통과 기록으로 바꾸지 않는다.

공개 lease API를 소비하는 성능 도구도 함께 수정했다. decoder 측정을 SDK lease 측정으로 오인하지 않도록
이름·범위를 바꿨고 생성·컴파일·수명 진단·benchmark 1회 호출이 통과했다
(`.tmp/golang-zero-w02-perf-check`). 이는 성능 비교용 표본이 아니다.
사용자 HTTP 계층/SDK 의미 경계·수명·보안 capability·기존 호출부·측정 도구 리뷰와 영향 재검사를 마쳤다.
실제 모듈·문서의 예전 설치 안내 정리는 W08/W09에서 수행한다. 정규식·URL 외부 의존성은 아직 남아 있다.

## W04 — 자체 정규식 parser/compiler

`internal/goruntime/ecmaregexp`에 Unicode `u` grammar, scoped i/m/s, 캡처·전후방 assertion·참조·
반복·Unicode class 컴파일을 구현했다. 명시적 group stack과 평평한 versioned instruction/edge/range
자료를 쓰며 100,000중첩도 Go stack 재귀 없이 컴파일한다. 문법·복잡도·context 오류를 구분한다.
같은 class source/flags의 자료를 공유하고 class 내부 중복 범위를 누적하지 않는다.
일반 문자열·WTF8 서로게이트의 캐시 키가 섞이지 않도록 code point를 고정 폭으로 직렬화한다.

고정 Test262의 정적 `u` pattern 3,930개(정상3,606/오류324)의 Node 컴파일 결과와 일치한다.
710개 원본 파일에서 추출했으며 전체 2,119개 파일의 동적/API scope 상태는 별도 manifest에 남겼다.
upstream JS를 실행한 적합성 통과로 표현하지 않는다. 추출은 기존 TypeScript 개발 API로 구문만 읽고
원본 SHA를 검사한다. 패턴 UTF16 units를 보존하며 정상 Go 테스트는 TS 설치·네트워크를 요구하지 않는다.

리뷰에서 Unicode property의 빈 이름 `\p{=Letter}` 허용과 `(?i-:...)` modifier 허용 차이를 고쳤다.
비캡처 괄호 뒤 중첩 quantifier, 중복 이름의 disjoint alternative 판정, 거대한 반복 범위,
\W의 Unicode ignore-case 집합 순서, modifier 상속 제거, 긴 escape의 취소 확인도 검토했다.
독립 구문 기대값·Node 비교·전체 정적 fixture·race·fuzz 통과. 마지막 fuzz는78,037회 무실패.
반복 property 자료의 저장 중복을 로컬 진단해 684,000 range를684개로 공유했다. 원시 1회×3표본은
`w04-evidence.json`에 남겼으며 기준 source hash 미수집 등 한계를 명시했다. 전체 성능 판단은 W10이다.
범위·실패·불변 자료·Unicode·생성 가능한 구조를 재검토했다. 매칭 실행과 SDK 연결은 W05/W07의 pending 작업이다.

## W05 — bounded 자체 정규식 매처

Program을 불변으로 공유하며 캡처·continuation·choice·undo journal은 호출별로 실행한다.
UTF8/WTF8 byte cursor로 정방향/역방향을 읽고 lookbehind의 역순 concat, atomic lookaround,
미참여 참조, 반복 캡처 초기화, greedy/lazy, 필수 빈 반복과 후속 backtrack을 구현했다.
입력 전체를 rune 배열로 복사하지 않으며 단일 문자 반복은 compact retry를 쓴다.
검색 전체에 1억 작업 budget과 1,024작업마다 context 확인을 적용한다.

기존42개, 고정 Test262 assertion에서 유도한95개 boolean 계약, 55×29 경계 조합,
12×12×6 pattern ×31 input의 보조 Node 비교가 통과했다. 마지막 집합은 잘못된 문법의
거부도 비교한다. 100,000 괄호/20,000 assertion 중첩, 1MiB 반복, 동시 재사용, 취소·budget 후
재사용, scratch 참조 해제와 race 통과. 30초 fuzz 2,262,657회 무실패.
상위 Test262 JS 전체나 capture-array API 통과로 주장하지 않는다. 동적 원본/API 분류 잔여는 W07에서 보완한다.

보조 oracle Node24.22에서 scoped Kelvin/long-s literal folding 차이를 발견했다.
ECMA-2622026 Canonicalize/UpdateModifiers와 Unicode17 C/S folding, 기존 Go 엔진은 자체 결과와 일치한다.
Kelvin matrix 기대값 두 개만 규격값으로 명시적으로 보정하고 독립 계약으로 보존했다.
관련 Node issue66277과 같은 원인/수정이라고 단정하지 않는다. 재현·규격 링크는 w05-evidence.json.

측정 중 기본 VM buffer4개가 escape해 짧은 ASCII 호출도 13,824B/4alloc을 쓰는 원인을 확인했다.
최대32개 고정 scratch만 캐시하고 독점 대여, 반환 시 입력/Program/context와 확장 배열 참조를 지운다.
동일 입력 전후5표본에서 warm ASCII는 0B/0alloc이었다. 원시 출력·정확한 전후 원본 해시/이전
원본 archive를 보존했다. 짧은 단일 스레드 microbenchmark이며 SDK 전체 성능 개선으로 확대하지 않는다.
의미·복원·캡처/역방향·수명·경쟁·오류 전파·자료 소유권을 리뷰하고 추가 회귀와 race를 재실행했다.
제품 QuickJS 교체는 W07, 의존 모듈 제거는 W08의 pending 작업이다.

## W06 — 자체 URL·IDNA runtime

`internal/goruntime/urlstd`에 URL constructor 파싱·상대 해석·직렬화·hostname/origin을 구현했다.
HTTP/nonHTTP/file/opaque, credentials/default port, IPv4 legacy 표기, IPv6, dot segment,
query/fragment의 빈 값 구분, USVString 경계를 포함한다. IPv6 값 검증에 표준 net/netip을 쓰고
URL 표현으로 직접 직렬화한다. net/url 재해석은 W07 HTTP adapter의 마지막 변환만 허용한다.

Unicode17 비전환 mapping/NFC/Punycode와 ContextJ/Bidi를 자체 구현했다. ASCII domain 호환 경로,
CheckHyphens/STD3/DNS-length 해제는 고정 WHATWG 정책이다. URL delimiter 거부는 host 단계다.
Mark 판정은 IDNA 전용 작은 표로 분리해 전체 regex property 자료를 요구하지 않는다.
공식 해시 확인 재생성 결과4개 파일이 byte 단위로 일치하고 전체 코드포인트의 Mark 판정도 일치한다.

고정 WPT constructor 파싱·직렬화/명시된 origin 전 사례, IDNA 및 이전 removed 사례,
UTF8 percent encoding, 기존 Node URL28개 통과. 공식 Unicode17 IDNA6,391개를 relaxed URL
정책으로 독립 검사했다. WPT7개가 U+3F8CD/U+3E8AC를 허용하지만 고정 Unicode17 mapping은
금지한다. 원본은 그대로 두고 source/input 해시·기존 결과·규격상 거부를 override manifest에 명시했다.
이 변경은 자체 결과를 정답으로 삼은 제외가 아니다. 구체적 이전 Unicode 버전 원인은 단정하지 않는다.

리뷰·회귀에서 opaque 마지막 space, empty port의 빈 host, 무시 문자로 빈 host가 되는 경우,
path caret encoding, overflow hex IPv4-like suffix를 고쳤다. 불변 base를32개 동시 호출로 확인했고
race 통과. URL왕복 fuzz593,911회, Punycode왕복 fuzz201,674회 무실패. 원시 실패·최종·fuzz·해시를
w06-evidence.json에 보존했다. 최종 URL/Unicode/정규식 원본 패키지 회귀가 통과했다.
setter/DOM/SearchParams/HTML의 비UTF8 encoding 인자는 SDK 범위 밖이며 통과로 주장하지 않는다.
인증 origin/중복해석·소유권·정수 경계·잘못된 입력·결정성·버전·링크 도달성 구조를 리뷰했다.
전체 성능·생성 SDK 소비자 연결은 W07/W10에서 이어서 검사한다.

## W07 — 단일 원본·정적 프로그램 방출

명시적 source/DAG 목록으로 자체 runtime만 방출한다. 원본 embed 목록도 명시적이고 test/tool은
포함하지 않는다. Go AST import literal만 SDK import root로 변경하며 string/pattern 값과
주석·라이선스를 보존한다. Unicode notice를 SDK에 함께 출력한다. 정적 matcher에 필요한 range
helper/type은 parser/property 구현에서 분리해 선택 생성 시 compiler 표를 요구하지 않는다.

정적 pattern은 생성 시 자체 Compile로 검증·컴파일하고 versioned nodes/edges/ranges/named captures를
Go 값으로 방출한다. 요청 경로에서 다시 컴파일하지 않는다. patternProperties와 XML/server part
projection이 참조하는 원문 메타데이터는 planner에서 유지한다. 동적 server schema와 regex format만
compiler/property 자료를 요구한다. HTTP·형식 URL은 자체 urlstd를 사용하고 HTTP 표현으로만 마지막에
net/url로 변환한다. 기존 internal NewPattern 테스트 helper는 정적 생성 검증으로 대체했다.

기존 regex42·format·CGO0 및 HTTP 단계/원인 검사가 통과했다. 고정 독립 assertion95개를 생성 정적·동적
경로에서 확인하고 구문3,930개를 생성 compiler에서도 확인했다. plain/static/dynamic 파일 선택,
중첩 import root·결정성·외부 import 부재를 검사했다. 리뷰에서 동적 wrapper의 wire 직접 import 누락을
고쳐 서버/inbound·보안·HTTP 영향 그룹과 집중 생성 검사를 재실행했고 전부 통과했다.
원본 패키지도 추가 fixture와 함께 race 통과했다.

Test262 동적 입력 보조 adapter가1,919개 Boolean trace(1,797 pattern,477개 파일)를 만들었고 전부
자체 matcher와 일치했다.2,119개 전체 파일 outcome을 별도 manifest에 보존했다.22개 JS 실행은
realm/descriptor/coercion API 및 비u lexical enumeration으로 불완전하며 통과로 집계하지 않는다.
큰 입력1,863호출은 준비 크기 제한 때문에 trace하지 않았다. 이는 SDK 제한이 아니며 전체 Unicode
경계 자료와1MiB 반복 검사는 별도로 통과했다. 전체 Test262 JS 적합성 통과로 주장하지 않는다.

자료 소유권·AST 경계·상수 직렬화·캡처/오류 전파·DAG/도달성·불변/동시 실행·원본/출력 일치를 리뷰했다.
생성 소비자 helper의 예전 모듈 pin은 W08에서 제거한다. 실제 무외부 모듈 격리 검증과 binary symbol
검증은 W08/W10에 남아 있으며 현재 결과에 포함하지 않는다.

## W08 — 외부 SDK runtime 모듈 제거

공용 multipart part schema projection에 남은 QuickJS 호출도 자체 Compile/Match로 교체했다.
Go module tidy로 생성기의 마지막 QuickJS 의존을 제거했다. 생성 소비자 helper와 link/reachability
검사, 예제에서 외부 module require/pin 및 example go.sum을 없앴다. 생성 검증은 readonly·offline·
CGO0로 실행하고 실제 module/package 그래프를 확인한다. 예제·배포 smoke·성능 준비 도구의
이전 go.sum 복사도 제거했다. QuickJS 고지를 공식 Unicode 전체 라이선스로 대체하고 npm과 맞췄다.
과거 probe·baseline 자료는 변경하지 않았다.

서버·callback/webhook·전체/독립/정적 선택/계산 조회를 포함한 새 소비자를 빈 module cache,
GOWORK/GOPROXY/GOSUMDB off, GOTOOLCHAIN local, CGO0에서 컴파일했다. 실제 그래프는
main1/external0, standard193/owned25 package이며 go.sum도 만들지 않고 cache도 비어 있었다.
고정 구문/매칭 생성 소비자·독립 client·typed inbound·linker·unused schema 검사 통과.
정적/computed/selected linker3종은 matcher와 compiler를 구분하며 정적 schema에는 compiler가 없었다.
이전 QuickJS compiler symbol 전제는 고쳐서 세 변형을 재검증했다.

공용 multipart package 회귀, Go example 로컬 HTTP 실행, server 포함 check:generate 통과.
Node/TypeScript가 없는 PATH에서도 Go 생성·configured package/import·실패17 전파·임시 정리 검사가
통과했다. import·module graph·test-only 우회·공지/라이선스·도구/예제 호출부를 리뷰했다.
전체 repository Go 회귀는 별도로 실행 중이며 아직 통과로 기록하지 않는다. 문서 정리는 W09에서 이어간다.

## W09 — 문서·Playground·출시 경로 정리

영문·한국어 시작 안내의 외부 모듈 설치 단계를 제거했다. README/npm/Go 예제·선택 비용·
API 지원 범위·스트림·전송 문서를 실제 표준 라이브러리 전용 동작에 맞췄다. URL/Unicode/정규식
기능은 함께 생성되며 사용자 HTTP 압축·실제 헤더·구조화 Raw 검증의 경계를 설명한다.
미출시 문구를 넣지 않았고 과거 anchor와 언어별 동일 code block을 유지했다.

Playground Go 안내에 Go1.27/표준 라이브러리 조건을 표시한다. WASM 결과 ZIP이 Unicode notice도
보존하도록 이전 go-only assertion을 보완했다. WASM으로 생성한 정적 정규식·IRI·regex 형식/server
SDK를 빈 모듈 cache·offline·CGO0에서 컴파일해 로컬 HTTP 응답 검증까지 통과했다.
Go 문서 실행기도 의존성 설치 없이 예제3개를 같은 조건에서 생성·컴파일·로컬 실행했다.

docs validation·전체 build, 언어/링크/다운로드 음성 사례6개, Playground 옵션/context/ZIP9개,
npm source pack와고지 일치, native distribution version/생성/실제 Go 호출, release dispatch/CI reuse
mock 검사 통과. 한국어 문장·제품 경계·이전 설명 잔존·소비 예제·고지/다운로드·출시 호출부를 리뷰했다.

실제 게시된 npm의 cold native-cache smoke는 이 후보를 게시해야 실행할 수 있다. 워크플로가 동일한
갱신 smoke 스크립트를 호출하도록 준비돼 있으며 이를 가짜 launcher 실행으로 대체하거나 통과로
기록하지 않았다. Native 후보 소비는 통과했다. push/게시/원격 dispatch는 수행하지 않았다.
전체 Go 회귀·자원 비교·6개 플랫폼 및 최종 전 영역 리뷰는 W10/W11에서 이어간다.

## W10 — 비용 검증과 측정 기반 보완

전체 Go/TypeScript 회귀의 첫 실행에서 Go 대상 경계 allowlist, header pattern 방출,
이전 compiler symbol 전제가 실패했다. 나머지 패키지와 TypeScript 실행/선언 matrix는 통과했다.
allowlist를 자체 runtime DAG에 맞추고 header coercion이 schema와 같은 준비된 pattern을
참조하도록 고쳤다. compiler symbol 전제 수정은 W08에 포함했다. 영향받은 header/multipart,
정적·계산 조회·선택 linker 검사는 통과했다. 최종 전체 Go 통과는 아직 별도로 필요하다.

초기 측정에서 발견한 비용을 다음과 같이 보완했다.

- NFC는 공식 NFC_QC=Yes와 canonical order가 확정된 문자열을 할당 없이 반환한다.
  Maybe/No와 잘못된 UTF8은 전체 정규화 경로를 유지한다. 반사 기반 안정 정렬을 제거하고
  실제 여러 combining mark가 있는 구간만 정렬한다. IDNA mapping builder의 작은 초기 용량도
  추가했다. 공식 정규화/IDNA 사례·race·정규화 fuzz 973,202회를 통과했다.
- 정규식 scratch의 전역 channel 경합은 CPU profile로 확인했다. 최대32개 고정 buffer arena와
  atomic lease, per-P pool hint로 대체했다. GC가 hint를 버리거나 오래된 hint가 남아도 CAS만
  소유권을 부여한다. 초과 동시 호출은 임시 상태를 쓰며 반환 시 입력·program·context·확장
  buffer를 지운다. 64 worker/강제 GC race와 matcher fuzz 849,291회를 통과했다.
- 비multiline 선두 start assertion이 반드시 실행되는 경우만 검색 시작 위치 반복을 생략한다.
  alternative/lookaround/repeat는 일반 경로를 유지하고 분석 비용도 전체 step budget에 포함한다.
  독립 경계·전체 자체 runtime race·후속 matcher fuzz 255,362회를 통과했다.
- 같은 정적 pattern은 원문 전체 SHA256으로 한 번 컴파일하고 공유한다. 성공한 schema program의
  참조를 모아 pattern별 literal-only 자료 파일로 방출하므로 제외된 operation의 첫 소유자가
  유효 선언을 잃게 하지 않는다. 미사용 자료는 linker가 제거할 수 있다. 독립 두 schema 소비자,
  정적/동적 고정 구문·매칭, XML/header/server 영향 검사와 linker 검사를 통과했다.
- 자체 source는 준비 시 정렬한 불변 문자열을 보존하고 방출 시 새 byte slice를 전달한다.
  같은 소스를 두 번 파싱/정렬하지 않는다. 소비자가 첫 출력 byte를 변경해도 재방출이 변하지
  않는 검사와 결정성·무외부 모듈 격리 소비자가 통과했다.

최종 producer의 features49/generation33/builds117/platform10 명령은 모두 성공했다.
6개 OS/architecture 조합은 CGO0 교차 빌드이며 실행은 native Linux amd64만 했다.
실제 Playground WASM build도 통과했다. 제공자107개와 서버 소비자30개 명령도 전부 성공했다.
7개 제공자×전체/1개/10개 선택의21개 사례에서 입력·출력 경로·생성 표면·진단 계약이 기준과 같다.
각 반복의 생성 해시도 일치했다. 기준 제공자의 생산 코드331개가 cab3cb8과 byte 단위로 일치하고
Go1.27.2/Node24.22.0 조건도 같다. 최종 생성 모드는 생산 코드356개가 같으며 실행 벤치마크의
자체 엔진 Go 소스도 최종 코드와 같다. 이후 보완은 생성 대상의 공유·방출 경로다.

최종 Go CI(형식/vet/tidy/모듈/전체 Go/Node 없는 생성), race, fuzz9종, 문서 build/실행 예제/도구,
Playground WASM/ZIP/격리 Go HTTP 소비자, 배포용6개 교차 빌드가 전부 통과했다. TypeScript 전체
실행/선언 matrix의 앞선 성공도 유지한다. 최종 CI에서 다시 돌린 TypeScript 일반 검사와 별도 구분한다.
모든 검증이 끝난 뒤 동일 소스·명령으로 정규식 동시 실행6종×5회를 다시 측정했다. 최초 추가 실행의
패키지 경로 누락 실패도 보존했고 수정한 별도 실행은 통과했다.

측정상 중대한 초기 비용은 원인·profile·수정 전후를 기록했다. NFC ASCII는 할당을 없앴고, 동시
matcher의 전역 channel 경합을 제거했으며 동일 Unicode pattern1,000개 소스는32.14MiB에서
16.26MiB로 줄었다. 최종 생성은1.49초 중앙값(기준1.46초)이다. 작은 JSON은0.04→0.09초, 단순
정규식은70.5→152.7ns/op로 증가가 남는다. 자체 소스 준비·VM의 비용이며 절대적 개선을 주장하지
않는다. Graph 전체 생성 중앙값은308.95→336.44초, RSS6522.5→6608.8MiB다. 무한·폭발적 증가나
측정 한도 초과 없이 동일한 생성 계약을 지켰다. 다른 조건으로 인한 기여를 개별 원인으로 단정하지 않는다.
소비자 cold 빌드·바이너리는 대체로 감소했다. 병적 정규식의 한도 도달 시간은 엔진 작업 단위가 달라
정상 처리 속도 개선으로 해석하지 않는다.

원시 기록23개 압축 파일·프로파일·실패/의도적 중단·기준/후보·재현 도구와 모든 해시는
`measurements/index.json`과 비교 문서에 보존했다. JSON은 값의 완전한 복원과 원본 해시를 보존하고
공백만 정규화한다. 실행 파일·빌드 cache는 임시 위치에 남기고 hash/크기를 기록했다. 공개 성능 수치나
통계적 유의성을 주장하지 않는다. 정규식/Unicode/소유권·동시성·header 호출부·정적 도달성·측정
조건·모드 누락·플랫폼·제품 통합 영역은 reviewed이며 확인된 결함을 수정하고 재검증했다.
전체 변경의 최종 W11 리뷰와 게시 후 npm 검사는 이 결과에 완료로 섞지 않는다.


## W11 — 전체 변경의 최종 리뷰

`w11-review.md`의 책임·정규식·Unicode/URL·전송/수명·준비/선택/방출·생성 표면/의존·문서/출시·
성능/재현·TypeScript 영향 영역을 전부 검토했다. W10의 수정과 영향 검사 결과를 재확인했다.
정적 추출 manifest에서 동적 검증이 통합 전이라는 오래된 설명을 고치고 보충 trace와 제외 범위를
명확히 했다. 계획의 기준 상태도 최초 조사 이력으로 표시해 실제 실행 기록과 혼동하지 않도록 했다.

Go CI/race/fuzz/docs/Playground/배포 교차 빌드의 성공을 확인한 뒤, 공유 matcher 최적화의 영향을
확인하기 위해 TypeScript 실행·선언 매트릭스를 최종 코드에서 `-count=1`로 다시 실행했고 통과했다
(312.63초). 생산 코드356개의 해시는 W10 측정 코드와 같다. 문서·검증 기록 외 제품 변경은 없다.
진단/산출물 형식은 바꾸지 않았으므로 관련 schema version을 임의로 올리지 않았다. 생성기 재사용은
release/module 버전 또는 깨끗한 VCS revision으로 분리되며 정적 프로그램은 version1을 확인한다.

계획 설명 갱신으로 stale 처리된 이전 단위는 원래 증거와 최종 통합 결과를 재검토해 새 기록을
연결했다. 이전 검사를 다시 실행했다고 표현하거나 waiver로 생략하지 않았다. 원래 단위 커밋과
검증 이력은 그대로 유지한다. 추가 TypeScript 실행·재검토 기록도 원시 압축 파일로 보존했으며
누적24개 기록의 외부·내부 해시와 JSON 복원을 확인한다.

검토 범위의 확인된 결함과 구현 중 결정할 설계 항목은 남지 않았다. 실제 게시된 npm의 빈 네이티브
캐시 검사는 게시 후 release workflow에서 실행할 사항으로 분리한다. 후보의 native 소비와 npm
source pack·워크플로 연결은 통과했다. push·게시·원격 dispatch는 수행하지 않았다.
