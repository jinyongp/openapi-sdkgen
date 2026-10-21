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

### Go package 스케줄 추가 진단

실제 native P 출력의 empty-cache `go build -p=1 ./api/...`는 3,600초 상한으로
종료됐다. peak session RSS는 9,507,672,064바이트로 해당 9,264 MiB 상한 안이었으며
원인은 시간 초과다. 이 표본은 실패로 유지한다. 관측한 model compiler 구간 중앙값은
약 8초이고 model implementation package는 392개다. 파일 묶기 뒤에도 전체 package
수는 588개여서 package 직렬화 비용을 별도로 조사한다.

다음 진단은 동일 Go 1.27.2·source·언어/최적화 옵션·CGO=0·GOMAXPROCS=4에서
compiler 스케줄만 바꾼다. 전체 3,600초 및 기존 동적 RSS 상한은 유지한다.

1. schema/model implementation 중 source 합계 32 MiB 이상인 package를 먼저
   `-p=1`, soft heap 6 GiB로 빌드한다.
2. 나머지 schema/model implementation 경로를 포함한 두 family 전체를 `-p=3`,
   compiler별 soft heap 2 GiB로 빌드한다. 직전 단계의 같은 cache만 사용한다.
3. `-p=1`, soft heap 6 GiB로 `./api/...` 전체를 빌드한다. public root·operations·
   server를 포함한 전체 빌드가 성공해야 진단 성공이다.

모든 단계는 하나의 owned session RSS 상한으로 보호한다. cold 표본은 첫 단계 전
새 empty GOCACHE에서 시작하며 단계 시간을 모두 합한다. 기존 실패 실행의 cache를
이어 쓰는 경우는 warm continuation 진단으로 표시하고 cold 표본에 넣지 않는다.
GOFLAGS·GOEXPERIMENT는 비우고 linux/amd64·GOAMD64=v1·GOGC=100을 명시한다.
직전 직렬 실행에서도 이 override가 없었음을 선택한 실제 process 환경에서 확인했다.
새 경로의 채택 여부와 최종 반복 조건은 실제 결과 뒤에 고정하며 source ownership,
defined PkgPath·공개 서명·runtime·전체 Graph 성공 기준은 그대로 적용한다.

### Native TS root profile 진단

public root project가 25분 넘게 약 8.9 GB RSS와 높은 CPU를 유지하며 diagnostics를
내지 않아, 비계측 대기를 중단하고 비용 함수를 계측한다. 중단은 agent의 진단 전환으로
별도 기록하고 signal/fail 원본을 보존한다. timeout·RSS 보호·semantic 오류로 바꾸지 않는다.

설치된 TS7 binary는 Go 1.26.4, source module `2bd066d87f5b+dirty` 빌드다. package와
binary hash를 유지하며 실제 `pprofDir` 지원 및 해당 commit의 CLI 취소/profile flush
경로를 검토했다. 이 upstream 원본과 packaged binary의 완전한 소스 동일성을 가정하지
않고, 생성된 profile을 `go tool pprof`로 실제 읽어 확인한다.

진단은 이전에 통과한 reference 출력을 쓰는 root project 하나, 새 outDir/build-info,
기존 strict·singleThreaded·soft heap 6 GiB, `gctrace=1`·`pprofDir`로 실행한다.
240초 또는 hard RSS보다 1 GiB 낮은 soft 지점에서 SIGINT를 보내고 30초 안에 profile
flush가 끝나지 않으면 종료하여 불완전 진단으로 표시한다. 외부 600초·기존 동적 RSS
보호도 유지한다. CPU와 allocs profile의 수집 성공은 전체 source·최종 consumer 성공이나
최종 timing 표본이 아니다. heap 조정·구조 변경은 실제 profile 뒤에 판단한다.

6 GiB 진단의 GC 원본은 약 7,871 MiB의 유지 heap과 약 2.5초짜리 반복 GC를 보였다.
soft limit보다 유지 heap이 큰 상태다. SIGINT 뒤 30초 안에 종료되지 않아 강제 종료했고
CPU/allocs profile이 flush되지 않았으므로 불완전 진단으로 남긴다. GC 원본 자체는 보존한다.
다음 단일 root 진단은 native TS의 soft heap만 9 GiB로 조정한다. hard RSS는 같은
동적 정책이며 최소 10 GiB 이상인 경우에 실행한다. 실제 child process의 GOMEMLIMIT·
GOMAXPROCS·GODEBUG를 선택적으로 대조하고 나머지 source/strict 옵션은 유지한다.
이 조건도 전체 fresh Graph 반복의 채택 조건으로 자동 간주하지 않는다.

9 GiB에서도 실제 유지 heap은 약 7,870 MiB, GC goal은 약 7,968 MiB여서 반복 GC가
이어졌다. child 환경의 9 GiB/4 CPU/gctrace 적용은 실제 process에서 일치했다.
240초 후 취소가 30초 안에 끝나지 않아 이 profile도 불완전하다. 다음 진단은 soft heap
10 GiB와 최소 11 GiB hard RSS headroom 조건, 480초 수집·외부 900초로 사전 고정한다.
hard RSS 상한 자체와 source/strict 강도는 같은 정책을 따른다.

10 GiB의 480초 CLI 수집도 취소 뒤 30초에 flush되지 않아 불완전으로 보존한다.
native local async API의 snapshot·파일별 diagnostics·독립 start/stop CPU profile·heap
profile을 작은 실제 fixture로 확인했다(2.46초, diagnostics 0, 두 profile 실제 판독).
이후 진단은 같은 config·실제 owner source 목록 일치를 확인하고 한 파일의 semantic 또는
declaration 비용을 최대 120초 수집한다. 검사가 진행 중이어도 별도 RPC로 profile을
종료하므로 CLI 취소 완료에 의존하지 않는다. 언어 서비스의 per-file 진단은 CLI 전체
source/emit/public consumer 검사나 최종 성능 표본으로 집계하지 않는다.

원본 config의 API snapshot이 reference project까지 여는 비용을 분리하기 위해, 통과한
dependency declaration 97,862개와 root owner 19의 원본 source 958개만 hardlink한 별도
진단 fixture를 사용한다. 원본 source·기존 reference 출력은 수정하지 않는다. reference
목록을 제거한 config와 각 source hash를 보존하고 API가 관측한 implementation source
집합이 958개와 정확히 같은지 검사한다. 첫 flat 진단의 soft heap은 6 GiB이며 기존 동적
hard RSS 정책을 유지한다. 이는 새 compiler 조건의 최종 timing 표본이 아니다. 읽을 수
있는 CPU/heap profile과 실제 관측 비용으로 다음 조사를 선택한다.

취소·flush 참고 원본:
[CLI](https://github.com/microsoft/typescript-go/blob/2bd066d87f5b/cmd/tsgo/main.go),
[execute](https://github.com/microsoft/typescript-go/blob/2bd066d87f5b/internal/execute/tsc.go),
[pprof](https://github.com/microsoft/typescript-go/blob/2bd066d87f5b/internal/pprof/pprof.go).

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
