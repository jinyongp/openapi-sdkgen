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
