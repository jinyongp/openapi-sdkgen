# 통합 검사에서 발견한 선언 계약 수정

최초 통합 실행은 Go 소비자166개와 TS 생성기522개를 통과한 뒤, 기존 catalog의
`baseline-oas30` 선언 규칙에서 실패했다. 모든 공개 route를 한 resource가 소유할 때
`ResourceOperations`가 canonical `Routes`와 같은 object 계약을 다시 선언했다.
기존 최소 fixture를 이전/수정 generator로 각각 생성해 이전 오류1개와 수정 후0개를
재현했다. 기존 checker 규칙을 유지하고 이 경우에만 canonical `Routes`를 재사용한다.
선택·named client의 private Link 의존성은 공개 route 개수에서 제외한다.
그 외 resource는 기존의 좁은 local 계약과 canonical Surface 상속을 유지한다.

다음 `aliases` fixture에서는 서로 다른 디렉터리의 동일한 상대 import 문자열을 checker가
같은 타입으로 오인했다. `import("./child.js").Surface`는 각 source 기준의 실제 module
경로와 qualifier·type argument·typeof·attribute를 함께 비교하도록 교정했다.
서로 다른 실제 대상 허용, 같은 대상을 다른 상대 경로로 복사하면 거절, 다른 qualifier는
허용하는 회귀를 추가했다. 수정 전 실패와 수정 후 전체 선언 self-test 통과를 확인했다.

기존 전체 conformance catalog와 선언 옵션 출력 검사가 통과했다. parameter call·부모/자식
registry·깊은 resource의 네 compiler/독립 declaration 소비 검사도18.13초에 통과했다.
W08을 명시적으로 다시 열고 수정·caller 재검토·단위 검사를 수행했다. 최종 TS suite와
통합 검사는 수정 뒤 다시 확인하며, Go 검증은 관련 생성 의존성이 같은지 별도로 대조한다.

원시 실행은 `.tmp/sdk-optimization-execution/`의 `resource-canonical-*`,
`inline-import-identity-red/green`에 있다. 초기 전체 통합 실패와 중간 수정의 한계도
그대로 보존한다. 이 수정은 전체 Graph·Cloudflare 재실행을 요구하지 않는다.
