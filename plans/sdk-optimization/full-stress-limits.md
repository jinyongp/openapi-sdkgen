# 전체 대형 사례의 참고 결과

2026-10-22 사용자 승인에 따라 전체 Graph·Cloudflare는 필수 성공 조건에서 제외했다.
이 기록은 이미 종료된 실행을 보존하며 추가 전체 실행을 요구하지 않는다.

| 실행 | 실제 결과 | 해석 |
| --- | --- | --- |
| 최신 Graph TS native 전체 | checker 557.55초, 29개 project 중 20개 완료, 다음 project RSS 11,779,776,512 B 보호 종료 | 전체 source·공개 consumer 통과 아님 |
| Cloudflare 전체 재시도 | 1,237.38초, RSS 11,380,215,808 B 보호 종료 | 생성은 완료됐으나 전체 compile·HTTP 성공과 최종 auditor report 없음 |
| Cloudflare retained warm 첫 준비 | toolchain 검증 전 `GOSUMDB=off` 때문에 차단 | compiler 미실행, compile 실패로 집계하지 않음 |
| Cloudflare retained warm 실제 실행 | 200.02초, RSS 11,195,817,984 B 보호 종료 | `internal/operations` compiler에서 종료, HTTP 통과 아님 |

Cloudflare retained module의 API 54,542파일·976,456,445 B는 최초 보존 inventory와
경로·크기·SHA가 모두 일치했다. warm compiler의 실제 package·RSS·swap 관측을 남겼다.
고정 soft heap과 hard RSS는 서로 다른 조건이며 보호 종료를 compiler 진단 오류로
바꾸지 않는다. 공유 페이지 중복 가능성이 있는 session RSS이지 cgroup 실사용량은 아니다.

Graph 최신 전체 생성의 managed 파일은 187,686개다. 직전 출력과 비교한 변경 88,863개 중
119개는 bounded contract와 facade 구조 변경, 88,744개는 정확한 generation fingerprint
치환뿐이며 그 외 변경은 0개였다. 관련 manifest·source ownership·18개 membership
집합 대조 결과도 보존했다. 이는 전체 compile이나 HTTP 성공 근거가 아니다.

`measurements/2026-10-22/full-stress-limits.tar.gz`는 204개 entry, 66,129,639 B다.
SHA-256은 `59395966618b312acfe8f3b8334edf22f77b2462bb7675e888d79f0240180a9e`이며
index의 모든 자료를 archive에서 다시 읽어 크기·SHA를 검산했다. 종료된 원시 trial,
project별 결과·config·ownership, 전후 manifest, 실패 분류와 실제 producer를 포함한다.
전체 기계 생성 SDK·임시 선언·build-info·실행 파일은 중복 포함하지 않았다. archive만으로
모든 과거 compiler 입력 바이트가 제공된다고 주장하지 않는다. 출력 비교 replay helper는
원래 inline 비교 이후 추가한 재현 절차임을 index에 명시했다.

선택 SDK의 실제 통과 범위는 `selected-sdk-verification.md`, Go 37개 전체 결과의 원본과
정확한 CI 입력 보정은 `go-corpus-attempt-1.tar.gz`를 따른다. 전체 대형 실패는 그 성공
집계에 포함하지 않는다.
