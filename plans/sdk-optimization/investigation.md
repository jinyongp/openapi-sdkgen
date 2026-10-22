# SDK 대형 출력 최적화 조사

조사일: 2026-10-21. 제품 기준: `5ada624a69ebfd85103248379970f78a8f3d2899`.
이 문서는 내부 개발 기록이다. 제품 구현과 성능 개선 완료를 주장하지 않는다.

## 현재 범위 결정

2026-10-22 사용자는 전체 MS Graph 성공을 필수로 강제하는 비용이 과하다고 판단하고
참고 검증으로 전환했다. `scope-decision-2026-10-22.md`와 갱신된 `plan.md`가 현재 범위다.
일반 제공자 38개, 대표 SDK와 Graph 선택 SDK의 정확성·호환성·배포 검증은 유지한다.
아래 분석과 과거 전체 Graph 실험은 당시 조사·결과의 이력이며 전체 성공을 뜻하지 않는다.

## 판정

첨부 보고서는 실험 후보의 우선순위를 정하는 자료로 타당하다. 실제 Graph 전체 strict 검사 성공이나 Go 대형 출력 개선율의 근거로 사용할 수는 없다. TypeScript closure 공유와 registry 분할, Go 동일 패키지 파일 묶기를 우선 검증한다. section 인덱스, validator 공유, Project References는 효과와 호환성을 확인한 뒤 채택한다.

첨부 문서의 권고·금지·목표는 검토 대상이다. 사용자 요청은 조사, 결정 근거 검토, devtools 개선 계획 등록이다. 첨부 스크립트 실행, 제품 구현, 배포를 지시한 것으로 해석하지 않았다. 현재 제품과 기존 결정에 맞는 보존 조건을 아래에서 별도로 채택했다.

## 입력과 출처

| 입력 | SHA-256 |
| --- | --- |
| `openapi_sdkgen_optimization_decision_report.md` | `8fe099c82d6e1397d6d83e32d68b10dad72b0b278989c0ac0c2a0f03416800e3` |
| `openapi_sdkgen_optimization_evidence.zip` | `1dbd544c115c11de205fabe9ac2c7b7424fdddc2bcbe060c3cca255df40e2a60` |
| Graph beta 원본 `.tmp/compatibility-regression/microsoft-graph-beta/openapi.yaml` | `46bead9a6459cbe5f66d31094b23a614eb8da8f46690ee61404d97523e23daec` |

첨부 위치는 사용자 제공 파일이다. ZIP의 README, 스크립트 8개, 원시 JSON 3개를 모두 읽었다. 첨부 스크립트는 기존 `/tmp` 디렉터리를 삭제하고 파일을 생성하므로 그대로 실행하지 않았다. 작은 의미 보존 재현은 별도 임시 경로에서 작성했다.

저장소의 `212adb9..5ada624` 범위에서 `internal/target/typescript`, `internal/target/golang`, `scripts/compat-benchmark`, `test/typescript/package.json` 차이는 없다. 보고서가 본 제품 코드와 조사 코드의 관련 구현은 같다. 배포 관련 기존 변경은 조사 도중 다른 작업에서 정리되었으며, 계획 파일 작성 전 worktree가 clean인 것을 다시 확인했다.

## 주장별 대조

| 주장 | 조사 결과 | 채택 범위 |
| --- | --- | --- |
| TS 동일 closure만 공유 | `execution_schema_modules.go`의 projection + 정렬된 이름 목록 전체 해시, 사용 횟수 > 1과 일치 | 부분 중복 제거를 검증할 근거가 충분하다. |
| execution마다 큰 목록 반복 | `execution_modules.go`는 exact bundle이 없을 때 import와 `Object.fromEntries`를 반복한다. | emission 비용을 줄이는 후보. 런타임 map 생성 비용도 측정해야 한다. |
| registry 거대 함수 | `client_registry.go`가 전체 base/pagination/stream, links, decoration, 정렬 등록을 한 함수에 방출한다. | 단일 canonical state 타입과 단계별 bounded 함수로 분할한다. |
| request section mapped type 반복 | `route_modules.go`의 route/ID/resource별 계산과 일치한다. | 합성 실험만으로 기본 적용하지 않는다. |
| Go SCC 분할 존재 | `partition.go`의 8 MiB 발동, 4 MiB 배치, public facade, private schema 직접 binding과 일치 | 새 패키지 분할기를 만들지 않는다. |
| Go validator가 모두 독립 | 이미 `plan_schema_views.go`에서 같은 알고리즘의 media/non-null view를 공유한다. | 기존 view 재사용을 유지하고 잔여 중복만 계량한다. |
| TS Graph 186,448개·약 2.36 GB | 저장된 CI 보고서에 186,448개, 2,358,735,553 B, 29,581 operation, omission 0, helper omission 1,208이 있다. | 해당 고정 입력과 측정 commit의 생성 결과로 확인된다. |
| 약 857 MB로 축소 가능 | ZIP에 Graph closure별 membership·바이트 분포 및 모델 계산 원본이 없다. | 검증되지 않은 바이트 모델. 목표나 예상 실측값으로 확정하지 않는다. |
| Go 477,701개·약 3.20 GB | `plans/golang-target/w11-performance.md`의 과거 생성 측정과 일치 | 현재 full Graph 컴파일 성공 근거가 아니다. |
| Go 38개 corpus 문서 검증 | `test/compatibility/ci/golang-results.json.gz`는 38/38, Go 1.27.2, server 포함, compile pass를 기록 | 실제 API 문서 29개와 fixture 9개다. 제공자 38곳으로 해석하지 않는다. 후보 구현의 새 검증은 필요하다. |

## 현재 CI의 의미

`test/compatibility/ci/provenance.json`은 [Actions 39456862841](https://github.com/jinyongp/openapi-sdkgen/actions/runs/39456862841), source `60de0a577051b3d751861816198523801c5bc092`를 가리킨다. Graph와 Go 압축 보고서를 해제한 내용의 SHA-256이 provenance의 digest와 일치함을 확인했다.

Graph 보고서는 `typecheck.status=not-run`, `generation-only measurement`다. `scripts/compat-benchmark/benchmark.go`의 `typecheckCandidate`와 `.github/workflows/compatibility.yml`이 이 정책을 명시한다. GitHub 도구로 조회한 Graph full job도 TypeScript 설치와 Go 컴파일이 skipped이고 generation-only gate만 통과했다. 조회는 첫 페이지 30개 job이며 전체 job 재감사를 주장하지 않는다. Go 38/38 집계는 저장된 보고서 전체를 읽은 결과다.

과거 `.tmp/summary-live-graph/compatibility-ci/report.json`에는 generation pass, typecheck `signal: killed`가 있다. 자원 요약은 전체 benchmark 명령의 peak RSS 15,539,322,880 B와 보호 감시 종료를 기록한다. 이 수치를 개별 checker의 순수 메모리로 해석하지 않는다. 다른 `.tmp/graph-path/memory.json`도 당시 자식 checker 보호 종료를 기록한다. 현재 코드의 새 full strict 결과와 구분해야 한다.

현재 `benchmark:compatibility` 명령을 재실행하는 것만으로 새 Graph strict 검증을 얻을 수 없다. W01에서 생성 전용 정책을 보존하면서 최적화 실험 전용 전체 검사 경로를 마련해야 한다.

## 첨부 실험의 신뢰 범위

원시 JSON의 모든 표본을 읽고 중앙값을 다시 계산했다.

| 실험 | 표본 | 재계산 결과 | 한계 |
| --- | --- | --- | --- |
| TS 목록 공유 | 각 3회, exit 0 | wall 3.27→1.62 s, RSS 278,648→182,388 KiB, instantiation 68,398→37,209 | TS 5.8.3. 고정 순서. 같은 650개를 앞에 붙이는 fixture로 실제 순서 보존 병합 비용은 포함하지 않는다. |
| Go 파일 묶기 | 각 6회, exit 0 | wall 0.465→0.3305 s, RSS 91,584→87,304 KiB | Go 1.23.2. 선언 2,400개 규모. 3.2 GB 실데이터와 다르다. |
| Go validator 공유 | 각 6회, exit 0 | wall 1.135→0.228 s | 거의 같은 본문 900개인 상한 실험. 실제 IR의 중복률을 보여주지 않는다. |

registry·section 실험 원시 결과, validator ns/op 원시 출력, 실제 40-operation 패치와 측정, Graph 바이트 모델 입력은 ZIP에 없다. 제공된 fixture 생성기로 기법을 재실험할 수는 있지만 보고된 숫자 자체의 독립 검산은 불가능하다. 반복 실행·nonce 수정·실행 순서·통계 수집 자동화도 첨부되지 않았다. 파일 묶기의 archive/binary 감소에는 파일명·디버그 자료 절감이 섞일 수 있으므로 stripped binary와 runtime 효과를 별도로 측정한다.

새 조사에서 첨부 성능 수치를 재측정했다고 주장하지 않는다. 정확한 툴체인의 현재 회귀 검사와 작은 의미 보존 재현을 실행하고, 대형 성능 검증은 구현 작업의 명시적 완료 조건으로 남긴다.

### v2 보완 자료 재검토

후속 ZIP `openapi_sdkgen_optimization_complete_v2.zip`의 SHA-256은
`2df012da5fc9fb621738578aab1c0573bee973a4c7dc6de5ec377cee96735b64`다.
위의 원본 자료 누락 지적은 최초 자료에 관한 기록이다. v2는 재현 가능한 합성
근거를 보강하고, 원본이 없는 정밀 수치와 857 MB 제품 예측을 철회했다.

독립 verifier에서 manifest 파일 5,921개, 완료 명령 기록 235개, source reference
98,204개의 hash 연결이 맞았다. 완료 명령은 pass 234개와 예상한 TS6202 negative
1개이며, 중단된 TS 표본 1개는 성공으로 집계하지 않았다. 네 설치 compiler의
작은 emitter-shaped baseline/candidate 총 8개 조합도 직접 통과했다.

| v2 실측 | 검산 결과 | 제품 결정에 남는 한계 |
| --- | --- | --- |
| TS wire / registry / section | 중앙값 3.065→1.58 s / 3.98→2.445→2.2 s / 1.58→1.41 s | 실제 Graph 측정이 아닌 합성 입력 |
| Go validator / file pack | build 1.77→0.28 s / SDK-cold 0.665→0.475 s | 실제 generated IR 중복률·최종 package 분포 미측정 |
| validator runtime 10쌍 | paired ratio 중앙값 0.9874, bootstrap 95% 구간 0.9665~1.1139 | +5% 비열등성을 입증하지 못함 |
| JS map 10회씩 | 57.9824→77.1950 ms, retained bytes 동일 | map 조립 비용 증가를 제품 실측으로 확인해야 함 |
| 작은 emitter-shaped 결과 | 4,052→4,950 B, +898 B | 단순 공유가 net bytes 감소를 보장하지 않음 |
| 실제 제품 matrix 86개 | 모두 `not_run_blocked` | TS 8개, Go corpus 76개, 별도 Graph Go 2개. 합성 성공으로 대체할 수 없음 |

v2 도구에서는 실제 반례 세 가지를 확인했다. packer가 `_ignored.go`를 활성 파일과
합쳐 duplicate symbol을 만들었고, Go 버전 사전 확인은 1.27.2여도 실행 환경은
`GOTOOLCHAIN=local` 때문에 1.27.3이 될 수 있었다. 실제 실행 matrix가 전부
blocked여도 최상위 프로세스는 exit 0이었다. 당시 기록 235개에는 후에 추가된
`command_file_artifacts`와 실행별 toolchain digest 필드도 없다.

외부 runner/packer를 제품 경로로 가져오지 않는다. W01의 작은 내부 진입점은 같은
실패를 nonzero로 기록하고, 확인한 Go 바이너리를 그대로 실행하며, 무시되는 파일을
audit 대상의 안전한 병합 후보에서 제외한다. 원본 첨부를 수정한 것으로 주장하지
않는다. Go의 무시 파일 규칙은 [공식 go 명령 문서](https://pkg.go.dev/cmd/go#hdr-Package_lists_and_patterns)와 일치한다.
Project References의 declaration 경계는 [공식 문서](https://www.typescriptlang.org/docs/handbook/project-references.html)에 따르되 실제 Graph SCC와 public consumer의 완료 가능성은 별도 실측한다.

W01의 실제 기준 조사와 남은 착수 조건은 `preflight.md`에 기록한다. v2의 합성
실험을 다시 제품 개선율로 승격하지 않는다.

후속 실제 작은 SDK 검사에서는 재귀 고정 property와 additionalProperties가 함께 있는
기준 출력의 TS2456을 네 compiler에서 확인했다. 축소 입력을 `probes/recursive-property.json`에
보존했고 원인을 `types.go`의 property index union과 재귀 감지 경계까지 추적했다.
이 기존 정확성 문제는 W01R에서 성능 후보와 분리해 먼저 수정하도록 계획했다.

## 구조와 영향 범위

### TypeScript: dependency 사실과 출력 표현 분리

원본 closure는 lowering이 계산한 `(schema identity, input/output projection)`의 정확한 집합이다. 최적화는 이 사실을 변경하지 않고 출력 표현만 공유해야 한다.

1. `execution_plan.go`의 sorted closure를 canonical 원본으로 둔다. projection별 ordinal과 reusable block 계획을 만든다. 동일 closure가 현재 공유하는 map identity도 유지한다.
2. 먼저 빈번한 exact closure를 base 후보로 사용한다. subset 검증과 실제 반복 import/entry 바이트 비용을 기준으로 이득 있는 base+delta만 채택한다. 전 operation 쌍의 교집합·모든 부분집합 탐색은 피한다. 이 방식으로 큰 반복을 설명하지 못할 때만 SCC/DAG block 계획을 추가한다.
3. 공유 block이 참조하는 전체 import graph 역시 소비 operation의 원래 closure 안에 있어야 한다. map에서 키만 거르는 전역 superset 모듈은 선택 경계를 지키지 못한다. dynamic-reference 보수적 fallback은 기존 범위를 유지한다.
4. Go가 부여한 ordinal로 merge하고 `Object.fromEntries` 또는 동등한 own data property 생성으로 map을 만든다. `Object.assign({}, shared, delta)`와 재정렬은 사용하지 않는다. 중복 ordinal/key를 계획 단계에서 거부한다. `Object.keys`의 정수형 키 우선 규칙도 baseline과 비교한다.
5. registry는 전체 base 단계 완료 → links 단계 → decoration 단계 → route/ID/link 정렬 등록 순서를 유지한다. 단계 사이 typed state는 한 번 선언한다. 청크별 mapped type, operation마다 별도 넓은 object shape, 반복 intersection을 늘리지 않는다.
6. section index는 route뿐 아니라 operation ID, exact/resource identity, resource `path` 제외, generic union·`never`·일반 `PropertyKey`의 현재 의미까지 보존한다. 공개 shape 정의는 canonical 계약에서 파생한다.

W01 실물 inventory에서는 단일 파일 크기가 client/types 42,655,818 B,
registry 39,808,834 B, routes/index 36,506,796 B, selective/types 33,439,373 B였다.
따라서 함수 분할과 별개로 대형 공개 타입 계약의 내부 모듈 분할을 W03에 추가했다.
import SCC는 44개가 순환이며 가장 큰 순환은 35파일·328,670 B다. 바이트가 가장 큰
SCC는 위 client/types 한 파일이다. checker 타입 인스턴스의 SCC를 측정한 결과는
아니며, references 도입만으로 큰 public consumer 비용이 해결된다고 판단하지 않는다.

영향: execution module 계획·파일 이름·fingerprint, selective/static/dynamic loader, named-client 공유, input/output wire schema 객체, 공개 Routes/Operations helper, bundler import graph, server addon, incremental manifest. `selective_modules.go`의 generation hash는 출력 변화에 맞게 갱신되어야 하며 이전 fingerprint를 고정하지 않는다. 서로 다른 generation provider 혼용 거절과 현재 ABI를 재검증한다.

### Go: package 배치와 파일 배치 분리

`emit.go`는 schema → models → 나머지 순으로 렌더링한다. schema/models만 모아서 partition하고 다른 directory는 즉시 sink로 보낸다. 따라서 `partitionPackageBound` 안에만 packer를 넣으면 threshold 미만 패키지와 `internal/operations` 등은 빠진다.

권장 위치는 최종 import binding과 partition 변환이 끝난 **원래 sink 앞의 공통 artifact 단계**다. 이 단계가 schema/model implementation, public facade, 비분할 패키지를 모두 처리한다. package path별 bounded buffer와 완료/flush 규약을 두고 비-Go artifact는 통과시킨다. 실패는 기존 publisher의 staging/rollback 경로로 전파한다. 단순한 package-map 전체 보관은 현재 메모리 경계를 깨므로 채택하지 않는다.

초기 pack 대상은 compiler가 소유한 안전한 함수/type/const 선언 파일이다. `init`, package 변수, build/GOOS/GOARCH/test 파일, cgo, blank/dot import, `//go:*`, `//line`, embed/linkname 등의 directive는 별도로 유지한다. `iota`와 const 선언 묶음, doc comment를 보존한다. import union과 alias 변경은 식별자의 실제 binding과 local shadowing을 보존해야 하며 문자열 치환으로 처리하지 않는다. 일반 Go 소스 병합기로 범위를 넓히지 않는다.

64/128/256 KiB는 실험 후보일 뿐 기본값이 아니다. 마지막 청크 외 고정 묶음 또는 안정된 hash bucket을 비교하고 operation 하나 추가 시 변경 파일·재컴파일 범위를 측정한다. 순차 번호 전체 재배치로 incremental 이득을 잃는 구현은 재검토한다.

public model 타입의 정의 패키지는 그대로 유지한다. **validator/codec 본문 공유도 출력 바이트를 바꿔 기존 크기 기반 partition 결과를 이동시킬 수 있다.** 파일 묶기를 partition 뒤에 놓는 것만으로 후속 최적화의 identity 위험이 해소되지는 않는다. W07은 전후 symbol→implementation-package와 reflect identity를 비교하고, 변경 시 기존 placement를 고정한 동일 그룹 내 공유 방식부터 검토한다. 새 안정 partition 정책으로 public defined type을 옮기는 변경은 별도 호환성 결정이 필요하다.

validator는 lowering 후 알고리즘·ref binding·dialect·direction·media·오류·재귀 프로그램 identity가 같은 경우만 후보로 삼는다. 이미 있는 view sharing을 재사용한다. context, path, State, `validationVisit.Program`, media ancestor, evaluation의 evaluated-property/item 정보를 유지한다. 공통 helper가 원래 프로그램 이름을 대체하여 재귀 감지나 오류 위치를 합치면 실패다. tiny hot-path helper와 JSON/media 경로는 별도로 판단한다.

영향: emit/partition/import rewrite, artifact path와 manifest ownership, runtime error·codec·cancel·stream, Go export data·compile cache·reflect, SDK 파일 수/원본 배포 해시. private identifier 단축·codec 구조 공유·domain import entry는 후속 후보이며 이번 계획에서 무조건 구현하지 않는다. 현재 Go runtime의 표준 라이브러리 전용 경계도 유지한다.

## 대안과 보류

| 대안 | 결정 근거 | 처리 |
| --- | --- | --- |
| 전역 schema map 한 개 | 출력은 줄지만 선택한 operation 밖의 schema import·fallback 경계를 넓힌다. | 제외 |
| closure별 정확한 공유 + delta | 현재 계획·loader 계약을 유지하며 반복 import/entry를 직접 줄인다. | 우선 |
| 모든 schema/operation를 각각 TS project로 분할 | shared declarations 반복 검사·프로젝트 수·거대 SCC 비용이 남는다. | 제외 |
| 이름 있는 public declaration + SCC 인식 references | implementation 의존성 단절에 효과가 있을 수 있다. 최종 consumer가 전체 declarations를 다시 로드하면 여전히 OOM일 수 있다. | W08 조건부 |
| 같은 Go package의 안전한 선언 묶기 | public import와 type identity를 보존하면서 file overhead를 줄인다. AST 전체 크기는 거의 유지된다. | 우선 |
| validator interpreter/Reflection DTO 전환 | hot-path, 오류, 타입 정체성 변화가 크고 실측이 없다. | 보류 |
| 새로운 Go domain entrypoint | 선택 소비자 비용에는 도움이 될 수 있으나 전체 생성/전체 컴파일 해결과 다르다. public API·배포 선택도 추가된다. | 별도 결정까지 보류 |

## 공식 자료 대조

- [TypeScript Performance](https://github.com/microsoft/TypeScript/wiki/Performance): 이름 있는 복합 타입의 캐시 이점, 진단·trace, project 분할의 반복 검사 비용을 설명한다. 이를 Graph OOM 원인 확정으로 해석하지 않는다.
- [Project References](https://www.typescriptlang.org/docs/handbook/project-references.html): referenced project의 declaration 소비, composite/declaration, 전체 implementation 포함과 `tsc -b` 동작을 확인했다. 단순 `.d.ts` 검사만으로 원본 검사를 대체하지 않는다.
- [Go package initialization](https://go.dev/ref/spec#Package_initialization): 여러 파일의 선언 순서가 compiler에 전달된 파일 순서에 달려 있다. 파일 병합의 초기화 제외 조건을 뒷받침한다.
- [Go compiler options](https://go.dev/src/cmd/compile/doc.go): package별 CPU/memory profile과 최적화 진단의 근거다. profiling 결과와 대표 timing 표본을 섞지 않는다.

## 실행한 검증과 남은 범위

원시 JSON 검산, Graph 입력 digest, CI 보고서 digest, 관련 제품 코드 동일성, GitHub Graph job의 skipped 검사 단계를 확인했다. 별도 Node 24.22.0 재현에서 빈 키·정수형 키·`__proto__`·`constructor`·Unicode 키의 own descriptor/reference/order 보존을 확인했다. UTF-8 byte 순서와 JS UTF-16 정렬 순서가 다른 입력도 확인했다. Go 1.27.2의 별도 모듈에서는 `[A Z] 1 2`가 무차별 병합 후 `[Z A] 2 1`로 바뀌어 초기화 순서 위험을 재현했다.

기존 TypeScript 생성기·Go 소비자 검사는 `GOTOOLCHAIN=go1.27.2 devtools run ci:dev ...`로 실행한다. 최초 `test:dev`는 잠긴 vault 때문에 실행되지 않았다. 비밀값 주입 없는 기존 `ci:dev`를 사용했으며 vault나 프로젝트 설정을 변경하지 않았다. 셸 기본 Go는 1.27.3이어서 정확한 1.27.2를 명시했다.

최종 실행 결과, 리뷰 기록, workstream ID와 작업 ID는 `verification.md` 및 `workstream.json`에 기록한다. 초기 계획은 실제 Graph full strict·Go compilation을 필수로 두었지만, 2026-10-22 사용자 결정으로 전체 Graph는 참고 검증으로 전환했다. 현재 필수인 대표·Graph 선택 SDK와 일반 제공자 검증은 `plan.md`를 따른다. 이번 초기 조사에서 제품 검증을 완료한 것으로 집계하지 않는다.
