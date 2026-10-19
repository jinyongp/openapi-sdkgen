# W02의 D27 영향 재검토

기준은 D27 계획과 `w05-final-code.json`의 1227개 소스다. W04 최종 manifest 대비 변경·추가 70개, 삭제 0개다. 기존 완료 기록은 역사로 보존한다.

| 영역 | 영향과 검토 | 상태 |
| --- | --- | --- |
| strict JSON·불변 모델 | Value는 불변 JSON만 보관한다. multipart source는 wire.MediaValue와 공개 MultipartData 슬롯에 두고 JSON 직렬화로 버리지 않는다. 기존 일반 JSON DTO 경로와 literal/link 제거 계약을 재검토했다. | 검토 완료 |
| 스키마·합성·투영 | multipart 입력만 media DTO를 사용한다. ordinary component와 출력 DTO는 원래 projection을 유지한다. schema State의 opaque source 플래그는 media 검증에서만 사용하며 const/enum·propertyNames·uniqueItems와 분리한다. | 검토 완료 |
| nullable·tuple·참조·With | 미디어별 state 경로·consume 슬롯·With 병합을 검토했다. 표현할 수 없는 source, 잘못된 경로와 겹친 source는 오류다. recursive/dynamic/unevaluated 일반 의미를 변경하지 않는다. | 검토 완료 |
| XML·정규식·format·DCE | XML 동적 참조의 지역 metadata 병합을 수정하고 일반 contentSchema 호출자를 재검토했다. 정규식·URL format 구현은 불변이며 공식 schema/regex/format·lazy literal 소비자를 독립 재실행한다. | 검토 완료 |

별도 실행 `w02-d27-check.json`에 명령·요청한 최상위 사례 14개·실제 통과 사례 34개·패키지 결과·소스/원본 로그 해시를 보존했다. 모든 요청 사례의 통과를 확인했다. 이 작업의 미검토 영역이나 확인된 미수정 결함은 없다. 후속 작업의 완료나 전체 기능 동등성을 주장하지 않는다.
