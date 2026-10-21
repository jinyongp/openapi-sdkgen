# 실행·채택 정책

사용자 요청 범위: workstream `88a65ff3-848d-4fff-a307-325ab4cbb221`의 구현·검증·단위
리뷰/커밋과 최종 통합 리뷰까지 수행한다. 공개 API·정의 패키지·선택 경계·strict 강도는
유지한다. 유료 runner·호스트 설정·push·release는 포함하지 않는다.

## 기준선과 관측 경계

- correctness 수정 W01R 전의 제품 기준은 `5ada624`와 동일한 현재 제품 소스다.
  W01R 뒤에는 공통 correctness 기준을 새 commit으로 고정한다. 이후 A/B는 동일 기준에서
  필요한 최적화만 달라야 한다. 미완료 기준선을 성공 표본으로 집계하지 않는다.
- Graph TS의 공개 route/operation ID 전체 29,581개를 기존 생성물에서 독립 동결한다.
  `public-consumer.mjs`가 후보의 public root key 동등성과 unknown ID/route 거절을
  canonical strict 전체 source와 함께 검사한다. response/input·Link·named/selective
  의미는 기존 compiler consumer/runtime 회귀 및 해당 변경의 새 경계 사례로 검사한다.
- Go는 최종 package/type/method/tag/alias AST inventory를 먼저 동결한다. 파일 묶기 뒤에는
  파일 경로를 제외한 symbol→package/signature가 같아야 한다. 실제 compiler·reflection·
  오류/media/cancel·selected/named 동작은 기존 partition/consumer/runtime 검사로 확인한다.

## 자원·캐시 정책

- 기존 8 GiB/900초 trial은 그대로 보존한다. 다음 단계별 Graph Go 진단은 가용 메모리
  약 11 GiB를 확인한 뒤 session RSS 9 GiB·1800초, Go soft heap 8 GiB로 사전 고정한다.
  compile/prepare/입력 해제/emit/publish phase와 GC를 기록한다. CPU profile은 intrusive
  진단이므로 일반 timing 표본에 넣지 않는다. 자원 상한을 바꾼 결과끼리 개선율을 계산하지 않는다.
- 최종 full-scale TS/Go 검사 예산은 compiler별 3600초, session RSS
  `min(12 GiB, 실행 직전 가용 RAM - 2 GiB)`다. 호스트/cgroup 상한이 더 작으면 그것을
  적용한다. 예산 변경은 다음 trial 전에 기록한다. 보호 종료를 컴파일 성공으로 바꾸지 않는다.
- 무거운 trial은 직렬 실행한다. Go build는 `-p=1`, CGO=0, GOWORK=off, Go 1.27.2,
  GOMAXPROCS=4다. compiler soft heap은 6 GiB이며 생성 heap과 구분한다. TS7은
  singleThreaded다. Node heap은 해당 RSS 예산보다 최소 1 GiB 작게 고정한다.
- fresh compiler process/no incremental을 기본으로 한다. Go generated-package cold와
  완전 empty-cache cold, warm consumer는 별도 집계한다. 기존 cache를 지우지 않고
  trial 전용 GOCACHE로 경계를 고정한다. profile/diagnostic 표본은 대표 timing에서 제외한다.

현재 가용 RAM에 따른 RSS 상한은 6 GiB보다 작아질 수 있다. 기존 compiler soft heap
6 GiB와 이런 hard RSS 상한의 조합은 GC가 목표 heap에 도달하기 전에 보호 종료될 수 있다.
이를 구분하기 위한 추가 Go 진단은 compiler soft heap 3 GiB, 기존 언어/최적화 flag,
fresh GOCACHE와 같은 직전-RAM RSS 정책으로 고정한다. gctrace를 켠 진단은 timing
표본으로 쓰지 않는다. 필요하면 R도 같은 3 GiB profile로 검사하며 이전 6 GiB 결과와
성능 개선율을 계산하지 않는다. 채택 측정 조건은 이 진단 결과 뒤에 다시 고정한다.
같은 Go runtime을 사용하는 native TS compiler도 이 구분이 필요한 경우 실패한 owner
project를 3 GiB profile로 별도 진단한다. 이미 통과한 reference 출력을 쓰는 단일 project
진단은 전체 fresh Graph 검사 성공이나 최종 timing 표본으로 집계하지 않는다.

## 성능 채택 판단

- 최우선 gate는 전체 source/public consumer의 strict 및 기존 계약 보존이다.
  파일 수만 줄거나 합성 fixture만 빨라진 결과로 채택하지 않는다. planner 비용과
  helper/import/delta까지 포함한 net bytes, compiler wall/RSS, 공개 배포 bytes를 함께 본다.
- 작은/중간 입력은 교차 순서 5쌍, runtime 공유는 최소 10쌍으로 측정한다. 표본의 중앙값·
  범위와 paired ratio를 보존한다. full Graph는 기존 계획의 compiler별 3회/Go 5회 정책을
  유지하며 동일 예산에서 이미 실패한 기준선을 개선율 목적으로 반복하지 않는다.
- **알고리즘/runtime 공유 채택의 내부 기준**은 paired runtime ratio의 95% 구간 상한
  1.05 이하다. 이는 제품 SLA가 아니라 최적화 채택을 위한 보수적 기준이다. 불충분하면
  반복 비용을 확인하거나 공유를 보류한다. B/op·allocs/op·live heap 회귀도 별도 검토한다.
  기존 view sharing 유지·파일 배치만 변경한 경우 새 hot-path 호출 비용은 추가하지 않는다.
- 임의의 최소 15% 개선 gate는 두지 않는다. 재현 가능한 실제 이득이 측정 잡음보다 작으면
  복잡한 후보는 보류한다. section index와 validator 공유는 보류 결론도 허용하되 원인·
  실데이터·호환성 및 후속 영향을 남긴다. 최종 Graph 성공 기준은 그대로 유지한다.

## 리뷰와 완료

각 task에서 실제 결과·변경 caller·미검증 범위를 확인한 뒤 해당 작업을 커밋한다.
devtools의 현재 run/basis에 근거를 기록하고 done 처리한다. W11에서는 최종 commit의
전체 source/consumer/runtime/output/docs와 Graph 네 TS compiler·Go corpus/Graph를
통합 리뷰한다. 남은 필수 검증이 있으면 workstream을 close하지 않는다.
