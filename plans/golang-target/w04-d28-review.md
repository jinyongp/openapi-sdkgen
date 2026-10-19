# W04의 D28 영향 재검토

기준은 D28과 `w06-reviewed-code.json`의 1258개 소스다. security·server·transport-url
구현과 인증 query·origin 정규화 파일은 W05 완료 해시와 같다.

| 영역 | 검토 | 상태 |
| --- | --- | --- |
| 서버·origin·변수 | 서버 상속과 selection/BaseURL의 순서·WHATWG origin 규칙을 보존한다. | 검토 완료 |
| 인증 선택·AND/OR·ID | 직접/제공자 자격 증명·익명·충돌 ID·작업별 선택 타입의 실행 caller는 바뀌지 않았다. | 검토 완료 |
| metadata·취소·동시성 | Begin의 호출별 context와 provider metadata snapshot을 보존한다. 새 stream 옵션의 map/포인터 컨테이너는 복사한다. | 검토 완료 |
| TLS·cookie·redirect·query | 기존 jar·credentials·인증 redirect 거부·독립 form query와 TLS 경계의 전용 소비자 세 개를 독립 재실행했다. | 검토 완료 |

세 소비자가 6.409초 통과했다(`w04-d28-check.json`). 이 단위의 미검토 영역이나 확인된
미수정 결함은 없다. W06의 새 코덱 실행 수명은 별도 리뷰로 처리한다.
