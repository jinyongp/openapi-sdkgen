# Go operation의 반복 HTTP 처리문 절감

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

작은/중간 실제 SDK에서 P와 E의 생성·compile·runtime를 구분해 측정하고, HTTP runtime는
기존 정책의 paired 10쌍·95% ratio 상한 1.05 및 allocation/GC 검토를 적용한다. 실제 native
Graph의 AST와 frontend 비용을 다시 측정하며 전체 compile 성공이 없으면 채택 증거가
완성되지 않는다. 최종 38문서·Graph 5표본·binary/archive·통합 검사는 W09/W11에 남긴다.
단위 리뷰·커밋 후 현재 basis에 검증을 기록한다. 파일 묶기 P의 이득과 혼합해 주장하지 않는다.
