# W03의 D28 영향 재검토

기준은 D28과 `w06-reviewed-code.json`의 1258개 소스다. transport·압축·Binary 기본
소유권은 W05 완료 해시를 유지하며, 공유 execution과 options 호출자를 재검토했다.

| 영역 | 검토 | 상태 |
| --- | --- | --- |
| HTTP 입력·옵션 | 파라미터 경계는 불변이다. StreamCodec override 포인터를 복사하고 실제 실행 상태는 호출별로 생성한다. | 검토 완료 |
| 응답 선택·Raw·오류 | finite 응답의 기존 선택·에러·헤더 검증을 보존한다. typed stream에만 TransferStream/FinishStream을 사용하고 Raw의 BindCancel은 유지했다. | 검토 완료 |
| Binary·압축·수명 | bytes+EOF 뒤 파서에 남은 데이터까지 typed 실행을 유지한다. 압축 전 metadata·네트워크 Body lease·request body 정리의 기존 caller를 확인했다. | 검토 완료 |
| 사용자 미디어·open media | EncodeBodyContext·DecodeHeaderContent에 child context를 전달하되 Execution lock을 복사하지 않는다. 일반 whole-media 코덱과 stream 코덱은 분리한다. | 검토 완료 |
| 구체 API·패키지 경계 | 독립 16개 최상위 HTTP·compression·Raw·custom/open media·namespace 검사와 실제 소비자 19개 통과 이벤트를 확인했다. | 검토 완료 |

명령·실제 결과·소스와 원본 로그 해시는 `w03-d28-check.json`에 있다. 이 단위의 미검토
영역이나 확인된 미수정 결함은 없다. 새 stream protocol/adapter 정리는 W06에서 보완하고
W06 최종 race로 다시 검사한다. W09 수신과 W11 전체 검증은 완료로 계산하지 않는다.
