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
초기 두 fixture 기대값 오류와 수정 후 결과도 삭제하지 않는다. W05의 네 compiler/
실험군/server profile 분류와 W08의 최종 전체 strict 성공은 아직 별도 필수 작업이다.
