# W08 리뷰 루프 — 검토 완료

기준은 계획 3·8·10절, D14·D16, W08 배정 기능96개·보충 계약이다.
task `4af366b3-e578-4740-8496-58bd1e9f075a`, run
`fa8cfa22-a0bb-401d-87b8-de49baa1fe26`, claim revision `1647`이다.

| 영역 | 상태 | 현재 경계 |
| --- | --- | --- |
| 생성 선택·Link closure·진단 | 검토 완료 | whole-document 명명·quarantine 뒤 선택하고 Link closure를 내부 의존으로 유지한다. 잘못된/빈/hidden 선택 E530, 전체 생략 E512, hidden Link E621을 실제 검사했다. ownership으로 선택 밖 진단과 dependency-only callback 진단을 제외한다. 생성 선택은 불필요 regex 패키지를 제거했다. |
| 독립 client·root 합집합·타입 | 검토 완료 | root/named의 concrete method·리소스·Link source와 code/category helper 범위를 분리했다. 명시 root와 기본 named 합집합의 HTTP 소비자, named-only import graph, keyword/main 이름 소비자가 통과했다. 합집합의 입력 계약 충돌로 각 entry의 유효한 리소스까지 생략되던 결함을 실제 컴파일 실패로 확인했다. entry별 선택자 조정과 복합 모델 이름을 분리했고 scalar·object의 별도 HTTP 소비자 및 기존 리소스 영향 검사가 통과했다. 전체 Go·CLI·공통 race434 이벤트와 최종 영향 race31 이벤트가 통과했다. Link 대상 alias와 Operation·Link 파생 이름의 전체 문서 예약도 검토했다. |
| 정적 reference·Prepare·Bind | 검토 완료 | sealed Ref·동기 Prepare·중복 route 제거·typed Bind와 typed Link 소비자 통과. 미준비/영/nil과 nil Reference 임베딩을 실행 전 거부한다. 외부 변경이 불가능한 prepared map과 Routes 복사본을 사용한다. 실제 nm에서 정적 경로의 registry·미선택 execute·regex compiler가 제거됐다. |
| 계산한 선택·lazy registry | 검토 완료 | 동시 Lookup과 정확한 ID/route·hidden/dependency 제외를 검증했다. strict Value 입력, Binary/MIME/complete/item/native source, Raw 원문·Stream 미선언 속성 보존, source 전용 Link·ByStatus를 실제 HTTP로 검사했다. NativeInput은 기존 carrier·코덱·실행 경로를 공유한다. source map snapshot·지연 열기·닫기·일회 소비·사전 취소·상충 입력을 검사한다. |
| metadata·원문·fingerprint | 검토 완료 | 공통 SourceMetadata의 원문 Unicode·큰 숫자·미지 확장·외부 참조·복사본·결정성·addon 전환 소비자가 통과했다. 후속 검토에서 addon 없는 schema format/default/제약 주석과 작업·재사용 응답 헤더의 폐기 예정 표시 누락을 확인해 보완했다. OpenAPI 3개 버전의 실제 Go AST 문서 검사와 기존 annotated enum 소비자가 통과했다. 제약 키 순서는 공통 계획에서 공유한다. 최종 영향 Go31·TS158 이벤트와 정적 검사가 통과했다. |
| inspect·공통 target 경계 | 검토 완료 | 공통 PlanInspector/OperationInspection과 TS alias·기존 JSON key를 보존한다. Go 계획 snapshot·resource call 복사와 별도 golang key를 검사했다. 분석 시 생성 설정을 무시하고 명시 Go 옵션을 검사하는 CLI 소비자 통과. 전체 TypeScript 패키지 race564.110초 통과와 CLI193 이벤트 재검증, 후속 Go/CLI/common race434 및 최종 TS158 이벤트가 적용된다. 초기 결합 명령의 도움말 기대값 실패를 전체 성공으로 기록하지 않는다. |
| 생성 관리·공유 영향 | 검토 완료 | CLI의 selected/named/metadata check·incremental 전환과 사용자 go.mod/go.sum/user.go/custom 파일 보존 통과. resource omission W513/E513·중복 path fallback과 Content-Type 매개변수 유실을 수정해 소비자로 확인했다. 새 패키지의 의존 경계 목록과 도움말의 TypeScript 전용 기대 문구도 갱신했다. 영향 race10개와 전체 CLI193개 이벤트가 통과했다. 독립 리소스 후속 수정까지 포함한 전체 회귀434 이벤트가 통과했다. 그 뒤 annotation 관련 10개 파일만 변경됐으며 최종 영향 Go31·TS158과 정적 검사로 재검증했다. |
| 측정·최종 근거·배정 | 검토 완료 | 실제 정적/동적/생성 선택 nm·동일 결과·바이너리 크기와 각각 cold/warm 3회 표본을 확인했다. 작은 fixture의 시간 구간이 겹쳐 속도 우열을 주장하지 않는다. 최종 소스에서 18개 빌드 표본과 nm를 다시 확인했고 96개 배정·5개 보충 계약을 feature-evidence에 연결했다. 구현/리뷰 validation은 이 기록을 기준으로 별도 등록한다. |

이 단위에 미검토 영역·확인된 미수정 결함은 없다.

최종 소스는 `w08-verified-code.json`의 1,336개 파일,
SHA-256 `7ffccd36305e34c249d070602db3341868a50faecc3737ea02563f3ca17f4b68`다.
전체 race434는 annotation 보완 전 후보 기준이며 이후 바뀐 10개 파일의 목록과
최종 Go31·TS158·vet/fmt/mod/diff 및 측정의 원시 로그·해시는
[w08-feature-evidence.json](w08-feature-evidence.json)에 있다. 테스트 전체 결과와
후속 영향 검사 결과를 합쳐 적용하며 전체 race를 마지막 소스에서 재실행했다고 하지 않는다.

정적·계산한 선택·생성 선택의 바이너리는 각각 10,503,134 / 11,293,319 /
10,463,496 bytes, 생성 파일은 71 / 71 / 62개다. 정적 참조는 레지스트리와 미선택
실행 코드를 제거하고 regex의 초기화 심볼 18개는 남긴다. 생성 선택은 그 패키지도
없애며 계산한 선택은 regex 심볼 156개와 전체 조회 경로를 유지한다.
Go build cache를 비운 표본의 중앙값은 6.005 / 6.469 / 5.871초,
warm 표본은 0.067 / 0.078 / 0.069초다. cold 구간은 겹친다.
작은 fixture에 대한 로컬 내부 결과이며 대형 명세·실행 성능·RSS·플랫폼·공개 benchmark
검증을 대신하지 않는다.
W09–W12와 전체 대응·성능·통합 리뷰는 계속 필수 범위다.

devtools revision `1656`에서 W08을 현재 정의 기준 완료했다. 구현 record는
`dad78801-03b1-435c-9f3f-b2a99580d4b7`, 리뷰 record는
`9c382a81-3777-468e-b2be-6037cda7be3b`다.


## W11 통합 변경의 영향 재검토

dynamic Stream도 공통 execute의 Accept fallback을 사용하도록 연결했다. 두 entry 합집합을 생성한 named 소비자에서 미사용 실행은 제거돼도 공유 정규식 패키지 초기화 일부가 남는 것을 실제 nm/deps로 확인했다. 생성 선택의 의존 제거와 구분하며 기존 typed/dynamic/선택·Link 소비자가 전체 Go CI를 통과했다.

최종 근거는 [W11 리뷰](w11-review.md), [기능/통합 증거](w11-feature-evidence.json),
[최종 소스 적용성](w11-verified-applicability.json)에 있다. 위의 이전 단위 소스와 실행 기록은
당시 이력으로 유지하며 최종 코드에서의 재검토와 혼합하지 않는다. W11의 별도 구현/리뷰
record는 `aa4df9e5-0c5d-4462-a282-e3642a77cac4`와
`8eaf5468-7404-4d77-9cd0-808a686a783d`다.
