# W09 구현 경계와 현재 검증

기준은 D29 계획 9·10.3절이다. 이는 진행 중인 구현 기록이며 W09 전체 완료가 아니다.

- 수신 path item은 compiler IR의 기존 매개변수 상속·본문·응답·보안·메서드 규칙을
  공유한다. `ReadInboundOperations`는 원래 webhook/callback pointer를 유지한다.
  host가 HTTP 경로를 정하며 callback expression을 실행하거나 endpoint를 자동 등록하지 않는다.
- 공통 `InboundScopes`·`CollectInbound`는 원문 identity·표현식·source operation과
  component 구분을 보존한다. 생성 선택 밖의 source callback과 hidden/omitted source는
  제외한다. 재사용 callback과 root webhook의 문서 소유권은 유지한다.
- server 확장 없는 수신 선언은 E505·document/block이다. 선택 밖 callback은 이를
  유발하지 않는다. 유효한 전체 문서 선언을 먼저 읽어 이름을 예약하고 실제 선택의 계약을
  준비한다. 선택 밖 잘못된 callback 때문에 유효한 선택이 막히지 않는다. 잘못된 선택 내
  수신 계약의 Go 계획은 E506·document/block으로 보고한다.
- TypeScript 수신 구현을 확인했다. 같은 미디어에 schema와 itemSchema가 있으면 수신은
  itemSchema의 Source로 처리한다. Go 입력 표현을 준비할 때 복사본에서 complete schema를
  제외하며 원문과 다른 미디어의 schema는 보존한다. 호출자의 송신 모드 선택 계약을
  수신에 임의 적용하지 않는다.
- 타입 경로의 validator·모델·코덱은 기존 직접 생성 계획을 사용한다. `server/runtime`은
  models-free이고 client core·transport·operations를 import하지 않는다. 실제 생성
  패키지의 server-only import graph로 이 방향을 검사했다. 이제 server addon이 Go
  Prepare에서 수신 계약을 준비하고 모델·webhook/callback 핸들러·응답 생성자를 방출한다.
  이 연결은 진행 중이며 모든 미디어와 범용 helper까지 완료했다는 뜻은 아니다.
- execution은 요청 본문과 열린 source를 응답 전송 종료까지 소유한다. complete body는
  기본 8 MiB 이내에서 한 번 읽고 CloneBody는 private bytes의 독립 reader를 반환한다.
  stream clone은 거부한다. child context 취소 후 source/body를 한 번 닫으며 clone/종료
  상태를 동기화한다. 익명 skip, 인증 미설정401, 인증오류500, 복제초과413, host 거부 본문,
  동시 clone와 metadata 변경 격리, 대기 read 취소를 실제 생성 runtime으로 확인했다.
- 실제 HTTP/1·HTTP/2에서 헤더를 먼저 받은 뒤 입력을 보내는 테스트를 추가했다.
  초기 구현은 세 흐름 모두 시간 초과했다. 원인은 첫 bytes 전까지 flush하지 않는 헤더와
  HTTP/1의 일반 응답 경로에서 남은 요청을 자동 drain하는 동작이었다. stream 응답은
  헤더를 즉시 flush한다. HTTP/1에서 입력이 살아 있으면 일반 응답도 duplex와 flush가
  필요하다. EOF·종료 여부를 동기화해 불필요한 capability 요구를 피한다.
  native writer에서는 본문 처리 전 duplex를 미리 활성화하고, 실제 전송에 필요한데 host
  adapter가 지원하지 않으면 헤더/source Open 전에 capability 원인과500을 반환한다.
  미지원 writer, 실제 세 교환과 종료의 수정 후 검사가 통과했다. 전체 stream codec과
  opaque host writer의 실패 정리는 후속 typed 통합에서도 검증한다.
- 열린 HTTP/1 요청에서 인증401과 handler500도 응답 전에 body Close가 drain하다가
  멈추는 것을 실제 재현했다. 오류는 private 원인을 노출하지 않는 유한 길이 본문으로
  먼저 전송하고 flush한 다음 요청을 정리한다. 읽지 않은 HTTP/1 오류 요청은 연결을
  재사용하지 않는다. 이미 보낸 상태는 수정하지 않으며 cleanup 오류는 원인과 함께
  OnError에 남긴다. HTTP1/2의 두 오류 상태를 포함한 7개 실제 교환이 통과했다.
- 초기 재현의 테스트 클라이언트 pipe도 context 취소 때 닫도록 보완했다. 그렇지 않으면
  net/http의 쓰기 goroutine 종료 대기 때문에 테스트 자체가 멈췄다. 내부 생성 소비자에
  `go test -timeout=45s`와 command WaitDelay를 적용해 진단과 종료를 제한한다.
  해당 테스트 프로세스 외의 작업을 종료하지 않았다.
- 수신 매개변수의 scalar 후보는 TypeScript가 문자열부터 검사한다. 응답 헤더용 기존
  Go HeaderScalar의 숫자 우선순위를 그대로 재사용하지 않는다. TypeScript는 query 배열의
  각 값에 쉼표 분할도 적용한다. 별도 수신 coercion·correlated schema 대조에서 이 관찰
  가능한 규칙을 보존한다. 별도 coercion과 생성 시 준비한 형태 그래프가 기존 직접 생성
  validator를 호출한다. 문자열·숫자·배열·tuple·조건부 item·patternProperties·상관된
  객체 대안, form/deepObject/label/matrix/raw-cookie, 선택 객체의 부재 검사가 통과했다.
  그래프는 핸들러 생성 시 한 번 준비하며 요청마다 스키마 원문을 해석하지 않는다.
- 실제 server addon으로 방출한 webhook/callback http.Handler가 host 지정 경로에서
  동작하며 원래 identity/expression을 유지한다. JSON 수신·파라미터 검증·auth 후보·
  CloneBody·타입 있는 응답의 실제 HTTP 검사와 잘못된 파라미터가 인증 전에 실패하는
  검사가 통과했다. 타입 있는 NDJSON 입력을 응답 타입으로 변환하는 실제 HTTP/1·HTTP/2
  교환도 입력 전 헤더 전송과 종료까지 통과했다. 응답 variant 필드는 비공개다.
- complete stream과 여러 body 미디어 방출을 연결했다. XML은 기존 직접 생성 코덱에
  요청/응답 방향을 뒤집은 준비 view를 전달하며 client 작업 파일로 방출하지 않는다.
  이 추가 경로의 개별 미디어·경계·오류 검증은 진행 중이다. 모든 미디어·typed 헤더·
  범용 normalized-schema helper·최종 전체 리뷰/검증은 여전히 남아 있다.

## 추가 통합 검증

- JSON/XML/form/text/binary 요청 미디어 유니언과 typed XML 응답·JSON/XML 헤더가
  실제 HTTP에서 통과했다. form을 custom codec으로 잘못 분류하던 415를 재현해 고쳤다.
- 기존 SSE Event fixture를 webhook으로 옮겨 complete·incremental·동시 선언의 필드
  보존과 잘못된 data의400을 검사했다. schema-only complete에 불필요한 coercion 그래프를
  방출하던 컴파일 오류를 고쳤다. 소진한 증분 입력의 오류는 핸들러가 Err를 반환하지
  않아도 송신 전 확인한다. 빈 필수 스트림과 프레임 오류도 같은 입력 단계에 속한다.
- 범용 `CompileSchema`·`Schema.Validate`·`Decode`·`DecodeInbound`·`Respond`를 추가했다.
  normalized keywords와 정의 집합을 받으며 URL fetch와 확장/program 실행은 하지 않는다.
  typed validator와 분리된 호환 경로다. 포맷은 typed 경로의 직접 생성 알고리즘을 공유한다.
  공식 unevaluatedProperties/unevaluatedItems 기대값 검사가 통과했다. 범용 XML·추가
  키워드/미디어·오류·동시성 검증은 아직 남아 있다.
- multipart 파트 디코딩은 `internal/multipartcodec` 아래에서 client response와 inbound
  request가 공유한다. server가 client core/transport를 참조하지 않는다. 공유 계층 이동 때
  네이티브 bytes를 source로 바꿔 기존 buffered view를 깨뜨린 회귀를 발견해 고쳤다.
  `BinaryFromBytesWithMetadata`와 buffer 복사는 파일 메타데이터와 Bytes 뷰를 함께 보존한다.
  기존 중첩·재귀 multipart 응답의 수정 후 검사가 통과했다.
- named multipart/form-data의 Content-Type 없는 문자열 파트에는 schema 추론 JSON 대신
  FormData의 text/plain 기본값을 적용한다. 실제 입력의 숫자·반복 배열·파일명/버퍼,
  필수 part header와 byte-limit 검사가 통과했다. fixture는 기존 공통 multipart 바이너리
  투영의 `format: binary` 계약을 사용한다. 순차 multipart와 송신·XML part 등은 남아 있다.
- 서버 응답에 한해 native multipart DTO 표시를 별도로 기록해 Output 투영의
  readOnly/writeOnly 의미를 유지하며 파일 source를 표현한다. multipart 인코더·item/frame
  생성·파트 한계도 `internal/multipartcodec`에서 공유한다. 실제 파일 응답은 DTO 준비 중
  source를 열지 않고 전송 때 한 번 열고 닫으며 filename/파트 헤더를 보존했다.
- 수신 순차 MIME은 Input 투영의 파트 계획을 사용한다. 송신 계획과 함수 이름을 구분해
  같은 webhook의 양방향 MIME에서 생긴 검증 함수 중복을 고쳤다. 실제 HTTP1/2에서 입력 전
  헤더와 filename/바이트 보존을 확인했다. native MultipartData adapter로 같은 교환도 통과했다.
  types의 stream media helper가 모델·MultipartData·portable Value·frame 경계를 구분한다.
- 범용 XML은 기존 wire parser/renderer를 사용한다. namespace·attribute·CDATA·wrapped
  배열·정의 참조와 HTTP Decode/Respond, XML contentSchema의 실제 값 검사가 통과했다.
  임의 스키마와 직접 생성 validator의 경계는 유지한다. 전체 XML 조합·동적 참조 등의
  최종 리뷰와 추가 검증은 남아 있다.
- 스트림 헤더 이후 source 오류를 실제 HTTP1/2에서 재현했다. 그냥 handler를 반환하면
  부분 응답 뒤에 정상 종료가 표시돼 client가 성공으로 읽었다. source/context 정리와
  OnError를 끝낸 다음 `http.ErrAbortHandler`로 net/http의 연결/stream 중단 경로를 사용한다.
  상태200은 바꾸지 않으며 client read가 실패하는 수정 후 검사가 통과했다.


## 본문·전송·열린 미디어 경계 재검토

- complete 기본 8 MiB와 stream 기본 1 MiB의 정확한 바이트 경계/초과, UTF-8 프레임,
  증분 합계가 8 MiB를 넘는 입력을 실제 생성 런타임으로 확인했다. complete 제한을
  증분 합계 제한으로 적용하지 않는다.
- 전송 전에 Content-Type·boundary·Content-Length 충돌을 검사한다. 충돌인데 source를
  열고 200을 보내던 재현을 고쳐 500과 Open 0회를 확인했다. 연결 취소 시 막힌 응답
  Read가 남는 재현은 송신 child context의 AfterFunc에서 Close하도록 고쳤다. Binary의
  기존 단일 Close 래퍼를 사용하며 정상 정리에서도 context를 먼저 취소한다.
- 사용자 코덱은 수신 DecodeInbound와 송신 Encode, 매개변수 DecodeParameter와 응답
  헤더 EncodeParameter를 사용한다. 실제 chunked trailer, hook 간 metadata 독립,
  옵션 map 복사, host source의 lazy Open/단일 Close와 정리 시 취소가 통과했다.
- 빈 querystring은 빈 문자열 값으로 존재한다. form 본문의 미선언 속성은 coercion에서
  지우지 않고 직접 모델 검증에 전달한다. closed form schema의 추가 속성 400을 확인했다.
- 선택 본문의 없음과 공백, JSON null, text/plain의 공백은 기존 TypeScript 규칙을 따른다.
  Content-Length를 부재의 근거로 쓰지 않는다. 실제 생성 소비자에서 길이 힌트 0인
  본문도 읽고, 필수 빈 JSON/text/form/binary를 400으로 처리하는 검사가 통과했다.
  binary 선택 입력의 빈 바이트는 존재하는 값이다. complete 순차 입력의 빈 배열은
  실제 body가 있고 schema가 허용하면 유효하고, 증분 필수 source의 빈 소진은 오류다.
- 열린 미디어 DTO에 native multipart 표현을 추가했다. MediaValueFromMultipart와
  AsMultipart는 slot 컨테이너를 복사하며 BinarySource는 소유권 계약을 유지한다.
  server의 wildcard media 모델만 native 투영을 추가하고 클라이언트 모델은 유지한다.
  실제 wildcard 수신/응답에서 파일명과 파일 바이트를 보존하고 JSON/XML/text/binary
  경로도 함께 통과했다. optional multipart의 빈 폼은 None으로 처리한다.
- 유한 named MIME의 XML·중첩 XML 파트도 기존 part schema/직접 XML 코덱을 사용해
  실제 값까지 검사했다. XML 대안의 첫 성공 객체를 후속 문자열 후보가 덮어쓰는
  공통 디코더 결함을 수정하고 기존 XML 및 열린 XML 호출자를 재검사했다.

최종 소스의 전체 Go race와 TypeScript 검증이 진행 중이다. 사용자 stream protocol과
adapter의 수신/응답 양방향 실제 HTTP/1·HTTP/2 통합도 추가 검사 중이다. 이 기록은
W09 완료나 미수행 검사의 통과를 뜻하지 않는다.

## 응답 헤더와 최종 범위 재검토

- WithHeaders로 넣은 선언된 선택 헤더가 검증을 우회하는 결함을 실제 생성 소비자로
  재현했다. X-Count의 정수 계약에 invalid를 넣어도204를 반환했다. extra header는 유지하되
  선언된 header의 raw 값은 JSON/XML/form/custom/simple 역변환 뒤 직접 스키마로 검사한다.
  per-entry header graph는 sync.OnceValue로 한 번 준비한다. 이미 typed 값이 있는 헤더의
  이중 지정은 기존처럼 거부한다. XML은 이미 준비한 response-header decoder를 재사용한다.
- exact200/range2XX/default의 실제 생성자와 complete NDJSON 응답을 검사했다. exact가
  존재하는200을 range 생성자로 보내거나 range가 있는201을 default로 보내면500이다.
  유효한201 complete source는1·2 프레임으로, default418은 text로 송신한다.
- custom protocol과 adapter가 모두 실행되는 HTTP1/2 교환에서 어댑터의10배/역변환을
  확인했다. 전체 addon runtime·typed webhook/callback의 실제 import graph 검사가 통과했다.
- normalized generic schema에서 strict recursive dynamic scope와 조건부 contains/unevaluated
  상호 작용을 실제 값으로 검사했다. 독립 CLI component callback도 HTTP1/2에서 실행했다.
