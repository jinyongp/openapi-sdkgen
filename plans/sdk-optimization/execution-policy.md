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

첫 warm 스케줄 진단은 당시 가용 RAM에 따른 8,550 MiB hard 상한에서 model compiler
세 개가 동시에 약 2.7~2.9 GB RSS를 사용하며 보호 종료됐다. 다음 continuation은 동일
source·cache·스케줄·Go 옵션을 유지하되 최소 11 GiB hard budget이 확보될 때만 시작한다.
최대 12 GiB·가용 RAM−2 GiB 정책과 3,600초 상한은 유지한다. 20분 안에 시작 조건이
충족되지 않으면 child를 시작하지 않는다. 두 실행 모두 warm 진단이며 cold 표본이 아니다.

11 GiB 이상 budget의 후속 전체 빌드가 `internal/operations`에서 RSS 보호 종료되면,
같은 source·cache·Go 옵션으로 그 package 하나의 compiler 단계를 조사한다. compiler
RSS 10,500 MiB 또는 240초 지점에서 해당 owned-session compiler에만 SIGQUIT를 보내
goroutine stack을 수집한다. 전체 600초·최소 11 GiB hard budget을 적용하고 source hash를
대조한다. 의도한 진단 중단이며 전체 build 성공·RSS 실패·성능 표본으로 바꾸지 않는다.
frontend·inlining·SSA 중 어느 단계인지 확인한 후에만 compiler 스케줄이나 별도 구조
후보를 고른다. 이 수집을 위해 검사·언어·최적화 옵션을 바꾸지 않는다.

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

flat fixture의 factory·registry 파일별 semantic 진단이 각각 오류 없이 끝나면,
같은 API의 file 생략 기능으로 loaded program 전체의 semantic 비용을 최대 120초
계측한다. 원본 CLI의 emit·references 실행과는 구분하며 compiler 옵션·실제 owner
일치·모든 profile 원본을 기록한다. RPC가 진행 중이면 수집을 끝낸 뒤 그 사실을 표시한다.

실제 program profile이 GC 73.05%·유지 heap 7,740.97 MiB를 보이면 다음 조사를 나눈다.
우선 같은 flat fixture의 `internal/routes/helpers.ts` 한 파일을 6 GiB soft heap·120초로
계측해 alias가 있는 파일까지 비용을 좁힌다. 이 진단은 최소 8 GiB hard RSS budget을
확보한 경우에만 시작한다. 이후 같은 program의 10 GiB soft heap 진단은 최소 11 GiB
hard budget과 기존 2 GiB 가용 RAM 여유를 확보한 경우에만 시작하며 240초를 수집한다.
12 GiB hard 최대치·source/strict 옵션은 그대로다. CLI 실행이나 최종 반복 조건으로
자동 채택하지 않으며 가능한 실제 semantic 완료와 profile 수집 완료를 별도 기록한다.

heap 10 GiB에서도 GC 비중이 높고 semantic 호출이 240초에 끝나지 않으면 추가 heap-only
반복은 하지 않는다. helper 파일의 원본 복제와 nested `Routes` lookup만 `Readonly` view로
감싼 복제, request-section lookup만 감싼 복제, 둘을 감싼 복제를 비교한다. 원본 958개
source·기존 declaration은 수정하지 않고 source 하나만 더한 diagnostic config/owner를
별도로 만든다. 같은 strict·soft heap 6 GiB·120초·최소 8 GiB RSS 조건을 사용하며,
각 profile·owner·source hash·진단을 보존한다. generic 관계와 실제 Graph 효과가 모두
확인되기 전에는 제품에 적용하거나 최종 성능·전체 source 성공으로 취급하지 않는다.
네 복제의 비교는 같은 11 GiB RSS 상한을 확보한 경우에만 직렬 시작한다. 시작 전
20분 동안 조건이 충족되지 않으면 child를 시작하지 않는다. 이 상한은 기존 최대
12 GiB·가용 RAM−2 GiB 정책 안이며 공통 soft heap 6 GiB와 source/strict 강도는 같다.

네 view에서 semantic 완료가 입증되지 않으면 원본 control source를 그대로 두고 compiler
GC 동작을 한 쌍으로 조사한다. 두 실행은 soft heap 10 GiB·GOGC=100·GOMAXPROCS=4·
11 GiB hard·strict·같은 owner 959개·120초 계측을 고정한다. 첫 실행은 `gctrace=1`,
다음 실행은 `gctrace=1,gcstoptheworld=1`만 사용한다. 설치 native compiler의 Go 1.26.4가
이 process-local 설정을 지원함을 고정 upstream runtime 문서에서 확인했다. 실제 child의
선택한 GC 환경만 기록한다. CPU 표본에서 idle GC가 차지하는 비율 자체를 wall-time
원인으로 단정하지 않고, write barrier·union 계산과 완료 여부를 대조한다. 이 설정을
제품 runtime이나 최종 compiler 조건으로 자동 채택하지 않는다.

GC 대조에서도 semantic 완료가 없으면 C-index를 별도 진단 source에서 조사한다.
canonical 선언의 route/ID/section과 실제 resource-call 유무를 읽어 private filter 세 개만
사전 계산하고 원래 value projection·generic 식은 보존한다. 원본 owner 958개와 probe
하나의 hash·소유권을 확인한다. control과 같은 10 GiB soft heap·GOGC=100·GOMAXPROCS=4·
`gctrace=1`·11 GiB hard·120초 API 계측을 적용한다. 이 진단 source는 최종 SCC 계획이나
제품 출력이 아니다. 효과가 있더라도 네 compiler의 generic/union/never/resource/negative
호환과 실제 생성 비용·전체 source/consumer 검증 전에는 C를 채택하지 않는다.

후속 C 실행은 11 GiB hard+2 GiB 여유 조건을 확보하지 못해 측정 child를 시작하지
않았다. host 설정을 바꾸지 않고 control/C를 둘 다 새로 10 GiB hard 조건에서 수집한다.
soft heap 10 GiB와 나머지 환경·120초 구간은 동일하다. 12분 내 headroom이 없으면
시작하지 않는다. 이전 11 GiB 결과와 시간 개선율을 계산하지 않고, 새 한 쌍의 완료 여부와
profile을 대조한다. 보호 종료도 그대로 보존하며 최종 strict gate의 예산·강도는 유지한다.

E의 첫 전체 Graph 생성은 구조·정확성 pilot으로 사용한다. 그동안 최대 3 GiB의 작은 C
fixture 호환 검사도 실행할 수 있으므로 이 pilot 생성 시간은 최종 5개 성능 표본에서
제외한다. 큰 Graph compiler/profile·생성 trial끼리는 직렬로 유지하며 최종 성능 표본은
관련 검사를 겹쳐 실행하지 않는다. 원시 pilot 결과와 실제 동시 실행 조건을 보존한다.

E의 첫 staged cold compile도 후속 resource 타입의 작은 회귀 검사와 겹쳤다. 이 실행은
empty cache에서 시작한 전체 compile correctness pilot으로 보존하고 최종 5개 성능
표본에서는 제외한다. 최종 source·compiler 조건을 고정한 후 겹치는 작업 없이 새 표본을
수집한다. 이 구분으로 원래 반복 수나 전체 compile 요구를 줄이지 않는다.

첫 E cold 실행은 schema/model family를 통과한 뒤 operations compiler가 끝나기 전에
3,600초 상한에 도달했다. RSS 보호 종료는 아니었다. 다음 Go 진단은 그 cache를 명시적으로
재사용해 `./api/...` 전체를 `-p=1`, compiler soft heap 6 GiB로 다시 확인한다. 모든 package를
포함하며 cold 표본이 아니다. operations compiler에만 CPU/heap profile을 지정하고
`gctrace=1`을 기록해 다음 구조·스케줄 판단의 근거를 확보한다. 언어·최적화 옵션은 유지한다.
기존과 같은 11,832 MiB hard와 3,600초를 적용하며 20분 내 2 GiB headroom을 확보하지
못하면 시작하지 않는다. profile이 완료되지 않으면 읽을 수 있는 증거로 집계하지 않는다.

E warm profile 실행은 258.87초·RSS 12,416,368,640바이트에서 11,832 MiB hard
보호로 중단됐고 CPU/heap 파일은 0바이트라 판독 가능한 profile이 아니다. P 제공자
표본 수집 뒤 E의 같은 source/cache를 대상으로 앞서 정의한 단일 operations SIGQUIT
진단을 적용한다. profiling flag 없이 기존 언어/최적화 옵션·soft heap 6 GiB를 유지하고,
compiler 10,500 MiB 또는 240초에서 stack을 수집한다. 전체 600초·같은 11,832 MiB
hard와 2 GiB headroom을 적용한다. 의도한 stack 중단은 compile 성공이나 성능 표본이 아니다.
이 E 단계 진단에서는 `gctrace=1`도 남겨 compiler의 유지 heap과 GC를 stack 단계와
대조한다. Go driver가 compiler stderr를 보류할 수 있어 그동안 보이는 driver GC를
compiler GC로 해석하지 않는다. 이전 P 진단 producer는 원본 그대로 별도 보존한다.

E stack은 `types2.packageObjects`의 함수 선언·서명 검사였고 backend 이전이었다.
compiler GC 원문은 유지 heap 9,404 MB와 약 2초 mark 구간, heap goal이 이미 유지
heap보다 작은 상태를 보였다. 다음 Go 진단은 같은 source/cache의 전체 `./api/...`를
profiling compiler flag 없이 soft heap 10 GiB·gctrace로 확인한다. hard 11,832 MiB와
2 GiB headroom은 그대로며 시간은 진단용 900초다. TS owner API 대조 종료 뒤 직렬
실행한다. 성공하더라도 warm 진단이며, 최종 cold 5회·3,600초 조건을 대체하지 않는다.
이 900초 진단이 850초에도 같은 operations compiler에 있으면, 현재 native PID/package/
시작 identity를 다시 확인해 SIGQUIT로 늦은 단계를 수집한다. 원래 900초 상한을 늘리지
않으며 agent의 stack 수집 중단으로 분류한다. 늦은 단계와 실제 compiler GC를 보존해
다음 구조·스케줄 판단에 쓰고 전체 compile 성공이나 natural timeout으로 바꾸지 않는다.

늦은 stack 요청 뒤에도 wrapper가 끝나지 않아 recorder가 실제 900.33초 timeout을
기록했다. 요청 signal과 recorder outcome을 모두 보존한다. 본문 frontend까지 진행했으나
전체 성공은 아니다. 다음 W09 검증은 원래 전체 3,600초 예산에서 같은 SDK/cache와
soft heap 10 GiB를 사용하고 profiling flag·gctrace·의도한 signal 없이 자연 종료를
기다린다. hard 11,832 MiB와 시작 전 2 GiB headroom을 요구하며 20분 내 확보하지 못하면
child를 시작하지 않는다. 이 warm 검증은 최종 cold 5표본을 대신하지 않는다. owned
process의 VmSwap/VmSize도 1초 간격으로 보존해 RSS만으로 전체 메모리 비용을 추정하지
않는다. 과거 sampler producer는 별도 동결한다.

### Native TS owner 20의 추가 비용 분리

named resource 후보는 root owner 19를 오류 0개로 통과했다. 다음 owner 20은 233초
관측 시점에 singleThreaded·soft heap 6 GiB에서 RSS 약 9.1 GB와 누적 CPU 365%를
보였다. 실제 retained heap이나 semantic hotspot은 아직 확정하지 않는다. 이 실행을
agent의 진단 전환으로 중단하고 원래 결과·signal을 보존하며 timeout/RSS 실패로 바꾸지
않는다. 해당 native PID의 실행 identity를 대조하고 SIGINT, 30초 뒤 남아 있으면 SIGKILL을
그 프로세스에만 보낸다. wrapper 종료 후에만 후속 무거운 진단을 시작한다.

이미 통과한 owner 0–19의 declaration과 owner 20의 원래 source만 새 tree에 hardlink해
read-only fixture를 만든다. 원본 SDK·이전 선언은 수정하지 않는다. 두 API 진단은 같은
config/실제 owner/전체 loaded program의 semantic, hard 11 GiB·GOGC=100·GOMAXPROCS=4·
gctrace·120초 계측·외부 600초를 고정하고 soft heap만 6/10 GiB로 나눈다. 각 source hash와
실제 loaded source 집합을 대조하고 CPU/heap profile을 실제 판독한다. 단순 RSS 관찰이나
API 수집을 전체 strict 성공으로 삼지 않는다. 두 결과를 보고 다음 전체 검사 조건을 정한다.
두 대조는 실제 program 59,670개·owner source 1,695개가 같고 판독 가능한 profile을
남겼으나 semantic은 모두 120초 미완료였다. 10 GiB에서도 union/type 비교가 남아 있어
heap만 더 올리지 않는다. 다음에는 같은 fixture·soft 10 GiB·hard 11 GiB·같은 120초/
600초 조건으로 client factory, registry, route helper 세 파일의 semantic을 각각 계측한다.
새 Go warm 진단 종료 뒤 직렬 실행하며 파일별 결과로 다음 source 후보를 고른다.
세 파일은 각각 semantic 오류 0개로 약 11.78/7.32/8.61초의 계측 구간에 끝났다.
이 결과로 특정 source의 구조 변경을 채택하지 않는다. API의 checker pool·전체 파일
수집 경로는 CLI와 다를 수 있으므로 API 시간으로 CLI 완료를 예측하지 않는다.
다음 검증은 같은 SDK/strict 옵션·singleThreaded·새 전체 declaration/build-info에서
native soft heap 10 GiB를 사용한다. 기존 동적 최대 12 GiB hard와 전체 3,600초는
유지하고, 전체 source/공개 consumer까지 자연 종료를 기다린다. 과거 6 GiB의 agent
중단과 새 profile을 구분하며 같은 조건의 성능 비교로 계산하지 않는다.

실행 중인 정확한 native PID·start identity·config를 확인한 뒤 Linux user-space
software CPU clock으로 30초 표본을 수집했다. signal·ptrace·process stop이나 host 설정
변경 없이 기존 binary의 Go symbol로 판독한다. 이 계측이 포함된 전체 실행은 correctness
pilot으로만 보존하고 최종 대표 시간 표본에서는 제외한다. 자연 종료 조건은 유지한다.
수집 11,703개에서 lost/unknown은 0개였고 runtime 계열 flat 표본은 11,604개였다.
관측 전후 VmSwap은 0, major fault는 4로 같았다. 이 구간을 swap 병목으로 해석하지 않는다.
판독한 non-runtime 호출 경로는 mapped type 안의 type argument constraint·indexed
access union 관계 검사였다. 전체 native가 실패할 경우, 같은 read-only fixture의
`internal/client/types.ts`와 `registry-contracts.ts`만 기존 API 조건(soft 10 GiB,
hard 11 GiB+2 GiB headroom, 120초 계측/600초 외부)으로 직렬 계측한다. 앞서 확인한
세 파일과 달리 이 두 mapped-type 선언은 파일별로 확인하지 않았으므로 원인 확정 전
제품 변경을 고르지 않는다. 그 뒤 독립 Go 전체 warm 검증을 같은 고정 조건으로 실행한다.

### P 크기 비교의 실제 제공자 표본

추가 크기 비교는 고정 holdout의 작은 `tvmaze.com`과 중간 `listennotes.com` 입력을
사용한다. 원본 Git blob과 SHA-256을 확인하고 P 이전의 동결 generator로 전체 출력과
정렬 첫 route 하나를 제외한 출력을 만든다. 같은 원본을 정렬 manifest 순서로 각각
64/128/256 KiB로 replay한다. 이 순서는 native emitter 순서와 다르므로 생성 시간의
대체 표본이나 Graph의 native 파일 수로 일반화하지 않는다. E와 V는 포함하지 않는다.

원본·세 크기의 전체 package/정의 타입/공개 서명/import binding을 전수 대조하고
files/bytes와 route 추가·삭제의 manifest delta를 계산한다. 네 variant의 empty-cache
전체 build와 같은 cache의 warm build를 각 5회 수행한다. 반복마다 순서를 회전하고
홀수 회는 역순으로 하며, Go 1.27.2·CGO=0·GOMAXPROCS=4·`-p=1`·soft heap
6 GiB·기존 동적 hard RSS를 적용한다. 각 build의 상한은 600초다. 무거운 Graph
진단과 겹치지 않게 시작하며 실패 표본과 원시 출력도 보존한다. 한 route의 양방향
delta는 변경 폭의 표본이고 실제 publisher 복구·stale 제거 검사의 대체가 아니다.

full 대 explicit-all-route 대조군의 managed 파일 hash를 비교해 선택 모드가 delta에
영향을 주지 않았는지 확인한다. 대조군 생성은 첫 timing 표본 전에 끝냈다. warm build가
recorder의 200 ms 관측 간격보다 짧으면 그 외부 시간을 비교 지표로 삼지 않는다.
같은 5개 cache/variant를 새 Go process에서 다시 확인하되 wrapper 내부 monotonic
시간을 보존한다. 기존 RSS 보호·환경·순서는 유지하며 다른 무거운 검사와 직렬 실행한다.

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
