# W01의 D27 영향 재검토

기준은 D27 계획과 `w05-final-code.json`의 1227개 소스다. W04 최종 manifest 대비 변경·추가 70개, 삭제 0개다. 기존 완료 기록은 역사로 보존한다.

| 영역 | 영향과 검토 | 상태 |
| --- | --- | --- |
| 옵션·CLI/config·manifest | generator 10개·output 9개·CLI 26개 파일은 W04 최종 소스와 같다. 옵션·fingerprint·사용자 파일 보존·rollback 기준을 유지한다. | 검토 완료 |
| Prepare·이름·독립성 | multipart 공개 model codec 파생 이름 EncodeMedia/DecodeMedia를 전체 문서에서 함께 예약한다. 입력/출력 occurrence와 body wrapper 예약을 재검토했다. 선택 전 이름 안정성·immutable Prepare·sink 중단을 유지한다. | 검토 완료 |
| 파일·import·생성 출력 | 조건부 multipart/XML/폼 모듈 closure를 명시하고 기존 cycle/누락 검사와 gofmt 실패 rollback을 유지한다. custom content header의 불필요 schema import 결함은 W05에서 수정했다. | 검토 완료 |
| 최소 소비자·실패 경로 | 기본 호출·중복 Emit·출력 실패·타깃 설정·manifest 소비자를 현재 소스로 별도 실행한다. W05 미디어 기능 자체는 이 작업의 완료 주장에 포함하지 않는다. | 검토 완료 |

별도 실행 `w01-d27-check.json`에 명령·요청한 최상위 사례 12개·실제 통과 사례 12개·패키지 결과·소스/원본 로그 해시를 보존했다. 모든 요청 사례의 통과를 확인했다. 이 작업의 미검토 영역이나 확인된 미수정 결함은 없다. 후속 작업의 완료나 전체 기능 동등성을 주장하지 않는다.
