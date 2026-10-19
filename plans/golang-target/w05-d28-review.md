# W05의 D28 영향 재검토

기준은 D28과 `w06-reviewed-code.json`의 1258개 소스다. 기존 W05 완료·D27 리뷰를
보존하고 다음 공유 caller의 변경을 다시 검토했다.

| 영역 | 검토 | 상태 |
| --- | --- | --- |
| XML·폼·직접 계획 | 요청·응답의 헤더 lowering을 하나로 추출했고 기존 header identity와 검증식을 유지했다. XML mapper는 직접 함수를 생성하며 런타임 문서를 해석하지 않는다. | 검토 완료 |
| JSON component·미디어 투영 | 새 response byte 슬롯은 ordinary component에 전파하지 않는다. 입력 MediaInfo·직접 XML query의 기존 scope를 유지한다. | 검토 완료 |
| named·positional·중첩 | prepareMultipartParts는 기존 wire 선택·header·nested 조건을 공유한다. 동적 source는 현재 파트만 만들고 complete는 알려진 파트를 미리 검증한다. | 검토 완료 |
| 자원·known length·실패 | static/dynamic reader는 같은 prepared header와 Binary.Open lease를 사용한다. EOF에 source만 닫고 closing frame의 parent context는 유지한다. custom source nil/typed-nil/음수 metadata를 조기 응답 전에 거부하는 회귀를 수정했다. | 검토 완료 |
| 코덱 방향·metadata | child context가 media/header/XML/nested 함수에 도달하며 Execution을 복사하지 않는다. whole-media Decode/DecodeInbound의 빌린 Body·snapshot·EOF trailer 의미는 보존한다. | 검토 완료 |

배정 기능과 보충 계약의 Go 최상위 23개를 별도 실행했고 29개 통과 이벤트·60.498초를
확인했다(`w05-d28-check.json`). 공통 XML·encoding 8개와 TypeScript 영향 107개도
같은 manifest에서 통과했다. 원본 로그 해시는 각 check JSON에 보존했다.

이 단위의 미검토 영역이나 확인된 미수정 결함은 없다. native stream protocol/adapter의
실행 수명 보완은 W06의 미완료 항목이며, W09 실제 handler·W11 전체 동등성·성능은
별도 범위다. 기존 XML 추가 속성·prefixItems와 finite multipart custom Decode 한계를 유지한다.
