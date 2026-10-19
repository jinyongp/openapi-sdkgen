# 타입과 모델 코덱

## 값의 존재 여부와 null

생성된 `types` 패키지를 import하세요. `Optional[T]`는 부재와 값을 구분합니다. `Some(value)`는
값이 있고 제로 값은 부재입니다. `Get()`은 `(T, bool)`, `IsSet()`은 존재 여부를 반환합니다.
`Nullable[T]`는 null과 실제 값을 구분합니다. `Null[T]()`·`NonNull(value)`로 만들고 `Get()`·
`IsNull()`로 확인하세요. 선택적이면서 null도 허용하는 필드는 두 래퍼를 조합합니다.

## JSON 값

`types.Value`는 불변 JSON 트리입니다. `ParseValue([]byte)`는 JSON을 파싱하고
`ValueFrom(value)`는 순환을 검사하며 지원하는 Go 데이터를 변환합니다. `Kind`·스칼라 접근자·
`Get`·`Equal`·JSON 직렬화는 파일을 열거나 스트림을 실행하지 않습니다. 파일은 `Binary`나
네이티브 미디어 슬롯으로 표현합니다.

## 생성 모델과 코덱

`models` 패키지는 기준 모델을 정의하고 루트 별칭은 같은 타입을 가리킵니다. 작업과 컴포넌트의
입력·출력 투영에 `readOnly`·`writeOnly`가 반영됩니다. 공개 코덱 이름은 모델별로 생성됩니다.
`Decode<Model>(ctx, value)`·`Encode<Model>(ctx, model)`을 사용하고 미디어가 필요하면 대응하는
변형을 사용하세요. 생성 코덱이 스키마 검증과 데이터 변환을 담당합니다. `encoding/json`만으로는
스키마를 강제하지 않습니다. 허용된 미선언 응답 속성은 보존하며 독립 코덱과 서버 출력은 계약을 검증합니다.

## 열거값과 합성

스칼라 열거 리터럴은 이름 있는 상수이며 구조화된 값·null 리터럴에는 생성된 팩토리를 사용합니다.
`<Model>Values`·`Is<Model>`·`Parse<Model>`·`Equal`로 선언된 값을 다룹니다.
객체 합성 타입에는 분기 생성자·`As<Branch>() (T, bool)` 접근자·갱신 도우미가 있습니다.
판별자는 선언된 분기를 선택하며 충돌하는 태그와 잘못된 조합은 검증 오류입니다.
튜플·prefix items·추가 속성·재귀 참조·불리언 스키마는 각 스키마 제약을 유지합니다.
정확한 이름과 방향별 투영은 생성된 선언을 확인하세요.

## 동적 값과 네이티브 미디어

동적 작업은 `types.Value` 또는 `types.NativeInput`을 사용합니다. `MediaValue[T]`는
`AsValue`·`AsBinary`·`AsMultipart`로 스키마 값·바이너리·multipart 슬롯을 구분합니다.
`Binary`·`SourceFactory`·코덱·소유권은 [스트림 API](./streaming.md)를 확인하세요.
