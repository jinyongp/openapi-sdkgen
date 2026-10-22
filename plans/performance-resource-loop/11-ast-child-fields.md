# AST 자식 필드 구조와 실제 노드의 분리

상태: 구현·동일 조건 비교·최종 소비자/CLI/race 검사·리뷰 완료. 후보를 채택한다.
기준은 `ed90657`이며 기준 runtime은 `f8dbb9e`와 같다.
직전 현재 제품 profile의 `transformPartitionValue`는 sampled 누적 CPU 8.06초·
할당 약 336 MiB였다. callback/자식 순회를 포함해 전부 제거할 수 있는 비용은 아니다.
반복 `Type().Field(index).Name` 조회에 0.89초가 귀속됐고 scalar field에도 reflect
재귀를 실행했다. 이 attribution은 같은 runtime과 같은 고정 입력의 근거로 재사용한다.

선택한 후보는 AST 타입의 자식 필드 구조와 실제 syntax를 분리하는 walker다.
각 package partition이 walker 하나를 소유하며, `reflect.Type`별 child field index만
한 번 계산한다. pointer/interface/slice/struct slot과 기존 metadata 제외 규칙을 유지하고
scalar field에는 내려가지 않는다. cache에는 AST·reflect.Value·callback·출력/파일 이름을
넣지 않는다. 크기는 해당 Go AST의 struct type 수에 묶이고 partition 반환 시 수명을 끝낸다.
같은 plan의 동시 emission은 각자 walker를 소유한다.

기존 slot 기반 교체·순서·타입 검사·오류·새 노드 미재방문·parser local object/scope
링크를 유지한다. 표준 AST visitor와 교체 cursor를 새로 도입하는 대안보다 현재 계약을
작게 유지하면서 반복 타입 분석을 제거한다. 부모 slice의 매 교체 선형 검색이나
고정된 AST node kind별 대형 switch, 전역 cache/새 의존성을 추가하지 않는다.

예산은 기존 microbenchmark 3쌍과 GitHub 최대 3쌍, TVMaze/ListenNotes 각 1쌍이다.
micro는 같은 parser tree의 순회를 비교하며 전체 생성 개선율로 일반화하지 않는다.
실제 입력은 같은 observer·Go 1.27.2·server/import root·환경으로 직렬 비교하고
모든 byte/hash가 같아야 한다. 최종 소비자/ownership/race·실제 CLI를 확인한다.
Go cache 하나·4 GiB와 scratch 2 GiB 상한, GOMAXPROCS=4·GOMEMLIMIT=2GiB,
개별 180초·RSS 3 GiB 보호를 유지한다. 전체 Graph/Cloudflare·전체 compiler matrix는 제외한다.

## 검증과 실측

같은 AST 100개 함수의 기존 순회 benchmark를 별도 기준/후보 test binary로 빌드했다.
각 300ms sample을 기준→후보/후보→기준/기준→후보로 실행했다. 파싱은 양쪽 모두
타이머 밖이고 walker의 최초 metadata 생성은 첫 loop에 포함된다. 후보는 실제
partition처럼 metadata를 재사용한다. 방문 수는 여섯 표본 모두 4,702회/op였다.
micro 중앙값은 175.813→87.909 µs/op, 양쪽 1 B/op·0 allocs/op였다. 초기 metadata
할당이 loop 수로 상각된 값이며 cache 생성이 무할당이라는 뜻은 아니다.
시간 약 50% 감소는 이 순회만의 결과다. CPU 모델 헤더는 저장 전에 제외했다.

전체 생성은 같은 observer·Go 1.27.2·server addon·import root
`example.test/graph/api`·GOMAXPROCS=4·GOMEMLIMIT=2GiB로 비교했다. GitHub 3쌍은
같은 교대 순서, TVMaze/ListenNotes 각 1쌍은 정확성 확인이다. CPU/heap profiling은
끄고 매번 새 출력으로 생성한 뒤 모든 managed 파일을 검산해 SDK tree를 제거했다.
개별 보호는 180초·RSS 3 GiB, pair/batch controller는 각각 480초/900초였다.

GitHub 각 3회 중앙값:

| 지표 | 기준 | 후보 | 변화 |
| --- | ---: | ---: | ---: |
| observer 실행 시간 | 36.9310초 | 36.1720초 | −2.06% |
| user+system CPU | 41.6468초 | 40.8868초 | −1.82% |
| 누적 할당 | 19,505,294,520 B | 19,506,544,256 B | +0.0064%, 실질 절감 미입증 |
| 최대 개별 process RSS | 637,190,144 B | 643,960,832 B | +1.06% |
| 출력 | 6,377개·204,611,637 B | 동일 | 전체 경로·byte·hash 일치 |

시간 범위는 기준 36.4711–37.0974초, 후보 36.0077–36.4265초였다. 대응 비율
0.98729/0.98634/0.97505로 세 쌍 모두 빨랐고 CPU도 모두 줄었다. 기하평균은
0.98288, 3^3 재표집의 선형 보간 percentile 95% 구간은 [0.97749, 0.98709]다.
작은 로컬 표본의 조건부 재표집 결과이며 모든 환경의 개선율로 일반화하지 않는다.
observer 시간에는 입력 해제 GC·hash·게시가 포함되며 순수 CLI benchmark와 다르다.

RSS는 세 쌍 모두 후보가 높았다. 작은 metadata cache의 byte와 이 증가분을 같은
것으로 단정하지 않지만, 관측 비용으로 기록한다. GC/동시성 설정은 바꾸지 않았다.
누적 할당·peak RSS 절감을 주장하지 않고, 구조 분석 중복 제거와 실제 생성/CPU의
작은 이득으로 채택한다. 기존 단위의 개선율과 더해 누적 개선율을 계산하지 않는다.

TVMaze는 양쪽 143개·3,172,460 B, ListenNotes는 201개·7,546,903 B가 같았다.
작은 입력 1쌍의 시간으로 개선율을 주장하지 않는다. SDK 입력 전체가 같으며 새
cold/warm compile 비용 비교는 하지 않았다. TypeScript·전체 corpus·전체
Graph/Cloudflare·compiler/platform matrix는 이번 검증 범위에 넣지 않는다.
로컬 수치를 공개 제품 문서에 게시하지 않는다.

## 최종 리뷰와 검사

| 영역 | 완료 근거 |
| --- | --- |
| cache 내용·수명 | `reflect.Type`→field index slice만 보관한다. AST/value/closure/파일 데이터가 들어가지 않고 partition마다 새 walker를 만든다. 같은 타입을 쓰는 다음 tree에서 항목 수가 늘지 않았다. |
| 순서·범위 | struct 원래 순서의 구조 slot만 유지한다. scalar에는 callback이나 자식이 없으며 기존 Obj/Scope/Unresolved metadata 제외를 유지한다. 표준 AST oracle의 모든 distinct identifier를 방문하고 identity 출력이 같다. |
| 교체 계약 | call·slice·index·binary expression의 여섯 위치를 모두 교체했다. 삽입한 새 노드는 재방문하지 않았다. 좁은 `*ast.Ident` declaration slot은 같은 오류를 내고 원래 이름을 유지한다. 실패 후 동일 walker로 다른 AST의 expression 교체도 성공했다. |
| 데이터 독립성 | metadata를 다음 AST에 재사용해도 앞 tree 출력이 바뀌지 않았다. 서로 다른 walker의 병렬 사례와 같은 plan의 emission/준비 결과 독립성·facade 원본 격리를 `-race`로 확인했다. |
| 소비자·게시 | 최종 top-level 26개 검사 통과. 기존 partition/packing·local binding·주석/정규화·sink 실패·formatting rollback·owned runtime byte·health consumer를 포함한다. 생성 소비자 race에 필요한 CGO를 명시적으로 켰다. |
| 실측 출처 | micro 6개와 실제 입력 10개 표본의 producer/input/binary/source/log hash·checkpoint·전체 inventory를 검산했다. callback 수가 같은 micro와 runtime 한 파일 차이인 실제 비교를 구분한다. |
| 실제 CLI | 최종 CLI로 세 실제 API를 각 새 경로에 생성했다. 모든 관리 파일 hash가 observer 후보와 같았고 SDK tree를 제거했다. 이 실행은 정확성 확인으로만 사용한다. |

기존 profile의 관련 코드/입력/실행 조건이 기준 runtime과 같음을 확인하고 attribution을
재사용했다. metadata cache 자체를 cache된 결과·전역 AST 저장소로 확장하지 않았다.
반복 scalar/type 조회의 제거가 실제 비용을 줄이는지 micro와 전체 생성으로 각각
검증했고, 표준 visitor에 필요한 새 교체 cursor나 추가 의존성은 도입하지 않았다.

검사 후 전용 Go cache 약 3.2 GiB, Go 임시 경로는 비어 있었다. 배포 기준 CLI·
현재/직전 제품만 남기고 오래된 CLI/observer와 benchmark test binary 4개
156,007,563 B를 제거했다. 남긴 실행 파일은 5개·187,795,478 B, cache 외 scratch는
약 347 MiB다. 제거 경로·크기·hash·build 출처는 보존하고 다른 작업의 cache는 건드리지 않았다.

[원시 근거](measurements/11-ast-child-fields.tar.gz)는 2,426,977 B,
근거 항목 253개와 색인 1개다. SHA-256:
`4953dc72806c7476180fa3bee587b89cd8b8a065c08a8aaa46973beba965a797`.
모든 항목의 크기·hash를 다시 확인했다. 재사용한 현재 profile·attribution·고정 입력·
기준/candidate build 출처·micro 6개·실제 입력 10개·전체 inventory·최종 검사·
CLI·정리 목록과 producer를 포함한다. 실행 파일·SDK tree·Go cache는 제외했다.
