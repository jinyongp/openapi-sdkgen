# W05의 D29 영향 재검토

기준은 D29와 `w07-complete-code.json`이다. 기존 미디어의 의도적 한계를 유지한다.

| 영역 | 검토 | 상태 |
| --- | --- | --- |
| XML·폼·직접 계획 | 공통 lowering/extension 추출 후에도 XML schema별 직접 mapper와 파라미터·part encoding을 유지한다. contentSchema의 공개 문자열을 변환하지 않는다. | 검토 완료 |
| JSON·미디어 투영 | response profile은 decoded HTTP 값에만 적용한다. Input projection과 native MIME 슬롯은 별도이며 일반 JSON component에 Binary 필드를 전파하지 않는다. | 검토 완료 |
| named·positional·중첩 | 기존 part selector·header·중첩 codec의 schema 계약과 bytes/slot 분리를 확인했다. Link는 multipart SourceInput을 EncodeBoundMedia로 읽고 대상 DecodeMedia가 검증한다. 파일은 Follow에서 열지 않는다. | 검토 완료 |
| 자원·길이·실패 | pull framing·known length·source lease와 metadata 선검증 소스는 유지한다. 새 native Link 소비자는 파일 open/close 횟수와 부분 label 교체가 원래 파일을 유지함을 실제 HTTP에서 검사한다. | 검토 완료 |
| 코덱 방향·metadata | whole-media 다섯 방향과 request/response metadata snapshot을 유지한다. ResponseContext를 Execution 전체에 덮어쓰지 않으므로 동시 요청 인코딩이나 inbound 입력의 검증이 느슨해지지 않는다. | 검토 완료 |

배정·보충 계약에 연결된 독립 최상위 23개·통과 이벤트 29개가 60.049초로 통과했다.
명령·현재 소스·원본 로그 해시는 `w05-d29-check.json`에 있다. 전체 후보 race 및
TypeScript 관련 회귀도 통과했다. 이 단위의 미검토 영역이나 확인된 미수정 결함은 없다.
W09 handler·W11 전체 대응/성능·W12 통합 리뷰는 별도 완료 조건이다.
