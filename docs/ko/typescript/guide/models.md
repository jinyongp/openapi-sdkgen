# 모델과 검증

생성된 요청·응답 타입은 선택한 API의 스키마를 따릅니다. 요청에는 `OperationInput`과
`RouteInput`, 이름 있는 컴포넌트 스키마에는 `ComponentInput`·`ComponentOutput`을
사용합니다. 입력·출력 투영에는 `readOnly`와 `writeOnly`가 반영됩니다.

## 선택 필드와 null

선택 필드는 생략할 수 있고, null을 허용하는 필드에는 `null`을 넣을 수 있습니다.
두 조건은 독립적입니다. 스키마가 허용하지 않으면 누락된 필드를 `null`로 바꾸지 마세요.
생성된 검증기는 실제 교환하는 데이터를 선언된 스키마에 맞춰 검사합니다.
TypeScript 검사는 생성 시 선택할 수 있으며 애플리케이션의 컴파일러를 사용합니다.

## 필요한 타입 찾기

컴포넌트·작업 타입 도우미는 [생성 타입](../reference/types.md), 요청 입력과 실패 처리는
[클라이언트 호출](./client.md), 검증된 스트림 항목은 [스트림 API](../reference/streaming.md)를
참고하세요.
