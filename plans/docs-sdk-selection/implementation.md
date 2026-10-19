# SDK별 문서 개편 구현 기록

작업은 `plan.md` W01–W10 순서로 수행한다. 단위마다 검사·리뷰 후 커밋하고 마지막에 전체 리뷰한다.
출시·push는 이 구현 작업에서 실행하지 않는다. workstream 식별자는 `workstream.json`에 있다.

## W01 — catalog와 경로 계약

28개 주제, 여섯 locale scope와 기존 52개 경로·430개 heading ID를 catalog/legacy 원본으로 만들었다.
SDK×표시 언어 URL 해석·전환, base 처리, 저장 설정 오류와 누락 주제 fallback을 구현했다.
`validateCatalog`는 중복 topic·경로·legacy와 잘못된 목적지를 거부한다.
실제 SDK 파일 집합 검사는 W02부터 개방된 타깃을 대상으로 사용한다.

검증: `devtools run -- node --test docs/scripts/doc-context.test.mjs`,
`devtools run -- node docs/scripts/validate-doc-context.mjs`.
리뷰: 네 조합의 모든 주제 경로, `.html`·query 우선, 유사 prefix·미존재 경로,
차단·잘못된 저장값, 중첩 locale 순서, legacy 누락과 영향 소비자를 확인했다.
중복 base 적용 가능성을 고치고 재검증했다. 공통 storage 계약은 기존 타입을 재사용했다.
제품 문서·기존 탐색 호출자는 아직 이 catalog를 사용하지 않으므로 기존 동작을 유지한다.

## W02 — TypeScript 정식 경로와 scope

기존 26개 주제를 EN/KO 정식 TypeScript 경로로 이관하고 모델·문제 해결을 보충했다.
공통 관리 출력 규칙은 locale별 partial로 분리했다. 여섯 scope의 탐색과 실제 페이지·내부 링크
검증을 기존 문서 명령에 연결했다. 이관 중 기존 경로 본문은 W08까지 유지한다.

검증: `devtools run validate:docs`, `devtools run build:docs` 통과.
리뷰: 이관 변환과 원본의 대응, 새 두 주제의 계약, 56개 경로·내부 링크,
중첩 scope 순서·표시 언어·검색 설정·공통 partial 제외와 기존 검사 호출자를 확인했다.
새 config 선언에 명시적 타입을 보완했다. Go는 아직 선택 UI에 개방하지 않는다.

## W03 — 두 선택기와 URL 상태

기본 테마의 desktop/mobile slot에 SDK·표시 언어 native select를 연결했다.
경로에서 현재 상태를 읽으므로 새로고침·뒤로가기도 같은 규칙을 사용한다. 전환은 같은 주제를
유지하고 fragment는 버린다. scope 변경은 Layout을 재생성해 열린 검색 상태를 폐기한다.
SDK 설정은 선택/직접 접속 시에만 저장하며 저장 실패는 이동을 막지 않는다.

검증: `build:docs`, context 계약 5건. 정적 preview 브라우저에서 EN→KO 같은 주제 이동,
HTML lang·선택값·제목 포커스·저장값, 뒤로가기, 320px 모바일 메뉴 전환을 확인했다.
320px에서 문서 전체 가로 넘침이 없고 숨긴 desktop select가 레이아웃에서 제외된다.
브라우저 오류는 favicon 404 한 건이며 제품 코드 오류는 없었다.
리뷰: SSR에서 window 접근 방지, base·query·path 우선, 키보드 native 동작과 label/hint,
숨긴 선택기, 검색 remount, 기존 테마 호출자를 확인했다. Go 개방과 전체 4 scope 검증은 W08에서 한다.

## W04 — Go 핵심 사용 흐름

EN/KO 시작·생성·inspect·원격 입력·호출/오류·모델·인증/전송·문제 해결 16개 페이지를 작성했다.
시작하기는 설치→사용자 모듈→의존성→전체 main.go→로컬 mock 호출까지 완결된다.
기존 Go 가이드의 검증된 API 설명을 주제별로 분리하고 Go CLI 설정을 반영했다.

검증: `devtools run -- node docs/scripts/verify-go-docs.mjs`는 실제 문서 코드 블록을
추출해 native generator로 SDK를 만들고 모듈에서 컴파일·실행해 `todo-1 Ship the SDK`를 확인한다.
`devtools run go:example`도 통과했다.
리뷰: 모듈/출력/import 경계, Go 버전과 의존성 소유권, 취소·응답 소유권, 오류와 presence,
CLI 분석과 생성의 구분, EN/KO 코드 대응을 확인했다. Korean inspect에 남은 TS 전용 절을 제거했다.
심화·참조 목적지의 파일 및 전체 링크 검사는 W05/W06에서 내용이 완성된 뒤 수행한다.

## W05 — Go 심화와 API 참조

파일/Link/stream·named/static/generation selection·빌드 비용·스키마 어휘·수신 가이드와
CLI·client·types·streaming·server 참조 24개 페이지를 추가했다. 공개 옵션은 실제 runtime
계약과 대조했다. metadata의 독립 package, Document 복사, source/stream 소유권과 소비 속도,
수신 한도·인증·duplex·헤더 전송 뒤 실패를 설명한다.

검증: `go test ./internal/target/golang -run 'TestGenerated(Pagination|InboundRuntimeBoundary|LiteralsAndUnionSnapshots|BinarySourceOwnership|CustomMediaDirections|LinkCalls)$' -count=1`
통과(실제 생성·컴파일·HTTP 경계). 40개 Go 페이지 EN/KO 코드 블록 일치 확인.
리뷰: template/runtime와 public namespace·옵션·모델 codec·metadata·static/dynamic 선택,
파일/stream close·취소·수신 오류 및 CLI 호출자를 확인했다. 변환 중 CLI target 표가 오염된 것을
수정하고 Go 전용 분석 필드로 보완했다. 전체 페이지·링크·검색 검사는 W06/W08에서 완성한다.

## W06 — Go 예제·지원·호환성·확장

Go 홈·예제 3개·Playground 진입·지원·호환성·확장으로 나머지 16개 페이지를 완성했다.
Go SSE용 최소 명세를 추가했다. 공개 Go 제공자 CI 측정이 없는 상태를 정확히 알리고 TS 표를
Go 근거로 재사용하지 않는다. 두 타깃의 공개 문서는 정식 출시 계약으로 작성했다.

검증: 112개 실제 페이지/내부 링크, `validate-sdk-content.mjs --all`의 EN/KO 전체 코드 일치·
출시 문구·Go consumer API 검사, `build:docs` 통과. `verify-go-docs.mjs`는 첫 호출·SSE·웹훅의
실제 전체 코드 블록 3개를 각각 생성·컴파일·실행하고 출력과 대조한다. 모두 통과했다.
리뷰: 28개 주제×4 조합, 입력별 기능 분리, SSE 종료/close/Err, 로컬 웹훅 상태,
공개 측정 근거와 구현 지원 계약의 구분, 확장 YAML/Go API를 대조했다.
존재하지 않는 공통 Sort 타입 표기를 실제 생성 모델의 Field/Direction 설명으로 수정했다.
Go 선택 개방은 Playground와 legacy가 완성되는 W08에서 한다.

## W07 — Playground 세션과 최신 실행

원문·결과·옵션·인증을 앱 메모리 세션으로 분리했다. 상단 SDK 선택기만 남겼고 현재 SDK 옵션은
결과 화면에서도 편집할 수 있다. 옵션은 SDK별 저장/URL 우선이며 빈 `with=`는 확장 없음이다.
트리/선택 파일은 SDK별로 복원하되 실제 결과에 존재하는 경로만 선택한다. 옵션 연속 입력은
즉시 이전 세대를 폐기하고 300ms 후 생성한다. 원문·원격 URL·인증은 저장/공유하지 않는다.
fetch abort·Worker 종료·instance identity·success/catch/finally epoch·ZIP snapshot을 적용했다.

검증: `build:docs`, `playground:verify`(실제 WASM 두 SDK/addon/ZIP), options/ZIP 계약 4건 통과.
브라우저에서 실제 업로드 원문의 locale 전환, 응답을 지연한 Worker의 TS→Go→TS,
이전 Worker error, abort를 무시한 늦은 URL 응답, ZIP 시작 직후 SDK 변경을 재현했다.
원문 유지·최종 SDK 결과·stale 결과/오류/loading/다운로드 폐기를 확인했다.
인증은 locale 변경 중 유지되고 Playground를 떠나 재진입하면 지워지는 것을 실제 UI로 확인했다.
중간 preview는 재빌드 후 옛 asset 캐시로 404가 발생해 관리 preview를 재시작하고 다시 확인했다.
리뷰: SSR/session 경계, 파일·fetch·예제 입력, 안전 옵션, worker 오류/취소, locale remount,
다운로드/복사·tree 호출자와 인증 삭제를 확인했다. 초기 옵션 watcher 중복 실행, source가 없는
느린 fetch의 SDK 전환 취소, 저장 함수 SDK 인자 누락을 고치고 재검증했다.
최종 4 scope 공개 상태와 모바일/키보드 전체 점검은 W08/W09/W10에서 수행한다.

## W08 — 전체 경로·검색·사이트 metadata 전환

SDK 112개·중립 홈 2개·호환 페이지 52개로 전환하고 두 SDK를 개방했다. 정적 호환 링크와
client replace, noindex/search 제외/canonical, 옛 430개 heading alias와 Go 분할 16절을 보존했다.
중립 홈은 선택 카드와 마지막 SDK 이어보기를 제공하며 강제 이동하지 않는다.
canonical/hreflang/sitemap은 SDK와 표시 언어를 catalog로 대응시킨다.

검증: `build:docs`, `validate-built-docs.mjs`의 114 canonical HTML·52 legacy·430 anchors·
실제 검색 documentIds 검사 통과. 각 SDK scope는 Playground를 제외한 27개 본문만 검색한다.
legacy/query/한글 앵커 및 검증기 결함 탐지 9건 통과. 브라우저에서 52개 옛 경로를 모두 열어
정식 목적지로 이동함을 확인했다. 네 scope에서 API명을 검색해 자기 scope 결과만 나오는 것도
확인했다(TS 16개, Go 7개 결과). source partial과 호환 페이지는 검색에 없다.
리뷰: 각 SDK/locale 경로와 정적 HTML lang, base·.html·query·fragment·no-JS 링크,
검색 모달 scope·배포 metadata·중립 홈·검증 호출자를 확인했다. 한글 수동 alias의 NFC/NFD 혼합과
정적 replace의 .html 생략을 수정해 재검증했다. 호환 Playground는 no-JS에서 양쪽 SDK 링크를 제공한다.

## W09 — 출시 안내·화면·배포 연결

README/npm README를 정식 TS/Go 지원으로 개편하고 언어별 시작/예제 링크를 연결했다.
TS CLI에서 Go 옵션을 제외해 선택한 SDK 설명만 보이도록 했다. main·PR·수동 docs 실행은
검사/빌드만 하고, stable release가 published native/npm/Homebrew의 Go 생성·실행 검증 뒤
동일 tag/commit/version의 문서·WASM을 빌드/배포한다. artifact에는 release.json을 남긴다.
실패한 docs job만 재실행할 수 있으며 기존 패키지 게시 흐름은 유지했다.

검증: `build:docs`, `verify:docs`, actionlint 전체 workflow 검사, bash 문법 검사 통과.
`smoke-go.sh`를 버전이 주입된 로컬 candidate 실행 파일로 실행해 버전·생성·mock 호출을 확인했다.
외부 패키지 게시와 published 배포판 smoke는 실제 release CI가 담당하며 이번 작업에서는 실행하지 않았다.
리뷰: release 의존성·실패/재시도·prerelease 배포 제외·태그 SHA 확인·권한·npm 3 OS fixture,
원래 native/npm/Homebrew publisher, 문서 설치/첫 실행, EN/KO 코드 일치와 SDK별 CLI를 확인했다.
320·960·1024·1280px 브라우저에서 선택기/Playground 가로 넘침을 확인했다. 960px 선택기
줄바꿈을 고쳤고, 실제 모바일 키보드 Home/Enter SDK 전환 뒤 sdk-mobile 포커스 복원도 확인했다.
입력 없는 SDK 전환의 공유 query가 이전 옵션을 남기는 문제와 숨긴 mobile select 포커스를 수정했다.
생성 상태/SDK·파일 수는 live status로 제공한다. 전체 재검토와 확대/색상/저장소 최종 점검은 W10에서 한다.
