# Emission 안에서 템플릿 파싱 공유

상태: 구현·관련 검사·실측 검산·실제 CLI 통합·최종 리뷰 완료. 채택한다.
기준은 `decb75b`이며 runtime은 `5ddb3a3`과 같다.

직전 최신 profile에서 파일마다 같은 embedded template를 읽고 파싱했다.
파싱의 sampled 누적 비용은 CPU 1.28초·할당 약 1.19 GiB, 읽기의 할당은 약
122 MiB였다. immutable source의 파싱을 emission 안에서 공유하는 것을 조사한다.
최종 Go 정규화는 그대로 유지한다.

cache는 `EmitTo` 한 호출에 속하고 source template 경로로만 식별한다. parsed
template와 named definition은 수정하지 않는다. rendered 결과·filePlan·파일 이름을
키/값으로 보관하지 않고 전역 cache도 만들지 않는다. 같은 plan의 동시 emission은
각자 cache를 가진다. 실패/성공 반환과 함께 소유권을 끝낸다.

내부 template의 이름은 generated artifact 대신 source template 경로로 명명한다.
실행 오류의 바깥 `render OUTPUT` 위치와 원래 오류 원인은 유지하고, 내부 위치는
정확한 source template·line을 가리킨다. 다른 파일의 최초 cache 항목 이름이
나오면 실패다. 표준 template의 비공개 tree를 수정하거나 실패한 실행을 재시도해
오류를 재작성하지 않는다. source 읽기 실패의 기존 정보도 유지한다.

검증은 기존 준비/출력 독립성·소비자·sink 실패·게시 rollback, 같은 template를
쓰는 서로 다른 파일/plan의 동시 실행과 오류 위치를 확인한다. 그 뒤 동일 observer의
GitHub 예비 한 쌍이 전체 byte/hash를 통과하면 최대 3쌍과 작은 두 입력을 비교한다.
실제 CLI도 기존 후보 inventory에 맞아야 한다. cache 4 GiB·scratch 2 GiB,
GOMAXPROCS=4·GOMEMLIMIT=2GiB·session RSS 3 GiB·개별 180초 보호를 유지한다.

## 실제 입력 비교

최종 후보와 직전 확정 제품을 같은 observer·빌드 옵션으로 비교했다. 고정 GitHub
3쌍을 기준→후보/후보→기준/기준→후보 순서로 실행했다. TVMaze·ListenNotes는 각
1쌍의 정확성 검사다. server addon·import root `example.test/graph/api`·Go 1.27.2·
GOMAXPROCS=4·GOMEMLIMIT=2GiB가 같고 CPU/heap profiling은 켜지 않았다.
새 프로세스/출력을 사용하고 각 실행의 검산 후 SDK를 제거했다. 별도 Go cache를
복제하거나 host cache를 비우지 않았다.

GitHub 각 3회 중앙값:

| 지표 | 기준 | 후보 | 변화 |
| --- | ---: | ---: | ---: |
| observer 실행 시간 | 40.8433초 | 39.2113초 | −4.00% |
| user+system CPU | 46.2820초 | 44.4578초 | −3.94% |
| 누적 할당 | 22,005,869,776 B | 20,565,052,032 B | −6.55% |
| 최대 개별 프로세스 RSS | 604.70 MiB | 612.66 MiB | +1.32% |
| 출력 | 6,377개·204,611,637 B | 동일 | 동일 |

시간 범위는 기준 40.5583–42.1883초, 후보 38.5720–39.8218초다. 대응 비율
0.96004/0.91428/0.98184는 세 쌍 모두 1보다 작다. 기하평균은 0.95164이며
3^3 재표집의 선형 보간 percentile 95% 구간은 [0.92404, 0.97708]다.
작은 로컬 표본의 조건부 재표집 결과이며 모든 환경의 개선율로 일반화하지 않는다.
시간에는 observer의 입력 해제 GC·hash·게시가 포함돼 순수 CLI benchmark와 다르다.

누적 할당 약 1.44 GB 절감은 동시에 차지하는 메모리의 절감이 아니다. RSS 범위는
기준 601.83–609.69 MiB, 후보 602.98–616.20 MiB였다. 범위는 겹치지만 세 쌍 모두
후보가 높았으며 peak RSS 절감으로 표현하지 않는다. GC 설정을 수치에 맞춰 바꾸지
않았다. bounded emission cache가 줄이는 반복 CPU/할당과 소폭의 RSS 증가를 함께
기록하고 채택한다. 이전 단위의 개선율과 더해 누적 개선율을 만들지 않는다.

TVMaze는 양쪽 143개·3,172,460 B, ListenNotes는 201개·7,546,903 B였다.
각 입력의 모든 경로·byte·hash가 같았다. 작은 입력의 시간은 각 1쌍이므로 개선율을
주장하지 않는다. 생성 SDK의 입력 파일이 같으며 Go cold/warm 컴파일 비용은 새로
측정하지 않았다. TypeScript·전체 Graph/Cloudflare·전체 corpus/compiler 매트릭스도
이번 검증에 포함하지 않는다. 로컬 수치를 공개 제품 문서에 게시하지 않는다.

## 리뷰와 검사

| 영역 | 완료 근거 |
| --- | --- |
| cache 소유권·상한 | 성공적으로 읽은 embedded template 경로별 parsed template만 보관한다. key 수는 사용하는 embedded source 수에 묶이며 generated 파일 수만큼 늘지 않는다. emission 밖으로 노출하거나 global state를 만들지 않는다. |
| 불변·definition | parser 결과와 named definition을 수정하지 않는다. 파일마다 새 실행 데이터와 buffer를 쓰며 `parameterContract` 등 기존 named template도 유지한다. |
| 데이터·동시 실행 | 같은 plan의 동시 emission과 서로 다른 plan/파일의 동시 emission, 반복 실행에서 각자 고유한 값을 출력했다. 최종 상태의 부모 generator 자체 `-race` 검사도 통과했다. |
| 진단·실패 | 재사용 후 다른 파일이 실패해도 현재 출력 파일·source template·line을 표시한다. missing source 읽기 오류는 기존 경로를 유지한다. 실행 오류의 wrapping과 원인은 보존한다. template의 내부 tree 수정·재실행·오류 문자열 치환을 하지 않는다. |
| 소비자·소유권 | 준비 결과 독립성/결정성·sink failure·formatting rollback·owned runtime byte ownership·health consumer·분할/packing 검사를 최종 상태에서 통과했다. 생성 소비자 race 검사를 포함한다. |
| 실측·출력 | 고유 표본 10개의 입력/바이너리/producer/log/source hash·checkpoint·전체 inventory를 검산했다. 각 입력의 모든 기준/후보 출력이 같다. runtime 변경은 `emit.go` 한 파일이며 측정 후 바뀌지 않았다. |
| 실제 CLI 통합 | 최종 CLI로 세 실제 API를 같은 server/import root 조건에서 각각 한 번 생성했다. 전체 관리 파일 집합과 파일별 hash가 observer 후보와 일치했다. 정확성 확인이며 새 속도 표본으로 집계하지 않는다. |

## 자원과 보존 정책

최종 전용 Go cache는 약 3.0 GiB로 4 GiB 상한 안이며 Go 임시 경로는 비어 있다.
race 검사에도 같은 cache를 사용했다. 기준 배포 CLI와 현재/직전 제품의 CLI·observer
5개만 남기고 task-owned 중간 실행 파일 17개·608,052,195 B를 제거했다.
각 경로·크기·SHA-256을 `binary-retention.json`에 보존했다. raw 측정·source·입력
hash·빌드 정보는 보존하며 source에서 다시 빌드할 수 있다. 남긴 실행 파일은
187,781,042 B, 정리 뒤 cache 외 scratch는 약 290 MiB다. 생성 SDK는 모두 제거했다.
이후 실험도 배포 기준/현재/직전 제품만 유지해 실행 파일이 단위마다 누적되지 않게 한다.

[원시 근거](measurements/06-emission-templates.tar.gz)는 1,471,511 B,
근거 항목 229개와 색인 1개다. SHA-256:
`707615152ce76fe95d87a648ddb4ed4f8c9bfefa123de3fd397a2b8f3d632de3`.
모든 항목을 다시 읽어 크기·hash를 확인했다. 원인 profile/attribution·빌드 출처·
실측·전체 inventory·최종 검사·CLI 확인·정리 목록과 생산 스크립트를 보존했다.
실행 파일·SDK 전체 출력·Go cache는 archive에 넣지 않았다.
