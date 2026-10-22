# 측정 중단과 자식 프로세스 수명

상태: 재현·구현·최종 검사·리뷰 완료. 기준은 `b97d32f`다.
제품 runtime은 바꾸지 않았다. 측정 도구의 cache 회수는 다음 단위에서 계속한다.

캐시 수명 조사에서 `compare-apis.py`/`resume-apis.py`는 사례별 cache를 보존하고,
`golang/measure.py`의 cold/warm cache 제거는 성공 경로에만 있었다. 기본 측정도
명시적 `GOCACHE`가 없으면 전역 cache를 사용한다. cache 회수 범위를 고치기 전에
측정이 중단됐을 때 owned compiler가 끝난다는 보장이 필요하다.

기준 recorder는 정상·timeout·RSS 경로에서는 owned session을 정리했지만,
recorder 자체 SIGTERM/KeyboardInterrupt에는 그 정리가 실행되지 않을 수 있었다.
작은 자식으로 먼저 재현했고, 외부 SIGINT/SIGTERM 요청을 non-passing 결과로 남긴
후 owned session 종료·대기·redactor 종료를 마치고 반환하도록 개선했다.
소유 관계 밖의 session이나 사용자 프로세스에는 신호를 보내지 않는다.

검증 범위는 정상·명령 실패·자식 signal·timeout/RSS·부모 중단·비메인 process
group의 owned 자식·기존 redaction/provenance다. Go 생성 성능 개선율은 주장하지 않는다.
추가 컴파일 cache를 만들지 않고 전용 cache 4 GiB·scratch 2 GiB 상한을 유지한다.
SIGKILL/호스트 종료처럼 처리할 수 없는 중단은 cleanup 보장을 주장하지 않는다.
이 단위를 확정한 뒤 측정 도구의 cache 회수와 전역 cache 격리를 계속 처리한다.

## 원인과 최종 변경

기존 recorder에 SIGINT/SIGTERM을 보낸 두 재현 모두 owned 자식이 계속 살아 있어
실패했다. 기준 source와 재현을 `baseline-source.tar.gz`에 보존했다. 첫 수정에서
부모 중단은 처리했지만 중첩 trial이 별도 session을 만들면 그 worker는 남았다.
PID/group별 네 재현에서 모두 확인했고 중간 source도 별도로 보존했다.

최종 변경은 SIGINT/SIGTERM 요청을 상태로 받아 종료 절차를 마친다. 최초 요청을
`requestedSignal`로 기록하고 `interrupted` 결과와 `128 + signal` 상태로 반환한다.
observer 예외를 포함해 `finally`에서 owned command를 종료·대기한 다음 redactor의
입력을 닫고 출력을 회수한다. redactor는 foreground signal로 먼저 죽지 않도록
별도 session에서 실행한다.

Linux process snapshot에서 command session과 자손을 구분한다. 관측한 자식 session은
중간 controller가 종료돼도 추적하고, 종료 직전에 ancestry를 다시 확인한다.
PID별 시작 시각을 대조해 관측한 프로세스를 종료한다. 이 추적은 회수용이며 RSS는
종전대로 원래 command session의 합이다. 중첩 session RSS를 합친 새 지표로
해석하지 않는다. 관측 전에 분리/고아화된 daemon이나 SIGKILL/host 종료까지
회수한다고 주장하지 않는다.

최종 리뷰에서는 snapshot 직후 새 session worker가 생기고 controller가 먼저
종료되면 ancestry가 끊기는 경합도 독립적으로 재현했다. snapshot을 얻은 뒤 실제로
worker를 생성하고 종료 함수의 첫 조회에 이전 snapshot을 반환해 OS의 해당
interleaving을 강제했다. 최종 종료는 controller를 먼저 멈추고 소유 관계를 다시
조회한 뒤 kill/대기하여 새 worker도 회수한다. PID 이름이나 오류 문자열의 예외로
처리하지 않았다. 경합 전후 source와 재현 실패도 보존했다.

## 검증과 리뷰

| 영역 | 완료 근거 |
| --- | --- |
| 기존 실패 재현 | 원래 recorder에서 PID 대상 SIGINT/SIGTERM 두 경우 모두 살아남은 자식을 확인했다. 테스트가 만든 PID만 정리해 재현 자체의 누수를 막았다. |
| 중첩 실패 재현 | session만 정리한 중간 후보에서는 새 session 자식이 네 취소 경우 모두 남았다. 중첩 recorder와 같은 경계를 검증에 추가했다. |
| 실제 OS 경계 | 최종 검사에서 PID/foreground group 각각 SIGINT/SIGTERM을 보냈다. 별도 group 자식과 중첩 session 자식은 종료하고, ancestry 밖의 unrelated session은 유지했다. |
| 고아 자식 | controller가 사라진 관측 상태에서도 worker와 그 후손의 소유권을 유지하고, 종료 후 상태를 비우는 별도 검사를 통과했다. |
| 결과·가림 | 중단 결과/요청 신호/비정상 종료를 확인했다. stdout이 가려진 상태로 끝까지 저장됐고 canary 원문은 남지 않았다. 기존 pass·fail·child signal·timeout/RSS·분리 group·실행 파일 식별 검사도 통과했다. |
| snapshot 경합 | snapshot 직후 실제 worker를 새 session에 생성하는 재현이 중간 후보에서 실패하고 최종 controller freeze 경계에서 통과했다. 테스트 소유 PID는 실패 시에도 회수했다. |
| 호출자·출처 | recorder 최종 8개 검사, API timing/짝짓기 2개 검사, Go 계약 비교 3개 검사 통과. 하이픈 파일을 discovery가 생략하므로 두 비교기 파일은 직접 실행했다. |
| 이전 자료 | worker/recorder hash guard는 유지한다. 과거 중단 자료를 회복할 때는 그 자료에 동결된 버전을 사용하도록 문서화했다. 기존 원시 기록은 바꾸지 않았다. |

제품 runtime과 생성 SDK는 변하지 않았으므로 Go/TypeScript 전체 생성이나 compile
성능 비교를 다시 실행하지 않았다. 재현 실패 시간과 최종 검사 시간을 속도 개선으로
비교하지 않는다. 전용 Go cache 약 3.0 GiB와 cache 외 scratch 약 290 MiB를 유지했다.
사례별 cache 보존/실패 시 미정리/기본 전역 cache 사용은 이 단위에서 해결했다고
표현하지 않고 다음 단위의 명시적 범위로 남긴다.

[원시 근거](measurements/07-trial-cancellation.tar.gz)는 1,516,747 B,
근거 항목 82개와 색인 1개다. SHA-256:
`d908ad3742b83d477865c1e4e3350424f76264b7f684f17763d6d8cad4b5bd1f`.
모든 항목의 크기·hash를 다시 확인했다. 원래/중간 source·세 실패 재현·최종 검사·
중간 리뷰 근거와 관련 producer를 보존했다. 실행 파일·Go cache는 포함하지 않는다.
검사 후 전용 cache 약 3.0 GiB, cache 외 scratch 약 294 MiB이며 Go 임시 경로는 비어 있다.
