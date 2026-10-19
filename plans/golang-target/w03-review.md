# W03 구현 리뷰

범위는 등록한 W03, 계획 5·6·6.1·10.3절과 D05/D11/D12/D23이다. 소스와 생성 결과,
공유 TypeScript 호출자, 생성 SDK의 실제 HTTP 동작을 함께 검토했다. W03의 첫 후보
`w03-code.json`은 이름 공간 결함 수정 전 기록으로 보존하며 완료 근거로 사용하지 않는다.
최종 후보는 `w03-final-code.json`이며 필수 검증을 완료했다.

## 검토 영역

| 영역 | 소스·관찰 범위 | 검토 상태 |
| --- | --- | --- |
| 공개 호출·입력·옵션 | 필수/선택/무입력, 옵션 컨테이너 복사, 초기화 전 호출, 재사용·동시 실행, 빈 값과 상속 | 검토 완료 |
| HTTP 직렬화 | 경로/쿼리/헤더/쿠키/querystring, style/explode/content/allowReserved, base path, 원래 wire 이름 | 검토 완료 |
| 본문·코덱 경계 | JSON/text/binary, 미디어 선택·범위, 사용자 encode/decode/parameter 방향, source 소유권·길이·실패 | 검토 완료 |
| 응답·오류 | 정확한 상태/범위/default와 미디어 우선순위, 성공 union, 유한 Raw, 타입 헤더·오류, 안전한 오류 출력 | 검토 완료 |
| 전송·압축·수명 | net/http 설정/연결/Jar/redirect, native 제어 헤더, 압축 체인, 중복 해제, EOF/Close/취소/timeout | 검토 완료 |
| 공통 의미·호환성 | 공유 상태/헤더/참조/binary 투영, 작업 생략 소유권·중복 ID, Go 메서드와 헤더, 영향받은 TypeScript 호출자 | 검토 완료 |
| 생성 구조·이름 | Prepare의 불변 계획, 작업별 파일, 계약 root 선예약, 원래 pointer 기반 충돌 이름, 내부 함수 이름 공간, 실제 의존 그래프 | 검토 완료 |
| 검증 현재성 | 현재 후보 전체 회귀·race·정적 검사·소스 해시 일치 | 검토 완료 |

## 확인하고 수정한 결함

- 요청 실패 뒤 nil 응답을 참조할 수 있던 경로를 제거했다. 초기화하지 않은 client도
  `OPERATION_NOT_PREPARED` 세부 오류로 종료한다.
- 유한 Raw를 완전히 읽고 닫도록 바꾸고, DTO 검증/변환이 끝날 때까지 실행 context를 유지한다.
  순차형 Raw는 읽지 않은 본문과 수명을 넘긴다. EOF 후 재차 읽어도 자체 취소 오류로 바뀌지 않는다.
- SDK 정리로 발생한 취소가 원래 디코딩/종료 실패를 덮지 않도록 구분했다. 외부 취소·deadline은 보존한다.
- source의 reader+error, nil/typed nil, 음수/불일치 길이, shared one-shot, 읽기 실패와 Close 소유권을 확인했다.
  요청 source 읽기 실패를 네트워크 오류로 분류하던 경로도 수정했다.
- client/요청 옵션의 header/map 복사를 확인하고 대소문자 중복을 결정적으로 합친다.
  raw header가 선언·전용 옵션을 덮지 않으며, cookie 능력은 실제 송신이 있을 때 검사한다.
- 쿼리 구분자 이스케이프, null/String 변환, cookie의 전체 값/항목별 변환, 폼 content,
  allowReserved의 값 한정 적용을 독립 wire 기대값으로 대조해 수정했다.
- HTTP text의 UTF-8 디코딩과 문자열의 USV 변환을 구분했다. 연속된 unpaired surrogate와 BOM도 검사한다.
- 응답 헤더의 합성 DTO가 루트 readOnly/writeOnly를 잘못 투영하던 문제를 수정했다.
  각 물리 값에 별도의 미디어 루트 정책을 적용하고 중첩 contentSchema 제약은 유지한다.
- 명시 type 없는 정수 enum/const의 헤더 변환과 원래 schema pointer·매개변수 문서를 보존했다.
- HTTP 오류 JSON에서 응답 결과·요청·본문이 따라 나가지 않도록 했다. 서버 코드와 typed body 접근은 유지한다.
- 표준 Go HTTP가 Header map 대신 native 필드에서 읽는 Content-Length/Transfer-Encoding/Trailer를 연결했다.
  충돌·불일치·표현 불가능한 입력을 조용히 무시하지 않는다. 설정된 DefaultClient.Transport도 유지한다.
- 버전별 format/contentEncoding binary 의미를 공유하고, raw binary 투영으로 false 스키마를 우회하지 못하게 했다.
  일반 범위는 `MediaValue[T]`로 타입이 있는 값과 소유한 바이너리를 구분하며 accessor는 snapshot을 반환한다.
- 중첩 component 모델이 HTTP root 이름을 먼저 차지하던 문제, 서로 다른 미디어 이름의 정규화 충돌,
  component/HTTP root/하위 검증 함수의 충돌을 수정했다. 여러 충돌이 함께 있는 유효 문서를 실제로 컴파일한다.
- core의 값 처리를 media/wire로 옮기고 W01의 임시 내부 Raw/전송 인터페이스를 제거했다.
  생성 패키지의 `go list -deps`에서 계획의 의존 방향과 wire의 표준 라이브러리 한정을 확인한다.

## 검증과 남은 범위

생성 SDK 소비자는 각각 race 검사로 실행한다. 압축 HTTP 소비자와 정규식 소비자는
추가 CGO=0 실행도 수행한다. 파일 rollback·기존 schema/DTO/literal/union/XML/format·
공식 fixture·실제 바이너리의 미사용 데이터 제거도 Go 전체 회귀에 포함한다.

최종 명령/결과와 정확한 코드 해시는 구현 기록 및 workstream의 구현/리뷰 record에 남긴다.
최종 Go 전체 회귀(95.316초), race(104.298초), 공유 TypeScript 호출자 회귀(11.625초),
vet/format/module/diff 검사를 통과했다. 1176개 소스의 해시와 추가 파일 유무도 확인했다.
최종 manifest SHA-256은 `eae77a33eedc7bc050c6bb77c7d288bcfbbf0f95ab2dfa6e27a05c56c1336eef`다.
전체 TypeScript 실행 519.775초는 최종 binary helper 위임 전 결과이며 해당 변경은 위의 호출자 회귀로 재검증했다.

W04 인증·서버 선택, W05 XML/폼/multipart 전체 미디어와 코덱의 나머지 방향,
W06 타입 스트림·프로토콜·어댑터, W07 리소스/Link/페이지/확장, W08 선택/metadata/inspect,
W09 수신 기능, W10 문서/도구, W11 전체 동등성·성능·플랫폼, W12 최종 리뷰는 남아 있다.
이 후속 범위를 W03 완료로 계산하지 않는다. 후속 변경이 공유 실행 경로를 바꾸면 해당 관찰을 다시 검증한다.


## W11 통합 변경의 영향 재검토

일반 호출의 암묵 Accept 삽입을 제거했다. 동일 원본 fixture의 실제 Go/TS SDK를 독립 기대값8종으로 확인하고 옵션·오류·원래 값·전송 횟수를 재검증했다.

최종 근거는 [W11 리뷰](w11-review.md), [기능/통합 증거](w11-feature-evidence.json),
[최종 소스 적용성](w11-verified-applicability.json)에 있다. 위의 이전 단위 소스와 실행 기록은
당시 이력으로 유지하며 최종 코드에서의 재검토와 혼합하지 않는다. W11의 별도 구현/리뷰
record는 `aa4df9e5-0c5d-4462-a282-e3642a77cac4`와
`8eaf5468-7404-4d77-9cd0-808a686a783d`다.
