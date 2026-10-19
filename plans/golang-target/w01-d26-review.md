# W01의 D26 영향 재검토

범위는 W01의 현재 완료 기준 A02/A03/A16/A22다. 전체 Go 기능 대응을 완료했다는
주장이 아니다. W01의 최초 영역별 리뷰는 `implementation.md`의 W01 절에 있다.
별도의 `w01-review.md` 파일은 없으며 그 경로를 근거로 사용하지 않는다.

기준은 `w04-final-code.json`의 1186개 소스와 SHA-256
`36a7abdc3bc5affa862b2b3abc5d0a7e8dd56a2f9b7ece5be2d29e7304f38b33`다.
W03 최종 manifest와 비교해 변경·추가한 파일은 24개이고 삭제는 없다.
`internal/generator/` 10개, `internal/output/` 9개, `cmd/openapi-sdkgen/` 26개는
모두 해시가 같다. 이 비교만으로 생성 경로의 실행 검증을 대신하지 않는다.

| 영역 | 변경 영향과 재검토 | 상태 |
| --- | --- | --- |
| 옵션·CLI/config | 타깃 등록·옵션 정규화·설정 우선순위 구현은 불변이다. 현재 코드에서 전용 사례를 다시 실행한다. | 검토 완료 |
| fingerprint·manifest | Go identity와 구형 TypeScript manifest 해석·사용자 파일 보존 경로는 불변이다. 현재 CLI·output 소비자를 검사한다. | 검토 완료 |
| Prepare·이름 | 보안 선택 타입·상수의 파생 이름을 전체 문서 예약에 추가했다. 기존 SHA-256 규칙·결정성과 생략 작업의 오류 비승격을 유지한다. | 검토 완료 |
| 파일·import | security/server/URL의 명시 closure만 추가했다. 누락·cycle 검사와 실제 package graph를 유지한다. | 검토 완료 |
| Emit·소유권 | EmitTo/sink·파일별 format은 해시가 같다. 입력 변경·반복 Emit의 독립성과 sink 실패를 다시 검사한다. | 검토 완료 |
| staging·rollback | publisher는 불변이다. 일부 파일을 준비한 뒤 format 실패해도 기존 manifest·사용자 파일을 보존하는 소비자를 재실행한다. | 검토 완료 |
| 최소 소비자 | 기본 health 메서드는 새로운 선택·인증 설정 없이 동작해야 한다. 실제 HTTP의 Do/Raw/context 소비자를 재실행한다. | 검토 완료 |

W04 전체 회귀에서 위 경계와 namespace/package graph의 소비자가 통과했다.
별도의 W01 전용 실행은 `w01-d26-check.json`에 요청한 12개 최상위 테스트와 실제
통과 목록·패키지 결과·실행 명령·원본 로그 해시를 보존한다. 모든 요청 사례의 통과와
소스 해시 일치를 확인한 뒤에만 이 작업의 검증 기록을 갱신할 수 있다.

확인된 W01 미수정 결함이나 미검토 영역은 없다. 릴리스·전체 타깃 동등성·후속 단위의
완료 상태를 변경하지 않는다. W04의 URL/인증 의미는 별도 W04 리뷰 범위다.
