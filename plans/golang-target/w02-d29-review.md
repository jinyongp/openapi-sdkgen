# W02의 D29 영향 재검토

기준은 D29와 `w07-complete-code.json`이다. W02의 값·스키마·모델 전체 책임과
W07이 변경한 응답 profile·binding 투영을 재검토했다.

| 영역 | 검토 | 상태 |
| --- | --- | --- |
| JSON·불변 값·숫자 | Value에 native source를 넣지 않는다. Link carrier의 map은 호출 내부에서 복사·병합하고 JSON 키·배열 index의 정확성은 보존한다. parser·Unicode·숫자 소스는 불변이다. | 검토 완료 |
| 스키마·투영·합성 | 응답 context만 추가/미평가 속성 false 거부를 완화한다. anyOf/oneOf는 strict match 우선, not/if도 전용 ControlFlow를 사용한다. schema 호출의 context는 로컬이고 요청 실행 context를 바꾸지 않는다. | 검토 완료 |
| nullable·tuple·recursive·With | 응답 wrapper가 profile을 보관해 getter/JSONValue가 같은 값을 처리한다. standalone/With와 요청은 strict이며 DecodeResponse/DecodeValidated만 응답 경계를 지정한다. tuple의 필수 prefix와 sealed constructor 계약은 리소스 공유 인자의 호환성 판정에 반영한다. | 검토 완료 |
| binding·native 값 | EncodeBound는 표현식 자료나 selector의 표현만 복사한다. native byte/file/factory를 열지 않으며 대상 실행의 검증을 대체하지 않는다. 일반 Encode와 request validation을 우회하는 전역 옵션은 없다. | 검토 완료 |
| content·XML·regex·format·DCE | 공개 content 문자열을 보존한다. regex/format·재귀 content·XML 직접 함수와 unused literal 링크 제거를 기존 전용 소비자로 재검증했다. | 검토 완료 |

독립 최상위 14개·통과 이벤트 34개가 52.848초로 통과했다. 명령·현재 manifest·원본
로그 해시는 `w02-d29-check.json`에 있다. 공유 response-properties·pagination 항목
fixture와 전체 후보 race, 마지막 affected Link/페이지 검사도 통과했다.

이 단위의 미검토 영역이나 확인된 미수정 결함은 없다. W08 선택·W09 수신·W11 전체
기능/성능과 W12 통합 리뷰는 별도 완료 조건이다.
