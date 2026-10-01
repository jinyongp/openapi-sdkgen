# 호환성 검증 결과

고정된 OpenAPI 문서로 TypeScript SDK를 생성하고 엄격한 타입 검사를 수행한
결과입니다. 다양한 실문서와의 호환성, 특정 기능의 동작 검증을 나눠서
보여줍니다.

기능별 지원 계약은 [OpenAPI 지원 범위](./capabilities.md)를 참고하세요.

## 결과 한눈에 보기

<CompatibilityResults locale="ko" />

**기본 클라이언트**는 추가 기능 없이 생성한 결과입니다. **필요한 기능 포함**은
웹훅이나 콜백을 수신하는 문서에 `--with server`를 적용한 결과까지 확인합니다.
기본 생성에 성공한 문서는 그 결과를 유지합니다. **생성된 API 작업**은 기본
SDK에 실제로 포함된 작업 수입니다. 생성이 막힌 문서는 0으로 계산하며,
서버 기능을 적용해 생성한 작업은 원본 보고서에서 별도로 집계합니다.

문서가 통과하려면 오류 검사를 마치고, 생성을 막는 문제가 없는 상태에서 SDK를
생성한 뒤, 생성물의 `@ts-nocheck`를 제거하고 TypeScript **7.0.2**의 엄격
모드로 타입 검사를 통과해야 합니다. 제공자의 실제 API를 호출한 결과는
포함하지 않습니다. SDK를 사용하는 프로젝트의 컴파일러 요구 사항은
[TypeScript 요구 사항](../guide/getting-started.md)에서 확인할 수 있습니다.

독립 표본은 **2026년 10월 1일**, v9.0.0 이후의 소스 변경 사항을 반영해
다시 측정했습니다. 이 페이지의 수치는 저장된 측정 결과이며, v9.0.0 릴리스
바이너리의 결과를 뜻하지 않습니다.

## 실문서 결과의 의미

### 독립 표본: 일반적인 문서 호환성

20개 제공자의 문서는 생성기를 실행하기 전에 고정된 APIs.guru 문서 모음에서
선정했습니다. 개발 중 사용한 7개 제공자는 제외하고, 파일 크기를 네 구간으로
나눠 구간마다 5개 문서를 골랐습니다. 재측정할 때도 같은 문서를 사용합니다.
OpenAPI 3.0과 3.1 문서로 구성되어 있어, 다양한 크기의 실문서 호환성을
확인하는 데 적합합니다.

Listen Notes의 웹훅과 UniCourt의 콜백은 서버 기능이 필요합니다.
기본 클라이언트만 생성하면 중단되지만 `--with server`를 적용하면 통과합니다.
나머지 문서는 기본 검사를 통과하며, 스키마 참조 처리 수정 후의 `eos.local`도
여기에 포함됩니다.

### 제공자가 공개한 3.2 문서: 일반적인 API 구조

Zenith Payments가 공개한 API 문서 2개는 모두 `openapi: 3.2.0`을 선언합니다.
제공자가 배포한 원본 바이트를 보존해 검증했으며, 두 문서 모두 생성과 타입
검사를 통과했습니다. 인증, 로컬 참조, 후속 호출 연결 등 일반적인 API 구조를
확인한 결과입니다.

이 문서들은 `QUERY`, `querystring`, `itemSchema` 같은 새 3.2 기능을 사용하지
않습니다. 따라서 해당 공개 문서와의 호환성을 보여주는 근거로 해석해야 합니다.
새 3.2 기능이 실서비스에서 사용되고 검증됐다는 근거는 아니며, 제공자
1곳의 문서 2개로 전체 제공자의 호환성 비율을 추정할 수도 없습니다.

## 새 3.2 기능은 어떻게 검증하나

기능 검증용 입력은 Resend의 실제 3.1 문서 1개와 직접 작성한 검증 문서 9개로
구성됩니다. 표준에 따른 동작, 참조 처리, 지원 경계를 각각 확인하도록 작성해
3.2 기능과 스트리밍, 참조 조합을 의도적으로 검증합니다.

위 표는 이 문서들의 생성과 엄격한 타입 검사 결과입니다. 별도의 유효·무효
입력 테스트, 직렬화 검사, 스트리밍 실행 테스트에서는 입력의 수용·거부와
실제 동작을 확인합니다. 표준 기반 검증 문서도 프로젝트에서 직접 작성한
테스트 입력입니다.

Resend 문서와 웹훅 검증 문서는 `--with server`가 필요합니다.
Fetch의 지원 범위를 확인하는 문서에서는 `TRACE`, `CONNECT`, `TRACK`을
진단하고 생성을 생략하며, 지원하는 API 작업은 생성합니다. 이처럼 문서가
통과해도 일부 작업이나 보조 기능의 생략 진단이 남을 수 있습니다.

새 3.2 기능을 사용하는 실서비스 문서가 공개되면 추가 검증 근거로 활용할
수 있습니다. 현재 해당 기능의 동작 근거는 표준 기반 테스트와 실행 테스트입니다.
선정 목적이 다른 검증 입력들을 합쳐 하나의 호환성 백분율로 표시하지 않습니다.

## 검증 근거 확인하기

요약과 문서별 표는 저장된 JSON 보고서에서 자동으로 구성합니다. 문서 빌드 시
입력 목록의 해시, 문서 구성, 집계 수치가 일치하는지 확인합니다. 다운로드 파일은
원본 바이트를 유지하며, 입력 식별 정보와 상세 진단도 확인할 수 있습니다.

<CompatibilityResults locale="ko" evidence />

<details class="details custom-block">
<summary>직접 재측정하기</summary>

저장소의 개발 도구를 설치한 환경에서 다음 명령을 실행합니다. 독립 표본은
고정된 원본 파일을 내려받아 검증합니다. 나머지는 저장소에 커밋된 공개 문서의
고정본과 테스트 문서를 사용합니다.

```sh
# 독립 표본
just agent compatibility-fetch
just agent compatibility-verify
just agent compatibility-benchmark

# 제공자 공개 문서
just agent compatibility-verify test/compatibility/production32.json test
just agent compatibility-benchmark test/compatibility/production32.json test .tmp/compatibility-production32.json 3m

# 기능·지원 경계 검증
just agent compatibility-verify test/compatibility/modern.json test
just agent compatibility-benchmark test/compatibility/modern.json test .tmp/compatibility-modern.json 3m
```

재측정 결과는 실행한 소스 버전에 따라 달라질 수 있습니다. 저장된 결과와
비교할 때는 같은 입력과 TypeScript 버전을 사용하세요. 자세한 측정 방법과
과거 결과는 저장소의
[호환성 검증 기록](https://github.com/jinyongp/openapi-sdkgen/tree/main/test/compatibility)에
있습니다.

</details>
