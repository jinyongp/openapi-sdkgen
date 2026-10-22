# 최종 Go 검증과 채택

사용자가 승인한 범위는 corpus37개 문서 전체(실제 API28개·fixture9개), Cloudflare
토큰 확인 선택 SDK, Graph 선택5개 SDK다. 모두 Go1.27.2·server 포함 전체 compile과
필요한 HTTP 계약을 통과했다. 원래38문서 집계의 Cloudflare 전체 실패는 수정하지 않았다.
선택 Go 출력의 실제 operation/route metadata도 고정 원본과 정확히 일치한다.

최종 TS 수정은 Go 생성 경로 밖의5개 TS source/test 파일에 한정됐다. 실제 Go 생성의
269개 의존 파일은 모두 같고, 고정 auditor의 `golang-generation` 분기는 TS emitter를
호출하지 않는다. CI 입력 보정까지 대조한37개 correctness 결과만 재사용했다.
이 근거는 새 전체 generator binary의 동일성·TS parity·성능 timing을 주장하지 않는다.
Go 소비자166개 검사는 CGO=1에서 모두 통과했고, 최종 Go core와 전체 SSOT도 통과했다.

## 최종 선택

- P는 최종 package/import binding 뒤128 KiB 묶기를 유지한다. 두 실제 제공자의 원본/
  64/128/256 KiB 계약12개·cold/warm80개·정밀 warm40개를 검토했다. warm cache-hit
  명령 비용의 이득은 해당 경계에서만 인정하며 cold compile 개선은 주장하지 않는다.
  operation 하나 변경 시 파일 쓰기가 늘어나는 비용은 `pack-size-decision.md`에 보존했다.
- E는 기존 parameter·response 계약을16개 이하 metadata batch로 표현하는 구현을 유지한다.
  기존6,434,421개 계약·정의 package를 보존하고 정확한 새8개만 허용했다. 실제 Graph
  AST3.65% 감소,33항목 HTTP/race와 실제 GitHub 두 호출의10쌍 runtime을 확인했다.
  paired ratio95% 상한1.0281/1.0208은 내부1.05 기준 이하이며 B/op·allocs/op는 같았다.
  원격 지연·전체 SDK 속도 향상으로 일반화하지 않는다.
- V의 추가 validator 공유는 보류한다. exact 중복0개이며 Program identity·정의 package와
  runtime 비용을 바꾸는 추가 공유를 도입하지 않는다.

## 실제 작은 SDK의 배포·링크 비용

동일 Todo 입력/server·Go1.27.2·CGO=0·`-trimpath -buildvcs=false`의 실제 이전/최종
생성 출력을 사용했다. archive는 이름·시간·권한을 고정했다. executable은 SDK의
`NewClient`를 생성하는 프로그램만 링크하므로 모든 API 호출을 유지한 binary가 아니다.
컴파일 archive 크기는 client import에서 도달하는 SDK package들의 `.a` 합이며 표준
라이브러리는 제외했다. 원시 필드명은 `sdkExportBytes`지만 순수 export 영역만이 아니라
object code 등을 포함한 archive 전체 크기다.

| 측정, B | 이전 | 최종 |
| --- | ---: | ---: |
| managed 파일 수 | 160 | 120 |
| managed source | 1,614,749 | 1,610,058 |
| deterministic source archive | 351,761 | 350,074 |
| SDK compiler archive (`.a`) | 10,998,804 | 11,011,724 |
| constructor client executable | 8,180,822 | 8,180,214 |
| stripped executable | 5,689,504 | 5,689,504 |

파일은25% 줄었지만 compiler archive는12,920 B 증가했고 stripped binary 크기는 같다.
binary 축소나 전체 compiler 메모리 개선을 채택 근거로 주장하지 않는다. 이 측정은 단일
작은 입력의 정적 크기 비교이며 반복 timing이 아니다. 실제 HTTP·warm 비교는 별도
`representative-performance.tar.gz`의 원시 표본을 따른다.

전체 Graph·Cloudflare는 `full-stress-limits.md`의 참고 실패로 남긴다. 최종 scoped 집계,
의존성 hash·원본 compile/HTTP 연결·실제 배포 입력과 producer는
`measurements/2026-10-22/final-integration.tar.gz`에 보존했다. 단위·최종 통합 리뷰에서
새 Go 결함이나 남은 필수 Go 검증은 발견되지 않았다.

후속 요청의 기존/최종 native 전체 생성·cold/warm 비용은 `representative-comparison.md`에
별도로 기록했다. 실제 API 세 개의 각5쌍 비교이며 위 P-only 실험이나 단일 Todo 정적
크기 비교와 섞지 않는다. 전체 Graph·Cloudflare는 이 후속 측정에서도 제외했다.
