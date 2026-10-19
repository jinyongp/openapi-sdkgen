# W02 구현 리뷰

범위는 승인한 W02와 A03/A04/A05/A21/A22다. W03 이후 HTTP·미디어 송수신·스트림·
선택·서버·도구·성능 통합은 이 리뷰의 완료 주장이 아니다. 기준 소스는 최종 검증 시
`w02-final-code.json`으로 고정했다. 기존 설계 probe는 제품 passing evidence로 대체하지 않는다.

## 영역별 검토

| 영역 | 검토 결과와 근거 |
| --- | --- |
| 공통 의미와 TypeScript 호출자 | schema plan 이동, reference 정규화, annotation/폐기 판단 추출을 확인했다. 선택한 이동 회귀 이후 공통화 최종 TypeScript 전체 회귀도 통과했다. 이후 변경은 Go 전용이다. |
| JSON 값 경계 | 반복형 parser, JSON 문법·마지막 중복 키·대소문자·정확한 키·WTF-8·UTF-8 전송 치환·숫자·코드 포인트를 검토했다. JSON 값은 비공개 상태를 갖고 컨테이너를 복사한다. `ValueFrom`은 순환·비유한 수·비JSON Go 값을 거부한다. |
| 공개 타입과 코덱 | 네 presence 계약, 제로값, nil collection, required 존재 검사, 입력/출력 투영, 단순 allOf, 튜플/rest, 재귀 포인터·alias cycle, 추가 속성 충돌, nested/root JSON 메서드를 확인했다. |
| enum·합성 API | scalar 상수, mixed factory, nullable enum, 주석형 enum, 값 목록·동등성, 독립 As snapshot과 평가 키를 사용하는 With를 검토했다. discriminator mapping/defaultMapping은 원래 합성 검증을 우회하지 않는다. |
| 직접 검증 프로그램 | 기본 제약·객체·배열·숫자·합성·조건·의존·contains·평가 집합·재귀/dynamic scope를 확인했다. 실행 오류와 불일치를 구분하고 상태는 호출별로 소유한다. 63개 독립 사례와 저장된 공식 fixture 64개를 사용한다. |
| format·regex | vocabulary의 assertion/annotation, 19개 format, 고정 URL 의존성, Unicode 17, lazy 패턴, 동시 사용, 실행 복잡도와 취소를 확인했다. 동적 regex의 무제한 cache는 없다. Node 대조 42개와 CGO=0 소비자도 검증한다. |
| content·XML | JSON/text/base64/XML 내부 검증과 외부 문자열 보존, root media 정책과 nested 경계를 확인했다. XML은 wire의 표준 라이브러리 파서와 직접 schema 디코더를 사용한다. namespace·attribute·wrapped array·CDATA·composition·condition·reference/dynamic scope를 검토했다. 일반 XML 송수신/인코딩과 multipart는 W05다. |
| 생성·의존 경계 | Prepare가 DTO·검증·문맥 view·XML 디코더·이름·import·런타임 closure를 확정한다. Emit은 template·파일별 format·sink만 수행한다. 원래 IR/annotation map은 SourcePlan에 남기지 않는다. |
| 기존 fixture·현재 증거 | 기존 정상 문서 18개의 schema root 230개를 Go 모듈로 생성·컴파일했다. HTTP 작업은 제거한 schema 소비자이므로 기존 작업 흐름까지 완료했다고 계산하지 않는다. 의도적 오류 문서는 기존 compiler/진단 회귀로 확인한다. |

## 확인한 결함과 수정·재검토

| 재현 조건 | 원인·수정 | 영향 검증 |
| --- | --- | --- |
| `$ref`와 같은 노드의 dynamic anchor를 하위 참조가 사용 | 참조 실행 전에 anchor를 scope에 등록하도록 고쳤다. | 재귀/dynamic JSON·XML 소비자 |
| W01 이름 정규화 충돌 | 순번 대신 원래 경로 SHA-256 8자리/4자리 확장을 적용하고 파생 이름도 예약했다. | W01 재검증·리뷰 두 결과를 새 basis에서 기록해 완료 |
| nil 루트 배열의 표준 JSON 직렬화 | 기본 slice alias가 null을 출력했다. 루트 named codec 경계를 적용했다. | nullable/tuple/root codec 소비자 |
| 미디어 루트와 모순된 content 선언 | 일반 컴포넌트 검증과 특정 root 정책을 혼동했다. 공유 private 알고리즘 위에 문맥 view를 준비하고 nested 경계를 보존했다. | content root/nested/ref/union 소비자 |
| GoDoc과 선언 사이 빈 줄 | Go AST가 주석을 선언 문서로 붙이지 않았다. 문서 바로 뒤에 선언을 출력한다. | 생성 AST의 type/상수/factory/필드 문서 검사 |
| XML 불일치가 anyOf 대체 분기를 차단 | XML 구문 실패를 실행 오류로 반환했다. 형식 불일치는 검증 오류로, context/regex 실행 오류는 그대로 유지한다. | XML/anyOf fallback 소비자 |
| XML 속성의 탭/개행과 문자 참조 | 원래 공백 정규화와 참조 해석의 순서를 구분했다. CDATA는 보존한다. XML 텍스트는 builder로 누적한다. | 속성·CDATA·entity·namespace·잘못된 구문 소비자 |
| XML 표현의 모호한 allOf 또는 확장 annotation | 알려진 표현 필드만 결정적 순서로 합치고 모호한 표현은 원래 위치의 target 진단으로 반환한다. 임의 객체 확장은 표현 비교에 넣지 않는다. | `GO_XML_REPRESENTATION` 진단과 XML extension 소비자 |
| 미사용 enum/const가 정적 JSON 바이너리에 잔존 | 패키지 초기화에서 파싱했다. zero-value lazy storage와 첫 사용 시 한 번 파싱으로 바꾸고 URL 기준값 초기화에도 적용했다. | 실제 GetHealth/GetUnused/GetUnusedURL 바이너리의 데이터 제거·보존 대조, 동시 초기화 race |
| 직접 넣은 Go 문자열과 JSON 값의 정체성 불일치 | UTF-8/WTF-8 밖의 Go 문자열은 비JSON 값으로 거부하고 정상 surrogate pair를 정규화한다. 정규화 후 map/추가 속성 키 충돌은 오류다. | JSON Value·DTO·키 충돌·lone surrogate 소비자 |
| 같은 allOf 필드의 deprecated가 사라짐 | 동일 검증 노드라도 annotation은 모두 보존해 폐기 판단에 반영한다. | 실제 생성 필드의 GoDoc·projection/allOf 회귀 |

## 검증 상태와 한계

개별 소비자와 영향 회귀, 최종 source basis의 Go target 전체·race·vet·format/module 검사와
TypeScript 공통화 영향 회귀는 통과했다. source hash도 기록 전에 확인한다.
W02 영역의 미검토·재검토 필요 항목과 확인된 미수정 결함은 없다.
실제 명령·결과는 [구현 기록](implementation.md)에 기록했다. passing evidence는 별도의
두 task validation에 기록하며 이 문서만으로 완료를 대신하지 않는다.

정규식 42개와 URL 28개 설계 대조는 전 규격을 열거한 증명이 아니다. 실제 제품 검증은
생성 모듈의 별도 실행이며 고정 엔진·Unicode·의존 버전이 바뀌면 영향 검증을 다시 한다.
독립 기대값에 따라 기존 TypeScript의 날짜/시간·끝 줄바꿈 버그는 Go에 복제하지 않았다.
그 차이를 TypeScript 코드 수정이나 전체 동등성 완료로 계산하지 않는다.

실제 도달성 검사는 미사용 JSON 리터럴과 URL 초기화 데이터까지다. 전체 XML·정규식·
스트림·메타데이터·선택/동적 조회의 도달성·성능·플랫폼 검증은 W11/W12에 남아 있다.
W08의 선택 구현은 root 이름뿐 아니라 중첩 DTO/enum/helper의 전체 문서 이름 예약도
보존해야 한다. 현재 선택 요청은 진단으로 거부하며 선택 안정성을 완료로 기록하지 않는다.


## W11 통합 변경의 영향 재검토

inline allOf의 재귀 DTO 확장을 활성 객체/validator 포인터로 고치고 원래 assertion과 cycle 거부를 유지했다. 새 실제 생성 소비자·최종 outer race·전체 Go CI와 원본 코호트를 재검증했다.

최종 근거는 [W11 리뷰](w11-review.md), [기능/통합 증거](w11-feature-evidence.json),
[최종 소스 적용성](w11-verified-applicability.json)에 있다. 위의 이전 단위 소스와 실행 기록은
당시 이력으로 유지하며 최종 코드에서의 재검토와 혼합하지 않는다. W11의 별도 구현/리뷰
record는 `aa4df9e5-0c5d-4462-a282-e3642a77cac4`와
`8eaf5468-7404-4d77-9cd0-808a686a783d`다.
