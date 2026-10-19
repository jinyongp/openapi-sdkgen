# W05 실행기 구현 기준

W04의 `Program`은 평평한 semantic instruction/edge/range 자료다. 캡처 번호·이름 참조 목록,
반복의 자식 캡처 구간, scope별 flags가 이미 결정돼 있다. matcher는 parser나 JS VM을 호출하지 않는다.

- 입력 cursor는 byte offset, 읽기는 Unicode code point다. forward/backward helper가 UTF8과 WTF8의
  고립 surrogate 및 인접 surrogate pair를 동일 의미로 읽는다. 기본 ASCII 경로에서 입력 전체를
  []rune로 복사하지 않는다. capture도 byte span으로 저장하며 참조는 code point별로 비교한다.
- 명시적 task continuation, 대안 choice, capture undo journal을 쓴다. 호출별 상태이고 Program은 불변이다.
  최대32개의 고정 크기 scratch를 독점 대여하고 커질 때만 확장한다. 반환 시 입력·프로그램·context 참조와 확장 buffer를 해제한다. W10에서는 전역 channel 경합을 실측해 고정 arena의 atomic lease와 sync.Pool hint로 바꿨다.
  GC가 hint를 버려도 arena에서 회수하며 중복 hint는 lease CAS로 거른다. 코드 노드가 공유돼도 실행 상태는 공유하지 않는다.
- continuation은 arena의 immutable linked task다. choice가 parent head·arena/journal checkpoint·
  cursor/direction·재개 action을 저장한다. 재시도 때 checkpoint를 복구한다. 현재 head와 살아 있는
  choice checkpoint보다 뒤의 arena는 회수해 반복마다 task가 영구 누적되지 않게 한다.
- concat은 lookbehind에서 역순으로 평가하고 alternative 선택 순서는 원래 순서다.
  capture 시작/종료 span은 방향을 반영한다. 미참여/전방/self 참조는 빈 문자열이다.
  이름 중복은 compiler가 배타성을 확인했으므로 참여한 group의 span을 선택한다.
- lookaround는 atomic이다. assertion 실패를 처리할 barrier choice와 성공 완료 task를 두고,
  성공 시 내부 choice를 버리며 cursor/direction을 복원한다. positive의 캡처는 유지하고 negative는
  캡처를 복원한다. native recursion으로 nested assertion을 평가하지 않는다.
- repeat는 매 회 자식 캡처 구간을 초기화한다. greedy/lazy 및 최소/최대, zero-progress 종료를
  continuation으로 표현한다. 최소 횟수의 빈 반복을 임의로 생략하지 않는다.
  예: `(|a){2}\1$`와 입력 `aa`는 마지막 반복의 대안으로 backtrack할 수 있어야 한다.
- 캡처 없는 단일 문자/class/dot repeat에는 compact retry action을 사용해 1MiB `a+`에
  문자당 choice를 보존하지 않는다. literal noncapture 단일-child wrapper는 투명하게 확인할 수 있다.
  복합/capture repeat를 같은 최적화로 바꾸지 않는다.
- 하나의 Match가 검색 시작 위치·assertion·역참조 비교를 포함한 100,000,000 작업 budget을 공유한다.
  1,024작업마다 context를 확인한다. 오류를 boolean 불일치로 바꾸지 않는다.
- CharacterClass range는 compiler가 `u` 의미에 따라 folding/negation을 준비했다.
  IgnoreCase이면 input rune을 unicode17.Fold한 뒤 membership을 검사한다.
  `\W`는 folded word set의 complement이고 `\P`의 u folding 순서와 구분한다.
- matcher 검증은 기존42개·독립 경계/캡처/취소/complexity·고정 Test262의 boolean 관련 사례·
  Node differential/fuzz·동시 공유 Program·ASCII/Unicode/큰 입력·메모리 수명으로 한다.
  JS 객체 API·다른 flags의 제외 사유를 유지한다. 정적 추출과 보충 동적 trace의 전체 파일
  결과·불완전 실행·준비 크기 제한은 별도 manifest로 구분하며 전체 JS API 통과로 주장하지 않는다.
