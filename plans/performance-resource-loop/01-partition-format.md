# Go 분할 출력의 중복 포맷 제거

상태: 구현·실제 API 교차 측정·관련 검사·최종 리뷰 완료. 채택한다.

## 원인과 결정

기준은 `v1.4.0`의 `6231d70af9d162ac7fd50099c028ed9860f36c0c`다.
고정 GitHub 원본 전체를 server addon과 함께 생성한 CPU 프로파일에서 총 표본은
65.92 CPU초, 프로파일을 포함한 실행은 59.02초였다. `formatPartitionFile`의
누적 비용은 27.72 CPU초(42.05%)였다. 내부의 `format.Node` 호출에 17.24초,
뒤이은 `format.Source`에 9.89초가 귀속됐다. 프로파일 시간은 전후 속도 비교에 섞지 않는다.

기존 함수는 변환한 AST를 `format.Node`로 포맷한 뒤 그 결과를 `format.Source`로
다시 파싱·포맷했다. Go 1.27.2의 `format.Node`는 grouped import가 있으면
원본 AST를 수정하지 않기 위해 출력·재파싱한 복사본을 만들고 import를 정렬한 뒤
다시 출력한다. 이미 최종 `format.Source`가 정렬과 정규화를 담당하므로 중복이다.

변환한 AST의 출력은 `go/printer.Config`에 맡기고, 최종 소스 정규화·구문 검사는
기존 `format.Source` 한 곳에 유지한다. `TabIndent`, `UseSpaces`, tab width 8을
명시한다. 캐시나 제공자별 분기, Go 내부 비공개 formatter flag는 추가하지 않는다.
분할 정책·바인딩 분석·파일 묶기·소유권·공개 API에는 손대지 않는다.

## 정확성과 리뷰 범위

| 영역 | 상태와 근거 |
| --- | --- |
| AST 출력·최종 포맷 책임 | 검토 완료. 구문 정규화와 오류 반환은 최종 `format.Source`가 담당한다. |
| 주석·문자열·문서 소유권 | 새 검사 통과. package/function 문서 연결, import/body 주석, 주석 모양 문자열, 정규 포맷을 확인한다. |
| 심볼·모델·검증기·지역 이름 | 기존 `TestPartition*` 3개와 새 검사 1개 통과. 생성 소비자의 race 실행을 포함한다. |
| 실제 입력 전체 출력 | TVMaze·ListenNotes·GitHub 양쪽 5쌍, 30개 표본 전체 managed hash 일치. |
| 묶기와 게시 소유권 | 관련 `TestPack*` 통과. 공개 계약·결정성·상한·실패·중복 경로·증분 게시 소유권을 포함한다. |
| 자원 한계·실험 보존 | 전용 cache 하나, 파일 검산 후 출력 삭제. 전체 Graph·Cloudflare는 실행하지 않는다. |

최초 단위 실행은 기존 recorder의 `CGO_ENABLED=0` 고정 때문에 기존 race 소비자 3개가
실행되지 못했다. 명령에 `env CGO_ENABLED=1`을 명시한 재검증은 통과했다.
원래 실패와 수정된 실제 argv를 함께 보존한다.

## 동일 조건 비교

기존·후보 CLI를 같은 Go 1.27.2, `-trimpath`, `GOFLAGS=-p=2`로 빌드한다.
고정 실제 원본 3개에서 `generate --target golang --with server`를 호출한다.
import root는 `example.test/matched/api`다. 각 입력에서 순서를 바꾼 5쌍,
새 출력 디렉터리, `GOMAXPROCS=4`, `GOMEMLIMIT=2GiB`를 사용한다.
Go 빌드는 측정하지 않으며 표본별 cold cache를 만들지 않는다.

기존 `compare-apis.py _child`의 fresh worker가 subprocess 시간을 측정한다.
RSS는 Linux `RUSAGE_CHILDREN`의 최대 개별 프로세스 high-water이며 동시 전체 메모리가 아니다.
200ms 간격 session RSS는 별도 recorder에 남긴다. CPU 시간도 함께 보존한다.
외부 recorder의 감시·hash·시작 오버헤드는 생성 시간에 넣지 않는다.

최초 driver는 24개 검산 완료 표본 이후 세 번째 GitHub 기준 생성의 기록 경계에서
143으로 종료됐다. 해당 생성 자체의 metrics와 recorder 결과는 성공이지만,
기록 프로세스 종료 코드는 120이었다. 원인은 확인되지 않았다. 해당 표본은
주 비교에서 제외하고 원본 archive·metrics·실패 종료 코드를 보존한다.
살아 있는 driver가 없음을 확인한 뒤 기존 24개는 그대로 유지하고 세 번째 쌍부터
다시 측정한다. 이후 실행은 완전한 한 쌍 단위로 나누며 이전 결과를 덮어쓰지 않는다.
첫 재개 시 실패 종료 코드를 발견한 검사도 실패로 보존한다.

## 결과와 한계

성공한 각 버전 5회의 중앙값이며 화살표는 기존→후보다. CPU는 user+system 합계,
RSS는 각 실행의 최대 개별 프로세스 RSS를 집계한 중앙값이다.

| 입력 | 생성 시간, 초 | CPU 시간, 초 | 최대 RSS, MiB | 파일 수·바이트 |
| --- | ---: | ---: | ---: | --- |
| TVMaze | 0.3160 → 0.3093 | 0.3776 → 0.3751 | 44.58 → 43.99 | 동일: 143개, 3,172,864 B |
| ListenNotes | 0.5396 → 0.5433 | 0.6310 → 0.6378 | 54.01 → 54.75 | 동일: 201개, 7,547,511 B |
| GitHub | 56.0083 → 48.2604 | 62.8727 → 54.1913 | 600.28 → 607.06 | 동일: 6,375개, 203,264,540 B |

GitHub 생성 시간 중앙값은 13.83%, CPU 시간은 13.81% 감소했다. 시간 범위는
기존 55.28–57.38초, 후보 45.79–49.02초다. 각 쌍의 후보/기존 시간 비율을 로그
공간에서 평균한 기하평균은 0.8534다. 가능한 5^5개 재표집의 percentile 95% 구간은
[0.8313, 0.8762]이며 이 표본에서 모두 감소 방향이다. 중앙값 비율과는 다른 통계량이다.

GitHub 최대 RSS 중앙값은 오히려 1.13% 증가했다. 두 버전의 관측 범위가 겹치며,
이번 변경으로 peak 메모리가 감소했다고 주장하지 않는다. CPU와 실행 시간 절감이
채택 근거다. 작은 입력의 중앙값 변화는 TVMaze -2.11%, ListenNotes +0.68%다.
시간 비율의 95% 구간은 각각 [0.5237, 1.0184], [0.9828, 1.0130]으로 1을 포함한다.
TVMaze 첫 기준 실행 0.864초도 제외하지 않았다. 작은 입력의 확실한 이득은 입증하지 못했다.

원시 자료와 검산 결과는 [근거 archive](measurements/01-partition-format.tar.gz)에 있다.
크기는 2,357,392 B, 근거 항목 291개와 색인 1개이며 동일 내용은 tar hard link로 중복 저장하지 않는다.
archive의 `evidence-index.json`에 있는 모든 항목을 다시 읽어 크기·SHA-256을 검산했다.
archive SHA-256은 `9fd80fea471cf770b5d79e2f88b4b0d0b99724e029e31f6f474a2cd6ec25f6ed`다.
CPU profile·단계별 checkpoint·빌드 source manifest·실행별 metrics/log·출력 inventory·
원본 중단 snapshot·제외 표본·최종 검산·통계·생산 스크립트를 포함한다.
SDK 출력·실행 파일·Go 캐시와 중복 입력 원본은 포함하지 않는다. 고정 입력은 기존
대표 API 근거 archive와 `compare-apis.py`의 경로·SHA-256으로 식별한다.

별도 검산은 기존 24개 표본 불변, 최종 30개 고유 표본, 제외 표본 1개,
실제 파일과 manifest의 일치 확인 후 저장한 3개 공통 출력 목록, 바이너리·입력 hash,
로그 hash, 측정 source manifest를 확인했다. 기준 대비 제품 변경은 `partition.go`와
그 검사 파일뿐이며, 측정한 후보 제품 소스와 최종 소스의 hash가 같다.

전용 캐시는 검사·측정 후 약 2 GiB, `GOTMPDIR`는 비어 있었다. 각 표본의 출력은
검산 후 제거했고 프로파일 출력도 원시 목록 보존 후 제거한다. 캐시 4 GiB·생성물
2 GiB 운영 상한을 넘기지 않았다. 추후 표본을 실행하며 이전 생성물을 복제하지 않는다.

OS 파일 cache·다른 작업·스케줄링은 완전히 격리하지 못했다. 중단 전 2쌍과 재개 후
3쌍을 함께 보존했으며 재개는 완전한 쌍 단위다. 첫 outer driver 전체가 성공했다고
주장하지 않는다. 생성 속도 이득을 Go cold/warm 컴파일, 생성 SDK HTTP 실행,
TypeScript, 전체 Graph·Cloudflare, 다른 플랫폼·컴파일러의 이득으로 확대하지 않는다.
공개 문서에 로컬 수치를 게시하지 않는다.

최종 리뷰는 AST 출력과 그 호출자, import·문서·문자열 계약, 분할·묶기·게시 경계,
실측과 실패 분모, 검증 자료 보존, 자원 상한을 확인했다. 해당 범위의 미해결 결함은 없다.
