# 전체 Graph·Cloudflare의 검증 범위 결정

2026-10-22 사용자는 MS Graph 전체 변환의 비용과 실용성을 재검토한 뒤
“전체 Graph는 참고 검증으로 전환”을 선택했다. 이 결정은 이전 계획의 전체 Graph
필수 성공·반복 측정 조건을 대체한다. 기존 원시 실패·완료 기록과 단위 개선은 유지한다.
이어서 사용자는 “37개 전체 + Cloudflare 선택 검증”을 선택했다. 이 후속 결정은
Cloudflare 전체 compile 필수 조건도 대체한다. 아래 범위가 두 결정을 반영한 현재 기준이다.

## 현재 필수 범위

- 고정 corpus 중 Cloudflare를 제외한 37개 문서(실제 API 28개·fixture 9개)를 Go 1.27.2에서
  server 포함 생성·전체 compile하고 필요한 HTTP 계약을 검증한다. Cloudflare는
  `GET /user/tokens/verify` 선택 SDK 전체 compile·HTTP 계약을 검증한다.
  원본 pin뿐 아니라 정확한 CI 입력 보정도 대조한다. 38개 전체 성공으로 집계하지 않는다.
- 고정 Graph 원본에서 실제 사용 시나리오의 정확한 route·operation ID를 선택하고,
  선택된 SDK의 전체 TS source·공개/negative consumer를 네 compiler에서 strict 검사한다.
  Go 선택 SDK도 생성·전체 compile·HTTP 동작을 검증한다. 선택한 경로와 전체 경로를
  혼동하지 않으며 선택 경계·named client·server·runtime 호환성은 유지한다.
- 기존 전체 제품 회귀·대표 fixture의 모듈/declaration·generic/Link·runtime 검사를
  최종 basis에서 확인한다. 완료된 성능 표본은 해당 코드·조건이 유효한 범위에 한해
  재사용하고 실제로 확인한 개선만 보고한다.
- 출력·증분/no-op/rollback·fingerprint/ABI·배포·문서 정합성을 검증하고, 작업별 리뷰와
  커밋 후 마지막 통합 리뷰·검증 기록을 완료해 workstream을 닫는다.

## 제외한 필수 조건

전체 Graph 29,581개 operation의 TS 네 compiler 성공, compiler별 3회 반복,
Go 전체 compile 성공 및 Graph 5회 생성/cold/warm/배포·binary/runtime 측정은
필수 완료 조건에서 제외한다. 이를 만족시키기 위한 추가 구조 변경·상한 증대·유료 runner·
호스트 설정 변경도 진행하지 않는다. 전체 Graph source 사용을 위한 references 도입을
소비자 설치·배포의 새 필수 조건으로 만들지 않는다.

전환 당시 진행 중인 전체 TS 검사는 추가 5분 안에 자연 종료됐다. 29개 project 중
20개가 통과하고 다음 project가 RSS 보호 종료됐다. 부분 통과·생성 성공·파일별 API
진단을 전체 strict 성공으로 바꾸지 않는다. 미실행 Graph Go warm·후속 전체 반복은
시작하지 않는다. Cloudflare 전체 출력도 약 976 MB로 확인했고, 전체 compile과 한 번의
제한된 warm 진단은 RSS 보호 종료됐다. 후속 사용자 결정에 따라 추가 전체 Cloudflare
생성·compile·warm 반복이나 이를 위한 구조 변경을 진행하지 않는다.

Graph는 인증 요구 조회/수정·그룹 소속 확인·메일 안내·로그인 세션 취소의 실제 API
5개를 선택했다. 이 SDK의 server 포함 TS 네 compiler·Go 전체 compile·두 언어 HTTP
검증이 통과했다. Cloudflare 선택 SDK도 전체 compile·기존 HTTP 계약이 통과했다.
정확한 원본·선택 key·결과·초기 테스트 실패와 출력 한계는 `selected-sdk-verification.md`에
기록했다. 작은 선택 수가 항상 작은 연결 모델을 보장하지는 않는다.

## 기록과 상태 처리

이전 `AC-TS-FULL`·`AC-GO-FULL`은 제거 이력으로 남기고, 선택 SDK 검증과 37개 corpus를
위한 새 acceptance를 연결한다. 이전 실패를 passing validation으로 바꾸거나
waiver로 숨기지 않는다. 변경된 정의에 필요한 claim sync·검증 basis를 갱신한다.
이미 끝난 작업의 근거는 관련 동작과 조건이 유지되는 경우에만 재사용한다.

현재 계획·실행 정책·workstream 정의에서 이 결정이 우선한다. `preflight.md`와
과거 실행·archive는 당시 범위와 결과의 이력으로 보존한다. 공개 문서와 CI에는
전체 Graph 지원 또는 전체 compile 성공을 새로 약속하지 않는다.
