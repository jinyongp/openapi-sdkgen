# W03의 D26 영향 재검토

범위는 W03의 현재 완료 기준 A06/A07/A10/A22다. 최초 상세 리뷰는
`w03-review.md`에 보존한다. 현재 기준은 `w04-final-code.json`의 1186개 소스,
SHA-256 `36a7abdc3bc5affa862b2b3abc5d0a7e8dd56a2f9b7ece5be2d29e7304f38b33`다.

| 영역 | 변경 영향과 재검토 | 상태 |
| --- | --- | --- |
| 호출·입력·옵션 | 무인자/필수/선택 입력·복사·재사용은 유지한다. 보안 선택 옵션을 추가했지만 기존 옵션의 상속·명시 빈 값을 유지한다. | 검토 완료 |
| HTTP 직렬화 | 파라미터 style/content/allowReserved 구현은 불변이다. base/origin 정규화만 보강하고 operation의 encoded path·query 조합을 유지한다. | 검토 완료 |
| 본문·코덱 | 인증을 완료한 뒤 본문 source를 연다. 본문 codec에는 최종 URL·인증 헤더 snapshot을 주고 실패 시 source를 소비하지 않는다. parameter 방향의 나머지 metadata 계약은 W05에서 검토한다. 소유권·길이·예외 경로는 유지한다. | 검토 완료 |
| 응답·오류 | 상태/미디어·헤더·Raw·HTTPError는 불변이다. 인증의 기존 오류 코드만 보존하며 외부 취소·timeout이 우선한다. 민감한 요청 정보의 JSON 제외는 유지한다. | 검토 완료 |
| 전송·수명 | cookie/Jar·native headers·redirect·압축 상태·lease EOF/Close/취소를 유지한다. credential 적용 후 Host를 native 필드로 옮긴다. | 검토 완료 |
| 공통 의미 | binary/status/header/reference 의미는 불변이다. 추가한 보안 ID helper의 전체 점유 검사는 두 타깃이 함께 사용하며 TypeScript 영향 회귀가 통과했다. | 검토 완료 |
| 생성·이름 | 보안 파생 이름을 선예약한다. 새 closure는 core→security/transport/types 방향이며 기존 schema/model/wire 경계는 유지한다. | 검토 완료 |
| 기존 호출자 | 보안 선언 없는 health·본문·응답·순차형 Raw와 custom codec의 실행 경로를 현재 코드에서 다시 확인한다. | 검토 완료 |

W04 최종 전체 회귀·race는 통과했다. W03만 분리한 실행은 `w03-d26-check.json`에
16개 최상위 테스트·하위 사례·패키지 결과·실행 명령·원본 로그 해시를 보존한다.
모든 요청 사례의 통과와 소스 해시 일치를 확인한 뒤 이 작업의 검증 기록을 갱신한다.

확인된 W03 미수정 결함이나 미검토 영역은 없다. W04의 새 인증/서버 기능은 별도로
검증·리뷰하며, W05–W12의 나머지 미디어·타입 스트림·리소스·수신·동등성·성능을 완료로 계산하지 않는다.
