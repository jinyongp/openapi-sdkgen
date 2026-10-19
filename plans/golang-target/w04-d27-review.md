# W04의 D27 영향 재검토

기준은 D27 계획과 `w05-final-code.json`의 1227개 소스다. W04 최종 manifest 대비 변경·추가 70개, 삭제 0개다. 기존 완료 기록은 역사로 보존한다.

| 영역 | 영향과 검토 | 상태 |
| --- | --- | --- |
| 서버·origin·변수 | core-server와 transport-url 소스는 W04 최종 manifest와 같다. 서버 상속/selection/BaseURL 우선순위·WHATWG origin 규칙을 유지한다. | 검토 완료 |
| 인증 선택·AND/OR·충돌 | security 런타임은 불변이다. 직접/제공자 선택·API key 위치·기본인증/URI·익명·ID 충돌과 operation-specific 타입을 재검토했다. | 검토 완료 |
| 메타데이터·취소·동시성 | W05의 HTTP snapshot 위임은 provider의 입력 metadata를 같은 표준 복사 의미로 보존한다. security가 실제 요청에 적용되는 위치는 바뀌지 않았다. 제공자 오류·취소·동시성·실제 TLS 소비자를 재실행한다. | 검토 완료 |
| 전송·cookie·redirect·URL 쿼리 | transport 4개 파일과 security 구현은 불변이다. Jar·credentials·인증 redirect 거부·독립 form query 직렬화와 final origin을 별도 재검증한다. | 검토 완료 |

별도 실행 `w04-d27-check.json`에 명령·요청한 최상위 사례 3개·실제 통과 사례 3개·패키지 결과·소스/원본 로그 해시를 보존했다. 모든 요청 사례의 통과를 확인했다. 이 작업의 미검토 영역이나 확인된 미수정 결함은 없다. 후속 작업의 완료나 전체 기능 동등성을 주장하지 않는다.

W05 마지막 파트 질의 수정 뒤 `w05-reviewed-code.json`의 1228개 소스에서 같은 세
W04 최상위 소비자를 다시 실행했다(`w04-d27-final-check.json`, 6.018초 통과).
추가 변경은 multipart 생성 질의·런타임·회귀 fixture뿐이며 서버·인증·origin 소스는
앞선 검토와 같다. provider/직접 인증·TLS·redirect·query/origin 기대값을 유지했다.
