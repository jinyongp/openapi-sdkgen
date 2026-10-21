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

최종 Go 파일 659,663개·590 package를 audit했다. 기본 8 MiB 상한에서 빠진
`client.go` 55,629,305바이트와 `operations/types.go` 29,377,260바이트는
`-only-large -max-file-bytes 0` 별도 bounded trial로 전체 AST를 확인했다.
각각 공개 signature 253,349개와 194,183개이며 후속 package/identity 비교에서 빠뜨리지 않는다.
두 대형 파일을 제외한 동일 최종 package·import 집합의 묶기 후보는 545,220파일이다.
이는 구현·컴파일 성공 또는 실제 절감량의 근거가 아니다.

Go 1.27.2, server 포함 전체 `./api/...`, CGO=0·GOWORK=off·GOMAXPROCS=4,
`-p=1`, compiler soft heap 6 GiB, fresh 전용 GOCACHE, 3,600초·session RSS 9 GiB
정책으로 기준 compile을 시도했다. 255.71초에 RSS 보호가 작동했고 관측 최대치는
9,665,454,080바이트다. 미완료 실패로 보존하며 최종 W09/W11 성공 기준은 유지한다.
원시 phase/profile·최종 파일 hash·기본/대형 AST inventory·실패 compile은 모두
`.tmp/sdk-optimization-execution/graph-go-*`에 있다. runtime 기준 표본은 아직 진행 전이다.
