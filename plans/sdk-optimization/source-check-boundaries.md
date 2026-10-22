# Graph 검사 경계의 진단과 다음 후보

2026-10-22 현재 결정은 `scope-decision-2026-10-22.md`를 따른다. 아래 전체 Graph
진단·references 후보는 실험 이력이다. 전체 Graph 성공과 추가 반복은 필수가 아니며,
대표 SDK·Graph 선택 SDK의 네 compiler strict·공개 계약과 기존 사용법을 검증한다.

현재 결론은 전체 strict 성공이 아니라 다음 구현 후보의 범위 확정이다.
단일 프로젝트의 최종 W03 Graph는 네 compiler에서 자원 상한 안에 완료하지 못했다.
실험별 원시 결과와 source/도구 해시는 `execution.md`와 `.tmp/sdk-optimization-execution`에 있다.

| 실제 TS7 references 진단 | 완료 project | 결과 |
| --- | --- | --- |
| 64 MiB / 25,000 source files | 8/13 | 다음 project RSS 보호 종료 |
| 32 MiB / 12,000 source files | 16/27 | 다음 project RSS 보호 종료 |
| 32 MiB, scratch declaration 주석만 생략 | 16/27 | 같은 project RSS 보호 종료 |

주석 생략은 검사 중 생성하는 임시 선언에만 적용했다. 원래 SDK의 public 문서와 주석은
유지하며 별도의 실제 compiler symbol/hover 회귀로 확인한다. 모든 owner source의 strict와
마지막 전체 declaration/public consumer 검사는 계속 필수다. 이 실험은 단계별 원인을
분리하기 위한 것이며 아직 성공한 fallback으로 채택하지 않는다.

32 MiB/reference의 16번 project를 5.9.3 API로 관측했다. owner source 1,848개와
reference 15개에서 program 37,988파일(declaration 36,140개)을 만들었고 binding까지
완료했다. 이후 전체 diagnostics 수집에서 heap 부족으로 종료됐다. 정확한 semantic
predicate/개별 파일 CPU hotspot은 아직 확정하지 않는다.

정적 의존성 조사에서 실패한 project는 registry prepare/ID 청크가 들어오는 첫 묶음이다.
청크는 `registry-contracts.ts`의 전체 `Routes`/`OperationRoutes`를 참조한다. resource
factory도 전체 `CallableRegistry`와 `routes/helpers.ts`를 참조한다. 따라서 작은 청크를
검사할 때도 전체 operation 계약을 다시 읽는다. 이 import 경계를 최소한으로 끊고
같은 전체 source/public consumer 검사를 다시 수행한다.
현재 근거만으로 section predicate가 주원인이라고 확정하지 않는다.

W08의 우선 후보는 다음과 같다.

- registry 단계의 메서드 shape를 문서 전체 map을 모르는 하나의 generic runtime 계약으로
  둔다. root는 전체 계약의 view를 사용하고 각 청크는 자기 operation의 canonical Contract와
  실제 LinkTargets에서 유도한 좁은 view를 사용한다. Link 순서·완료 map identity와 수명은 같다.
- ID 등록 청크도 자기 ID가 읽는 canonical operation의 call/link 타입만 참조한다.
- resource factory는 자기 operation의 Contract에서 call/pagination 타입을 유도하고,
  child factory의 필요한 registry view를 합친다. 전체 route helper로 우회하지 않는다.
  공개 Surface의 exact route brand·resource input·문서·child surface는 보존한다.
- 위 구현은 runtime registry 값·공개 import 경로·입력/응답·생성 선택 경계를 바꾸지 않는다.
  원래 source 소비와 설치에 compiler나 references를 필수로 추가하지 않는다. references
  도구는 내부 전체 검사 경로다. 공개 배포 형태를 바꿔야 한다면 별도의 구체안과 근거를 남긴다.

검증은 실제 대형 Link/private/named/stream/pagination fixture의 네 compiler source와
declaration/hover/runtime 회귀, local import graph의 전체 map 의존성 제거, 실제 full Graph
source ownership·네 compiler·전체 public/negative consumer 순서다. server add-on은 전체
Graph의 별도 profile에서 생성한 source와 실제 server export consumer를 함께 검사한다.
작은 fixture나 일부 project 통과로 전체 Graph 성공을 대체하지 않는다.

## W08 operation ID 계약의 bounded projection

후속 파일별 native API 진단에서 `client/types.ts`와 `registry-contracts.ts`의
문서 전체 `OperationRoutes[ID]`를 다시 `Routes`로 인덱싱하는 경로가 120초 안에
끝나지 않았다. `ClientRoutes`를 경유하거나 `& keyof Routes`를 명시하는 대조도
완료하지 못했다. 이 진단은 프로파일을 켠 별도 loaded program이며 전체 CLI 검사는 아니다.

새 후보는 기존 bounded route contract part마다 ID 계약 `Operations`를 유도한다.
전체 `Operations`는 이 부분 계약들을 상속하고, registry는 그 계약의 `call`·`links`를
읽는다. client의 ID 호출은 같은 canonical `call`과 기존 operation identity를 조합한다.
따라서 전체 route key union과 ID의 route union을 다시 비교하는 중첩 인덱싱을 제거한다.
공개 route·ID·generic helper·brand, 선택/named client 경로와 runtime 등록은 유지한다.
추가 public 객체 shape나 `any`·cast·검사 생략은 없다.

실제 Graph 입력을 복제한 대조에서 client·registry·route index 파일의 semantic 검사는
각각 6.99초·10.39초·4.61초에 오류 없이 끝났다. loaded program 59,670파일과 owner
1,695개는 원본과 일치했다. 이 수치는 원인 분리 결과이며 전체 Graph 개선율은 아니다.
생성기 반영 후 기존 네 compiler·세 모듈·독립 declaration, generic 호출·key 동등성,
private Link·등록 순서·깊은 resource·ID 없는 route 검사는 통과했다. 이후 사용자 결정으로
전체 Graph 성공 gate는 제외했으며, 대표·선택 SDK의 source/declaration/public/negative와
server 검증으로 최종 수용 범위를 고정한다.
