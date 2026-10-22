# Graph·Cloudflare 선택 SDK 검증

이 문서는 `523cbd9`의 초기 검증 기록이다. 이후 선언·hover 수정까지 반영한 최종
Graph 선택 출력은 `final-typescript-verification.md`에서 새로 검증했다. Go 관련 입력은
동일성을 대조해 재사용하며 최종 corpus 결과는 `final-go-verification.md`를 따른다.

2026-10-22 사용자는 전체 Graph를 참고 검증으로 전환하고, 이어서 **corpus 37개 전체 +
Cloudflare 선택 API 검증**을 승인했다. 전체 corpus 38개 성공이나 전체 Graph 지원을
주장하지 않는다. 이 초기 실행의 제품 소스는 `3560450`과 같았다.

## 실제 결과

| 대상 | 검사 | 결과 |
| --- | --- | --- |
| Graph 선택 5개 | server 포함 TS 275파일·869,083 B / Go 155파일·3,553,341 B 생성, 모든 manifest hash 대조 | 통과 |
| Graph TS 전체 선택 출력 | 5.7.3 / 5.9.3 / 6.0.3 / 7.0.2, 한 source project, 전체 source strict·isolated declaration·전체 공개 선언 consumer | 모두 통과 |
| Graph 선택 계약 | 고정 원본의 정확한 route·operation ID 5개, 양성 사용례·선택 밖 key·잘못된 path 타입 거절 | 네 compiler 통과 |
| Graph Go 전체 선택 출력 | Go 1.27.2, server 포함 모든 package `go test ./...` | 통과 |
| Graph 실제 HTTP | 두 언어에서 인증 요구 조회/수정·그룹 소속 확인·메일 안내·로그인 세션 취소, 권한 오류·응답 검증 오류까지 각 7건 | 통과 |
| Cloudflare 선택 출력 | `GET /user/tokens/verify`, `user-api-tokens-verify-token`, Go server 포함 126파일·1,845,453 B, 전체 compile | 통과 |
| Cloudflare 실제 HTTP | 기존 corpus fixture 그대로 재사용, 경로·query·인증 header·성공·403·잘못된 enum 응답 | 3건 통과 |

Graph HTTP 검사는 route와 operation ID 양쪽 공개 진입점을 사용하고, 실제 loopback
서버에서 method/path/query/body/Authorization을 대조한다. 응답 body와 HTTP·schema
오류 전파도 확인했다. Microsoft·Cloudflare 실제 서비스 인증·가용성 검사는 아니다.
named/selection/generic/Link의 넓은 회귀 범위는 기존 제품 suite와 별도 단위 matrix로
검증하며 이 다섯 API만으로 전체 기능을 검증했다고 해석하지 않는다.

원본 SHA는 Graph `46bead9a6459cbe5f66d31094b23a614eb8da8f46690ee61404d97523e23daec`,
Cloudflare `179f1cd2bb3921aad64f9dcf05d45a0f9b9905fb2c1ea3ca2aabef53383ed2b3`이다.
generator SHA는 `d9b77bd77beecea47dcaeb678b2e38b0843b338a4656980f2ba2670fd6fd2b0c`이다.
선택 Graph key는 새 출력에서 추출해 기대값을 만들지 않고, 기존에 동결한 원본 catalog와
대조했다. 컴파일러별 실제 시간은 약 4.17/3.72/4.17/1.87초이며 비교 성능 표본이 아니다.

## 실패와 선택 한계

첫 Graph Go 실행은 모든 package를 컴파일한 뒤 PATCH HTTP 검사에서 실패했다.
테스트가 빈 204 응답을 가정했지만, 고정 원본은 해당 PATCH 성공에
`2XX application/json` entity를 선언한다. 원본 1,198,797행의 계약을 확인해 fixture를
200 JSON entity로 교정했고 전체 Go/HTTP 검사가 통과했다. 제품 코드는 바꾸지 않았다.
최초 fixture·producer·실패 로그와 교정 후 자료를 모두 보존했다.

처음 선정한 사용자 목록/조회/수정·sendMail·그룹 구성원 다섯 API는 생성만 통과했다.
TS 14,461파일·39,626,028 B, Go 24,115파일·1,116,588,640 B로 연결 모델 범위가 컸다.
이 선택의 compiler·HTTP 검사는 실행하지 않았다. `graph-selection.json`과
`graph-selected-usage.mts`는 이 초기 생성 전용 입력이며 통과한 사용례가 아니다.
실제 통과한 범위는 `graph-selection-bounded.json` 및
`graph-selected-bounded-usage.mts`다. 선택 operation 수가 작아도 연결 모델이 넓으면
출력이 커질 수 있으며, 선택 생성은 모든 API 조합의 자원 상한을 보장하지 않는다.

## 재현과 원시 자료

`measurements/2026-10-22/selected-sdk-verification.tar.gz`에는 작은 선택 SDK의 전체
source·manifest·strict 설정·선언/JS 출력·consumer·HTTP fixture·실행 producer와 결과를
보존했다. 2,140개 entry의 내용·크기·SHA를 다시 대조했다.
archive는 7,052,323 B, SHA-256
`6eb992b9ef002cec95054c7a2646ef76b2ba35d581abe2c99462ef6cecd0e531`이다.
고정 대형 원본과 툴체인 실행 파일은 archive에 중복 포함하지 않고 실제 hash를 기록했다.
초기 대형 선택 SDK는 정확한 manifest와 생성 결과만 포함했다.

리뷰에서는 원본 pin·기대 key의 독립성·전체 선택 source ownership·검사 강도·실패 분류·
HTTP 직렬화와 응답 검증·과거 Go corpus 재사용 범위를 확인했다. 이 자료는 최종 제품
통합 검사와 workstream close를 대체하지 않는다.
