# W04 — 요청 section 사전 인덱스 보류

결정: 후보 C를 기본 출력에 넣지 않는다. 실제 checker hotspot이 확인되면 다시 검토한다.
Graph 전체 strict 성공과 공개 generic 계약은 그대로 필수다.

현재 `RouteSourceForSection`, `OperationIDSourceForSection`,
`ResourceOperationRouteForSection`은 canonical `RouteRequestSections`와 `Routes`를
읽는다. route/ID가 있는 section만 허용하고, resource method는 이미 binding된 `path`를
제외한다. `SelectedOperationSections`는 exact/resource 구분을 유지한다. 새로운 index는
이 관계, ID 없는 route, union/never, readonly/optional과 resource 제약을 모두 보존해야 한다.

원자료의 C 결과는 실제 Graph checker 증거가 없는 합성 실험이다. 실제 Graph에서는
A+B 및 대형 타입 분할 후에도 187,687 root source/consumer를 구성하던 중 Node heap
부족이 발생했다. 7 GiB heap/8 GiB RSS smoke는 74.51초에 실패했다. 가용 RAM 약
12.60 GiB 확인 후 사전 정책대로 9.55 GiB heap/10.55 GiB RSS·3,600초 상한에서
pinned TypeScript 5.9.3 API로 다시 진단했다. 이 실행도 121.30초에 heap 부족으로 끝났고,
`createProgram` 완료와 명시적인 semantic 검사 호출에 도달하지 못했다.

원시 결과는 다음에 있다.

- `.tmp/sdk-optimization-execution/graph-ts-combined-256-typescript-5-9/result.json`
- `.tmp/sdk-optimization-execution/graph-ts-combined-256-phase-run/result.json`
- `.tmp/sdk-optimization-execution/graph-ts-combined-256-phases.jsonl`

이 자료는 section predicate가 현재 Graph 실패의 원인이라는 근거를 제공하지 않는다.
작은 section index를 추가해 전체 소스 구성 실패가 해소됐다고 주장할 수 없다.
따라서 C의 실제 이득·불확실성을 확인하지 못한 상태에서 새 출력과 유지보수 비용을
추가하지 않는다. C 미채택을 이유로 strict 검사나 public 계약을 완화하지 않는다.

`emitRequestSectionHelpers`의 본문은 변경하지 않았다. W03의 변경은 input/raw map을
canonical 타입 모듈에서 가져오는 부분이며 section 판정식은 그대로다. 관련 caller와
기존 input DX·named input·deferred selection·공개 helper 선언을 정적으로 대조했다.
현재 전체 생성기 520 test와 W03의 대형 source/declaration consumer 네 compiler
통과 근거를 재사용한다. 런타임/컴파일러 전체 통합 검사는 W11의 필수 조건으로 남는다.

다음 우선순위는 W05/W08의 source ownership·SCC·declaration 소비 경로다. 모든 SDK
source를 strict 검사하고 전체 public consumer를 검사해야 한다. 그 후 semantic 검사
profile에서 section predicate가 반복 비용의 원인으로 드러나면 W04를 다시 열어
index 유무를 같은 실제 입력·예산에서 비교한다. 현 단계 factorial에는 C를 넣지 않으며
없는 variant를 성공 또는 성능 이득으로 집계하지 않는다.
