# W05 리뷰 루프

범위는 계획 6절·D18/D24와 W05다. W01–W04의 완료 기록을 이 문서로 대체하지 않는다.
W06 타입 스트림과 W09 실제 수신 handler 연결은 각 작업에서 검증한다. W05의 공통
codec 방향과 소유권은 구현하고, 그 작업들의 실제 통합 검사는 미리 통과로 기록하지 않는다.

| 영역 | 상태 | 검토와 실제 검증 |
| --- | --- | --- |
| 공통 multipart 투영·참조·JSON 보존 | 검토 완료 | 공유 HTTP 계획만 투영을 소유한다. TS 입력 helper는 위임한다. Go root materialization은 해당 입력 occurrence에 한정하며 nested/prefix fallback·패턴·대안·조건부를 검사했다. |
| multipart DTO·nullable·tuple·합성 | 검토 완료 | Binary/Nullable·태그/분기·With·tuple의 source 슬롯을 보존한다. JSON DTO 계약과 propertyNames/const/enum/unique 검사를 분리했다. 표현 불가·잘못된 pointer·겹친 슬롯은 오류다. |
| multipart HTTP wire·옵션·파트 헤더 | 검토 완료 | 실제 named/positional/nested, FormData blob 파일명·plain 헤더·수동 빈 map, 기본/명시 미디어·binary 대안·필수/선언 헤더·custom/XML header를 검사했다. 미사용 encoding은 source metadata에만 남는다. |
| multipart 읽기·실패·Close·취소 | 검토 완료 | pull reader에 writer goroutine/pipe·전체 파일 버퍼·임시 파일이 없다. 64 MiB source·known length·Open/Read 실패·후속 미개방·진행 중 Open/Read 취소·느린 Close 대기·조기 HTTP 응답을 검사했다. RSS 측정은 W11이다. |
| 자유 형식·strict JSON·불변성 | 검토 완료 | 공개 MultipartValue/MultipartData, 생성자·slot/container 복사·일반 JSON component 분리·잘못된 문자열 오류와 source 유실 거부를 검사했다. |
| XML 분기·namespace·이름·오류 | 검토 완료 | 세 OpenAPI 버전, legacy/nodeType·anyOf/oneOf/if/dynamic·content XML을 검사했다. alias 충돌·Unicode prefix·CDATA/속성·본문 보존·DTD/이름/중복 namespace 오류·취소도 확인했다. |
| XML parameter·content header·파트 | 검토 완료 | 직접 스키마 프로그램을 사용한다. XML 매개변수/헤더와 선언·추가·패턴·반복 파트의 XML 선택을 실제 HTTP와 일반 content XML 호출자에서 재확인했다. |
| 폼 | 검토 완료 | 버전별 nullable·explode 배열/객체·JSON 필드·이스케이프·단일/복수 본문을 검사했다. finite form 응답은 기존 TS처럼 사용자 Decode 코덱을 요구한다. |
| MediaCodec 5개 방향 | 검토 완료 | Encode/Decode/EncodeParameter/DecodeParameter 실제 HTTP, DecodeInbound 공통 빌린 본문 경계, nil/typed nil·Open/Read/스키마/방향 누락/취소·물리 Close 횟수·독립 metadata·EOF trailer를 검사했다. 실제 handler는 W09다. |
| 생성 책임·이름·의존성 | 검토 완료 | multipartQueryPlanner가 직접 파트 질의, xmlEncodePlanner가 XML 매핑을 소유한다. 필요 helper만 closure에 넣고 전역 스키마 registry/interpreter는 없다. 공개 codec 이름 예약·package graph·순환/누락·sink/gofmt rollback 호출자를 재검토했다. |
| 마지막 수정의 최종 검사 | 검토 완료 | 1228개 소스의 w05-reviewed-code.json에서 Go target race 184.659초, generator/output/CLI/공통 계획과 vet/format/module/diff 통과. TS 55건 61.830초와 공통 8건은 해당 소스 해시가 불변임을 확인해 재사용한다. 기능 15개의 실제 통과 사례를 w05-feature-evidence.json으로 연결했다. |

확인하고 수정한 결함:

1. 중첩 multipart tuple의 바이너리 투영과 prefix의 itemEncoding fallback이 누락됐다.
   공통 투영과 실제 Go HTTP 소비자를 함께 수정했고 TypeScript 기존 회귀를 재확인했다.
2. custom 미디어 파트가 객체이면 JSON fallback이 host codec보다 먼저 실행됐다.
   기존 TypeScript의 dispatch 순서에 맞춰 고치고 JSON/XML/custom 세 파트 wire로 확인했다.
3. response MediaContext.Response가 본문 없는 snapshot이라 코덱 인자의 빌린 Body와
   달랐다. 컨테이너는 분리하고 같은 SDK 소유 Body를 읽게 연결했다.
4. MultipartValue 바이너리 문자열 구성에 panic 가능한 내부 constructor가 있었다.
   오류를 값에 보존하고 모델 codec·객체/배열 구성·wire/JSON export에서 전파한다.
5. multipart reader Close가 아직 Open 중인 source를 취소하지 못했다. 파트 실행에
   자식 context를 주고 Close/실패에서 취소하도록 수정했으며 독립 생성 race 검사가
   2.031초로 통과했다. 뒤에 추가한 CTL/DEL 헤더 거부는 다시 확인할 예정이다.
6. 기본 파트 코덱 확장 후 하위 기본 encoding을 무한히 만드는 생성 단계 재귀를
   재현했다. 해당 회귀 명령은 741.836초에 SIGQUIT로 실패했고 통과 근거가 아니다.
   선언한 중첩 multipart에만 하위 계획을 생성하도록 수정했다. 상한 있는 multipart/
   폼 회귀가 14.463초, 기본 XML 옵션·숫자 필드명 검사가 2.025초로 통과했다.
7. custom content 헤더만 있는 작업이 사용하지 않는 schema import를 내보냈다.
   실제로 직접 schema 함수를 부르는 헤더만 import를 요구하게 수정했다. 코덱 오류/
   헤더 방향과 XML parameter/header 검사 4.000초, 선언 헤더 포함 그룹 6.244초 통과.
8. dynamic XML 참조의 지역 이름 metadata가 target의 루트 이름으로 바뀌어 자식이
   왕복 중 사라졌다. 지역 metadata를 target 매핑에 덮어쓰고 native/content XML 그룹
   6.558초로 재확인했다.
9. 단순히 수동 MIME 프레임을 생성하면서 FormData의 blob 파일명·plain part 헤더와
   다르게 동작했다. 헤더 선언/명시 map의 수동 모드를 구분하고 Clone/Merge의 nil
   보존, named/positional 모드 제약과 schema 기본 미디어를 연결했다. 독립 wire/모드/
   source 수명 검사가 4.219초로 통과했다.

XML의 임의 추가 속성 및 prefixItems 기반 XML 매핑은 기존 TypeScript XML codec도
사용하지 않는다. 이 관찰을 추가 기능 구현 요구로 바꾸지 않는다. multipart 위치별 schema
선택은 별도 HTTP 규칙이므로 prefixItems를 유지한다.

10. nil/typed-nil 파트 source가 조기 HTTP 응답을 만나면 읽기 전에 성공할 수 있었다.
    Binary.Validate로 Open 없이 identity/metadata를 검사하며 실제 reader 오류는 소비할 때 전파한다.
11. 합성 binary 파트가 일치한 분기의 contentMediaType 대신 고정 fallback을 사용했다.
    직접 생성한 파트 질의로 첫 유효 binary 분기와 raw 문자열 assertion을 보존한다.
12. DecodeMedia가 DTO로 보존하지 못한 source를 조용히 버릴 수 있었다. 소비 슬롯을 추적하고
    경로 누락·겹침·잘못된 pointer와 표현 불가능한 source를 오류로 거부한다.
13. multipart response까지 입력 binary projection을 적용해 불필요 공개 타입/import를 생성했다.
    Input occurrence에서만 media DTO를 선택하고 공개 모델 변환 자료는 MultipartData로 명명한다.
14. 취소 callback과 명시 Close가 경쟁하면 실제 source Close 완료 전에 반환할 수 있었다.
    종료를 한 번 동기화하며 느린 물리 Close를 기다리는 생성 race 검사로 확인했다.
15. 복사한 코덱 trailer가 EOF에 확정되는 값을 잃었다. SDK lease를 공유하는 Body proxy에서
    독립 snapshot map에 최종 trailer를 복사한다. 실제 plain/gzip HTTP를 함께 검사했다.
16. 옵션으로 선택한 중첩 multipart에 하위 schema 질의가 연결되지 않아 nullable 객체의
    기본 미디어가 JSON 대신 text/plain이 됐다. root 파트 질의와 반복 위치 경로를 재사용하고
    일반 미디어 기본값도 직접 함수에 낮췄다. 새 소비자는 선언 필드·반복 배열·이름이 미정인
    patternProperties 경로를 모두 검사한다. 기본 encoding을 재귀 확장하지 않는다.

후보 1227개 소스의 전체 통과 결과는 마지막 수정의 완료 근거로 대체하지 않았다.
최종 manifest SHA-256은 `4adb9c923fa8c128578151012bbb058e23699bd80ae293e8e7e0b723236caf3d`다.
새 중첩 소비자 2.134초와 multipart/XML 그룹 59.049초에 이어 위의 전체 race와 static이
통과했다. 전체 실행은 Go·generator·output·CLI·공통 패키지의 실제 테스트 통과 이벤트
383건을 포함한다. 생성 소비자도 각각 race로 실행하며 기존 regex/compression의
CGO=0 검사도 유지한다. 로그·명령·도구·정확한 소스와 실제 통과한 기능별 사례를 보존했다.

미검토 영역·재확인 필요 영역·확인된 미수정 W05 결함은 없다. W06 typed stream과 W09
실제 handler, W11 전체 호환성·성능·플랫폼, W12 전체 최종 리뷰의 완료를 주장하지 않는다.
