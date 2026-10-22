# Go operation의 반복 HTTP 처리문 절감

현재 최종 수용 범위는 `scope-decision-2026-10-22.md`를 따른다. 사용자가 전체 Graph를
참고 검증으로 전환했으므로 아래 전체 Graph compile·반복 계획은 실험 이력이다.
기존 AST·계약·HTTP 10쌍 근거는 유지하고 일반 제공자·Graph 선택 SDK와 최종 통합을 검증한다.

최종 상태: E를 기본 구현으로 유지한다. 실제 AST/계약·HTTP10쌍·corpus37개전체와
Cloudflare/Graph 선택 SDK·최종 제품 SSOT 검증을 통과했다. `final-go-verification.md`에
채택 근거와 정적 export/binary 비용을 기록했다. 전체 Graph compile 성공은 주장하지 않는다.
아래 진단과 당시 미완료 gate는 실험 이력이다.

현재 Graph의 다음 병목은 파일 개수보다 `internal/operations` 한 package의 frontend 크기다.
전체 schema/model implementation 빌드는 통과했으나 전체 빌드는 해당 package에서
RSS 12,225,490,944바이트로 보호 종료됐다. 별도 SIGQUIT 진단은 `types2.recordUse`와
선언·receiver 타입 검사에 있었으며 SSA backend 이전이었다. backend의 `-c`를 낮추는
방법을 해결책으로 채택하지 않는다.

실제 source와 publisher hash를 대조한 AST 감사 결과는 다음과 같다.

| 항목 | 실제 값 |
| --- | ---: |
| Go 파일 | 45,696 |
| source bytes | 597,978,236 |
| AST 노드 | 78,054,414 |
| identifier 노드 | 37,526,117 |
| 메서드 / 본문 bytes | 473,577 / 311,977,431 |
| private free function / 본문 bytes | 87,204 / 77,194,732 |
| signature에 local type 이름이 없는 private 함수 본문 bytes | 974,425 |

마지막 분류는 body 이동 가능성의 증명이 아니다. 단순 함수 이동만으로 큰 절감을
기대할 근거도 부족하다. 기존 정의 type의 package와 공개 method/alias/tag는 유지한다.
새 implementation package나 codec·validator 공유, 이름 축약은 이 단위에 넣지 않는다.

먼저 두 가지 반복문을 기존 core 동작의 순서 있는 반복으로 표현하는 후보 E를 검증한다.

- 여러 매개변수의 `Get`·`AddParameter`·오류 반환을 작은 `ParameterBinding` 묶음으로
  전달한다. 기존 `Parameter` 계약을 그대로 재사용한다. header 이름 등록은 값 존재 검사
  전이며, 앞선 값만 보이는 custom encoder의 request snapshot과 첫 오류 중단을 유지한다.
- 여러 응답의 `ResponseScore` 선택은 `ResponseChoice`와 하나의 `ResponseSelection`
  상태로 처리한다. 원래 branch index·순서·동점의 첫 선택·kind·binary fallback을 유지한다.
  decode/validate, error 생성, cancellation, body/stream 정리 코드는 바꾸지 않는다.

묶음은 16개 이하의 slice view로 나누어 큰 operation에서도 한 번의 descriptor 임시 배열이
입력 크기에 따라 커지지 않게 한다. 하나 이하의 항목에는 기존 직접 표현을 유지한다.
새 public generator 옵션은 없다. 기본 적용은 아래 실제 검증 뒤에 결정한다.

검증은 누락된 값·header 등록 순서·상태 변경 뒤 encoder metadata·첫 오류·취소,
batch 경계의 branch index와 동점·wildcard/status/media 선택, 기존 HTTP/media/stream
consumer를 포함한다. generator의 기존 operation 계약과 추가 core 내부 helper를 구분해
모든 기존 Go 정의 package·서명·method set·alias·JSON tag를 대조한다. 새 내부 선언은
정확한 이름·서명의 allowlist로만 확인하며 기존 계약 차이를 무시하지 않는다.
Graph import path에 고정한 목록은 `go-operation-additions.json`이다. 3개 정의 type와
그 공개 struct 선언, 기존 `Execution`의 새 method 및 `ResponseSelection`의 method를
합쳐 정확히 8개 계약만 추가 허용한다. 선언은 구현 source에서 검토한 것이며 실제
Graph 비교의 차이를 보고 목록을 넓히지 않는다.

작은/중간 실제 SDK에서 P와 E의 생성·compile·runtime를 구분해 측정하고, HTTP runtime는
기존 정책의 paired 10쌍·95% ratio 상한 1.05 및 allocation/GC 검토를 적용한다. 실제 native
Graph의 AST와 frontend 비용을 다시 측정하며 전체 compile 성공이 없으면 채택 증거가
완성되지 않는다. 최종 38문서·Graph 5표본·binary/archive·통합 검사는 W09/W11에 남긴다.
단위 리뷰·커밋 후 현재 basis에 검증을 기록한다. 파일 묶기 P의 이득과 혼합해 주장하지 않는다.

실제 E Graph는 모든 기존 6,434,421개 계약이 같고 새 8개만 추가됐다. 같은 operation
package의 source는 579,570,728바이트, AST/identifier는 75,204,993/35,781,439개다.
각각 기준보다 3.08%/3.65%/4.65% 감소했으며 package와 정의 type 개수는 같다.
이는 실제 생성물에서 직접 계산한 값이다. compiler 시간·메모리나 HTTP 개선율은 별도
실측 없이 주장하지 않는다. 최초 생성과 cold compile은 작은 후속 검증과 겹친 pilot으로
보존하며, 최종 성능 반복 수에는 넣지 않는다.

실제 두 GitHub 호출의 paired ratio 95% 상한은 1.0282/1.0209로 1.05 이하고,
B/op·allocs/op는 양쪽이 같다. 두 로컬 transport 호출의 증거이며 속도 향상이나 모든
HTTP 경로의 증거로 확대하지 않는다. 첫 전체 cold는 3,600초 timeout, 후속 intrusive
warm은 operations에서 RSS 보호로 끝났고 profile 두 개는 0바이트다. 별도 stack/GC
진단 결과와 최종 전체 compile·반복 검사가 남는다. 원시 결과는 `execution.md`를 따른다.

후속 stack은 선언·서명 검사, 늦은 stack은 함수 본문 검사였으며 모두 frontend였다.
soft 10 GiB의 별도 900초 warm 진단에서 865초에 늦은 stack을 요청했고 recorder는
900.33초에 종료했다. peak RSS는 12,278,591,488바이트, GC 원문의 유지 heap은
13,738 MB까지 올라갔다. agent signal과 실제 timeout을 함께 보존하며, 중단하지 않은
전체 compile 결과나 더 큰 heap의 해결 증거로 해석하지 않는다.

V06E의 단위 범위는 구현·AST/계약·행동·HTTP 비용과 실제 compile pilot의 결과/원인
검토다. 이 범위는 완료했으며 실제 전체 성공·38문서·최종 5표본은 V09/VIntegration의
필수 gate로 유지한다. E를 포함한 최종 기본 채택과 workstream 종료는 아직 아니다.
