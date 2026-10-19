# W04의 D29 영향 재검토

기준은 D29와 `w07-complete-code.json`이다.

| 영역 | 검토 | 상태 |
| --- | --- | --- |
| 서버·origin·변수 | 기존 server 상속·BaseURL·WHATWG origin 코드는 유지했다. Link server는 원래 응답 URL을 기준으로 resolve하고 외부/잠금 closure 소비자가 정확한 실제 URL을 확인한다. | 검토 완료 |
| 보안·AND/OR·ID | Link options는 대상 작업의 SecurityRequirement 타입을 받는다. 필수 대안은 required invocation, 상태별 target이 다르면 tag와 선택 분기를 검증한다. 원본 요청 options를 상속하지 않는다. | 검토 완료 |
| provider·metadata·취소 | 기존 execution/provider snapshot과 호출별 context를 재사용한다. Follow에는 HTTP/source open이 없으며 실제 실행에서만 인증과 전송을 한다. | 검토 완료 |
| TLS·cookie·redirect·query | 기존 Go 전송의 네이티브 능력과 헤더 충돌 정책을 유지한다. browser Fetch 금지 헤더를 추가하지 않는다. 세 전용 소비자가 6.257초로 통과했다. | 검토 완료 |

명령·현재 manifest·로그 해시는 `w04-d29-check.json`에 있다. 상태별 보안·오프라인
상대 server의 새 Link 소비자도 통과했다. 이 단위의 미검토 영역이나 확인된 미수정
결함은 없다. W09 수신 인증은 별도 구현 범위다.
