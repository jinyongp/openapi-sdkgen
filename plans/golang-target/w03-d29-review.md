# W03의 D29 영향 재검토

기준은 D29와 `w07-complete-code.json`이다.

| 영역 | 검토 | 상태 |
| --- | --- | --- |
| HTTP 입력·옵션 | 새 resource와 Link가 기존 직접 호출을 사용한다. 파라미터별 serialization과 typed/raw 헤더 소유권은 유지한다. SourceInput snapshot은 요청 전송을 시작하지 않는다. | 검토 완료 |
| 응답 선택·Raw·오류 | body 없는 선언의 media fallback을 수정했다. 실제 Content-Type이 있어도 NoContent typed 결과를 유지하며 유한 Raw 데이터는 보존한다. 응답 전용 profile은 ReadResponse 뒤 로컬 context이며 request/declared header 검증을 완화하지 않는다. | 검토 완료 |
| 원본 snapshot·오류 source | Raw/HTTPError의 Link snapshot은 원래 status/headers/URL/전체 Value를 보관한다. 공개 Raw 필드 변경에 영향받지 않는다. 오류에는 명시 Data/Details/Fields 조회와 정적인 code/category helper를 추가하되 Error() 본문 비노출을 유지한다. | 검토 완료 |
| Binary·압축·수명 | binary Raw에서 복사한 bytes를 후속 native body로 연결한다. binary source와 압축 원본 헤더·response lease의 실행 소스는 불변이다. sequential Raw는 Link가 소비하지 않고 body 표현식을 요청 전 거부한다. | 검토 완료 |
| 코덱·구체 API·패키지 | whole-media/custom/open media와 schema response profile의 연결을 확인했다. resource 패키지 의존을 명시하고 실제 import cycle 검사를 유지했다. | 검토 완료 |

독립 최상위 16개·통과 이벤트 19개가 38.476초 통과했다. 명령·manifest·원본 로그는
`w03-d29-check.json`에 있다. 이 단위의 미검토 영역이나 확인된 미수정 결함은 없다.
W09 수신과 W11 전체 검증은 별도 완료 조건이다.
