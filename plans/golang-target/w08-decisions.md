# W08의 구체 API와 검증 경계

기준은 D29 계획 8절이다. 아래는 그 계약을 구현한 공개 API와 측정 해석이며,
W08 또는 후속 작업의 완료 기록을 대신하지 않는다.

- `operations.Name()`은 `Ref[NameOperation]`을 반환한다. 참조는 생성 코드만 구성하며
  원래 operation ID와 route를 보존한다. `selective.Prepare(refs...)`는 동기적으로
  중복 route를 제거한다. `Bind(client, ref)`는 `(NameOperation, error)`를 반환하며
  결과의 `Call(input)`과 `Links`에서 구체 타입을 유지한다. nil·제로 참조와 준비하지
  않은 참조를 HTTP 전에 거부한다. nil 인터페이스를 임베딩한 외부 구조체도 포함한다.
- `LookupOperation`·`LookupRoute`는 `(Ref[DynamicOperation], bool)`을 반환한다.
  내부 레지스트리는 첫 조회에서만 `sync.Once`로 구성한다. 정적 참조에는 동적
  binder 함수를 보관하지 않는다. 동적 변환을 호출할 때만 해당 구현에 연결한다.
- `DynamicOperation.Call(Value)`의 입력은 원래 wire 이름으로 구성한
  `path`·`query`·`header`·`cookie`·`querystring`·`body` 객체다. 미디어 선택 본문은
  `{contentType, value}`로 표현한다. 생성된 엄격한 코덱과 기존 실행 경로로 검증한다.
  `Do`는 투영된 불변 Value, `Raw`는 실제 응답과 전체 decoded 값, `Stream`은 지연
  시작하는 `Stream[Value]`를 반환한다. 응답의 보존 대상 미선언 속성도 유지한다.
- Value는 파일이나 무한 source를 표현하지 않는다. `CallNative(types.NativeInput)`은
  불변 `Value`와 별도 `Binaries`·`Sources`를 받는다. map 키는 Value에 실제 존재하는
  위치의 JSON Pointer이며 요청의 `/body` 아래만 허용한다. 항목 factory는 전체
  `/body` 또는 미디어 선택의 `/body/value`를 대체한다. 중첩 파일 슬롯을 보존한다.
  이 입력은 기존 Link carrier와 같은 계약을 공유하며 새 전송 경로를 만들지 않는다.
  map은 호출 구성 시 복사한다. Binary 버퍼와 factory의 소유권·재실행 계약은 유지하고
  실행 전에 source를 열지 않는다. `SourceOnce`는 복사본 사이에서도 한 번만 열린다.
- 동적 `Links.Follow(name, raw, invocation)`은 원래 Link 이름을 받는다.
  `ByStatus(status)`는 정확·범위·default definition을 명시적으로 고른다. invocation의
  `SourceInput`·`Input`은 Optional Value이며 네이티브 입력용 `SourceNative`·
  `InputNative`와 각각 동시에 지정할 수 없다. 원본 private 응답 snapshot을 사용하고
  대상 입력은 대상 스키마로 검증한다. Follow에서는 통신하거나 source를 열지 않는다.
- 기본 root와 독립 client는 자기 선택의 concrete Operations·Routes·리소스·Link source를
  가진다. 실행 의존 대상은 내부에 남긴다. Link 대상의 필요한 공개 타입 alias도 제공한다.
  전체 문서에서 이름을 예약한 뒤 선택을 적용한다. 독립 entry는 root·다른 entry·
  metadata·selective를 import하지 않는다. keyword·main 이름의 패키지는 접미사로 구분한다.
  리소스 선택자의 입력 계약은 각 entry의 선택에서 조정한다. 다른 entry의 다른 경로 타입
  때문에 유효한 선택자가 생략되지 않는다. 복합 선택자 모델은 client별 이름을 예약하고,
  실제 bound operation 실행은 공유한다. 생략 진단과 strict 옵션도 각 entry에 적용한다.
- 원문 metadata 직렬화는 두 target이 공유한다. addon에서만 원래 문서 전체를 저장하고
  `Document()`는 매번 독립 bytes를 반환한다. 큰 JSON 숫자의 원문 의미·미지 확장·외부
  참조를 보존한다. 기본 entry와 operations의 `OpenAPIVersion`·`OpenAPIVersionLine`은
  문서 전체를 import하지 않는 상수다. inspect는 공통 PlanInspector를 사용하되 기존
  TypeScript JSON key를 보존하고 Go 결과를 `golang` key로 제공한다.
  metadata 확장 없이도 모델 주석에 format·default·example·제약 등 원래 annotation을
  남긴다. 폐기 예정 작업은 root·named·리소스·정적 참조의 Go 문서에 표시하며 매개변수와
  재사용 응답 헤더의 Deprecated 주석도 보존한다. 기본값을 실행 중 자동 주입하지 않는다.
- 리소스 생략은 W513이며 `--fail-on-resource-omission`에서 E513이다. 충돌·지원하지
  않는 경로 segment·중복 또는 누락 path 매개변수를 진단한다. 구체 Routes와 Operations는
  가능한 경우 유지한다. 생성 선택은 소유 작업 밖의 extension 진단을 끌어오지 않는다.

실제 linker 검사는 정적·계산한 선택·생성 선택을 같은 프로그램으로 비교한다. 정적
선택에서 전체 레지스트리와 선택하지 않은 실행 메서드는 제거됐다. 선택하지 않은 패키지의
일부 초기화 데이터는 남을 수 있다. 생성 선택은 그 패키지의 소스 컴파일까지 제거한다.
초기 3회 cold/warm 표본은 구간이 겹쳐 컴파일 속도 차이를 주장할 수 없었다. 마지막 소스의
재측정과 원시 결과는 별도로 기록한다. 로컬 측정은 내부 검토 근거이며 공개 benchmark는
계획대로 GitHub Actions 측정 run을 사용한다.
