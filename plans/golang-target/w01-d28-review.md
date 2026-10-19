# W01의 D28 영향 재검토

기준은 D28과 `w06-reviewed-code.json`의 1258개 소스다. W05 완료 소스 대비 변경·추가
51개, 삭제 0개다. generator·output·CLI와 module 파일의 해시는 W05 완료 때와 같다.

| 영역 | 검토 | 상태 |
| --- | --- | --- |
| 옵션·CLI·출력 소유권 | 기존 설정·fingerprint·사용자 파일 보존·staging/rollback 구현은 불변이다. | 검토 완료 |
| 이름·Prepare 독립성 | requestBodyShapes가 이름 예약과 lowering의 단일 근거다. complete/stream과 여러 MIME의 파생 이름을 전체 문서에서 먼저 예약한다. response item과 request item도 분리한다. | 검토 완료 |
| 파일·import 경계 | stream 모듈의 직접 dependency closure와 cycle 검사를 검토했다. multipart 전용 client의 불필요 models import를 수정했다. 출력 wrapper가 실제 참조하는 패키지로 계산한다. | 검토 완료 |
| 소비자·실패 경로 | 전용 12개 검사를 새로 실행해 모두 통과했다. 최소 생성·고정 이름·중복 방출·sink 중단·포맷 실패 rollback을 포함한다. | 검토 완료 |

명령·원본 로그·manifest 해시는 `w01-d28-check.json`에 있다. 이 단위의 미검토 영역이나
확인된 미수정 결함은 없다. W06 코덱 자체의 context 정리 보완은 W06 리뷰에서 계속
처리하며, 이 기록은 W06 또는 전체 기능 완료를 주장하지 않는다.
