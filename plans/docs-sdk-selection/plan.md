# SDK 언어를 선택하는 문서·Playground 개편 계획

구현 완료: W01–W10의 단위별 결과와 최종 리뷰는 [구현 기록](implementation.md),
검증 범위·수치·제약은 [최종 검증](final-verification.json)에 기록했다.
아래 내용은 승인된 설계와 완료 기준이며, 실제 배포 실행은 별도 출시 단계다.


상태: 구현 전 상세 계획. 사용자 요구와 아래 권장 설계를 구분한다.
기준 커밋은 `fa1a051`이며, 제품 코드·현재 공개 문서는 이번 계획 작업에서 수정하지 않았다.

## 목표와 범위

SDK 언어 `TypeScript`/`Go`와 표시 언어 `English`/`한국어`를 독립적으로 선택한다.
선택한 SDK 안에서 시작하기, 생성, 호출, 타입·검증, 스트림, 수신 서버, 예제,
API 참조와 Playground까지 끝낼 수 있어야 한다. 메뉴·검색·이전/다음·본문 링크·다운로드도
같은 선택을 따른다. TypeScript 기본 문서에 Go 링크만 추가하는 현재 구조를 교체한다.

Go 정식 출시와 docs 개편을 같은 출시 묶음으로 내보낸다. README·npm README도 두 SDK의
정식 사용 안내로 맞춘다. 개발 중 배포판 상태를 출시 결과의 문구로 남기지 않는다.
SDK 생성기의 API·동작이나 지원 범위는 바꾸지 않는다. 기존 Playground의 파일·URL·예제 입력,
브라우저 내 생성, 인증·CORS·로컬 네트워크 안내, 입력 크기 제한, 코드 보기·복사·ZIP을 유지한다.
새 편집기, 서버 실행 서비스, 브라우저 타입 검사, CLI 전체 옵션의 Playground 노출,
프레임워크 교체, 신규 공개 성능 수치, 외부 검색 서비스는 이번 범위에 넣지 않는다.

사용자가 요구한 범위는 언어별 문서 분리와 선택, Playground 연결, 계획 보완이다.
아래 경로·상태·모듈 설계는 이를 구현하기 위한 권장 결정이다. 작업 등록이나 제품 구현은
아직 수행하지 않았다. 후속 구현은 이 문서를 기준으로 작업별 리뷰·커밋을 수행한다.

## 현재 구조에서 확인한 사실

- 현재 공개 문서 경로는 언어별 27개, EN/KO 합계 54개다. 기존 경로 전부를
  [페이지 대응표](page-map.json)에 기록했다. 전체 문서 본문을 이번 조사에서 전수 리뷰한 것은 아니다.
- 홈은 TypeScript SDK를 소개하고, `/guide/getting-started`는 Node.js·pnpm을 기본 경로로 삼는다.
  생성 클라이언트·스트림·수신 서버 API 참조도 TypeScript를 전제로 한다.
- `.vitepress/navigation.mjs`는 표시 언어만 인자로 받는다. `validate-locales.mjs`도
  EN/KO 한 쌍의 경로·코드 블록·링크를 검사하므로 SDK 축을 추가해야 한다.
- 검색은 VitePress 1.6.4의 local search다. 현재 인덱스 경계는 표시 언어다.
- Playground는 `target` query, Go 패키지/import 경로와 두 addon을 처리한다. TypeScript
  `typecheck`는 현재 Go/WASM `Options`와 브라우저 `GenerationOptions`에 없다.
- `runGeneration`은 타깃을 snapshot하지만 입력 원문을 세션 상태에 남기지 않는다.
  기존 Worker ID는 응답 연결용이며 최신 화면 요청만 반영하는 세대 번호와 다르다.
- 현재 설정 저장은 코드 테마·파일 트리 확장 상태뿐이다. SDK 선택·타깃별 생성 옵션의
  저장과 입력 원문 유지 정책은 새로 정해야 한다.

## 결정표

| ID | 권장 결정 | 관찰 가능한 결과 |
| --- | --- | --- |
| D01 | URL의 SDK와 표시 언어가 최우선이다. 선택 설정은 URL 해석을 덮지 않는다. | Go 링크 직접 접속·새로고침 시 TS 화면이 먼저 나타나지 않는다. |
| D02 | `/typescript/…`, `/golang/…`, `/ko/typescript/…`, `/ko/golang/…`를 정식 경로로 쓴다. | 링크 공유·검색 결과·브라우저 기록으로 선택을 복원한다. |
| D03 | `/`, `/ko/`는 중립 제품 홈과 SDK 선택 화면이다. 저장값으로 강제 이동하지 않는다. | 첫 방문에는 두 선택이 보이고 재방문에는 최근 선택의 이어보기 링크를 제공한다. |
| D04 | SDK·표시 언어는 각각 이름이 있는 native `select` 두 개로 제공한다. | 데스크톱·모바일·키보드에서 같은 두 축을 고른다. |
| D05 | SDK 변경은 같은 주제, 표시 언어 변경은 같은 SDK·주제로 이동한다. | 없어진 주제는 catalog의 명시적 fallback으로 이동하고 안내한다. |
| D06 | 단일 주제 catalog에서 URL·탐색·검색 scope·전환·검증 경로를 파생한다. | 경로를 다른 파일에서 다시 추측하지 않는다. |
| D07 | 공통 규칙은 locale별 Markdown partial로 공유하고 SDK별 정적 페이지에서 포함한다. | 선택한 SDK의 HTML·검색에 다른 SDK 소비자 예제가 섞이지 않는다. |
| D08 | VitePress locale scope를 SDK×표시 언어 4개와 중립 2개로 구성한다. | 기본 local search 엔진으로 scope별 인덱스를 분리한다. |
| D09 | 기존 경로는 정적 호환 페이지로 유지한다. 기존 TS 페이지는 TS 정식 경로로 연결한다. | 저장한 Go 선호 설정 때문에 과거 TS 링크의 의미가 바뀌지 않는다. |
| D10 | Playground의 SDK는 문서 URL과 같은 상태를 사용한다. 타깃 전환은 원문을 재사용한다. | 파일 재업로드·인증된 URL 재요청 없이 다른 SDK를 생성한다. |
| D11 | 입력 원문·응답 파일·인증 정보는 메모리에만 보관한다. | localStorage·공유 URL에 문서 내용이나 자격 증명이 들어가지 않는다. |
| D12 | 안전한 생성 옵션은 SDK별로 저장한다. URL에서 명시한 값이 우선이다. | TS↔Go를 오가도 Go 패키지·import 경로를 잃지 않는다. |
| D13 | SDK 변경은 새 생성, 표시 언어 변경은 기존 결과 유지로 처리한다. | 번역을 바꿀 때 불필요한 재생성이나 인증 요청이 없다. |
| D14 | 새 결과는 입력·타깃·옵션 snapshot과 최신 요청 세대가 모두 맞을 때만 반영한다. | 늦은 fetch·Worker 성공·실패·finally가 현재 결과를 덮지 않는다. |
| D15 | 공개 호환성 결과·성능 증거는 SDK별 실제 출처를 사용한다. | TS 결과를 Go 검증 결과처럼 표시하지 않는다. |
| D16 | 작업별 검증→리뷰·수정·재검증→커밋, 마지막에 전체 리뷰를 수행한다. | 기능 코드만 넣고 단위 리뷰·최종 검증을 생략하지 않는다. |
| D17 | Go 정식 출시와 docs 개편을 같은 버전·출시 묶음으로 배포한다. | 출시 docs·README·npm README는 Go를 정식 지원으로 설명하고, 해당 배포판의 설치·생성을 검증한다. |
| D18 | 공개 origin은 기존 README의 `https://jinyongp.dev`, base는 `/openapi-sdkgen/`를 사용한다. | canonical·언어 대안 링크가 같은 배포 주소를 따른다. |

## 페이지와 콘텐츠 개편

대응표에는 기존 27개 경로를 빠짐없이 분류했다. 기존 Go 통합 가이드는 여러 Go 주제로
나눈다. `reference/typescript-types`는 각 SDK의 `reference/types`로,
`guide/selection-benchmarks`는 `guide/selection-cost`로 연결한다. 선택 비용 페이지의 TS
공개 측정 출처는 유지하고, Go는 공개 CI 측정이 없는 수치를 채워 넣지 않는다.

각 topic의 SDK별 담당 작업 단위도 `page-map.json`에 지정해 본문 누락을 확인한다.
아래 28개 주제를 두 SDK·두 표시 언어로 제공한다. 총 112개 SDK 페이지와 중립 홈 2개다.
기존 페이지 52개는 호환 경로다. shared partial은 이 페이지 수에 포함하지 않는다.

| 묶음 | 주제 |
| --- | --- |
| 시작 | SDK 홈, 첫 SDK 만들기, 생성·검증, API 찾기, 원격 명세 |
| 사용 | 클라이언트, 모델·값의 존재 여부, 파일·Link·스트림, 인증·전송 |
| 구성 | 이름 있는 클라이언트, 선택 API, 선택 비용, 사용자 JSON Schema 어휘 |
| 수신·지원 | 웹훅·콜백, 문제 해결 |
| 예제 | 예제 개요, AI 스트리밍, 웹훅 수신 |
| 참조 | 참조 개요, CLI, 클라이언트 API, 서버 API, 스트림 API, 타입, 지원 범위, 호환성 결과, 확장 |
| 실습 | Playground |

공통 partial의 대상은 설치·입력·참조·진단·관리 출력·확장 규칙이다. 소비자 코드,
설치 후 실행 경로, 특정 옵션, 타입·오류·자원 수명·전송 제약은 SDK별로 작성한다.
Go 첫 실행은 사용자 Go 모듈을 사용하며 Node.js·pnpm을 필수로 설명하지 않는다.
Go 가이드·README·npm README의 “미출시”, “다음 릴리스”, 소스 미리보기 안내는 개편 때 제거한다.
Go 시작 경로는 함께 출시하는 native CLI와 사용자 Go 모듈을 기본으로 한다. npm 설치도 같은
릴리스 버전으로 Go 생성을 지원해야 한다. 소스 빌드는 선택 항목이며 정식 설치를 대신하지 않는다.
버전 번호는 기존 Releaseway 버전 계획의 결과를 사용하며 이 계획에서 임의로 정하지 않는다.
실제 게시 전 배포판 검증을 마치고, docs와 패키지를 같은 출시 기준으로 공개한다.
TypeScript의 npm·ESM·타입 검사·번들러 설명은 TS 문서에 둔다.

Go 참조는 root API, `types`, `operations`, `selective`, 이름 있는 `clients`, `metadata`,
`server/webhooks`, `server/callbacks`의 공개 진입점을 다룬다. Optional/nullable,
`Value`/`NativeInput`, raw 응답·오류, context 취소, Reader/Stream close와 소유권,
security provider, codec, Link·페이지, generic/typed 수신을 관련 사용 가이드와 연결한다.
기능 지원 주장은 기존 feature manifest·실제 생성 코드·소비자 증거와 대조한다.

새 Go 예제는 로컬 mock HTTP/SSE와 생성 수신 handler로 끝까지 실행 가능해야 한다.
AI 예제는 결정적인 mock 출력을 사용하고 실제 AI 서비스 호출로 설명하지 않는다.
표시 언어 간 코드·주석·예제 값은 같게 유지한다. SDK 간 코드 블록의 동일성은 요구하지 않는다.
전체 본문, 코드, 표, 이미지 설명, 이전/다음 링크의 TS 전제를 각 이관 작업에서 검토한다.

## 책임과 경계

| 모듈·위치 | 소유하는 책임 | 경계 |
| --- | --- | --- |
| `.vitepress/docs-catalog.mjs` | topic ID, SDK/locale 지원, suffix, 탐색 순서, fallback, legacy·fragment 매핑 | Vue·window·localStorage를 import하지 않는 데이터·순수 함수 |
| `.vitepress/doc-context.ts` | URL 해석·정규화·scope와 전환 목적지 계산, 선택 설정 schema | SDK URL은 catalog에서 얻는다. 저장값은 URL을 덮지 않는다. |
| `theme/components/DocsContextControls.vue`·Layout | 두 선택 UI, router 이동, 상태 안내·포커스 | SDK를 따로 ref로 복제하지 않는다. 모바일도 같은 계약을 사용한다. |
| `navigation.mjs`·`config.mts` | catalog에서 locale scope·nav/sidebar·local search 설정 구성 | SDK·locale 데이터의 두 번째 원본을 만들지 않는다. |
| `docs/_shared/{en,ko}/` | 공통 규칙 partial | 독립 공개 경로·검색 문서로 노출하지 않는다. |
| `docs/{typescript,golang}/`, `docs/ko/{typescript,golang}/` | SDK별 본문·예제·참조 | 최종 정적 HTML은 선택한 SDK의 완결된 문서다. |
| `theme/playground/session.ts` | 메모리 원문·출처·SDK별 옵션·결과 snapshot·현재 요청 세대 | 앱 세션에서 유지하며 브라우저 재시작 복원을 약속하지 않는다. |
| `playground/preferences.ts` | 검증된 SDK 선호·안전한 옵션·테마 저장 | 원문·파일 내용·인증·URL credential은 저장하지 않는다. |
| `playground/wasm.ts`·worker | 요청 연결, Worker 교체·종료, 생성 결과 | UI 최신 요청 판단은 session 계층이 맡는다. 중단 API는 기존 pending을 모두 종료한다. |
| `LegacyRedirect.vue`·생성 스크립트 | 예전 HTML 경로·앵커의 연결과 no-JS 대체 링크 | 새 SDK 페이지의 내부 링크로 legacy URL을 생성하지 않는다. |
| `docs/scripts/validate-doc-context.mjs` | catalog·실제 페이지·번역·링크·검색 scope·legacy 정합성 | nav의 목록만 검사하지 않고 실제 파일 집합과 대조한다. |

추가 backend나 새 검색 패키지는 쓰지 않는다. 현재 VitePress·Vue·MiniSearch 기반을 유지한다.

계획의 격리 검증은 다음 명령으로 다시 수행한다. 제품 문서를 바꾸지 않고 `docs/.tmp/`에만 빌드한다.

```sh
devtools run -- node plans/docs-sdk-selection/routing-search-probe.mjs
```

## URL·선택·이전 링크 계약

1. URL 파싱은 `/openapi-sdkgen/` base를 제거한 뒤 SDK·locale·topic을 판별한다.
   생성할 링크에는 base를 정확히 한 번 붙인다. `/golangish/` 같은 유사 prefix는 SDK로 보지 않는다.
2. 정식 경로에서는 SDK query를 사용하지 않는다. `?target=…`가 충돌하면 경로가 이기고
   중복 query를 `replaceState`로 제거한다. 다른 유효한 생성 옵션은 유지한다.
3. 중립 홈의 저장 SDK는 이어보기와 선택 UI 기본값에만 사용한다. 명시적으로 고른 SDK는
   현재 표시 언어의 `guide/getting-started`로 이동한다. SDK 홈의 로고는 현재 SDK 홈으로 연결한다.
4. SDK·locale 전환은 history에 새 항목을 추가하고, 초기 query 정규화만 replace한다.
   뒤로/앞으로 가기에서도 URL이 유일한 선택 원본이다. 탭 간 storage 변화로 현재 페이지를 이동시키지 않는다.
5. 설정이 없거나 손상·차단·용량 초과 상태여도 URL 기반 문서는 동작한다.
   알 수 없는 SDK 경로는 정상 404로 처리하고 지원하는 두 SDK의 진입 링크를 제공한다.
6. 주제 전환은 존재하는 목적지만 만든다. fragment는 명시적으로 대응한 경우만 보존하고,
   대응하지 않는 fragment는 제거해 페이지 상단으로 이동한다. fallback 시 안내 문구를 표시한다.
7. 기존 TS 페이지의 heading ID는 새 TS 페이지에서도 alias로 유지한다. 기존 Go 가이드의
   여덟 절은 `page-map.json`의 `fragmentDestinations`로 분리한다. 한국어의 분해된 Unicode ID도 보존한다.
8. 기존 `/playground?target=golang`은 Go로, `target=typescript` 또는 query 없는 과거 Playground는
   TS 정식 경로로 연결한다. 현재 선택 설정으로 과거 링크의 의미를 바꾸지 않는다.
9. 호환 페이지는 서버 301을 가정하지 않는다. JS가 있으면 `location.replace`, 없으면 화면에
   실제 목적지 링크를 제공한다. `search: false`, `noindex`, canonical을 지정하고 검색·목록에서 뺀다.
   기존 `.html`·디렉터리·hash URL을 각각 점검한다. 게시 환경은 현재 정적 사이트 경로를 유지한다.

## 검색·정적 페이지 설계와 검증 근거

VitePress 1.6.4의 built-in locale scope로 검색을 분리한다. locale key의 등록 순서는
`ko/typescript`, `ko/golang`, `typescript`, `golang`, `ko`, `root`다.
현재 `getLocaleForPath`는 처음 맞는 key를 쓰므로 `ko`를 먼저 등록하면 한국어 SDK 경로가
중립 scope로 잘못 분류된다. 이 잘못된 순서도 probe에서 재현했다.

각 scope의 HTML `lang`은 실제 표시 언어인 `en-US`/`ko-KR`로 둔다. locale key가 SDK를
포함하더라도 UI에는 두 SDK와 두 표시 언어만 보인다. 기존 native locale menu는 모든 scope의
`label`을 빈 값으로 두어 렌더하지 않고, 별도 native select 두 개로 대체한다.
현재 VPNavBar/VPNavScreen translations의 `currentLang.label` 조건을 확인했다.

중립 홈 검색은 중립 scope만 사용한다. SDK 문서 검색은 해당 scope만 가져온다.
표시 언어의 검색 UI 번역은 scope별로 같은 번역을 적용한다. 검색 모달이 열린 상태에서
전환할 때 scope로 search wrapper를 key 처리하여 이전 검색 결과와 비동기 로드를 폐기한다.
공통 partial, legacy 페이지, 내부 개발 문서가 SDK 검색 인덱스에 별도 항목으로 들어가지 않게 한다.
검색 UI·단축키·포커스 관리는 기존 기본 테마 동작을 유지한다.

실제 설치 버전으로 격리된 여섯 scope 정적 사이트를 빌드했고, 각 검색 chunk의 `documentIds`가
자기 scope의 문서만 포함함을 확인했다. [결과](routing-search-probe.json),
[실행 스크립트](routing-search-probe.mjs)를 보존한다. 처음 두 harness assertion은 출력 chunk
경로·파일명 가정 때문에 실패했다. 최종 검사는 파일명 대신 실제 검색 데이터를 대조한다.

공식 근거는 [1.6.4 경로 해석](https://github.com/vuejs/vitepress/blob/v1.6.4/src/shared/shared.ts),
[1.6.4 local search 구현](https://github.com/vuejs/vitepress/blob/v1.6.4/src/node/plugins/localSearchPlugin.ts),
[local search 안내](https://vitepress.dev/reference/default-theme-search)다. 최신 안내의 버전 차이는
설치된 1.6.4 소스와 위 격리 빌드로 확인한다. 이 probe가 실제 개편 사이트의 검색·접근성 통과를
대신하지는 않는다. canonical·hreflang을 만들 때는 catalog의 같은 SDK·주제를 사용하며,
기본 sitemap의 첫 경로 segment 추정에 중첩 locale 처리를 맡기지 않는다.

## Playground 상태와 전환 계약

- 입력 경로는 기존 파일·URL·예제다. 읽은 원문을 메모리에 보관하고 출처 label과 함께 유지한다.
  타깃을 바꿀 때 원문을 재사용하며 URL을 다시 가져오거나 인증을 재요청하지 않는다.
- `session.ts`는 앱 내에서 하나다. SDK·표시 언어 페이지 이동으로 Playground component가
  다시 만들어져도 원문·옵션이 사라지지 않는다. 새로고침 시 업로드/URL 원문 복원은 약속하지 않는다.
  예제 링크는 공개 예제 원문으로 재생성할 수 있다.
- 문서 상단의 SDK 선택기가 유일한 조작 지점이다. Playground에는 현재 SDK를 명확히 표시하고
  해당 옵션만 노출한다. 서로 독립적인 두 타깃 dropdown을 유지하지 않는다.
- 입력이 없을 때 SDK 변경은 준비 상태만 바꾼다. 입력이 있으면 이전 결과를 현재 결과에서
  즉시 제거하고 새 타깃·옵션 snapshot으로 재생성한다. 실패하면 새 타깃의 오류와 재시도 동작을 보인다.
- 표시 언어 변경은 생성 결과를 유지한다. 기존 diagnostics 원문은 생성 당시 결과로 유지하고
  UI·버튼·상태 안내만 번역한다. 문서 선택 변경으로 입력을 다시 읽지 않는다.
- 공통 옵션은 `server`·`metadata`, Go 옵션은 package·import path다. TS `typecheck` UI는 추가하지 않는다.
  Go 기본값은 현재의 `api`, `example.com/consumer/internal/api`다. 안전한 옵션은 SDK별로 저장한다.
- URL의 명시적 옵션 → 해당 SDK의 저장 옵션 → 기본값 순이다. `with=`는 addon 없음으로 해석한다.
  새 URL 직행은 그 URL의 예제·옵션을 적용한다. 앱에서 사용자가 SDK/locale만 바꾸는 경우는
  현재 session 원문과 옵션을 보존한다. source를 포함하지 않는 옵션은 공유 링크로 만들 수 있다.
- 공유 URL에는 SDK는 경로로, 예제 ID·검증된 addon·Go 옵션만 query로 담는다.
  업로드·인증 URL의 원문이나 원격 URL·인증 헤더·암호를 URL에 넣지 않는다.
- 실제 생성 조건을 바꾸는 입력·SDK·옵션 변경마다 세대를 증가시킨다. fetch에는 AbortController를
  쓰고, 완료 후에도 세대를 검사한다. Worker는 SDK 변경·다시 시작 시 종료하고 pending을 거절한다.
  Promise success/catch/finally 모두 자기 세대일 때만 파일·오류·loading을 바꾼다.
- 고정된 `busy` 상태 때문에 SDK/locale 선택을 막지 않는다. 언어 전환은 현재 요청을 유지하며,
  SDK 전환은 이전 요청을 폐기한다. 이전 Worker error가 새 Worker를 종료하지 않도록 instance identity도 검사한다.
- ZIP 생성은 현재 결과 snapshot을 사용한다. 생성·SDK 변경·내보내기 상태가 겹치면 다운로드를
  비활성화하고, 이미 시작한 내보내기는 snapshot이 현재 결과와 맞을 때만 다운로드한다.
  ZIP 이름·선택 파일·강조 언어·진단 요약도 결과 snapshot의 SDK와 일치해야 한다.
- Playground를 떠나면 진행 중 fetch·Worker를 취소하고 인증 정보를 지운다. 원문과 안전한
  옵션은 앱 메모리에 남긴다. 재진입 시 현재 SDK·입력·옵션에 맞는 결과만 복원하고 아니면 재생성한다.
- 인증 정보는 Playground 내 SDK/locale 전환 동안 메모리에서만 유지한다. Playground를 떠날 때,
  다시 시작하거나 탭을 닫을 때 지운다. 원문·옵션을 초기화할 때는 진행 중 fetch/Worker도 폐기한다.
- storage의 기존 코드 테마 키는 유지한다. 파일 트리 확장·선택 파일은 SDK별로 분리하고
  실제 결과에 존재하는 경로만 복원한다. 손상된 저장 데이터는 무시한다.

## 접근성과 화면 기준

데스크톱 header와 모바일 메뉴에 같은 label을 가진 SDK·표시 언어 선택기를 둔다.
숨긴 쪽은 포커스 순서에서도 제외한다. SDK 전환 후 선택한 값과 새 페이지 제목을 알 수 있어야 한다.
선택기 옆에 “선택하면 해당 문서로 이동합니다”를 안내하고 접근성 설명으로 연결한다.
자동 이동은 선택기의 change로만 발생하며 focus/hover로 이동하지 않는다.
일반 페이지 전환은 제목/주 콘텐츠로 포커스를 옮기고, Playground 전환은 새 화면의 SDK
선택기 또는 오류 안내로 포커스를 복구한다. native select 내부 키 조작을 가로채지 않는다.

생성 중·완료·실패 상태는 색 이외의 텍스트와 적절한 live status로 알린다. 잘못된 Go 옵션은
해당 입력과 오류를 연결한다. 라이트/다크, 키보드만 사용, 모바일 320 CSS px, 200% 텍스트 확대,
400% 줌에서 선택·검색·옵션·파일 선택·다운로드가 가려지지 않는지 확인한다.
스크린리더 접근성 트리 확인과 지원 AT의 실제 동작 확인 가능 여부를 기록하며,
자동 검사나 격리 probe만으로 WCAG 전체 준수를 선언하지 않는다.

## 작업 단위·의존성·리뷰·커밋

아래 검증은 승인 당시의 완료 조건이다. 실제 실행 결과는 구현 기록과 최종 검증 파일을 기준으로 한다.
이관 중에는 catalog의 `publishedTargets`로 실제 개방된 SDK를 구분한다. W01은 순수 catalog
계약을 검사하고, W02부터 실제 페이지 검사는 개방된 SDK와 기존 경로를 대상으로 한다.
W08에서 Go를 개방하려면 112개 SDK 페이지와 52개 호환 페이지를 모두 검사해야 한다.
최종 검증은 두 SDK가 모두 개방됐는지도 강제해, Go를 숨긴 상태로 완료를 선언하지 못하게 한다.

각 단위는 구현 후 지정 범위를 리뷰하고, 발견한 결함을 수정·재검증한 뒤 커밋한다.
추가 결함이 없고 검증 근거가 최종 상태에 적용될 때 단위를 닫는다.

| 단위 | 결과·주요 파일 | 선행 | 검증과 리뷰 범위 | 커밋 경계 |
| --- | --- | --- | --- | --- |
| W01 | catalog·28주제·52legacy·옵션/상태 계약 구현. `docs-catalog.mjs`, `doc-context.ts`, 실제 파일 집합 검증 | 없음 | 네 조합 전환, base 중복·유사 prefix·저장값 손상·없는 주제, 대응표 누락 검토 | 순수 계약·catalog·계약 검증 |
| W02 | 새 정식 TS EN/KO 페이지와 공통 partial 추가, 6 scope 설정·탐색 구성 | W01 | 기존 TS 사용 흐름·heading/코드·시작하기, 4 SDK scope 중 준비된 TS 인덱스와 중립 root 확인 | TS 이관·공통 원본·scope 기반 |
| W03 | SDK/표시 언어 선택 UI와 URL 상태 연결. Layout·모바일·preferences | W02 | SSR/새로고침, path 우선, 뒤로/앞으로, storage 차단·다른 탭, 키보드·모바일 전환 | 선택 UI·탐색 상태 |
| W04 | Go 시작·모델·호출·오류·생성·API 찾기·원격 입력·인증·전송·문제 해결 EN/KO 작성 | W02 | native CLI→사용자 Go 모듈→mock 첫 호출, 사용한 모든 SDK 코드의 실제 생성·컴파일·실행, Go 수명·의존 경계 리뷰 | Go 핵심 사용 흐름 |
| W05 | Go 파일/codec/stream/Link·페이지·named/selective·선택 비용·metadata·수신 API와 타입 참조 완성 | W04 | 실제 생성 API와 문서 대조, 정상/취소/close·오류 예제 실행, 공개 namespace·feature 근거 누락 검토 | Go 심화·참조 |
| W06 | Go AI 스트림·웹훅 예제와 지원/호환성/확장 페이지 완성, TS 대응 설명 재검토 | W05 | 로컬 mock 스트림·수신 예제 실행, Go 옵션/증거 필터, EN/KO 코드 일치, TS 결과 오용 검사 | 예제·지원 증거·콘텐츠 완결 |
| W07 | Playground 세션·단일 SDK 선택·안전한 옵션·최신 요청 규칙 구현. UI·preferences·WASM worker | W03 | 파일/URL/예제 뒤 전환, A→B→A와 늦은 성공/실패, Worker error/취소, locale·뒤로/앞으로·ZIP snapshot, 인증 회귀 | Playground 상태·입력/결과 연결 |
| W08 | 정적 호환 페이지·앵커, SDK별 검색·사이트 metadata 완성. legacy 생성·검증·config | W06,W07 | 52옛 경로·전체 기존 heading alias·Go 분할16절, 실제 index 문서ID와 결과 scope, NO-JS 링크·base·canonical | 기존 링크·검색·검색엔진 metadata |
| W09 | 전체 문구·내부 링크·모바일·접근성·README/npm README를 정식 출시 안내로 정리, `docs.yml`·`release.yml`의 배포 연결과 문서 검사 갱신 | W08 | 실제 파일과 catalog 일치, 로컬/타입별 예제, 잘못된 SDK 링크·미출시/소스 미리보기 문구 잔존·다른 SDK 코드 혼입 리뷰 | 문서·UX·패키지 진입 안내 통합 보완 |
| W10 | 전체 최종 리뷰와 완료 기록. legacy 원문 중복 제거·남은 개발 문서 링크 정리 | W09 | 아래 전체 검증, 요청 범위 전부 재검토, 영향받은 기존 TS 호출·playground·문서 검사 재확인 | 최종 정리·검증·리뷰 기록 |

W02~W07은 추가·이관 과정의 통합 제약이 있다. 새 Go 경로가 준비되기 전에는 전역 Go 선택을
사용자에게 개방하지 않고 현재 사이트의 기존 진입 경로를 유지한다. W08에서 전체 SDK 분리로
전환한다. W08 이전 중간 커밋을 각각 완성된 출시 결과로 취급하지 않는다.
최종 완료 전까지 기존 원본을 복구 가능한 상태로 유지하고, W10에서 본문 중복을 정리한다.
실제 release·push·외부 task 등록은 현재 계획 보완 작업에서 수행하지 않는다.
후속 구현에서는 아래 동시 출시 조건까지 준비하며, 이를 공개 문서에 미출시 안내를 남길 이유로 사용하지 않는다.

## Go와 docs의 동시 출시

현재 `.github/workflows/docs.yml`은 main push에서 바로 Pages를 배포한다. 이를 그대로 두면
개편 docs가 Go 배포판보다 먼저 공개될 수 있으므로 이번 이관의 배포 경계를 함께 바꾼다.

- main·PR에서는 docs 검증·빌드를 계속 수행한다. 개편 docs의 운영 배포는 해당 Go 정식
  릴리스의 배포 검증이 완료된 뒤 같은 태그·릴리스 커밋에서 만든 artifact를 사용한다.
- `.github/workflows/release.yml`의 `prepare-release`가 정한 tag/version/commit을 기준으로
  `publish-github-release`, `publish-npm`, `verify-npm-release`, `publish-homebrew`와 연결한다.
  실제로 안내하는 native·npm·Homebrew 설치 경로가 같은 Go 지원 버전을 제공하는지 확인한다.
- native 실행 파일과 npm 배포판 각각에서 `--target golang` 생성 후 사용자 모듈에서
  SDK를 컴파일·실행한다. README·Go 시작하기의 명령이 같은 버전에서 동작해야 한다.
- Pages 배포는 위 배포 검증과 docs 최종 검증을 통과한 출시 묶음에 연결한다. main에서
  개편 docs가 먼저 자동 배포되거나, 다른 커밋의 WASM을 재사용하지 않게 한다.
- 배포판 게시가 실패하면 기존 docs를 유지한다. 배포판 게시 후 docs 배포만 실패하면
  같은 검증된 태그/artifact로 docs 배포를 재시도하며 이미 게시한 패키지를 다시 만들지 않는다.
- W09에서 배포 연결과 재시도 경계를 구현·리뷰하고, W10에서 같은 태그·버전·커밋의
  패키지·WASM·docs·설치 예제 근거를 확인한다. 실제 게시 실행은 별도 출시 단계다.

## 전체 검증과 완료 판정

- EN/KO×TS/Go 네 조합에서 시작→생성→mock 호출→오류→스트림/수신→참조→Playground 경로를 실제로 따른다.
- catalog의 28주제×4 페이지, 중립 홈 2개, legacy 52개를 파일·HTML·링크·검색 집합과 대조한다.
  SDK 간 지원하지 않는 주제를 숨겨 누락을 덮지 않는다. EN/KO parity는 같은 SDK끼리 검사한다.
- 네 검색 scope 각각에서 공통어·SDK별 API명을 검색하고 다른 scope가 결과·스니펫에 나타나지 않는지 확인한다.
  인덱스 검사는 파일명 패턴 대신 실제 documentIds를 사용한다.
- 주제·SDK·locale 전환, 하위 경로 직접 접속, query 충돌, base, fragment, 새로고침·뒤로/앞으로,
  저장 설정 없음/손상/차단, JS 없는 호환 페이지를 검증한다.
- 실제 WASM 생성과 ZIP에 대해 두 타깃·두 addon을 검증한다. 문서 locale 변경 중 실행,
  느린 인증 fetch·빠른 연속 전환·worker crash·내보내기 중 전환을 재현한다.
- 새로 작성·변경한 소비자 예제를 두 SDK에서 생성·컴파일·실행한다. 모든 API 모양 설명을
  하나의 Todo 문서가 지원한다고 가정하지 않고 주제별 최소 명세를 둔다.
- `devtools run validate:docs`, `devtools run build:docs`, `devtools run playground:verify`,
  `devtools run go:example` 및 기존 docs 도구 테스트를 수행한다. 새 catalog·경로·세대 계약 검사는
  기존 docs 검사 명령에 통합한다. TS 예제는 기존 관련 예제/검증 명령을 사용한다.
- 접근성·모바일 확인은 실제 빌드 사이트로 수행한다. public docs와 다운로드 자산에 내부
  작업 경로·로컬 benchmark 수치·개인 머신 사양을 넣지 않는다.
- 출시용 문서·README·npm README에 미출시·다음 릴리스·소스 전용 안내가 남지 않아야 한다.
  해당 Go 지원 버전의 native·npm·Homebrew 배포와 docs/WASM의 릴리스 기준이 일치해야 한다.
- 작업별 리뷰가 끝나고 전체 최종 리뷰에도 미검토 영역·확인된 미수정 결함이 없어야 완료다.
  아직 실행하지 않은 브라우저·예제·접근성 검증은 통과로 기록하지 않는다.

## 이번 계획 보완 검토

이번에 검토한 영역은 라우팅/설정, 페이지·번역 구조, 검색, legacy·앵커,
Playground 입력·옵션·비동기 상태, 증거 표기, 접근성, 작업 경계·검증이다.
계획을 반복 대조하며 다음 누락을 보완했다.

1. 표시 언어만 나뉜 검색 → 6 scope·nested 우선순위·현재 버전 격리 빌드로 구체화.
2. 타깃을 바꾸면 입력을 잃는 문제 → 메모리 session·입력 snapshot·URL 재요청 없는 재생성으로 결정.
3. Worker 요청 ID만으로 화면 일관성을 보장한다고 보기 어려움 → 세대·Worker identity·fetch 취소·ZIP snapshot 규칙 추가.
4. Go 통합 가이드 하나로 참조를 대신하는 누락 → 28주제×4와 공개 namespace·실행 예제 범위 명시.
5. URL/저장 설정 충돌·브라우저 기록·locale 변경으로 재생성되는 빈틈 → URL 우선·명시 query 우선·locale 결과 유지 결정.
6. 기존 Go 가이드의 분해 Unicode 앵커·옛 Playground query → 실제 heading ID와 52개 경로 대응표 작성.
7. 신규 TS typecheck UI·새 편집기를 암묵적으로 추가할 위험 → 기존 WASM·입력 계약을 유지하는 범위로 정리.
8. Go 출시와 docs 개편을 별개로 취급한 오류 → 같은 정식 출시 묶음으로 정정. 미출시 문구 제거, 배포판 설치 검증, main의 docs 선행 배포 차단과 출시 실패·재시도 경계 추가.
9. 부분 이관 중 Go 메뉴가 빈 페이지로 이어질 위험 → 추가→전체 전환→원문 중복 제거 순서와 W08 전환 경계 명시.
10. 페이지 완성 전에 새 validator가 모든 경로를 요구하는 순환 → 공개 SDK별 중간 검사와 두 SDK 모두를 강제하는 최종 검사로 구분.

이 검토는 구현 계획의 범위·계약 검토다. 실제 사이트 개편·전체 콘텐츠 검수·접근성 검증은
아직 하지 않았다. 설치 버전의 검색 분리·중첩 경로 probe와 페이지/legacy 대응표 검사는 수행했다.
