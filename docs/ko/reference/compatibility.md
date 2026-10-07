# 호환성 검증 결과

OpenAPI 문서별 SDK 생성과 TypeScript 타입 검사 결과입니다.

GitHub Actions에서 측정한 결과입니다. 표에서 측정한 실행을 확인할 수 있으며,
결과와 함께 [실행 정보](/compatibility-results/provenance.json)를 내려받을 수 있습니다.

## SDK 생성 결과

**API 호출용 SDK**는 API에 요청을 보내는 코드입니다. **웹훅·콜백 포함 SDK**는
외부에서 받는 요청을 처리하는 코드도 함께 생성합니다.

표의 **20개 중 18개 성공**은 전체 문서 20개 중 18개에서 SDK 생성과 타입
검사가 성공했다는 뜻입니다. **생성된 호출 API 수**는 생성에 성공한 SDK의
호출 메서드 수입니다.
웹훅·콜백이 있는 문서는 `--with server` 결과를 포함합니다.
**SDK 생성 시간**은 문서를 읽고 SDK 파일을 저장하는 데 걸린 시간이며,
요약 표에는 문서별 시간의 합계를 표시합니다.

<CompatibilityResults locale="ko" />

웹훅·콜백이 있는 문서는 `--with server`로 수신용 코드를 함께 생성할 수 있습니다.
수신 코드는 들어온 요청을 파싱·검증하고 타입이 지정된 핸들러에 전달합니다.
사용 방법은 [웹훅·콜백 수신 가이드](../guide/server.md)를 참고하세요.

## 필요한 API만 생성하기 {#graph-selection}

생성할 API를 지정하면 필요한 호출과 의존 코드만 SDK에 포함됩니다.
Microsoft Graph beta를 예로, 사용자·그룹·드라이브 API 9개를 선택해 측정했습니다.
아래 표는 Actions에서 측정한 선택 SDK의 생성 결과입니다.

<GraphSelection locale="ko" />

[API 선택 설정](../guide/selective-client.md#generation)으로 애플리케이션에 필요한
부분을 생성할 수 있습니다. 아래 문서별 결과는 전체 API 생성 기준입니다.

## 어떤 문서를 확인했나

### 주요 API 제공자

GitHub, Stripe, Cloudflare, GitLab, DigitalOcean, Twilio의 공개 API 문서입니다.
대규모 API와 여러 파일로 나뉜 스키마를 포함합니다.
각 제공자의 생성 결과와 시간은 아래 첫 번째 표에서 확인할 수 있습니다.
6개 문서 모두 SDK 생성과 타입 검사를 통과했습니다. Microsoft Graph beta는
[필요한 API만 생성하기](#graph-selection)에서 생성 시간과 용량을 별도로 비교합니다.

### 독립 표본

APIs.guru에서 선정한 20개 제공자의 OpenAPI 3.0·3.1 공개 문서입니다.
API 호출용 SDK는 18개 문서에서 생성됐습니다. Listen Notes의 웹훅과
UniCourt의 콜백에 서버 기능을 적용하면 전체 20개 문서에서 생성됩니다.

### 제공자 공개 문서

Zenith Payments가 공개한 OpenAPI 3.2.0 문서 2개입니다.
인증, 스키마 참조, 후속 호출 연결을 포함한 두 문서 모두 SDK 생성에 성공했습니다.

### 기능별 예제 문서

Resend의 실제 API 문서 1개와 직접 작성한 예제 문서 9개입니다.
OpenAPI 3.2, 스트리밍, 참조 처리를 다룹니다. Resend 문서와 웹훅 예제는
서버 기능을 함께 적용해 생성합니다.

기능별 지원 범위는 [OpenAPI 지원 범위](./capabilities.md)에서 확인할 수 있습니다.

## 문서별 결과

문서별 결과에서 호출 API 수와 생성된 웹훅·콜백 핸들러를 확인할 수 있습니다.
문서 이름을 누르면 측정에 사용한 OpenAPI 원본(JSON·YAML)이 열립니다.
웹훅 수신만 있는 문서는 호출 API가 0개이며 웹훅 핸들러가 생성됩니다.
원본 보고서와 입력 목록도 내려받을 수 있습니다.

<CompatibilityResults locale="ko" evidence />
