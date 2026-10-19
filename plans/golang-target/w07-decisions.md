# D29: W07 계약의 검증 기록

구현·리뷰 기준은 plan.md 7.2절이다. 이는 기능 범위를 줄이지 않고 Go의 구체 타입과
전송 환경 차이를 명시한다. 작업별 최종 근거와 W07 완료는 별도 기록한다.

- 공통 extension planner에 envelope·sort·pagination·오류 스키마 의미를 모았다.
  공통 HTTP plan에는 소스 소유권·Link 대상/closure/server·resource terminal을 둔다.
  TypeScript의 synthetic IR·타입 문자열 렌더링은 어댑터에 남긴다.
- 동일한 response-properties fixture를 실제 Go/Node 소비자로 검사했다. 닫힌 응답의
  writeOnly 속성도 현재 TypeScript가 미선언 속성으로 보존한다. Go의 응답 profile을
  맞추고 요청·standalone의 strict 검증은 유지했다.
- 동일한 pagination-item-variants fixture로 scalar·tuple의 서로 다른 응답 제약,
  envelope와 순회를 검사했다. TypeScript는 객체 속성의 minLength 차이를 타입 JSDoc
  차이로 E653 거부한다. 이 결과를 느슨하게 바꾸지 않았다. Go는 허용한 전체 항목
  계약을 union으로 생성해 직렬화 때 첫 응답의 제약을 잘못 적용하지 않는다.
- 실제 HTTP에서 SourceInput의 binary·multipart 파일·항목 factory를 후속 요청으로
  전달하고 열기·닫기 횟수와 Follow의 무통신을 검사했다. 같은 미디어의 전체 schema와
  itemSchema가 다른 경우도 포함한다. 전체 Body와 부분 contentType/value 교체는
  별도 타입 API로 구분한다.
- component Link·component response 참조와 명시 Origin 입력을 실제 Go HTTP로
  확인했다. Node Fetch 테스트의 Origin 생략은 browser 환경 위임 계약이다. Go는
  네이티브 헤더를 금지하거나 browser의 자동 Origin 공급을 흉내 내지 않는다.
- 다른 작업의 Raw source·직접 만든 source·다른 대상 override는 Go 컴파일러가
  거부한다. 유효한 문장부호 경로 `/-`가 리소스 이름 때문에 생성 전체를 막던 오류를
  재현했고 Go 이름 fallback에서 메서드 terminal을 사용하도록 수정했다.
