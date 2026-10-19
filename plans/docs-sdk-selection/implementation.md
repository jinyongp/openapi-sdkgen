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
