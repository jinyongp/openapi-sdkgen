# Lowered validator 추가 공유 보류

W07 결정: 이번 변경에서는 추가 validator 공유 V를 적용하지 않는다. 기존 view 공유와
Program·path·media·recursion·evaluation·cancellation 동작을 유지한다. Go 파일 묶기 P의
검증과 전체 Graph compile은 별도 필수 조건이며, 이 보류로 대체하지 않는다.

## 실제 기회와 비용

동결한 원래 Graph의 최종 `internal/schemaimpl` 189,358파일에서 lowered check 함수
382,814개를 조사했다. 최종 package·정확한 imports·서명·body를 같게 두고 선언 이름만
제외했을 때 중복은 0개다. 모든 함수에 Program identity guard가 있다. 서로 다른
Program ID를 삭제하고 같은 body로 취급하는 단순 공유는 수용할 수 없다.

visit.Program만 runtime 매개변수로 보존하는 별도 기회 계량은 10,019묶음,
138,736,336 반복 body 바이트를 보였다. 원래 전체 출력 4,045,045,462바이트의 약
3.43%이며 wrapper·새 인자·helper·호출과 배치 보존 비용을 빼기 전의 상한이다.
실제로 RSS 보호 종료가 관측된 `schemaimpl/p10`의 해당 기회도 7,306,796바이트다.
이 수치로 compiler peak 감소나 전체 compile 성공을 예측할 수 없다.

Program ID는 recursion/no-progress 방문 identity에 사용된다. 공유하려면 호출마다
원래 binding을 보존해야 한다. 현재 partition은 byte 배치의 영향을 받으므로 공유를
기존 배치 앞에 넣으면 defined type의 PkgPath가 바뀔 수도 있다. 배치 뒤 같은 package
안에서 wrapper를 유지하는 후보는 가능하지만, 추가 hot-path 호출·export data와
debugging 비용을 실제로 비교해야 한다. 현재 exact 중복이 없는 상황에서 그 복잡도를
기본 출력에 추가할 근거가 부족하다.

## 검증과 한계

- `validator-duplicate-tool-tests-2`는 서로 다른 Program A/B와 최종 package를 합치지
  않고 exact 중복과 parameterized 기회를 구분하는 검사를 통과했다.
- `graph-validator-duplicates-schemaimpl.json`은 source manifest SHA
  `4d7053fad27a3f7837721c4f8d6fec1743674011380c8162c8ffe9d06d42b4a4`에 묶인다.
  도구는 source를 수정하지 않으며 기존 view wrapper를 lowered 중복으로 재집계하지 않는다.
- V에 해당하는 schema planner·view planner·schema template·runtime 코드는 원래 제품과
  같다. 새 helper·binding·runtime 호출이 없으므로 V의 runtime 개선이나 비열등성을
  주장하지 않는다. 기존 runtime 기준 9단계와 320 benchmark 관측은 기준 증거다.
  P 후보의 전체 consumer·runtime·Graph 검증은 W09/W11에서 별도로 확인한다.

후속 V를 다시 검토하려면 원래 partition/type identity를 고정한 구현, wrapper 비용을
포함한 실제 net bytes, 전체 compile 및 export data, Program/path/media/evaluation/cancel
parity, 최소 10 paired runtime 표본과 95% ratio 상한 1.05 이하 근거가 필요하다.
현재 합성 결과나 반복 body 상한은 그 검증을 대신하지 않는다.
