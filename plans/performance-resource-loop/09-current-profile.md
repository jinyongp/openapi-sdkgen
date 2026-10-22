# Facade AST의 소유권과 중복 파싱

상태: 조사·구현·실측·소비자/CLI 검증·최종 리뷰 완료. facade view 후보를 채택한다.
기준 제품 runtime은 `b97d32f`이며 작업 시작 revision은 `9641f3f`다.
템플릿 파싱 공유 이후 비용 순위를 새로 확인한다. 기존 profile의 attribution을
현재 제품의 성능 개선 근거로 재사용하지 않는다.

고정 GitHub Go+server를 현재 observer로 한 번 실행해 CPU·heap/allocation profile과
phase checkpoint를 수집한다. 모든 출력 파일은 06 단위 최종 제품 inventory와 대조하고
검산 후 SDK tree를 제거한다. profiling 실행은 비교 benchmark 표본에 넣지 않는다.
그 후 상위 비용과 실제 호출 구조를 조사해 다음 후보의 소유권·출력 계약을 결정한다.
구현은 원인과 호환성을 확인한 후보만 별도 단위로 진행한다.

조사 예산은 profile 1회와 bounded attribution/코드 조사다. 기존 전용 Go cache만
사용하고 별도 cache를 만들지 않는다. GOMAXPROCS=4·GOMEMLIMIT=2GiB,
RSS 보호 3 GiB·개별 180초, cache 4 GiB·scratch 2 GiB 상한을 유지한다.
전체 Graph/Cloudflare·SDK cold compile·전체 compiler matrix를 실행하지 않는다.

## 원인과 후보 결정

현재 profile에서 `bindPartitionImports`에 sampled CPU 9.58초와 누적 할당
4,422.17 MiB가 귀속됐다. 그중 파싱은 1.75초, 재작성 후 포맷은 7.22초였다.
renderer는 먼저 `format.Source`로 정규화한 뒤 sink가 같은 파일을 다시 파싱해
import를 연결하고 정규화한다. 여러 binding pass의 순서는 schema→models이며,
각 pass 사이의 정규화와 이름 선택을 바꾸지 않는 것이 정확성 경계다.

후보는 per-artifact 정규화 책임을 emission 경계로 옮긴다. 렌더링한 원문을 기존
첫 binding pass가 직접 파싱·재작성·정규화하고, 실제 import가 없어 그 pass가
정규화하지 않았다면 마지막에 기존 `format.Source`를 실행한다. 준비된 runtime
source는 이미 정규화된 소유 byte를 복사해 사용한다. binding pass의 순서,
scope/object 해석, import alias 선택, 최종 표준 정규화는 그대로 유지한다.

모든 binding을 한 AST에 합치는 대안은 앞 pass에서 제거된 import와 새 alias가
다음 이름 선택에 영향을 주므로 이번 후보로 선택하지 않는다. 최종 AST 정규화를
없애는 대안은 05 단위에서 이미 기각했다. 문자열 치환·제공자 예외·전역 cache도
추가하지 않는다. 소스의 canonical 여부는 한 artifact를 처리하는 동안만 추적한다.

관련 검사 후 GitHub 예비 한 쌍의 모든 byte/hash가 같으면 최대 3쌍과 두 작은 실제
입력으로 비교한다. raw source 주석·local shadow·source 문자열만 있는 경우·binding
없는 파일·잘못된 렌더링·반복 emission/동시 emission도 확인한다. 최종 제품 CLI와
observer 출력이 같아야 채택한다. 기존 자원 상한과 실패 표본 보존을 유지한다.

## 첫 후보 기각과 소유권 후보

raw source를 먼저 binding하는 후보는 import 주석이 있는 작은 differential 검사에서
정규화→binding 경로와 빈 줄 배치가 달랐다. 격리된 재현에서도 실패했다. 최초 관련
검사에는 CGO off 상태로 실행된 consumer `-race` 설정 오류도 있었으며 이 오류를
제품 결함으로 계산하지 않는다. 후보 source와 두 실패 기록을 보존하고 runtime은
기준과 완전히 같게 복구했다. 주석/템플릿 예외나 공백 치환을 추가하지 않는다.

별도 attribution에서 facade의 원본 재파싱에 sampled CPU 1.14초·누적 할당 약
1 GiB가 귀속됐다. `emitFile`이 이미 소유한 같은 원본 AST와 FileSet이 있는데도,
독립 수정 때문에 전체 구현 body까지 다시 파싱하는 비용이다.

다음 후보는 원본 AST를 재사용하는 짧은 수명의 facade view다. 수정하는 File,
import GenDecl, public function/signature/parameter field, type alias spec만 복사한다.
함수 body는 새 forwarding body로 대체하고 원본 body는 복사하지 않는다. 주석과
변경하지 않는 signature node는 같은 원본 FileSet 아래 읽기 전용으로 사용한다.
facade byte 출력을 끝낸 뒤에만 구현 AST를 재작성한다. 전역 cache·AST 장기 보관·
직렬화 재파싱·다른 FileSet의 위치 재해석을 추가하지 않는다.

원본 AST 무변경, 반복 출력, unnamed/blank/variadic parameter, import·주석·alias·
mutable variable 실패 계약을 확인한다. 같은 observer와 전체 파일 hash 비교,
최대 3쌍·실제 CLI·관련 소비자/race 검증을 통과할 때만 채택한다.

## 비교 결과와 채택 범위

기준과 후보를 같은 observer·Go 1.27.2·server addon·import root
`example.test/graph/api`·GOMAXPROCS=4·GOMEMLIMIT=2GiB로 비교했다.
profiling은 끄고 GitHub 3쌍을 기준→후보/후보→기준/기준→후보 순으로 실행했다.
TVMaze와 ListenNotes는 각 1쌍의 정확성 확인이다. 개별 180초·RSS 3 GiB 보호를
유지하고, pair/batch를 감싸는 controller에는 각각 480초/900초 deadline을 사용했다.
각 실행은 새 출력 경로를 쓰며 전체 파일 검산 후 SDK tree를 제거했다.

GitHub 각 3회 중앙값:

| 지표 | 기준 | 후보 | 판단 |
| --- | ---: | ---: | --- |
| 누적 할당 | 20,565,495,648 B | 19,507,286,648 B | 5.15% 감소 |
| observer 실행 시간 | 39.1786초 | 38.9640초 | 중앙값 −0.55%, 일관된 이득 미입증 |
| user+system CPU | 44.4049초 | 44.0829초 | 중앙값 −0.73%, 이득 단정 안 함 |
| 최대 개별 process RSS | 640,819,200 B | 636,657,664 B | 중앙값 −0.65%, 쌍별 방향 불일치 |
| 출력 | 6,377개·204,611,637 B | 동일 | 전체 경로·byte·hash 일치 |

기준 시간 범위는 38.5634–40.9574초, 후보는 38.7254–39.2898초로 겹친다.
대응 시간 비율은 0.98843/1.01039/0.95928, 기하평균 0.98581이다.
3^3 재표집의 선형 보간 percentile 95% 구간 [0.96554, 1.00560]은 1을 포함한다.
작은 로컬 표본의 조건부 진단이며 제품 전체 속도 개선율로 일반화하지 않는다.
RSS도 두 쌍은 후보가 더 높았다. 누적 할당 감소를 동시 메모리·peak RSS 감소로
해석하지 않는다. 이 후보는 독립 AST 재파싱과 약 1.06 GB의 반복 할당을 제거하는
이득으로 채택하며, 생성 시간·최대 메모리 절감의 확정 근거로 쓰지 않는다.

TVMaze는 양쪽 143개·3,172,460 B, ListenNotes는 201개·7,546,903 B가 같았다.
작은 입력 각 1쌍의 시간으로 개선율을 주장하지 않는다. 생성 SDK의 전체 파일이
같고 새 cold/warm compile 비용 비교는 실행하지 않았다. TypeScript·전체 corpus·
전체 Graph/Cloudflare·compiler/platform matrix도 이번 검증 범위에 넣지 않았다.
로컬 수치는 공개 제품 문서에 게시하지 않는다.

## 검증과 최종 리뷰

| 영역 | 완료 근거 |
| --- | --- |
| AST 수명 | 구현용 parser AST와 같은 FileSet으로 facade를 먼저 byte 출력한다. 그 뒤 구현을 재작성하며 facade AST가 호출 밖으로 나가거나 plan/global에 저장되지 않는다. |
| 변경 경계 | File·import GenDecl·function/type/parameter field·blank parameter identifier·type alias spec 중 수정하는 노드만 복사한다. 읽기 전용 signature/doc와 위치를 유지하고 버릴 body는 복사·재파싱하지 않는다. |
| 원본 격리 | 원본 syntax/object link snapshot과 scope map을 별도로 대조했다. 정상·반복 호출·mutable public variable 오류 후에도 원본이 같았다. unnamed/blank/variadic forwarding, alias·주석·불필요 import 제거도 확인했다. |
| 검사 자체의 재현 | 첫 snapshot 검사는 `ast.Fprint`의 map key 순서 때문에 실패했다. 표준 구현의 `MapKeys()` 순회를 확인하고 scope map을 별도로 비교하도록 고쳤다. 이때 runtime은 바뀌지 않았다. 최초 fixture 원문과 hash·실패 기록도 보존한다. |
| 소비자·게시 | 최종 상태의 관련 top-level 23개 검사가 통과했다. 기존 분할/packing·local binding·문서/정규화·준비 결과 독립성·sink 실패·formatting rollback·owned runtime byte·health consumer를 포함한다. 생성 소비자의 `-race`에 필요한 CGO를 명시적으로 켰다. |
| 동시 사용 | 원본 view·동일 plan 반복 emission·동시 template emission을 부모 generator `-race`로 확인했다. 각 emission이 소유한 AST만 사용한다. |
| 전체 출력·출처 | 10개 고유 표본의 입력/바이너리/producer/log/source hash·checkpoint·전체 inventory를 검산했다. runtime 차이는 `partition.go` 한 파일이며 observer 코드는 같다. |
| 실제 CLI | 최종 CLI로 세 실제 API를 각각 새 경로에 생성하고 모든 관리 파일 hash를 후보 observer와 대조했다. 모두 같고 검산 후 SDK tree를 제거했다. 속도 표본으로 추가 집계하지 않는다. |

초기 CPU/heap profile도 06 단위와 전체 출력이 같았다. intrusive 실행은
38.290초·20,567,376,360 B 누적 할당이었다. 게시 후 GC의 잔존 heap 약 3.86 MiB는
관측 시점이 달라 peak RSS 원인이나 현재 후보 성능의 전후 표본으로 쓰지 않는다.
raw-source 후보는 실제 출력 계약 때문에 기각했고 facade view는 소유권 경계에서
원인을 줄였다. 출력 차이를 예외 처리하거나 GC/동시성 설정을 수치에 맞춰 바꾸지 않았다.

검사 후 기존 전용 Go cache 약 3.1 GiB와 빈 Go 임시 경로를 확인했다.
배포 기준 CLI·현재/직전 제품만 남겨 오래된 CLI/observer 2개·74,309,201 B를 제거했다.
남긴 실행 파일은 5개·187,787,410 B, cache 외 scratch 약 316 MiB다.
정리 경로·크기·hash와 재빌드 출처를 보존했으며 다른 작업의 cache는 건드리지 않았다.

[원시 근거](measurements/09-current-profile.tar.gz)는 2,587,835 B,
근거 항목 257개와 색인 1개다. SHA-256:
`63e248290ff59b675c5cadfb5a3cf16e22da7f0b4774bcec4a504272e8d5adc8`.
모든 항목의 크기·hash를 다시 읽어 검증했다. 최신 profile·attribution·고정 실제 입력·
기각 후보/최초 snapshot fixture·실패 기록·10개 실측·전체 파일 inventory·최종 검사·
CLI 확인·정리 목록과 producer를 포함한다. 실행 파일·SDK tree·Go cache는 제외했다.
