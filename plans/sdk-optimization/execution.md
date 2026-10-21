# Workstream 실행 기록

## W01 공통·TS 선행 경로

기준 제품은 `5ada624`와 같다. 기존 `preflight.md`의 실제 TS 네 compiler 실패와
186,448파일의 누락·추가·hash 불일치 0 결과를 재사용한다. 입력·제품 소스·옵션이 같다.
현재 요청은 workstream 전체 구현을 승인했으므로 이후 작업별 수정·검사·리뷰·커밋을 진행한다.

`public-consumer.mjs`로 원본 Graph의 route/operation ID를 각각 29,581개 동결했다.
Link surface는 원본에 없으며 그 부재도 보존한다. 후보 consumer는 모든 SDK source와
정확한 public key 동등성·unknown route/ID 거절을 함께 검사한다. 실제 todo SDK에서
실행 경로를 확인했다. full Graph 성공은 이 도구 준비로 대체하지 않는다.

Go 전용 미완료 항목은 W01G와 전용 validation으로 옮겼다. 전체 package/symbol
inventory·safe pack 후보율·Graph compile 시도·reflection/runtime 기준을 그대로 요구한다.
Go W06/W07은 W01G에 의존하고, TS correctness W01R은 공통 W01에 의존한다.
이 분리는 계약을 삭제하지 않으며 최종 Graph TS/Go 성공 수용 기준도 유지한다.

정책은 `execution-policy.md`에 고정했다. 소프트 생성 heap과 compiler heap을 구분하고
프로파일을 일반 timing에서 제외한다. validator runtime 공유의 내부 채택 기준과
반복 수·불확실성·보류 조건을 사전 기록했다. 기존 대표 consumer/runtime/partition
회귀에 계약을 연결하며 관련 소스가 같은 과거 통과 근거만 재사용한다.

단위 리뷰에서 public consumer의 Link surface 부재 처리 오류를 수정했다. 원래 Graph에
없는 `$links`를 요구하지 않으며, Link surface가 있는 SDK의 exact key 검사는 유지한다.
Go phase helper는 실제 todo 생성의 최종 174 artifact(Go 173파일) hash를 publisher manifest와 대조했다.
제품 코드의 semantic 변경은 아직 없다. Go 단계 진단 결과는 W01G에 추가한다.

## W01R 재귀 property의 index union

`propertyIndexTypes`는 고정 property 타입도 추가 속성 index의 union에 포함한다.
기존 재귀 감지는 additional/pattern만 보아 직접·중첩·상호 재귀 property를 놓쳤다.
감지 대상에 고정 property를 포함해 기존 deferred index 표현을 사용한다.
새 타입 우회나 runtime 변경은 없다. input readonly와 output mutable을 유지한다.

축소 원본과 기존 runtime performance fixture는 각각 네 compiler 전체 source strict
검사를 통과했다(8/8). 직접·중첩·상호 재귀의 허용 값, 잘못된 property/additional 값,
필수 값 누락과 readonly 쓰기 거절을 회귀 검사에 추가했다. 기존 composition/pattern/
JSON Value 재귀 검사와 전체 `ci:dev typescript-generator`도 통과했다.
원시 결과는 `.tmp/sdk-optimization-execution/{recursive,runtime}-fixed-*`와
`.tmp/ci-tests/`에 있다. 단위 리뷰에서 index union을 구성하는 caller와 재귀 DFS의
visited 범위를 대조했다. 이 correctness 커밋을 후속 TS 성능 비교의 공통 기준으로 쓴다.

## W01G Go 최종 산출물과 실패 기준

고정 Graph의 server 포함 실제 publisher 출력은 659,664 artifact, 4,045,045,462바이트다.
CPU profile을 켠 진단은 1,385.66초·session 최대 RSS 8,555,765,760바이트에 완료됐다.
compile 12.82초, prepare 완료 137.83초, emit 완료 1,383.42초,
publish 완료 1,384.40초다. profile과 일부 correctness 검사 실행이 포함되어
일반 timing 또는 기존 옵션이 다른 477,701파일 결과와의 개선율에 사용하지 않는다.

최종 Go 파일 659,663개를 audit했다. 기본 8 MiB 상한에서 빠진
`client.go` 55,629,305바이트와 `operations/types.go` 29,377,260바이트는
`-only-large -max-file-bytes 0` 별도 bounded trial로 전체 AST를 확인했다.
각각 공개 signature 253,349개와 194,183개이며 후속 package/identity 비교에서 빠뜨리지 않는다.
대형 파일의 package 이름을 원래 inventory에 합치면 실제 package는 588개다.
첫 audit의 590은 미파싱 파일의 빈 package 이름 두 개를 별도 집계한 값이므로 정정한다.
두 대형 파일을 제외한 동일 최종 package·import 집합의 묶기 후보는 545,220파일이다.
이는 구현·컴파일 성공 또는 실제 절감량의 근거가 아니다.

Go 1.27.2, server 포함 전체 `./api/...`, CGO=0·GOWORK=off·GOMAXPROCS=4,
`-p=1`, compiler soft heap 6 GiB, fresh 전용 GOCACHE, 3,600초·session RSS 9 GiB
정책으로 기준 compile을 시도했다. 255.71초에 RSS 보호가 작동했고 관측 최대치는
9,665,454,080바이트다. 미완료 실패로 보존하며 최종 W09/W11 성공 기준은 유지한다.
원시 phase/profile·최종 파일 hash·기본/대형 AST inventory·실패 compile은 모두
`.tmp/sdk-optimization-execution/graph-go-*`에 있다. runtime 기준 표본은 아래에서 보완했다.

Go runtime 최초 실행은 URL benchmark의 세 기대값에서 실패했다. 현재 제품은
URI/IRI 문법을 구분하지만 benchmark는 IRI 사례에서 Unicode 허용 인자를 생략했고,
`reg-name`의 유효한 percent encoding을 URL parser 정책으로 거절하길 기대했다.
기존 format 회귀와 [RFC 3986 §3.2.2](https://www.rfc-editor.org/rfc/rfc3986#section-3.2.2),
[RFC 3987 §2.2](https://www.rfc-editor.org/rfc/rfc3987#section-2.2)를 대조해 측정 기대값을
수정했다. Unicode URI 거절·잘못된 percent escape 거절도 함께 측정하며 제품은 바꾸지 않았다.
최초 실패 원본은 `go-runtime-baseline`에 남긴다.

새 `go-runtime-baseline-2`는 Go 1.27.2에서 전체 9단계 pass다. 실제 schema 활성
JSON/XML/multipart/body lease/stream/regex/concurrency/URI·IRI/NFC 320 benchmark
표본(64 case×5), stream/body/large JSON live heap, 별도 contention profile과 race를 확보했다.
profile·forced GC 결과는 ordinary timing에 섞지 않는다. 기존 helper는 child command를
별도 session으로 실행하므로 바깥 recorder RSS를 전체 runtime peak로 사용하지 않는다.
각 command의 GNU time 및 재귀 process RSS 기록을 사용하며 helper의 7 GiB 보호가 적용된다.
Go 제품·38문서 입력이 기준 소스와 같아 기존 server 포함 38/38 correctness와
157 consumer의 package/alias/reflect/error/media/cancel/selection 기준 검사를 재사용한다.
최종 최적화 후보의 38문서·Graph와 runtime은 W09에서 새로 확인한다.

## W02 projected closure 부분 공유

TS 공통 correctness 기준은 `25d5626`이다. 새 planner는 projection별 descriptor의
정확한 소비 closure 집합을 묶는다. 64개 이상 descriptor를 두 개 이상의 큰 closure가
함께 쓰는 경우에만 block을 만들고, 각 closure는 그 부분집합과 delta만 import한다.
쌍별 교집합이나 전역 superset은 없다. exact closure의 모듈과 map identity를 유지한다.
Go에서 정한 ordinal을 heap으로 merge하여 UTF-8 순서·JS 숫자 own key 순서와
`__proto__`/`constructor`의 일반 data property·prototype을 보존한다.

리뷰에서 projection을 바꿔 순회할 때 이전 projection의 delta를 다시 추가하던 결함과
strict unused/indexed access 오류를 수정했다. input/output·중첩 순환·dynamic/Link·선택
caller, collision 등록, 두 단계 fingerprint hashing을 대조했다. 빈/단일/분리/작은 교집합,
여러 interleaved block, projection, descriptor/map identity·property flags·두 client 격리·
실제 잘못된 요청 거절 및 결정적 재방출을 검사했다. 전체 생성기 519 test pass,
마지막 추가 assertion을 포함한 새 행동 검사와 네 compiler whole-source strict도 통과했다.

실제 full Graph 후보는 TS 187,198파일·789,774,425바이트다. 기존
186,448파일·2,358,735,553바이트 대비 파일 750개 증가, 소스 66.52% 감소다.
helper/import/delta를 포함한 실측이며 누락 import와 `@ts-nocheck`는 0이다.
새 merge 호출은 기존 inventory의 `Object.fromEntries` 패턴 수에 포함되지 않으므로
그 도구의 map 감소를 runtime map 개수 감소로 해석하지 않는다.
생성 141.97초·RSS 3,447,164,928바이트는 단일 진단이다. 기존 생성과 heap 정책 등이
달라 timing 개선율로 사용하지 않는다. 전체 공개 29,581 route/ID consumer를 포함한
TS 5.9 strict는 100.23초·관측 RSS 8,032,882,688바이트에 Node heap 부족으로 종료했다.
이 실패는 W03/W05/W08에서 해결하며 소스 절감으로 전체 성공을 대체하지 않는다.

todo와 고정 GitHub의 동일 binary/도구/옵션·직렬 교차 순서 5쌍은 생성·전체 strict
각 20/20 pass다. 둘은 공유가 켜지지 않아 source byte/manifest가 같고, 작은 timing 차이를
개선으로 주장하지 않는다. checker 중앙값은 todo 1.243/1.243초, GitHub 11.670/11.882초
(기준/후보)이며 GitHub 기준 범위가 11.236~17.717초로 넓다. JS/gzip은 기존 하네스의
full/static loader/root/named 14비교가 모두 같다. Resend는 해당 하네스가 server add-on을
설정하지 않아 E505로 막혔으며 그 실패를 남기고 지원되는 두 입력을 측정했다.

실제 공유를 쓰는 Graph의 첫 두 distinct output closure provider를 결정적으로 선택해
runtime 전체 import를 포함하여 bundle했다. JS 3,789,053→3,465,791바이트,
gzip 485,978→451,989바이트다. fresh Node module 초기화 10쌍의 geometric mean ratio
0.9468, paired bootstrap 95% 0.9059~0.9930으로 사전 1.05 상한 기준을 통과했다.
첫 warmup pair는 별도 보존했다. filesystem cache는 warm/미지정이며 HTTP hot path
측정이나 full Graph checker 성공을 뜻하지 않는다. 원시 표본·도구/source/binary hash는
`.tmp/sdk-optimization-execution/{closure-paired,closure-provider-size-2,graph-closure-startup,graph-ts-closure-*}`에 있다.
동일 Graph 초기화의 timing 이후 GC로 측정한 retained heap 중앙값도
24,792,424→23,876,064바이트로 감소했다. B/op·HTTP ns/op 결과로 해석하지 않는다.

## W03a registry의 단계별 청크

512개를 넘는 registry는 현재 256 operation 단위의 준비 모듈과 ID 등록 모듈을 사용한다.
작은 SDK의 기존 경로는 유지한다. 청크는 자기 base/pagination/stream 값을 closure로
보관하고 한 개의 `PreparedRegistryChunk` 계약을 구현한다. Root는 모든 base 준비 후
모든 Link 연결, 모든 decoration, 정렬 route 등록, public 노출, 정렬 ID/Link 등록 순으로
실행한다. 완료 map은 client마다 하나이고 Link closure들이 같은 map을 참조한다.
ID 정렬은 route 청크 순서와 별도 계획하여 Go의 기존 key 순서를 유지한다.

공개 호출 map과 내부 완료 map은 기존 Routes/OperationRoutes의 mapped view로 유도한다.
모듈 경로는 준비 단계에서 collision 검사에 등록하고, actual root의 retained operation만
해당 모듈에 방출한다. named union에서 root가 private로 쓰는 route를 공개하지 않는다.
리뷰 중 private-only 청크에서 불필요한 CallableRegistry import가 생기는 문제를 수정했다.

700 일반 operation과 300 private Link chain, 경계를 넘는 Link 순환·ID 없는 route·hidden·
Unicode/숫자/`__proto__`/`constructor` 키를 실제로 생성·strict compile·실행했다.
full 및 private/named root에서 정렬 map·callable identity·두 client 격리·Link 실행·stream·
pagination과 잘못된 호출 거절을 확인했다. deterministic 재방출도 같다.
실제 고정 GitHub 전체 source와 기준 public/negative consumer는 네 compiler 모두 pass다.
원시 결과는 `registry-chunk-behavior-4`, `registry-github-*`에 있다.
청크 128/256/512 비교와 대형 공개 타입 분할, full Graph 평가는 이어지는 W03/W05에 남는다.

## W03b 공개 타입과 selective resource 타입의 분할

큰 full SDK의 Client·selective 호출 map은 canonical Routes/OperationRoutes에서 유도한다.
route/ID/input/raw 계약은 기존 public facade 뒤의 256 operation 단위 타입 모듈에 둔다.
generic selective resource는 128 node 단위로 나누며 child 방향의 참조만 만든다.
공통 helper는 RouteKey를 canonical Routes에서 읽어 resource module 간 순환을 만들지 않는다.
준비 단계에서 소유권과 artifact 경로를 등록한다. scoped/named client는 기존의 좁은
타입 import를 유지하며 문서 전체 map으로 바꾸지 않는다.

리뷰에서 각 resource 청크마다 전체 tree를 다시 수집하던 비용을 제거하고 prepared
node/ID plan을 재사용했다. 기존 wide-resource 표현 검사는 새로 방출된 전체 타입 모듈에
연결했다. 대형 consumer에서 guaranteed/optional/empty 선택, 미선택 resource 거절,
readonly input·mutable output과 구체 값 타입을 확인했다. 전체 생성기 520 test pass다.
마지막 focused full SDK 검사는 네 compiler의 strict source/declaration emit 및 원본 TS를
볼 수 없는 declaration-only consumer와 실제 runtime까지 통과했다.

추가 compiler symbol 리뷰에서 mapped property가 기존 `@example`을 잃는 문제를 찾았다.
canonical route/ID 문서 원본에 기존 call JSDoc을 두어 수정했고 route/ID hover의 HTTP
설명과 example을 실제 symbol에서 재확인하는 회귀를 추가했다.
`public-type-final-declarations-hover`는 이 수정 후 83.88초에 pass했다.

최초 결합 Graph는 187,686파일·722,122,359바이트, import 누락/`@ts-nocheck` 0이다.
client types 957바이트, registry 42,946바이트, route index 25,298바이트, input facade
22,362바이트, helper facade 39,045바이트, selective types 2,989바이트로 분리됐다.
registry 모듈 최대 328,158바이트, route 타입 모듈 최대 864,603바이트,
resource 타입 모듈 최대 273,528바이트다. import SCC는 기존과 같은 44개 비단일 component,
최대 35파일이며 가장 큰 단일 component는 4,841,854바이트다.
이는 example 문서 보존 수정 전 진단 수치다. 최종 source byte와 성능은 후속 최종 basis에서
재확인하며 이 수치를 최종 산출물 수치로 재사용하지 않는다.

실제 GitHub에서 prototype 청크 128/256과 512/256을 각 5쌍 비교했다. 모든 생성·전체
strict·원래 ID→route 참조 검사가 통과했다. 생성 중앙값은 128/256 4.918/4.924초,
512/256 4.918/4.920초다. checker는 각각 11.867/12.065초, 12.073/11.678초이며
표본 범위가 겹친다. 첫 generation의 1.317 ratio 등 outlier도 그대로 남겼다.
속도 우열을 확정하지 않고 128보다 모듈 수를 줄이고 512보다 함수 크기를 낮추는 256을
유지한다. 이 비교는 example 문서 수정 전 후보이며 최종 성능 주장에 사용하지 않는다.

독립 consumer는 원본 Graph의 ID→route 29,581 연결도 동결하도록 강화했다.
원래 key 집합과 동일하며 하나의 route literal을 틀리게 넣은 probe가 TS2344·nonzero로
거절됐다. 새 mapped Client를 자동 freeze하려던 첫 측정 실패는 남기고, 후보 간 비교에는
원본에서 동결한 외부 reference를 명시하는 경로를 추가했다. `section-index-decision.md`의
C 보류와 별개로 최종 Graph 전체 strict는 여전히 미충족이다. 프로그램 구성 단계의 heap
실패 원본을 보존하며 W05/W08 fallback의 전체 source 소유권/strict·전체 public consumer를
검증해야 한다. 이 단계에서 workstream 완료로 처리하지 않는다.

W03 최종 basis `c355cdf`의 Graph를 다시 생성했다. 문서 보존 후 최종 TS는
187,686파일·731,936,445바이트로 원본 대비 68.97% 감소했다. 모든 source hash를
실제 publisher manifest와 대조해 누락·추가·불일치 0을 확인했다. import 누락과
`@ts-nocheck`도 0이고, helper omission 경고 원문 1,245,543바이트는 원본과 완전히 같다.
앞의 722,122,359바이트 수치는 최종 수치로 대체한다. 생성 130.18초·RSS
3,328,045,056바이트는 단일 진단이며 matched timing 개선율로 사용하지 않는다.

독립 public reference의 literal route/ID를 사용해 원래 `OperationMethod` field map도
재구성했다. candidate의 canonical operation identity 안에서 Client map과 양방향 호환을
검사하므로 SDK 사본마다 다른 nominal symbol을 혼동하지 않는다. 최종 실제 GitHub
전체 source·정확한 key/ID→route·legacy 호출 map은 네 compiler 모두 pass했다.
W03의 구조·계약 검증은 완료하고, 최종 Graph 전체 source/consumer strict는 W05/W08에서
실제로 완료해야 한다. 프로그램 구성 실패를 성공으로 바꾸거나 수용 기준에서 빼지 않는다.

## W05 전체 검사 실패와 소유권 진단

최종 W03 Graph의 TS7 단일 프로젝트 검사는 전체 source와 독립 public consumer를 포함해
실패했다. 첫 trial은 RSS 10,800 MiB를 설정해 직전 가용 RAM 기준 허용값을 조금 넘었으므로
정책 밖 진단으로 보존한다. 허용 범위 안인 10,600 MiB의 새 trial도 78.20초에 RSS
11,194,957,824바이트로 보호 종료됐다. 어느 결과도 strict 성공이나 성능 개선율이 아니다.
원시 결과는 `graph-ts-combined-final-typescript-7-smoke{,-2}`다.

`inventory.mjs`에 전체 파일·크기·정적/동적/type import 연결 export를 추가했다.
새 `project-plan.mjs`는 해시와 실제 source 목록을 대조하고 global script·augmentation·
triple-slash reference를 거절한 다음 SCC를 자르지 않는 DAG 소유권을 생성한다.
모든 source는 정확히 한 project에 속한다. `check-projects.py`는 fresh config·build-info·
declaration 디렉터리를 만들고 각 project를 strict 검사·declaration emit한다. compiler의
실제 `--listFiles`도 해당 project의 소유 source와 대조하므로 다른 project의 원본을
실수로 다시 읽는 경우를 성공으로 처리하지 않는다. 마지막에는 원본 TS가 없는 디렉터리의
전체 declaration과 독립 public consumer를 strict 검사한다. composite가 요구하는
incremental metadata는 매번 새로 만들며 이전 성공 캐시를 사용하지 않는다.

64 MiB/25,000파일 상한은 13개 project를 만들었다. TS7에서 앞의 8개가 pass했고,
9번째는 RSS 10,488,287,232바이트로 10,000 MiB 상한에서 보호 종료됐다. 완료된 declaration
80,939파일도 389,268,727바이트여서 declaration 의존성 비용을 무시할 수 없다.
이는 fallback 성공이 아니며 32 MiB 단위 및 scratch declaration의 comment 비용을
구분하는 후속 실험으로 연결한다. SDK 원본의 주석·문서·타입·공개 import는 수정하지 않는다.
원시 계획·프로젝트별 결과는 `graph-project-plan-64m-2`, `graph-project-native`에 있다.

도구 행동 검사는 JS/native compiler 모두에서 SCC·nominal symbol identity·전체 public
map을 검사하고, 내부 TS2322·변경/추가 source·augmentation·기존 output 재사용을 거절한다.
초기 두 fixture 기대값 오류와 수정 후 결과도 삭제하지 않는다. 이후의 W05 실험군/
server profile 분류는 아래 결과로 갱신하며 W08의 최종 전체 strict 성공은 계속 필수다.

W05 진단을 추가 완료했다. R을 다시 생성한 186,448개 artifact hash는 원래 기준선과
전부 같다. A는 187,198파일, B는 186,936파일, 최종 AB는 187,686파일,
실제 Graph의 AB-server는 187,748파일이다. 각 native 생성물의 publisher manifest와
전체 strict config의 source 집합이 정확히 일치한다. R/B의 5.9.3, AB의
5.7.3/5.9.3/6.0.3, AB-server의 네 compiler를 추가 실행했다. 새 strict trial 9개 중
8개는 Node heap 부족, server TS7은 RSS 보호 종료다. AB의 이전 최종 source TS7 실패도
보존한다. A의 앞선 5.9.3 및 원래 네 compiler 실패는 해당 당시 예산/consumer로 남긴다.
예산은 직전 가용 RAM에 따라 달라졌으므로 timing 개선율이나 동등 예산 승률을 계산하지 않는다.

JS 새 consumer는 원래 29,581 route/ID와 literal 연결·legacy call map 외에 실제 Graph
user 모델, 생성 request/response, query/body/parameter helper, union/never와 잘못된 입력을
사용한다. 타입 성공은 아직 확인하지 못했다. server export를 직접 사용하는 추가 probe도
W08의 최종 profile에 포함한다. C는 `section-index-decision.md`의 근거로 미채택이며
없는 C variant를 실행 또는 성공으로 집계하지 않는다.

32 MiB/reference 진단의 16번 project를 5.9.3 API로 다시 관측했다. 실제 project reference
15개와 1,848개 owner source에서 program 37,988파일(그중 declaration 36,140파일)을
구성했다. program 구성 31.76초/RSS 4.97 GB, binding 37.99초/RSS 6.13 GB까지 완료한 뒤
전체 diagnostics 수집에서 heap 부족으로 종료됐다. 이 구간은 semantic 검사 전후의
옵션/전역/선언 진단도 포함하므로 특정 predicate나 개별 파일을 CPU hotspot으로 확정하지
않는다. raw source 구성을 못 마친 이전 단일 프로젝트 실패와는 다른 관측이다.

`w05-verified-summary.json`은 실제 compiler identity·실패 원인·canonical strict·전체
source 집합·R hash 동일성과 위 phase 결과를 다시 대조한 결과다. 전체 Graph pass는
여전히 0이다. 다음 최소 후보와 유지할 계약은 `source-check-boundaries.md`에 정리했다.
W05는 실패 분류와 W08의 실행 경로 확보로 완료하며 AC-TS-FULL을 통과 처리하지 않는다.

## W06 최종 sink의 안전한 선언 묶기

`EmitTo`의 기존 partition/import binding 뒤 공통 sink에 native packer를 연결했다.
모델·schema 구현 및 facade·operations·비분할 package가 같은 경로를 거친다.
최종 package와 정확한 import 집합이 같은 파일만 묶고 선언 본문은 그대로 보존한다.
package 변수·init·directive·cgo/blank/dot import·platform/test/ignored 파일·큰 파일·
비생성 파일은 원래 경로로 전달한다. assembly/embedding 경계가 있는 package도 제외한다.

payload buffer는 총 8 MiB, 동시 group은 128개로 제한하고 가장 먼저 열린 group을
결정적으로 flush한다. 기본 target 128 KiB는 아직 비교 전 후보값이며 64/128/256 KiB
실제 비교에서 확정한다. 단독 파일은 원래 경로로 내보내고 묶인 파일의 경로는 최종
package/import 집합/첫 원래 경로의 전체 SHA-256으로 만든다. 중복 원래 경로와 원래/
묶인 경로의 양방향 충돌을 거절한다. 오류 뒤에는 flush나 sink write를 계속하지 않는다.
위 buffer 상한은 payload 기준이다. 모든 원래/최종 경로 중복 감지용 hash metadata는
artifact 수에 비례하며 전체 생성기의 메모리가 8 MiB라는 뜻이 아니다.

`go-pack-unit-3`은 실제 race consumer의 import/local shadowing·iota·초기화·alias/
defined type·method set·reflect PkgPath·JSON tag, 제외 규칙·sink 오류·중복/충돌·
buffer/group cap·큰 파일/비Go 전달·빈/final flush·반복 출력 검사를 통과했다.
`go-pack-callers`에서는 기존 immutable plan 재방출과 private symbol partition·binding·
모델/validator consumer를 다시 통과했다. 첫 trial의 CGO 설정과 충돌 fixture 오류는
보존했다. 기본 크기 비교·실제 Graph 최종 inventory·publication 증분/rollback·전체
corpus/Graph compile은 W06 후속/W09/W10에서 완료하며 이 단위 커밋만으로 통과 처리하지 않는다.

실제 작은 SDK의 추가/삭제 operation을 native Generator→packer→publisher로 생성해
incremental 교체·stale 제거·no-op `CheckManaged`·수정된 packed 파일 충돌·사용자 파일
보존을 검사했다. `go-pack-publication-tests`가 pass다. generic publisher의 실제 write/
rename failure rollback 검사는 최종 output suite에서 함께 확인한다.

크기 후보 진단용 `BenchmarkPackFinalOutput`은 동결 publisher manifest의 각 artifact를
해시 검증하고 native packer와 publisher로 replay한다. 절대 경로와 fresh output만
허용한다. 작은 실제 입력 `go-pack-small-replay-2`는 174→128 artifact,
1,649,027 output bytes·616,668 peak buffer bytes였다. 이 도구는 manifest 정렬 순서로
replay하므로 실제 emitter 순서·전체 generation 성능·최종 채택 크기의 대체 근거가 아니다.
큰 입력의 후보 진단 뒤 실제 최종 Generator 출력과 전체 compile을 별도로 검증한다.

## W08 registry/resource 타입 의존성의 국소화

registry 단계 메서드 shape를 runtime의 `PreparedRegistryChunkContract` 한 곳에 두었다.
root는 전체 call map의 view를, prepare/ID 청크는 자기 canonical operation Contract와
실제 LinkTargets의 view를 사용한다. LinkTargets intersection에서 implicit index signature를
추론하지 못하던 첫 strict 실패는 기존 `Mutable` mapped view로 해결했다. 값의 타입을
`any`나 새 broad index signature로 바꾸지 않았다.

resource factory도 자기 operation Contract의 call/resourceCall/pagination view와 child
factory의 registry view를 사용한다. 전체 client registry·route helper를 import하지 않는다.
공개 Surface 문서와 route/resource brand, root call binding·Link 순서·client별 state는 같다.

`narrow-registry-resource-behavior-3`은 네 compiler strict source·declaration emit·원본 TS가 없는
declaration consumer, 전체 native source/runtime, public hover, private/named·cross-chunk Link·stream/pagination을
통과했다. 추가 compiler-host 검사는 prepare/ID 청크와 leaf resource에서 전체 registry/
route facade를 사용할 수 없게 한 상태로 strict 검사했다. 단순 source 문자열 검사가 아니라
실제 import 경계 검사다. 전체 생성기·실제 최종 Graph 검사는 다음 단위에서 계속한다.

실행 세션 복구 과정에서 이전 Go 큰 replay의 `result.json`과 완료 manifest가 없고
프로세스도 남아 있지 않음을 확인했다. `graph-go-pack-128-replay/interruption.json`에
중단으로 기록했고 성공·timeout·제품 실패로 집계하지 않는다. 새 경로의 재실행에는 매 trial
직전 `min(요청 상한, 12 GiB, 가용 RAM - 2 GiB)`를 적용한다. task context는 관측한
이 workstream의 W05/W06/W07 run만 takeover해 workspace의 비공개 경로로 복구했다.

## W06 실제 Graph에서 발견한 facade 제외 수정

중단 후의 native replay는 659,664→369,519 artifact, 4,045,045,462→3,969,206,721바이트였다.
전체 package cold compile은 5,883 MiB 예산에서 RSS 6,223,519,744바이트로 보호 종료됐다.
package별 분포를 다시 보니 `models`의 181,927개 facade가 전혀 줄지 않았다.
기존 partition facade는 detached generated header를 제거하므로 marker 필터가 모두 제외한
원인이었다. 공통 sink 위치만으로 facade coverage가 입증되지 않는 실제 결함이다.

native `EmitTo`와 동결 native 출력 replay에서만 생성 출처를 명시하도록 고쳤다. public
파일·기존 사용자 파일은 publisher의 ownership 규약으로 분리하며 기존 파일을 packer에
넣지 않는다. 생성 출처가 있어도 license/header comment·directive·init/var·ignored/platform
파일 등 기존 제외 규칙은 유지한다. 단순히 모든 표식 없는 파일을 허용하지 않는다.

`go-pack-facade-tests`는 실제 native partition의 표식 없는 facade를 사용해 native EmitTo가
파일을 묶는지 확인했다. 원래 정의 package를 baseline AST에서 독립 동결하고 실제 Go
consumer의 reflect PkgPath·type alias·method set·JSON tag·값 동작을 대조해 통과했다.
수정된 원본/License header 보존·기존 pack 및 partition binding 회귀도 함께 통과했다.
새 큰 replay·compile와 기본 target 크기 비교는 계속 필요하다.

## W07 lowered validator 중복의 실제 계량

새 native AST 도구는 최종 package·정확한 import 집합·함수 서명·body를 비교하며 함수의
선언 이름만 key에서 제외한다. Program ID·body의 binding/호출 이름·direction/media 경로는
그대로 둔다. 기존 view wrapper는 lowered check body가 없으므로 중복 기회로 재집계하지 않는다.

동결 Graph의 `internal/schemaimpl` 189,358파일에서 check validator 382,814개,
body 382,088,702바이트를 조사했다. 모든 body에 Program identity guard가 있고,
완전히 같은 bound body 중복은 0이다. Program을 지운 뒤 같은 함수라고 판정하면 이 결과를
과장하게 된다. 별도 기회 계량은 visit.Program만 보존할 runtime 매개변수로 치환하며
원래 exact hash와 함수/파일 예시를 따로 남긴다. 10,019개 묶음·138,736,336 반복 body
바이트가 나왔다. 전체 4,045,045,462바이트 출력의 약 3.43%에 해당하는 기회 상한이며
새 wrapper·인자·helper·배치 보존 비용을 뺀 절감량이나 성능 결과가 아니다.

`validator-duplicate-tool-tests-2`는 같은 body의 서로 다른 Program ID와 다른 package를
정확한 중복으로 합치지 않고 parameterized 기회만 별도로 집계함을 확인했다.
원시 보고서는 `graph-validator-duplicates-schemaimpl.json`, recorder는
`graph-validator-duplicates-schemaimpl`이다. V의 실제 채택/보류는 P의 전체 compile과
이 비용·identity/runtime 경계를 함께 검토한다. 최종 W07 결정은
`validator-sharing-decision.md`의 추가 공유 보류다. exact 중복 0개, 매개변수화 비용과
partition/type identity를 근거로 한다. schema planner·view planner·schema template·runtime의
원래 제품 대비 diff가 없고 runtime 기준 9단계가 모두 통과했음을 다시 검산했다.
공유 구현의 성능을 주장하지 않으며 P의 W09/W11 compile·runtime gate는 그대로 남는다.

## W08 좁은 계약 적용 뒤 전체 Graph 재검사

`graph-ts-narrow`는 187,686파일·761,986,482바이트다. 원본보다 약 67.7% 작지만
이는 compiler 성공을 뜻하지 않는다. source inventory의 최대 SCC는 35파일·4,841,854바이트이며
32 MiB/12,000파일 목표로 28개 ownership project를 만들었다. 모든 source는 한 project만
소유하고 scratch declaration에만 comment 제거를 적용한다. 원래 SDK는 바꾸지 않는다.

`graph-ts-narrow-native-1`은 strict·실제 `isolatedDeclarations`를 켜고 fresh config,
build-info, declaration 출력으로 실행했다. 19개 project가 통과한 뒤 public root를 포함한
project 19에서 RSS 보호 종료됐다. 전체 531.53초이며 전체 source/public consumer 성공은
미완료다. 이전 16개 통과보다 진행했지만 이를 완료나 전체 compile 속도 개선으로 집계하지 않는다.

Go compiler와 native TS의 soft heap 3 GiB 진단도 각각 RSS 8,621,887,488바이트와
8,591,290,368바이트로 8 GiB 상한을 넘었다. Go는 실제 compiler package
`example.test/graph/api/internal/schemaimpl/p10`이 가장 큰 메모리 사용 주체였다.
TS 진단은 이미 통과한 reference 출력을 쓴 단일 root project 검사였으므로 전체 fresh
검사 표본이 아니다. 두 진단 모두 실패 기록이며 3 GiB를 채택 측정 조건으로 확정하지 않는다.

checker는 각 project와 최종 consumer 직전에 기존 최대 12 GiB·가용 RAM−2 GiB 정책을
다시 적용하고 실제 RSS/Node heap 예산을 결과에 저장한다. Node heap은 RSS보다 최소
1 GiB 작다. `narrow-project-tool-tests`는 실제 isolated declaration flag를 포함한 JS/native
compiler, ownership·public negative·오류 전파 검사를 통과했다(22.20초). 이 검사는 작은
fixture의 도구 검증이며 Graph 성공 증거와 구분한다.

## W06 native 전체 생성과 직렬 cold 빌드

`graph-go-native-final-generation`은 실제 native generator·server addon·product publisher로
1,336.35초에 완료됐다. peak session RSS는 8,415,866,880바이트다. 최종 manifest는
285,344 artifact이며 원본 659,664개보다 줄었다. sorted frozen replay의 219,063개와
구분한다. native 순서에서 `models`는 55,485파일, `schemaimpl/p10`은 1,070파일이며
같은 replay는 각각 31,471개·645개다. bounded group flush는 입력 순서의 영향을 받는다.

`graph-go-native-final-cold-compile-1`은 새 GOCACHE·Go 1.27.2·CGO=0·GOMAXPROCS=4·
soft heap 6 GiB·`-p=1 ./api/...`로 실행했다. 9,264 MiB RSS 예산에서 3,600.45초
시간 초과로 끝났다. peak session RSS는 9,507,672,064바이트이며 RSS 보호 종료가 아니다.
Go 전체 compile 성공은 아직 확보하지 못했다. compiler 관측은 큰 schema package 뒤
model implementation의 `p249`까지 진행했으며 전체 package 완료 수로 집계하지 않는다.

같은 실행의 GOFLAGS·GOEXPERIMENT·GOOS·GOARCH·GOAMD64·GOGC override가 없음을
실제 Go process에서 확인했다. 환경 전체나 자격증명은 저장·출력하지 않았다.
큰 implementation을 먼저 직렬 빌드하고 작은 implementation을 제한 병렬 처리한 뒤
전체 root를 빌드하는 추가 스케줄 진단은 `execution-policy.md`에 사전 기록했다.
아직 실행·채택한 결과가 아니며 기존 timeout을 성공으로 바꾸지 않는다.

## W08 module profile·증분 오류 전파와 native 진단

`registry-strict-module-profiles`는 700 public/300 private operation fixture를 네 compiler의
strict NodeNext·Bundler·CommonJS에서 검사했다. 실제 `isolatedDeclarations` source 생성과
source가 없는 별도 declaration consumer가 모두 통과했다(301.73초, 최대 RSS
1,678,426,112바이트). 기존 CommonJS CI의 relaxed profile을 strict 근거로 재사용하지 않는다.
`project-check-incremental-tool-tests`도 통과했다(43.59초, 366,579,712바이트).
성공 뒤 source를 바꾸고 inventory/ownership을 새로 만든 실제 semantic 오류가 실패하며,
이를 고친 새 실행은 통과한다. 기존 성공 declaration을 재사용하지 않는 경로를 확인했다.

12 GiB 요청의 전체 native 검사 `graph-ts-narrow-native-12g-1`은 project 0~18 통과 뒤
root project 19에서 1,636.28초 동안 진단을 내지 않아 profile 조사로 전환하며 중단했다.
중단 이유와 실제 signal은 `interruption.json`에 보존했다. RSS·timeout·semantic 오류나
전체 성공으로 집계하지 않는다. CLI의 6/9/10 GiB soft heap 진단 세 번은 취소 뒤에도
profile flush가 끝나지 않아 불완전하다. gctrace와 실패 원본은 보존하며 빈 CPU 파일을
판독 가능한 profile로 취급하지 않는다. 작은 async API smoke는 snapshot·owner 확인·
semantic 진단·CPU/heap profile 판독까지 통과했다. 큰 원본 config API 시도는 snapshot
구성 도중 가용 RAM에 따른 6,224 MiB 보호 상한을 넘어 종료됐으며 owner 확인 전이다.
이들 언어 서비스 진단은 전체 CLI·declaration·public consumer 성공을 대신하지 않는다.

## W06 실제 Graph 전체 계약 대조

`graph-go-native-final-audit-run`은 최종 source 전부를 parse cap 없이 검사했다
(141.84초, 최대 RSS 906,264,576바이트). 기준선의 큰 두 파일은 기존 unlimited
supplement로 보완했다. publisher manifest와 모든 audit source hash·전수 coverage가 일치한다.
`graph-go-native-final-streamed-contract-comparison`은 원본 Go 659,663파일과 후보
285,343파일에서 각각 6,434,421개 계약을 비교해 차이 0건으로 통과했다
(54.67초, 최대 RSS 541,319,168바이트). 정의 type의 package/name과 공개 AST 서명·
import binding이 동일하다. filename은 비교하지 않으며 compiler/runtime 성공은 별도다.

비교 도구의 첫 3/6 GiB 실행은 각각 RSS 보호로 실패했다. 큰 facade의 import 문자열을
각 공개 서명에 붙인 임시 list가 원인이었다. import prefix의 해시 상태를 복사하고 서명을
순차 처리하도록 고쳐 같은 해시 의미와 계약별 occurrence 수를 유지했다. 기존 양성·음성
회귀가 통과했고, 실제 전수 대조도 원래 3 GiB 도구 예산 안에서 완료됐다. 실패 원본을 보존한다.

## W08 실제 native CPU·heap profile

별도 flat fixture는 source owner 19의 원본 958개와 이미 검사한 dependency 선언
97,862개를 hardlink했다. API가 실제로 읽은 program은 57,127파일이며 implementation
source 집합은 958개와 정확히 일치했다. strict·noCheck=false는 실제 compiler options에서
확인했다. 원본 SDK와 reference 출력은 수정하지 않았다.

`graph-ts-flat-api-factory-run`은 53.11초·최대 RSS 7,154,118,656바이트로 profile 수집을
마쳤고 factory semantic 진단은 오류 0개다. registry도 37.70초·6,666,031,104바이트로
수집을 마쳤으며 오류 0개다. 두 실행의 CPU·heap 파일은 `go tool pprof`로 실제 판독했다.
이는 해당 파일의 언어 서비스 진단이며 전체 source·emit·consumer 성공이 아니다.

`graph-ts-flat-api-program-run`은 file을 생략해 loaded program 전체의 semantic 호출을
120초 계측했다. profile 수집은 141.72초·9,379,581,952바이트로 완료됐으나 semantic
호출 자체는 끝나지 않았다. CPU 표본 464.14초 중 background GC가 339.07초(73.05%)를
차지했고 heap profile의 유지 객체는 7,740.97 MiB다. 6 GiB soft limit보다 크다.
checker 경로에서는 `getConstraintFromIndexedAccess` → `getIndexedAccessTypeOrUndefined`
→ `getUnionTypeEx` → `addTypesToUnion`의 비용이 크다. 이 단계에서 특정 public alias나
section predicate가 원인이라고 확정하지 않는다. 원본 두 profile과 owner·config·binary
hash, 실패·중단 결과를 모두 보존한다. API의 checker pool 경로는 CLI singleThreaded
실행과 구분하며 이 계측을 최종 성능 표본으로 사용하지 않는다.

호출 경로 해석에 참고한 고정 upstream 원본은
[API 진단 schema](https://github.com/microsoft/typescript-go/blob/2bd066d87f5b/internal/api/proto.go),
[진단 handler](https://github.com/microsoft/typescript-go/blob/2bd066d87f5b/internal/api/session.go),
[indexed access·union 계산](https://github.com/microsoft/typescript-go/blob/2bd066d87f5b/internal/checker/checker.go)이다.
마지막 파일의 SHA-256은 `32384dde486c33c59c5695505f4c951c3009c6d7e50589cb5b18d0ea92cbf161`이다.
큰 파일은 connector가 본문을 반환하지 않아 동일 commit의 raw 원본을 읽었다.
packaged binary의 `+dirty`를 이유로 source와 binary가 완전히 같다고 가정하지 않는다.

`graph-ts-flat-api-route-helpers-run`은 owner 일치·두 profile 판독까지 완료했다
(136.85초, 최대 RSS 7,290,998,784바이트). `routes/helpers.ts` 한 파일의 semantic
호출은 120초에 끝나지 않았으며 background GC가 CPU의 71.16%였다. 파일까지 비용을
좁혔지만 어느 별칭인지와 section index의 채택 여부는 아직 확정하지 않는다.

작은 generic probe는 직접 nested lookup을 조건부 `infer` lookup이나 별도 mapped
view로 대체하는 두 후보를 조사했다. 공통 필드 접근은 통과했으나 기존
`Routes[Route]["input"]`에서 새 generic 별칭으로 전달하는 방향이 TypeScript 5.9.3의
TS2322로 실패했다. 리터럴·union 값이 비슷해도 generic 관계를 보존하지 못하므로
둘을 제품에 적용하지 않았다. 원본 probe·실패 결과를 보존한다.

## W06 warm 스케줄 진단의 자원 종료

`graph-go-native-final-staged-warm-diagnostic`은 이전 timeout cache를 이었다.
큰 schema package 두 개는 cache로 완료했고 model implementation family를 `-p=3`,
compiler soft heap 2 GiB로 진행하다 871.73초에 RSS 보호 종료됐다. 당시 실제 hard
상한은 8,550 MiB, 관측 peak는 8,972,869,632바이트다. 마지막 sample의 세 compiler는
`modelsimpl/p337`, `p340`, `p345`이며 각각 약 2.7~2.9 GB RSS였다. source 크기만으로
compiler RSS를 예측할 수 없음을 보여 준다. 이 실행은 compile 성공이나 cold 표본이 아니다.
후속 continuation의 최소 headroom을 정책에 먼저 고정했으며 실제 결과로 채택을 판단한다.

`graph-ts-flat-api-program-10g-run`도 두 profile 수집과 owner 확인을 완료했다
(255.78초, 최대 RSS 10,994,176,000바이트). semantic 호출은 240초에 끝나지 않았고
background GC는 CPU 표본의 70.17%, 유지 heap은 8,509.14 MiB였다. 다른 구간 길이와
진행 상태의 profile이므로 앞의 6 GiB 실행과 속도 개선율을 계산하지 않는다.
추가 heap-only 반복보다 helper 표현의 분리 조사를 우선한다.

`projection-generic-homomorphic-probe`는 기존 object를 그대로 유도하는 `Readonly`와
`Pick` view의 공통 필드 및 generic 양방향 전달을 통과했다(TypeScript 5.9.3, 1.05초).
이는 필요한 작은 호환성 검사이며 제품 채택이나 Graph 성능 개선의 증거가 아니다.
원본 helper를 복제한 control과 두 lookup 종류에 각각/함께 `Readonly`를 쓴 네 개의
진단 source를 별도 경로에 준비했다. 원래 958개 source hash는 전부 일치했고 각 config는
그 source와 probe 하나만 소유한다(`owner-attribution-probe-preparation`, 1.45초).
원본 SDK·기존 declaration은 수정하지 않았다. 실제 비교 실행과 공개 계약 검사는 아직 남았다.

두 번째 warm 진단은 11,656 MiB hard budget에서 schema/model implementation family
전체를 통과했다(해당 stage 436.60초). 이후 `./api/...` 전체 빌드는 `internal/operations`
compiler에서 1,085.01초에 RSS 보호 종료됐다. peak session RSS는 12,225,490,944바이트다.
Go 전체 성공은 아직 미충족이며 이 결과를 cold 표본에 넣지 않는다. 이 package의
정확한 compiler 단계를 확인할 별도 stack 진단을 정책에 먼저 기록했다.

별도 `graph-go-operations-phase-capture-run`은 93.51초·RSS 11,294,830,592바이트에
stack 수집을 마쳤다. 의도한 SIGQUIT로 build는 종료했으며 성공 표본이 아니다.
main goroutine은 `types2.recordUse` → 선언/receiver 검사 → `noder.checkFiles`에 있었다.
SSA 이전이므로 backend worker 수만 낮추는 방향을 해결책으로 채택하지 않는다.
실제 publisher hash와 대조한 AST 감사(`graph-go-operations-cost-run`, 20.99초)는
operation package의 45,696파일·597,978,236바이트·78,054,414 AST 노드를 확인했다.
반복 HTTP 처리문의 좁은 후보 E와 검증은 `operation-batching-decision.md` 및 W06E에
독립 등록했다. 아직 제품 구현·효과를 입증한 것은 아니다.

네 Readonly probe는 모두 소유권 959개 일치·읽을 수 있는 CPU/heap profile을 얻었지만
120초에 semantic 검사는 완료하지 못했다. control/values/sections/both 수집 시간은 각각
136.54/131.46/130.93/131.32초, peak RSS는 7.247/7.946/7.497/8.083 GB다.
수집 시간은 검사 완료 시간이 아니며 해당 view를 채택하지 않는다.

별도 GC 대조는 같은 control source·soft heap 10 GiB에서 default/STW 모두 120초에
semantic 미완료였다. 실제 native child의 선택한 환경을 기록했으며 peak RSS는
10.321/10.535 GB다. default CPU 표본 135.24초 중 `addTypesToUnion` 누적은
94.01초(69.51%), `runtime.memmove` flat은 48.50초(35.86%)였다. 이 실행의 GC 비용은
낮았으므로 이전 profile의 background GC 비율만으로 병목을 설명하지 않는다.
heap/GC 변경을 더 반복하기보다 private section filter의 사전 계산을 별도 진단한다.

실제 CLI TS 5.9 전체 검사 `graph-ts-narrow-5-9-full-1`은 project 0–18을 통과하고
root 19에서 Node heap OOM(SIGABRT)으로 실패했다. root는 367.33초·RSS
11,236,098,048바이트, 실제 hard/heap은 11,411/10,387 MiB였다. 전체 791.19초에
28개 중 19개가 완료됐고 마지막 공개 consumer는 실행하지 못했다. RSS 보호 실패나
전체 strict 성공으로 바꾸지 않는다. 원본 narrow SDK와 ownership source는 그대로다.

`section-index-probe-preparation-2`는 9.58초·RSS 423,858,176바이트에 완료했다.
원본 958개 hash 및 고정 oracle의 route/operation ID 각각 29,581개와 정확히 대조했다.
새 11,804,074바이트 진단 source는 private route/ID/resource section filter만 교체하며
value projection은 그대로 둔다. 첫 준비 실패는 owner의 `.ts` 계약을 `.d.ts`만으로
찾던 도구 assertion이었고 원본과 함께 보존했다. 아직 C의 성능·제품 호환 증거는 아니다.

## 후속 후보 E/C — 검증 진행

W06E 추가 뒤 계획 구조는 15 task/16 validation과 저장 본문·전체 ID·W09 의존성까지
다시 대조했다. 최초 검사에서 실행 중인 W00에 `execution_ready`까지 요구한 assertion은
잘못된 검사 조건이었다. 구조 유효성과 실행 가능 상태를 구분해 전체 대응 검사를 다시
통과했고 `current-plan-verification.json`에 남겼다. 이전 단위의 변하지 않은 방법·역사적
결과는 개별 검토 후 현재 basis에 명시적으로 연결했다. 과거 실패를 성공으로 바꾸거나
현재 후보의 최종 성공으로 재사용하지 않았다.

Go E는 기존 `Parameter`를 재사용한 최대 16개 binding과 상태를 이어 가는 응답 선택을
구현한 후보다. 0/1개는 기존 직접 표현을 유지한다. 기존 매개변수·codec·응답·stream
회귀는 16.52초·RSS 668 MB에 통과했다. 추가 core 계약 검사(3.48초)와 33개 매개변수/
33개 응답의 실제 HTTP·race 검사(5.93초)도 통과했다. missing header 등록 시점·encoder
snapshot·첫 오류·취소·batch 경계·동점·wildcard 선택을 확인했다. 최초 새 검사 실패는
test import alias 오류였으며 원본을 보존하고 수정했다. 전체 Graph와 runtime 비용은 남는다.

Go 계약 도구는 새 내부 helper만 정확한 이름·서명·import binding·1회 추가로 허용할 수
있게 했다. 기존 이름 변경·삭제, 예기치 않은 추가, 허용 목록 누락·중복·다른 서명은
통과하지 못한다. 기존 양성/음성과 추가 허용 경계 검사 3개는 실제로 통과했다(0.30초).
Graph의 전체 기존 계약 비교는 별도로 수행한다.

11 GiB hard 진단은 headroom 부족으로 child를 시작하지 않았다. 새 10 GiB hard 대조는
같은 control/C source 소유권 959개·program 57,128개·soft heap 10 GiB와 실제 child의
동일 GC 환경을 확인했다. control은 120초 semantic 미완료, C는 9.14초 계측 구간 안에
semantic diagnostics 0개로 완료했다. 수집 전체는 132.27/22.09초, peak RSS는
10,275,872,768/8,621,797,376바이트다. profile 수집은 CLI·declaration emit·전체 source/
공개 consumer 성공이 아니며 검사 완료 시간의 개선율로 계산하지 않는다.

이제 canonical `InputSections`와 실제 resource reachability에서 세 private filter 집합만
기존 최대 256 operation 계약 청크에 생성하는 C 후보를 W08에서 검증한다. 값 projection과
public generic 식·작은 SDK/selected/named의 기존 import 범위는 유지한다. 새 runtime
module이나 generator 옵션은 추가하지 않는다. 초기 W04 보류 판단을 바꾸려면 네 compiler의
generic/union/never/ID 없는 route/resource/negative 및 실제 Graph 전체 검증이 필요하다.
