# W09 리뷰 루프

기준은 D29 계획 6·7·9·10.3절과 W09 task다. claim revision `1657`,
task `95599dda-3061-4f95-8331-a568d5c7cae1`, run
`614fafa8-dd74-4b0f-95a6-f38ef205f71b`에서 작업했다. 마지막 진행 checkpoint는 `1663`이다.
최종 구현 검증과 별도 리뷰 검증이 기록되기 전에는 완료로 취급하지 않는다.

| 영역 | 상태 | 검토·수정·재검토 근거 |
| --- | --- | --- |
| 공통 수신 의미·위치·선택 | 검토 완료 | compiler IR의 기존 path-item 상속·보안·메서드·body·response 규칙과 target-neutral collector를 검토했다. 문서 webhook/component callback, 선택한 operation callback의 소유권·원래 pointer/표현식/소스 작업을 보존한다. hidden/omitted/선택 밖 callback 제외, E505 document/block, E506 계약 실패, whole-document 이름 예약과 선택 안정성 검사가 통과했다. 별도 CLI 생성 component callback을 실제 HTTP1/2 host 경로에 연결했다. |
| 타입 핸들러·모델·출력 variant | 검토 완료 | source 입력과 Output의 readOnly/writeOnly 투영, private response variant/생성자, 직접 validator, host 경로·callback identity를 검토했다. exact/range/default 우선순위와 완전 NDJSON 응답을 실제 생성 API로 확인했다. WithHeaders가 선언된 선택 헤더 검증을 우회해 invalid 정수를204로 보내는 결함을 재현하고 lazy 직접 schema graph로 수정했다. XML decoder 중복 등록은 기존 decoder 재사용으로 고쳤다. 원시/타입 헤더와 XML/custom 호출자를 재검사했다. |
| 매개변수·인증·본문 복제 | 검토 완료 | 문자열 우선 coercion·tuple/조건부/pattern/상관된 대안, form/deepObject/label/matrix/cookie와 빈 querystring을 검토했다. 매개변수 검증 뒤 Authenticate, 5개 Params 그룹·raw security/candidate·metadata snapshot·동시 독립 CloneBody를 확인했다. 익명 skip·401·500·413·host rejection 정리와 입력 실패 단계가 통과했다. form 미지 필드를 버리던 결함은 보존 후 strict 모델 검증으로 고쳐 closed schema400을 확인했다. |
| 전체 미디어·한계·코덱 | 검토 완료 | JSON/XML/text/form/binary/named MIME/순차 MIME/open media와 custom codec 방향을 검토했다. form 분류415·MIME 기본값400·open runtime 누락·XML 대안 객체 덮어쓰기를 재현해 수정했다. XML·중첩 XML part의 실제 값, wildcard 파일명/바이트, optional 부재·공백·JSON null과 거짓 Content-Length0, 기본8MiB/1MiB 정확한 바이트/초과 및 UTF8 프레임을 확인했다. chunked trailer·hook별 metadata 독립·옵션 map 복사와 source lazy Open/단일 Close가 통과했다. |
| 증분 입력·응답·duplex·수명 | 검토 완료 | 실제 HTTP1/2에서 NDJSON·MIME와 custom protocol/adapter의 양방향 교환, 느린 소비·연결 취소·막힌 read·lease를 검사했다. 입력 전 헤더 대기의 교착, HTTP1 조기401/500 drain 교착, flush 후 source 실패가 정상 EOF로 보이는 결함, 취소 뒤 막힌 응답 read 잔류를 각각 재현해 duplex/초기 flush·유한 오류 선송신·ErrAbortHandler·송신 context AfterFunc Close로 고쳤다. 헤더/source Open 전 capability·미디어 충돌 실패도 확인했다. |
| 범용 정규화 schema/runtime | 검토 완료 | CompileSchema·Validate·Decode·DecodeInbound·Respond/정의 집합·JSON/XML·포맷/콘텐츠·동적 참조·조건부 평가를 검토했다. 공식 unevaluatedProperties/Items 사례, strict recursive dynamic scope, conditional/contains/unevaluated, namespace/attribute/CDATA/wrapped XML·정의 override·XML contentSchema가 통과했다. malformed normalized 계약·정규식, 취소·동시 재사용·정의 컨테이너 변경, URL fetch/프로그램 실행 부재를 확인했다. typed 경로는 범용 해석기를 사용하지 않는다. |
| 의존 경계·공유 호출자 | 검토 완료 | multipart decode/encode/item/frame/bound는 client와 server가 아래층 internal/multipartcodec를 공유한다. 버퍼를 source로 바꿔 Bytes 뷰를 깨뜨린 회귀는 BinaryFromBytesWithMetadata와 복사로 고쳤다. 실제 full addon runtime 및 typed webhook/callback go-list graph에서 root/client core/transport/operations/selective 의존이 없고 runtime은 models도 가져오지 않는다. 공유 client multipart/XML, 공통 IR, 기존 TypeScript 제공자·타입·runtime을 재검사했다. |
| 최종 검증·기능 근거 | 검토 완료 | 최종1383개 소스 SHA256 df8c5cd75ecc097cf9250c0c1dedfdd05e9df4c8fc688f6717c31843cd068ef3에서 Go-target/common/CLI race461개 통과 이벤트(동일 이름 중복 제외455), Go395.449초와 CLI9.346초를 확인했다. vet/fmt/module/diff가 통과했고 exact source와 원시 로그 해시를 보존했다. 이전 전체 패키지 race와 최종 영향 수신/XML/MIME 검사가 통과했다. TypeScript 전체 도구·제공자 검사도 통과했다. 배정5개 기능과3개 보충 계약을 실제 증거에 연결했다. |

이번 단위의 리뷰는 검사 횟수가 아니라 위 8개 영역과 확인 결함의 원인·영향 호출자를
기준으로 수행했다. 이 단위에 미검토 영역·확인된 미수정 결함은 없다.
별도 구현·리뷰 validation과 task 완료의 등록 식별자는 구현 기록에 남긴다.

허용된 경계는 유지한다. 호스트가 listener·인증 정책·callback URL 연결을 소유한다.
XML의 arbitrary additional/prefixItems 매핑은 D29 및 W05의 기존 제한을 유지한다.
범용 helper는 normalized 키워드/정의 집합만 받으며 원격 참조를 가져오거나 확장을 실행하지
않는다. 사용자 hook의 공유 상태 동시성은 호스트 계약이다. 전체 문서·도구·성능·플랫폼의
통합 완료와 전체 최종 리뷰는 W10–W12에서 계속 수행한다. commit/push/출시는 하지 않았다.
