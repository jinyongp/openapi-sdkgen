# Go 재작성의 임시 저장소 수명

상태: emission 소유 scratch 후보의 구현·실측·최종 소비자/CLI/race 검사·리뷰 완료.
scratch 후보를 채택한다. 한 AST의 binding 통합 후보는 주석 이동으로 기각했다.
기준은 `741ebb3`다. 현재 Go 생성에서 global symbol 참조는
같은 group·같은 이름이어도 `ast.NewIdent`로 대체된다. 이에 따른 노드 생성·교체와
callback 비용을 현재 CPU/heap profile로 확인한다. 이전 타입 조회 최적화의 이득을
이번 후보의 근거로 재사용하지 않는다.

현재 제품으로 고정 GitHub Go+server profile 1회를 수집하고 11 단위의 전체 출력
hash와 대조한다. profiling은 benchmark 표본에 넣지 않는다. 실제 변화가 없는
참조와 이름/import가 바뀌는 참조를 구분할 수 있는지, 필요한 내부 작업이 identifier
재작성인지 일반 AST 변환인지 호출자와 오류·scope 계약을 확인해 결정한다.

원본 노드 재사용은 Obj/위치/주석 소유권에 영향을 줄 수 있다. source 위치 초기화,
주석/파일별 예외, 문자열 치환으로 byte 차이를 맞추지 않는다. 최종 정규화는 유지한다.
관련 검사 후 예비 GitHub 한 쌍의 모든 출력이 같을 때만 최대 3쌍과
TVMaze/ListenNotes 각 1쌍, 최종 소비자·ownership/race·실제 CLI로 진행한다.

cache 4 GiB·scratch 2 GiB, 전용 Go cache 하나, GOMAXPROCS=4·GOMEMLIMIT=2GiB,
개별 180초·RSS 보호 3 GiB를 유지한다. 전체 Graph/Cloudflare·전체 compiler matrix는 제외한다.

## 조사 후 후보 선택

현재 profile에서 같은 group의 `ast.NewIdent` 할당은 약 16.5 MiB, 전체 qualification은
약 89.5 MiB였다. 이름이 실제로 바뀌는 경우도 포함하므로 모두 불필요한 할당이 아니다.
재귀 walker의 list cumulative 값은 같은 callback stack과 겹치므로 합산하지 않는다.
원본 identifier 재사용은 위치/Obj 정보를 바꿀 위험에 비해 비용 기여가 작아 우선하지 않는다.

반면 import 바인딩은 sampled CPU 10.07초·누적 할당 약 4.30 GiB였다. 이 중 파싱이
1.91초·1.40 GiB, 재작성 후 포맷이 7.60초·2.79 GiB였다. 현재 emission은 schema→models를
같은 파일에 따로 적용해 두 import를 함께 쓰는 파일을 다시 파싱·정규화한다.

후보는 ordered binding source를 한 AST에 적용하고 최종 직렬화·정규화는 한 번만 한다.
각 source 적용 뒤 unused import를 제거하고 import metadata를 갱신해 다음 alias의
사용 이름 목록을 기존 순서와 같게 유지한다. 한 source 경로도 같은 helper를 사용한다.
새 alias counter는 source마다 초기화하며 local shadow와 기존 오류/소유권을 유지한다.
주석 위치와 중간 정규화 제거의 영향을 sequential oracle와 전체 byte/hash로 검증하고,
차이가 나면 제공자/주석 예외를 추가하지 않고 후보를 보완하거나 기각한다.
private binding helper는 중간 소스이므로 sequential oracle에서 code token·주석 순서·
alias/scope 의미와 정규 Go 형식을 확인한다. 공개되는 최종 managed 파일은 전체
경로·byte·hash를 그대로 맞춰야 한다. 중간 소스의 공백만을 제품 계약으로 확대하지 않는다.

## 바인딩 후보 기각과 임시 저장소 후보

두 source 통합 후보는 순차 oracle에서 import 주석의 token 순서를 바꿨다.
validator/formatter 주석이 다른 import 앞으로 이동하는 작은 재현을 보존하고,
원래 세 runtime/test 파일을 기준과 같은 byte로 복구했다. 주석 예외나 위치 조작을
추가하지 않는다. 이 후보는 실제 API 성능 비교 전에 정확성 검사에서 기각했다.

다음 후보는 AST/정규화 순서를 그대로 두고 직렬화 buffer의 소유권·수명만 분리한다.
emission마다 formatter 하나를 만들고 binding·facade·implementation이 같은 scratch를
직렬 재사용한다. AST·정규화 결과·artifact bytes는 보관하지 않는다. retained capacity는
1 MiB로 제한하며 큰 파일도 정상 처리한 뒤 큰 scratch를 바로 버린다.
같은 plan의 동시 emission은 서로 다른 formatter를 사용한다.

현재 지원 Go 1.27.2의 full-source `format.Source`는 별도 output buffer를 반환한다.
결과가 scratch 재사용·caller의 artifact 수정과 독립적인지 실제 formatter/consumer
검사로 확인한다. 최종 포맷·주석·import 단계는 바꾸지 않고 전체 byte/hash를 맞춘다.

## 실제 입력 비교

현재 제품과 후보를 같은 observer·Go 1.27.2·server addon·import root
`example.test/graph/api`·GOMAXPROCS=4·GOMEMLIMIT=2GiB로 비교했다. GitHub 3쌍은
기준→후보/후보→기준/기준→후보 순서다. CPU/heap profiling은 끄고 새 출력으로
생성한 뒤 전체 관리 파일을 검산해 SDK tree를 제거했다. 각 생성은 180초·RSS 3 GiB,
pair/batch controller는 각각 480초/900초 보호를 유지했다.

GitHub 각 3회 중앙값:

| 지표 | 기준 | 후보 | 변화 |
| --- | ---: | ---: | ---: |
| 누적 할당 | 19,507,482,312 B | 18,619,785,008 B | −4.55% |
| observer 실행 시간 | 37.7703초 | 37.6673초 | −0.27% |
| user+system CPU | 42.8520초 | 42.2990초 | −1.29% |
| 최대 개별 process RSS | 636,104,704 B | 640,229,376 B | +0.65% |
| 출력 | 6,377개·204,611,637 B | 동일 | 모든 경로·byte·hash 일치 |

세 쌍 모두 할당·시간·CPU가 줄었다. 시간 비율은 0.96042/0.99801/0.98605,
기하평균 0.98137, 3^3 재표집의 선형 보간 percentile 95% 구간은
[0.96593, 0.99541]이다. 시간 범위는 기준 36.2785–39.2195초, 후보
35.7723–37.6950초로 겹친다. 작은 로컬 표본의 조건부 재표집 결과이며 모든
환경의 개선율로 일반화하지 않는다. 시간에는 observer의 입력 해제 GC·hash·게시가
포함돼 순수 CLI benchmark와도 다르다.

가장 명확한 이득은 약 0.89 GB의 반복 할당 제거다. 작은 시간 중앙값 차이로 큰
속도 향상을 주장하지 않는다. RSS는 두 쌍에서 높고 한 쌍에서 낮았다. 1 MiB scratch
보관 상한과 process RSS는 다른 값이며, 누적 할당 감소를 RAM 절감으로 계산하지
않는다. GC/동시성 설정은 바꾸지 않고 bounded scratch 수명과 할당/CPU의 이득으로
채택한다. 기존 단위의 개선율과 더해 누적 개선율을 만들지 않는다.

TVMaze와 ListenNotes는 각 1쌍을 실행했다. 각각 양쪽 143개·3,172,460 B와
201개·7,546,903 B로 전체 파일이 같다. 작은 입력의 시간은 개선율로 쓰지 않는다.
SDK 입력 전체가 같으며 새 cold/warm compile 비용 비교는 실행하지 않았다.
TypeScript·전체 corpus·전체 Graph/Cloudflare·compiler/platform matrix도 이번 범위에
넣지 않았다. 로컬 수치를 공개 제품 문서에 게시하지 않는다.

## 검증과 최종 리뷰

| 영역 | 완료 근거 |
| --- | --- |
| 소유권·범위 | formatter 하나가 EmitTo 호출에 속하며 binding·facade·implementation에 같은 pointer를 전달한다. AST·정규화 결과·artifact bytes를 cache에 저장하지 않고 global pool도 없다. |
| 포맷 계약 | schema→models의 기존 순서, 파싱·import pruning·printer mode·최종 `format.Source`를 유지한다. 처음의 binding 통합 source/실패 재현은 별도로 보존했고 그 변경은 최종 runtime에 없다. |
| 반환된 byte | 다른 AST를 포맷해 scratch를 재사용해도 이전 반환 값이 같다. 이전 artifact를 수정해도 다음 포맷은 독립적이다. 여러 독립 formatter의 병렬 사례를 race 검사로 확인했다. |
| 용량·오류 | 1 MiB를 넘는 실제 source도 정상 포맷한다. 작업 후 큰 capacity는 0이 되고, 잘못된 package 이름으로 포맷이 실패해도 큰 capacity가 남지 않는다. 이는 보관 용량이며 큰 입력 처리 중의 순간 메모리 상한은 아니다. |
| 검사 실행 확인 | 추가 검사가 처음 fixture 문자열에 삽입된 위치를 실행 전에 고쳤다. 실제 Go test event에서 독립성 세 하위 사례와 큰 buffer 정상/오류 검사가 실행·통과했음을 확인했다. |
| 소비자·게시 | 최종 top-level 28개 검사 통과. 기존 partition/packing·주석·정규화·local binding·AST 소유권·준비 결과/동시 emission 독립성·sink 실패·formatting rollback·owned runtime byte·health consumer를 포함한다. 생성 소비자 race를 위해 CGO를 명시적으로 켰다. |
| 원시 실측 | 고유 실제 입력 표본 10개의 producer/input/binary/source/log hash·checkpoint·전체 inventory를 검산했다. runtime 차이는 `emit.go`와 `partition.go` 두 파일이며 측정 후 바뀌지 않았다. |
| 실제 CLI | 최종 CLI로 세 실제 API를 각 새 경로에 생성했다. 모든 관리 파일 hash가 후보 observer와 같았고 tree를 제거했다. 속도 표본으로 추가 집계하지 않는다. |

표준 full-source formatter의 현재 지원 동작과 새 독립성 검사를 함께 확인했다.
수명 분리를 위해 필요한 private 호출자/fixture만 수정했고 공개 API나 생성 코드
구조는 바꾸지 않았다. 주석 문제를 파일별 fallback으로 숨기거나 최종 형식을 약화하지 않았다.

검사 후 전용 Go cache 약 3.3 GiB, cache 외 scratch 약 365 MiB, 빈 Go 임시 경로를
확인했다. 오래된 CLI/observer 2개·74,315,569 B를 제거하고 배포 기준·현재/직전 제품
5개·187,801,950 B만 남겼다. 제거 경로·크기·hash·build 출처는 보존했으며 다른 작업의
cache는 건드리지 않았다.

[원시 근거](measurements/12-identifier-rewrites.tar.gz)는 2,778,434 B,
근거 항목 247개와 색인 1개다. SHA-256:
`d77db79d2abbf49c68143908b5b34990db74517bb893a3221a017717e04853f2`.
모든 항목의 크기·hash를 다시 확인했다. 현재 profile·attribution·고정 입력·기각 binding
source/실패 검사·최종 scratch source/build·10개 표본·전체 inventory·최종 검사·CLI·
정리 목록과 producer를 포함한다. 실행 파일·SDK tree·Go cache는 제외했다.
