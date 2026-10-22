# 중간 AST 출력의 열 정렬 비용

상태: 조사·동일 조건 비교·복구·최종 검사·리뷰 완료. RawFormat 후보를 기각했다.
제품 runtime은 기준 `f8dbb9e`와 완전히 같다. 이전 단위는 facade 재파싱을 줄였지만
생성 시간·peak RSS의 일관된 이득은 입증하지 못했다.

AST 재작성 뒤 `formatPartitionFile`은 printer로 소스화하고 `format.Source`로
최종 정규화한다. 첫 출력도 tabwriter 열 정렬을 실행하고 최종 정규화가 다시
같은 역할을 한다. 이 중간 출력은 외부로 나가지 않으며 최종 정규화는 유지해야 한다.
Go 1.27.2의 공개 `printer.RawFormat`은 첫 정렬을 생략하고 trimmer를 유지한다.
이를 이용해 중복된 열 정렬만 줄일 수 있는지 조사한다.

현재 제품으로 CPU·heap/allocation profile 1회를 수집하고 09 단위의 모든 출력
hash와 대조한다. profiling 수치는 비교 표본에 넣지 않는다. 호출 경로에서 중간
tabwriter의 기여를 확인한 다음 후보를 구현한다. 주석/문자열/숫자·넓은 선언·서명·
import·정규화 회귀 검사 후 고정 GitHub 예비 한 쌍의 전체 byte/hash가 같을 때만
최대 3쌍과 TVMaze/ListenNotes 각 1쌍으로 진행한다. 실제 CLI와 소비자·race 검증도 유지한다.

비공개 printer mode·문자열 치환·위치 정보 초기화·주석 예외·최종 정규화 생략은
사용하지 않는다. Go cache 하나를 재사용하고 새 cache를 복제하지 않는다.
cache 4 GiB·scratch 2 GiB, GOMAXPROCS=4·GOMEMLIMIT=2GiB, 개별 180초·
session RSS 보호 3 GiB를 유지한다. 전체 Graph/Cloudflare·전체 compiler matrix는 제외한다.

현재 profile 출력 6,377개·204,611,637 B는 09 단위와 전부 같았다. 최종
`format.Source`를 제외한 call stack에서 중간 printer에 sampled CPU 7.10초와
누적 할당 약 2.05 GiB가 귀속됐다. 이 경로의 tabwriter 누적 할당 939 MiB에는
downstream buffer/trimmer도 포함하므로 전체가 제거 가능하다고 계산하지 않는다.
이는 후보의 병목 근거이며 profile 37.193초를 전후 비교 표본으로 쓰지 않는다.

## 실제 입력 비교와 기각

같은 observer·Go 1.27.2·server addon·import root `example.test/graph/api`·
GOMAXPROCS=4·GOMEMLIMIT=2GiB로 GitHub 3쌍을 기준→후보/후보→기준/
기준→후보 순서로 비교했다. TVMaze와 ListenNotes도 각각 1쌍을 실행했다.
각 생성은 새 경로를 쓰고 전체 managed 파일 검산 뒤 SDK tree를 제거했다.
개별 180초·RSS 3 GiB를 유지하고 pair/batch controller는 480초/900초로 제한했다.

GitHub 각 3회 중앙값:

| 지표 | 기준 | 후보 | 해석 |
| --- | ---: | ---: | --- |
| observer 시간 | 40.9750초 | 42.4458초 | +3.59%, 채택할 속도 이득 없음 |
| user+system CPU | 46.3913초 | 48.0456초 | +3.57%, 이득 입증 못 함 |
| 누적 할당 | 19,508,328,664 B | 19,419,647,976 B | −0.45% |
| 최대 개별 process RSS | 635,289,600 B | 634,548,224 B | 중앙값 −0.12%, 쌍별 방향 불일치 |
| 출력 | 6,377개·204,611,637 B | 동일 | 전체 경로·byte·hash 일치 |

시간 비율은 1.07026/0.96198/1.03590으로 3쌍 중 2쌍에서 후보가 느렸다.
기하평균은 1.02170, 3^3 재표집의 선형 보간 percentile 95% 구간은
[0.97760, 1.06273]이다. 기준 범위 38.0618–45.8316초와 후보 범위
40.7358–44.0892초가 겹친다. 작은 로컬 표본이며 환경 변동의 원인을 확정하거나
모든 환경의 성능 회귀율로 일반화하지 않는다. 불리한 표본도 전부 보존했다.

작은 할당 감소에 비해 생성 시간·CPU의 실질 이득을 입증하지 못했고, 최대 RSS도
일관되게 줄지 않았다. 이 작업의 시간/자원 목표를 개선하는 채택 근거가 부족해
원래 printer mode를 복구했다. 별도 GC/동시성 조정이나 파일별 예외는 추가하지 않는다.
모든 runtime Go 파일이 기준 build source와 같음을 hash로 확인했다.

TVMaze 143개·3,172,460 B와 ListenNotes 201개·7,546,903 B도 양쪽 전체 출력이 같았다.
작은 입력의 1쌍 결과를 속도 개선율로 쓰지 않는다. 후보를 기각했으므로 새 실제 CLI
통합·부모 generator race·SDK cold/warm compile matrix는 실행하지 않았다.
기준 제품의 기존 통합/race 근거를 유지한다. TypeScript·전체 corpus·전체
Graph/Cloudflare·compiler/platform matrix도 이번 범위에 넣지 않았다.

## 유지한 검사와 리뷰

새 `TestPartitionIntermediateFormattingRemainsCanonical`은 표준 `format.Source`를
oracle로 삼는다. 열 폭이 다른 field/tag/상수, import·행/블록 주석, raw string의
탭·수직 탭·formfeed·줄바꿈, escaped 문자열, 숫자 prefix/exponent, variadic signature,
빌드/함수 지시문을 확인한다. 독립 입력의 정규 소스와 byte가 같아야 한다.

후보에서 기존 partition/consumer 검사와 새 5개 하위 사례가 통과했다. 복구 후
같은 최종 검사를 실행해 top-level 10개와 새 하위 사례가 모두 통과했다.
생성 소비자 `-race`에 필요한 CGO를 명시적으로 켰다. 형식/주석/문자열 보존,
재작성 서명·local binding·AST 소유권·정상/실패 경계를 리뷰했다. 새 검사는 ordinary
Go test discovery에 포함되며 별도 timing/percentage CI gate를 만들지 않는다.

10개 고유 표본의 입력·바이너리·producer·로그·source hash와 phase/checkpoint·
전체 inventory를 검산했다. 후보의 유일한 runtime 차이는 중간 printer mode였다.
기각 source·build/profiling 출처·모든 표본을 보존하고 후보 CLI/observer 2개
74,315,425 B를 제거했다. 배포 기준·현재/직전 제품의 5개 실행 파일은 유지한다.
최종 전용 Go cache 약 3.2 GiB, cache 외 scratch 약 331 MiB이며 Go 임시 경로는 비어 있다.

## 다음 조사

같은 현재 제품 profile에서 `transformPartitionValue`의 sampled 누적 CPU는 8.06초,
할당은 약 336 MiB였다. callback과 자식 순회를 포함하므로 전부 제거 가능한 비용이
아니다. 매 struct의 `Type().Field(index).Name` 조회에는 0.89초가 귀속됐다.

현재 walker는 syntax의 scalar field에도 reflect 재귀를 실행하고, 타입/필드 구조를
노드마다 다시 해석한다. 다음 단위에서는 타입별 child-slot metadata 또는 표준 AST
방문자와 교체 cursor를 비교한다. metadata와 syntax/객체 링크의 소유권·수명을
구분하고, alias 선택 순서·대체 타입 검사·오류·local scope·새 노드 재방문 정책을
유지해야 한다. 단순 `ast.Inspect` 후 부모 slice를 매 교체마다 선형 검색하면 넓은
literal에서 이차 비용이 될 수 있으므로 그런 구현은 선택하지 않는다.

[원시 근거](measurements/10-intermediate-alignment.tar.gz)는 2,331,534 B,
근거 항목 192개와 색인 1개다. SHA-256:
`935c7dea516998e09a0954bac91cc4dd040e6e72879b751831bb526ba69ac6f8`.
모든 항목의 크기·hash를 다시 확인했다. 현재 제품 profile·정확한 attribution 명령·
고정 입력·기각 source/build·10개 표본·전체 inventory·최종 검사·runtime 복구와
실행 파일 정리 목록·producer를 포함한다. 실행 파일·SDK tree·Go cache는 제외했다.
