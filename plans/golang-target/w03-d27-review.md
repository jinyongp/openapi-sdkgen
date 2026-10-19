# W03의 D27 영향 재검토

기준은 D27 계획과 `w05-final-code.json`의 1227개 소스다. W04 최종 manifest 대비 변경·추가 70개, 삭제 0개다. 기존 완료 기록은 역사로 보존한다.

| 영역 | 영향과 검토 | 상태 |
| --- | --- | --- |
| HTTP 입력·파라미터 | XML parameter encoder와 작성 중 요청 snapshot을 추가했다. 기존 style·explode·content 검증과 header/cookie 소유권을 유지한다. | 검토 완료 |
| 응답 선택·Raw·오류 | 응답 decoder에 실제 Content-Type을 전달한다. finite custom multipart는 JSON DTO 출력으로 유지한다. Raw sequential lease와 응답 선택/HTTP 오류 의미는 불변이다. | 검토 완료 |
| Binary·전송·압축 수명 | Binary.Validate는 source를 열지 않는 identity/metadata 검사다. Open의 실제 reader 오류와 one-shot 의미는 유지한다. nil custom multipart source의 조기 응답 성공 문제를 수정했다. transport 소스 4개는 불변이다. | 검토 완료 |
| 사용자 코덱·메타데이터 | Request/Response snapshot 공통 구현을 media에 두고 core가 위임한다. Decode는 SDK 소유의 같은 빌린 Body를 읽고 snapshot 컨테이너는 독립이다. EOF trailer 복사·압축 후 본문·오류/취소/Close를 검토했다. | 검토 완료 |
| 공개 계약·패키지 경계 | 기존 namespace·구체입력·기본수명·compression·open media 소비자를 별도 실행한다. W06 typed stream과 W09 handler 실제 통합은 해당 단위의 완료 조건으로 유지한다. | 검토 완료 |

별도 실행 `w03-d27-check.json`에 명령·요청한 최상위 사례 16개·실제 통과 사례 19개·패키지 결과·소스/원본 로그 해시를 보존했다. 모든 요청 사례의 통과를 확인했다. 이 작업의 미검토 영역이나 확인된 미수정 결함은 없다. 후속 작업의 완료나 전체 기능 동등성을 주장하지 않는다.
