# 구현 전 조사 보완 기록

조사일: 2026-10-21. 범위: W01의 측정 경로·실제 기준 출력 조사와 구현 계획 보완.
제품 최적화 W02 이후는 착수하지 않았다. W01 전체 수용 기준의 완료 여부는
아래 남은 조건과 devtools validation으로 판단한다.

## 확인한 기준과 실행 조건

- 제품 기준은 `5ada624a69ebfd85103248379970f78a8f3d2899`다. 현재 제품 소스와 CI
  `60de0a577051b3d751861816198523801c5bc092`의 `cmd`, `internal`, `go.mod`, `go.sum`,
  `scripts/golang-compat`, corpus manifest 네 개의 diff가 없다. 새 변경은 내부 도구와 계획이다.
- Graph 원본 SHA-256은 `46bead9a6459cbe5f66d31094b23a614eb8da8f46690ee61404d97523e23daec`다.
  generator binary SHA-256은 `1f07cb725748cac7e73c927a26bd408ba857b0bbba48243c7f65ee0ba858da25`다.
- Go 1.27.2의 실제 binary를 확인·해시하고 같은 binary로 실행했다. Node는 24.22.0,
  TypeScript는 5.7.3·5.9.3·6.0.3·7.0.2다. TS7은 native checker와 Node wrapper를 함께 관측했다.
- 로컬 smoke 정책: 생성 900초, checker/build 600초, session RSS 합 8 GiB, Node heap
  7 GiB, GOMAXPROCS=4, TS7 `singleThreaded`. Go Graph 생성에는 `GOMEMLIMIT=6GiB`,
  server addon을 명시했다. 이는 최종 CI 예산이나 제품 SLA가 아니다.
- 생성과 checker는 별도 프로세스다. RSS는 200 ms 간격 session 합이며 shared page가
  중복될 수 있고 순간 peak를 놓칠 수 있다. worker가 별도 process group을 만들더라도
  같은 session이면 중단한다. 별도 session으로 daemonize한 작업은 지원하지 않는다.
- 첫 TS 생성 시도는 `--typecheck` 누락을 발견해 SIGTERM으로 중단했다. 54.01초의
  중단 표본은 보존했고 성공·성능 표본에서 제외했다. strict 생성 초반에 잠깐 겹쳤으므로
  아래 생성 시간으로 독립 성능 비교나 개선율을 계산하지 않는다.

## 실제 Graph TypeScript 기준 결과

`--typecheck`로 재생성한 186,448개 TS source 전체를 명시적인 `files`로 검사했다.
canonical strict 옵션을 사용했고 `@ts-nocheck` 파일과 unresolved relative import는
각각 0개였다. publisher manifest와 inventory의 186,448개 파일을 모두 대조해
누락·추가·hash 불일치가 각각 0개임을 확인했다.
원래 helper resource omission 경고는 1,208개였다.

| 단계 | 결과 | 시간 | 관측 session RSS peak |
| --- | --- | ---: | ---: |
| 전체 strict 소스 생성 | pass | 171.05 s | 3,109,306,368 B |
| TS 5.7.3 | SIGABRT, JS heap 부족 | 108.86 s | 8,012,361,728 B |
| TS 5.9.3 | SIGABRT, JS heap 부족 | 91.93 s | 8,029,855,744 B |
| TS 6.0.3 | SIGABRT, JS heap 부족 | 85.29 s | 8,014,041,088 B |
| TS 7.0.2 native | RSS 보호 종료, SIGKILL | 53.57 s | 8,659,902,464 B |
| 전체 source/import/closure inventory | pass | 133.67 s | 1,804,767,232 B |

네 시도 모두 전체 strict 검사 미완료다. 이 예산의 실패를 모든 환경에서의 불가능성으로
일반화하지 않는다. checker가 완료되지 않아 parse/bind/check의 최종 extended diagnostics도
없다. 따라서 특정 semantic helper를 최종 병목으로 확정하지 않는다. 전체 public consumer
검사 성공도 없다. native 관측에는 `tsc` binary의 SHA-256
`4f2de6782864…`가 포함되며 Node heap 측정으로 대체하지 않았다.

최초 5.7 trial은 compiler library 전체 hash 자동 수집을 추가하기 전에 실행했다.
Node binary·`tsc.js`·tsconfig hash는 당시 기록에 있지만 `_tsc.js` 등 전체 compiler
library의 실행 시점 증거는 부족하다. 후속 trial과 새 도구는 이를 기록한다. 동일 compiler
디렉터리를 사용했다는 사실을 소급해 완전한 toolchain manifest였다고 주장하지 않는다.

## 실제 출력 분포와 설계 영향

전체 TS bytes는 2,358,735,553 B다. 기존 exact closure 공유 후에도 map 2,272개에
8,966,127개의 entry가 남았다. input map 713개, output map 1,559개이며 map당 entry
중앙값 6,885개, 최대 6,912개다. projected descriptor 15,699개가 1,895개의 사용 집합으로
나뉜다. 반복 entry 8,950,428개는 공유 기회의 수이며 제거 가능한 순 바이트가 아니다.
map statement는 547,490,211 B, 대응 descriptor import는 1,016,938,206 B였다.
새 helper·binding·import·delta·map 조립 비용을 차감한 실제 후보 출력이 필요하다.

| 큰 단일 파일 | 바이트 | 계획 영향 |
| --- | ---: | --- |
| `internal/client/types.ts` | 42,655,818 | W03의 공개 타입 계약 분할 추가 |
| `internal/client/registry.ts` | 39,808,834 | W03 단계별 함수·canonical state |
| `internal/routes/index.ts` | 36,506,796 | W03 public facade·타입 모듈 분리 |
| `selective/types.ts` | 33,439,373 | W03 full/selective 계약 일치 검사 |
| `internal/routes/helpers.ts` | 14,216,547 | W04 semantic 비용을 실제 측정한 뒤 section index 판단 |
| `internal/routes/inputs.ts` | 11,020,710 | 선언 consumer 비용과 함께 관측 |

import SCC는 총 186,322개이며 그중 44개가 여러 파일을 포함한다. 가장 많은 파일과 가장
큰 순환 바이트를 가진 SCC는 35파일·328,670 B다. 바이트가 가장 큰 SCC는 42,655,818 B의
client/types 한 파일이다. 원시 `scc.json`의 모든 component를 집계했다. 초기 summary의
`largestSccFiles=1`은 바이트순 첫 component의 파일 수였으므로 전체 최대 파일 수로
해석하면 안 된다. 도구를 고쳐 두 최대값을 구분했고 큰 비순환 파일을 넣은 회귀 검사를 추가했다.
이 자료는 type-instantiation graph가 아니다. references 후보는 declaration·consumer
전체 비용까지 검증해야 하며 W03을 건너뛸 근거가 아니다.

## Go 기준 근거와 보존 계약

저장된 [Actions 기준 결과](https://github.com/jinyongp/openapi-sdkgen/actions/runs/39456862841)의
Go 보고서 해제 SHA-256은 `417c420bc3fccd5fdb6048b9c8fa403dd8ea47f5268f6ffff2cfa83e85e9d0b7`로
provenance와 일치한다. corpus 문서 38개 모두 현재 로컬 입력 SHA-256과 일치하고,
holdout 20개는 Git blob pin도 일치했다. 보고서는 모두 Go 1.27.2, server addon,
compile pass다. 실제 API 문서 29개·fixture 9개이며 Graph는 포함하지 않는다.
제품 소스 동일성이 확인됐으므로 compile correctness만 재사용한다. 서로 다른 실행
환경의 시간·RSS를 새 로컬 성능 기준선으로 합치지 않는다. W09 후보에는 재검증이 필요하다.

전체 Graph Go 생성·빌드·안전 후보율의 새 결과는 별도로 기록한다. 과거 477,701파일·
3,195,326,552 B는 EmitTo render/format/hash 결과이며 디스크 게시와 compile을 포함하지
않았다는 점도 확인했다. 새 CLI 게시 측정과 과거 수치의 시간 비율을 계산하지 않는다.

새 server 포함 전체 CLI 생성은 **900.68초 timeout**, 관측 peak 6,411,071,488 B였다.
완성된 API 출력이 게시되지 않았으며 stage 일부만 남았다. 따라서 Graph 전체 compile과
전체 안전 후보율 audit는 **not-run**이다. 일부 stage를 완성된 baseline으로 승격하지 않는다.
이 결과만으로 Go compiler OOM이나 publisher가 근본 원인이라고 확정할 수 없다.
`GOMEMLIMIT=6GiB`는 소프트 제한이며 GC 비용에 영향을 줄 수 있다. 과거 309초 측정의
EmitTo 경계와 이번 디스크 게시·메모리 조건은 다르다. 다음 진단은 compile/prepare/emit/
format/partition/publish 시간과 GC를 분리해 기록하고, renderer의 최종 sink에서 inventory를
수집하는 경로와 실제 게시 경로를 구분한다. 생성 예산은 compiler의 6 GiB 정책과 별개로
사전 확정하며 실제로 생성한 전체 출력만 compile 대상으로 삼는다.

작은 실제 Go todo 예제는 server 포함 생성·전체 package compile이 통과했다. audit는
173파일·1,653,688 B·21 package, 안전 후보 159파일·618,805 B, 같은 package/import
집합에 둘 이상 모인 후보 61파일을 기록했다. 정의 타입·public receiver/signature·tag
inventory도 남았다. 이것은 Go Graph의 안전 후보율이나 실제 pack 효과가 아니다.

## 새로 발견한 기준 코드의 타입 오류

Go runtime fixture를 TS로 생성해 작은 public consumer를 검사하는 과정에서 네 compiler
모두 `internal/schemas/node.ts`의 Input/Output에 **TS2456**을 냈다. negative consumer에서
missing operation 오류가 함께 났어도 positive가 실패했으므로 정상 계약 검증 pass가 아니다.
`probes/recursive-property.json`으로 축소했고 네 compiler에서 동일 오류를 다시 확인했다.

`types.go`가 additional index 값에 고정 property의 타입도 합친다. `next`의 self 참조가 이
union에 들어가지만 `objectIndexHasSelfReference`는 additionalProperties와 patternProperties만
검사한다. 그 결과 `Record<string, ... Input ...>`를 통한 순환 alias가 남는다. 기존 재귀
회귀 검사는 additional/pattern 쪽 참조를 다루며 이 조합을 놓쳤다. W01R을 별도 선행
correctness 작업으로 등록하고 후속 TS 성능 비교는 그 수정 소스를 공통 기준으로 삼는다.
현재 제품 코드는 수정하지 않았다. Graph에도 동일 오류가 있다는 검증은 아직 없다.

하네스의 정상/잘못된 호출 smoke는 기존 TypeScript todo 예제로 별도 수행했다.
네 compiler에서 정상 `listTodos` consumer는 통과하고 `missingOperation`은 TS2339로
거절됐다. runtime fixture의 실패 기록은 그대로 유지하며 다른 입력의 성공으로 덮지 않는다.

## 도구 교정과 리뷰

`4251698`에서 내부 `preflight:perf`와 read-only inventory를 추가했다. v2 외부 도구를
제품 구현으로 채택한 변경이 아니다. block/fail/signal/timeout/protection은 nonzero이고,
확인한 Go binary와 실제 실행 binary가 같다. `_`·`.` 파일은 안전한 Go 후보에서 제외된다.
공개 AST signature·receiver·JSON tag·defined type의 최종 package 경로를 기록한다.

첫 리뷰에서 Go receiver 출력의 FieldList 처리 오류를 고쳐 재검증했다. 두 번째 리뷰에서는
별도 process group 자식의 중단 누락, 빠진 입력의 blocked 기록, SCC 최대값 혼동을 고쳤다.
변경된 runner·caller·출력 해석을 다시 확인했다. 최종 recorder 행동 검사 5개, Go audit의
무시 파일·init·변수·platform·directive·정상 선언 7개 subcase, TS inventory/config/SCC와
no-check 거절, JS key/descriptor/prototype/UTF-8 의미 재현이 통과했다.

## 구현 착수 전에 남는 조건

1. 전체 Graph Go 생성 timeout의 단계·GC 진단과 최종 package inventory를 확보한다. 완료된
   출력이 없으면 안전 후보율의 분모를 확정할 수 없다. 실패한 stage의 일부 파일만으로
   full Graph의 pack 이득을 계산하지 않는다. build를 못 했으면 compile not-run으로 남긴다.
2. W01의 full public/negative·선택 import graph 계약을 실행 가능한 입력과 연결한다.
   작은 생성 SDK의 올바른 호출과 잘못된 operation ID 검사는 하네스 smoke이며 전체
   Graph·named/selective·reflection/error 계약의 대체 증거가 아니다.
3. W01R에서 재귀 property 타입 오류를 먼저 해결하고 TS 기준 소스를 다시 고정한다.
   Go validator는 기존 view sharing 뒤 남은 실제 lowered 프로그램의 의미 중복률을
   W07에서 계량한다. 이름을 지운 AST나 문자열 중복을 의미 동일성으로 취급하지 않는다.
4. 실제 hot-path·HTTP/runtime의 기준 분산과 허용 폭을 W07 후보 측정 전에 확정한다.
   +5%는 추가 조사 경고선, 15% 최소 이득은 채택 gate로 사용하지 않는다. v2 합성의
   불확실성으로 성능 비열등성을 승인하지 않는다.
5. 최종 CI의 RAM/cgroup·reserve·최대 wall budget을 후보 실행 전에 기록한다. local
   8 GiB 실패를 숨기려고 검사 제외나 `noCheck`/`skipLibCheck`로 수용 기준을 낮추지 않는다.

각 단위는 리뷰·검사·커밋하고 마지막 W11 통합 리뷰를 유지한다. 이번 선행 보완의
통합 리뷰와 workstream 정의 대조는 `verification.md`에 기록한다.
