# 요청 section 사전 인덱스 판단

현재 상태: W08에서 C 후보 검증을 재개했다. 후속 native profile에서 큰 union 구성 비용을
확인했고, 같은 10 GiB hard/soft 조건의 private filter 대조에서 원본은 120초 미완료,
C는 9.14초 계측 구간 안에 semantic 오류 0개로 완료했다. 전체 CLI/source/declaration/
public consumer 성공은 아직 아니다. 원본 value projection·공개 generic 식을 유지하고
canonical `InputSections`와 operation emitter와 같은 resource reachability를 사용해
기존 256 operation 계약 청크에 literal membership만 추가하는 후보를 검증한다.
작은 SDK와 selected/named의 기존 helper/import 경로는 유지한다. 네 compiler의 독립된
이전 mapped-filter oracle·양방향 generic·union/never/PropertyKey·ID 없는 route·resource/
negative·isolated declaration과 실제 Graph 전체 검사 후 최종 판단을 갱신한다.

후속 후보의 네 compiler source/isolated declaration/독립 consumer와 양방향 generic
검사는 통과했다. 실제 Graph의 116개 계약 청크에서 3×6 membership을 원본 선언 oracle과
대조한 결과도 차이 0개다. 전체 native strict는 이 필터와 별개인 기존 resource root의
중복 callable 표현에서 TS2859 312개로 종료됐다. 그 표현의 canonical `Surface` 재사용
수정은 W08의 별도 후속 단위이며 최종 전체 source/public 성공까지 추적한다.

canonical callable 참조만 적용한 실제 Graph에서는 root 오류 158개가 남았다. 후속
canonical Members·상속 RegistryCalls 후보는 네 compiler의 확대 계약 검사를 통과했고
새 실제 Graph root도 오류 0개로 통과했다. 다음 전역 owner의 비용을 CPU/heap으로
조사 중이며, C의 전체 source/public consumer 성공과 최종 채택은 아직 완료하지 않았다.

아래는 W04 당시의 보류 판단과 근거다. 후속 실험을 과거 판단의 당시 근거로 섞지 않는다.

## 초기 W04 판단

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
