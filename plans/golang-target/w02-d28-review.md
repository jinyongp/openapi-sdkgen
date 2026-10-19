# W02의 D28 영향 재검토

기준은 D28과 `w06-reviewed-code.json`의 1258개 소스다. shared schema lowering,
strict JSON·model state·schema validation·regex·format 구현은 W05 완료 해시와 같다.

| 영역 | 검토 | 상태 |
| --- | --- | --- |
| strict JSON·숫자·불변 값 | 바이너리는 MultipartData 슬롯에 보존하고 Value에 host resource를 넣지 않는다. Binary.Bytes는 이미 버퍼인 값만 복사한다. | 검토 완료 |
| 스키마·투영·합성 | request item은 Input, response item은 Output projection이다. 전체 response schema와 증분 itemSchema를 분리했고 기존 JSON component의 정체성은 보존했다. | 검토 완료 |
| nullable·tuple·recursive·With | 새 SourceFactory alias는 일반 DTO codec을 대체하지 않는다. 재귀 multipart의 물리적 facet 선택과 전체 recursive assertion을 분리했다. | 검토 완료 |
| XML·format·regex·DCE | 기존 contentSchema·XML·URL·정규식과 unused literal 경계의 코드는 불변이며 별도 검사를 다시 실행했다. | 검토 완료 |

`w02-d28-check.json`의 독립 최상위 14개·전체 통과 이벤트 34개를 확인했다. 이 단위의
미검토 영역이나 확인된 미수정 결함은 없다. W06의 확장 수명 보완과 W11 전체 동등성은
별도 미완료 범위로 유지한다.
