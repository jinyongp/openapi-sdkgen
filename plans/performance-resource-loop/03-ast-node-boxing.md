# AST 순회의 불필요한 인터페이스 복사 제거

상태: 구현·관련 검사·실제 API 비교·최종 리뷰 완료. 할당 절감을 근거로 채택한다.

기준 제품은 `50fb270`이며 제품 소스는 `2fd6bd1`과 같다. 앞선 heap profile에서
본문이 같은 `transformPartitionAST` 경로에 sampled 누적 할당 약 1.27 GiB가
귀속됐고, 그중 약 1.20 GiB가 `reflect.Value.Interface`를 통한 복사였다.
누적 할당을 동시에 점유한 메모리 크기로 해석하지 않는다.

기존 순회는 AST의 모든 필드를 일단 `Interface()`로 변환한 뒤 `ast.Node`인지
검사했다. 노드가 아닌 struct 값과 문자열/슬라이스의 헤더도 박싱 과정에서
복사되고 바로 버려졌다. 문자열의 내용이나 슬라이스 backing array 전체 복사를 뜻하지 않는다.
최초 후보는 동적 pointer/interface slot의 기존 assertion을 유지하고 다른 정적 값에
`ast.Node` 구현 여부를 검사했다. 실제 예비 1쌍은 누적 할당이 25,640,249,632→
24,328,023,808 B로 줄었으나 시간은 46.75→48.20초로 늘었다. 1쌍으로 회귀나
원인을 확정하지 않았고, 이 후보를 최종 표본에 섞지 않았다. 원본 소스·실험은
`type-check-prototype.tar.gz`로 보존했다.

실제 제품 호출자는 parser에서 만든 `*ast.File`을 쓰는 emitter 한 곳이었다.
고정 Go 1.27.2 표준 AST의 `Pos` 구현 56개는 모두 pointer receiver였다.
최종 후보는 진입점을 `*ast.File`로 명시하고, reflection은 내부 순회 구현으로
감춘다. 파서 AST의 pointer/interface slot만 노드로 검사한다. struct 값과
문자열/슬라이스의 헤더를 불필요하게 박싱하거나 모든 scalar에 일반형 method-set
검사를 하지 않는다. 전역 type cache를 추가하지 않는다. nil·교체 가능성·식 재작성·
객체/스코프 순환 제외 규칙을 유지한다.

새 optional benchmark는 파싱을 타이머에서 빼고 100개 함수 AST의 읽기 전용
순회를 실행한다. 후보 선택용으로 양쪽을 순차 5회 측정했다. 같은 4,702회 노드
방문을 유지하며, 시간 중앙값은 323,077→245,844 ns/op, 할당 중앙값은
205,129→0 B/op, 할당 수는 9,211→0 allocs/op였다. 이는 최초 일반형 검사의 결과다.
파서 AST 전용 최종 후보는 같은 방문 수에서 시간 중앙값 178,632 ns/op,
0 B/op·0 allocs/op였다. 첫 표본의 각각 2 B/op·1 B/op도 원시 결과에 유지한다.
순차 미시 실험으로 전체 생성 속도나 최대 RSS의 개선율을
결론 내리지 않는다. benchmark 로그의 개인 CPU 모델 행은 저장 전에 제외한다.

## 실제 입력 비교

직전 확정 제품과 최종 후보를 같은 `profile-go` 소스·Go 1.27.2·`-trimpath`·
`GOFLAGS=-p=2`로 빌드했다. CPU/heap profiling 없이 GitHub 3쌍을 순서 교차 실행하고,
TVMaze·ListenNotes는 정확성 확인용 각 1쌍을 실행했다. 원래와 같은 server addon,
`example.test/graph/api`, `GOMAXPROCS=4`, `GOMEMLIMIT=2GiB`, 개별 180초·session
RSS 보호 3 GiB 조건이다. 실행마다 새 프로세스·새 출력을 사용하고 검산 뒤 출력을 제거했다.

아래는 GitHub 각 3회 중앙값이다. 누적 할당은 observer의 게시 완료 checkpoint,
시간·CPU·RSS는 fresh subprocess worker다. observer의 입력 해제 GC와 출력 hash 등도
포함하며 순수 CLI나 생성 SDK 컴파일/HTTP 성능과 다르다.

| 지표 | 기준 | 후보 |
| --- | ---: | ---: |
| 실행 시간 | 47.2149초 | 45.9215초 |
| user+system CPU | 53.1050초 | 51.6038초 |
| 누적 할당 | 25,640,194,392 B | 24,328,544,264 B |
| 최대 개별 프로세스 RSS | 607.52 MiB | 604.33 MiB |

누적 할당은 5.12% 감소했다. 각 쌍의 후보/기준 비율은
0.94878/0.94883/0.94887로 일관됐으며 약 1.31 GB의 임시 할당을 줄였다.
누적 할당을 동시 메모리 점유량으로 해석하지 않는다.

단순 시간 중앙값은 2.74% 낮지만 첫 두 쌍은 각각 약 1% 느렸고 마지막 쌍은 빨랐다.
시간의 대응 기하평균 비율은 0.99624, 가능한 3^3개 재표집의 percentile 95% 구간은
[0.9780, 1.0102]다. 1을 포함하므로 실제 생성 속도 향상을 입증했다고 안내하지 않는다.
RSS 중앙값 감소도 0.53%로 작고 관측 범위가 겹친다. 큰 폭의 peak 메모리 이득을 주장하지 않는다.
첫 단위의 CLI 개선율과 여기의 observer 중앙값을 더해 전체 개선율로 쓰지 않는다.

GitHub 6,377개·203,199,878 B는 최종 후보의 모든 표본에서 기존 파일별 hash와
같았다. 작은 입력도 전체 출력이 같았다. TVMaze는 143개·3,172,460 B,
ListenNotes는 201개·7,546,903 B다. 작은 입력의 각 1쌍으로 성능 개선율을 주장하지 않는다.

## 리뷰와 검증

| 영역 | 완료 근거 |
| --- | --- |
| 입력 계약·호출자 | 실제 제품 호출은 parser 결과를 다루는 emitter 한 곳이다. 진입점을 `*ast.File`로 명시하고 reflection을 내부 구현에 둔다. |
| 순회·변환 | pointer/interface slot의 assertion·교체 검사·nil 처리·순회 순서, Obj/Scope/Unresolved 제외 규칙을 유지한다. |
| 독립 문법 oracle | `ast.Inspect`와 고유 식별자 방문 집합을 비교한다. generic·channel·select·label·Obj/Scope/Unresolved라는 사용자 필드 이름과 identity 변환의 문법 불변을 확인했다. |
| 소비자·게시 | 최종 `TestPartition*`·`TestPack*` 통과. 실제 생성 소비자의 race 검사를 포함한다. |
| 원시 검산 | 고유 표본 10개, 같은 observer 소스, 바이너리·입력·생산 스크립트·로그·runtime source hash·checkpoint·출력 inventory를 대조했다. 런타임 제품 변경은 분할 순회 파일 하나다. |
| 자원·범위 | 전역 runtime cache나 제공자별 예외를 추가하지 않았다. 단일 전용 Go cache 약 2.2 GiB, Go 임시 경로는 비어 있다. 기존 4 GiB cache·2 GiB 생성물 상한 안이다. |

측정 후 실제 제품 소스는 바뀌지 않았다. 독립 oracle과 optional benchmark를 포함한
검사 코드는 최종 상태에서 검사했다. 관련 범위의 미해결 결함은 없다.
전체 compiler/corpus 매트릭스·전체 Graph·Cloudflare·다른 플랫폼·TypeScript나
생성 SDK HTTP/Go cold·warm 비용을 이번에 새로 검증했다고 주장하지 않는다.

## 보존

[원시 근거](measurements/03-ast-node-boxing.tar.gz)는 3,025,180 B,
근거 항목 192개와 색인 1개다. 모든 항목을 다시 읽어 크기·hash를 확인했다.
SHA-256: `ffda1aa5e857edd7ae1fb660fdf1a8c559a79e66020d0dca372f6b230125ed5b`.
초기 일반형 후보의 소스와 미시/실제 예비 결과도 별도 inner archive에 유지했다.
예비 결과를 최종 후보 표본에 섞지 않았다. 최초 hardware header를 포함한 private
보정 로그는 archive에 넣지 않고, 정식 미시 로그는 저장 전 CPU 모델 행을 제외했다.
SDK 출력·실행 파일·Go cache는 포함하지 않는다. 공개 문서에 로컬 수치를 게시하지 않는다.
