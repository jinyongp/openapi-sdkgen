# W02의 D26 영향 재검토

범위는 W02의 현재 완료 기준 A03/A04/A05/A21/A22다. 최초 상세 리뷰는
`w02-review.md`에 보존한다. 현재 기준은 `w04-final-code.json`의 1186개 소스,
SHA-256 `36a7abdc3bc5affa862b2b3abc5d0a7e8dd56a2f9b7ece5be2d29e7304f38b33`다.

W03 최종 manifest와 비교하면 공통 schema 계획 5개, Go schema lowerer 2개,
`plan_model` 1개, wire runtime 6개, schema runtime 6개, model runtime 1개는 모두 해시가 같다.
새 보안 공개 타입은 기존 types 패키지에 추가되므로 패키지 컴파일과 closure를 다시 확인한다.

| 영역 | 변경 영향과 재검토 | 상태 |
| --- | --- | --- |
| 공통 의미·TypeScript | schema 의미와 TypeScript schema 호출자는 불변이다. 새 공통 함수는 HTTP 보안 대안 ID만 소유한다. 영향받은 보안 호출자의 별도 회귀가 통과했다. | 검토 완료 |
| JSON 값 | parser·정확한 키·WTF-8·binary64·불변 snapshot 구현은 불변이다. 새 types 선언이 value identity를 바꾸지 않는다. | 검토 완료 |
| DTO·코덱 | presence·nullable·tuple·read/write 투영·추가 속성·재귀 포인터의 root/하위 codec은 불변이다. 공개 이름 선예약에 보안 파생 이름을 포함한다. | 검토 완료 |
| enum·합성 | 상수/factory·literal lazy 값·As snapshot·With 평가 의미는 불변이다. 기존 소비자를 현재 생성 결과로 컴파일한다. | 검토 완료 |
| 직접 검증 | 제약·평가 집합·합성·조건·참조/dynamic scope 및 실행 오류 구분은 불변이다. 직접 사례와 공식 fixture 소비자를 재실행한다. | 검토 완료 |
| format·regex | 엔진·Unicode·버전은 유지한다. HTTP 파서의 필수 import가 추가돼도 format 기준값의 별도 초기화는 미사용 바이너리에서 제거돼야 한다. 실제 binary 대조를 재실행한다. | 검토 완료 |
| content·XML | 루트 미디어 정책·JSON/text/base64/XML 내부 검증은 불변이다. 일반 XML 송수신은 여전히 W05다. | 검토 완료 |
| 생성·의존 | models/schema/wire의 의존 방향과 frozen 계획은 유지한다. 새 HTTP closure는 schema 해석을 core로 옮기지 않는다. | 검토 완료 |
| fixture | 기존 18개 문서의 schema 소비자를 다시 실행한다. 이 fixture의 HTTP 작업 전체를 검증했다고 계산하지 않는다. | 검토 완료 |

W04 최종 전체 회귀·race는 통과했다. W02만 분리한 실제 실행은 `w02-d26-check.json`에
14개 최상위 테스트와 하위 사례·패키지 결과·실행 명령·원본 로그 해시를 보존한다.
모든 요청 사례의 통과와 소스 해시 일치를 확인한 뒤 이 작업의 검증 기록을 갱신한다.

확인된 W02 미수정 결함이나 미검토 영역은 없다. 고정 URL/regex 표본의 통과를 전 규격의
완전한 증명으로 확대하지 않는다. W05–W12 전체 미디어·스트림·선택·수신·성능·플랫폼은 남아 있다.
