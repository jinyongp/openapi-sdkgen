# 대형 패키지 생성 중 살아 있는 메모리

상태: 조사·진단 도구 검증·리뷰 완료. 기준은 `5b9312d`다. 앞선 누적 할당 감소를 peak RSS 감소로
재해석하지 않는다. 실제로 동시에 살아 있는 plan·렌더링 source·AST·게시 buffer를
구분해 다음 구조 변경의 대상을 결정한다.

기존 heap profile은 게시 후 GC이므로 생성 중 보유 데이터를 보여주지 않는다.
optional observer에 artifact prefix 최초 도달 시 강제 GC 후 heap snapshot을 남기는
진단을 추가한다. 제품 runtime이나 출력 순서는 바꾸지 않는다. 고정 GitHub의 첫
model implementation 출력에서 한 번 수집하고 전체 파일 hash를 현재 제품과 대조한다.
그 관측점은 peak 순간을 보장하지 않으며 GC/profiling 시간·RSS는 성능 표본으로 쓰지 않는다.

보유 source가 주요 원인이면 RAM·CPU·임시 디스크의 교환 관계를 검토한다.
무조건 동시성을 늘리거나 GC 수치를 조정하지 않는다. lossless 메모리 저장과
소유 범위의 디스크 저장, 불필요한 원본 보유 제거를 실제 기여와 계약에 따라 비교한다.
새 구조는 source/output bytes·초기화/오류/소유권·독립성을 유지해야 한다.

진단 예산은 작은 prefix snapshot 검증과 대형 heap snapshot 1회, bounded attribution이다.
기존 전용 Go cache 하나·4 GiB, cache 외 scratch 2 GiB, GOMAXPROCS=4·GOMEMLIMIT=2GiB,
개별 180초·RSS 보호 3 GiB를 유지한다. 전체 Graph/Cloudflare·전체 compiler matrix는 제외한다.

## 관측과 결정

고정 GitHub에서 첫 `internal/modelsimpl/` artifact 직전 강제 GC 후 실제 HeapAlloc은
266,839,760 B였다. pprof의 표본 합계는 259.49 MB이고, `bindImports`가 반환해
패키지 단위로 보관 중인 formatted source가 그중 126.92 MB(48.91%)를 차지한다.
분할 metadata·참조 이름의 잔존도 확인했다. 게시 후 GC profile은 2.21 MB로,
게시 후 snapshot만으로는 이 보유 비용을 식별할 수 없었다.
pprof의 MB 표기·통계 표본과 실제 HeapAlloc은 다른 값이다. 선택 관측점은 peak를
보장하지 않으며 이 실행의 시간/RSS로 제품 개선을 주장하지 않는다.

원인은 immutable plan과 함께 대형 패키지의 렌더링 결과를 분할 단계까지 보관하는
수명이다. 다음 단위에서는 이 경계에 소유권이 명확한 lossless source 저장을 검토한다.
먼저 표준 라이브러리의 빠른 압축과 현재 보관을 비교한다. CPU·누적 할당·peak RSS·
작은 입력 비용이 나빠지는지 실제로 확인하고, 전체 source/output bytes를 유지한다.
disk spool은 추가 I/O·page cache·실패 경로가 생기므로 우선 채택하지 않는다.
같은 소스를 다시 렌더링하면 formatting/binding을 반복하므로 그 비용도 피한다.
GC 수치 조정·제공자 예외·최종 format 생략은 해결 방향에서 제외한다.

## 검증과 리뷰

- optional 진단 기본값에서는 추가 GC/profile이 없으며 제품 runtime Go 파일 605개는
  직전 빌드 manifest와 같다. 작은 기본 실행과 prefix 실행을 각각 검증했다.
- TVMaze 두 실행 각각 143개·3,172,460 B와 GitHub 6,377개·204,611,637 B의
  전체 inventory·manifest·실제 파일 hash가 직전 제품과 일치했다.
- 옵션 한쪽 누락 두 경우, 기존 profile 보호, prefix 미일치까지 네 실패 경로를
  검증했다. 미일치는 생성 후 진단 실패이며 유효한 SDK를 남길 수 있음을 README에 명시했다.
- prefix 검사·최초 1회 기록·기본 경로·입출력 소유권·실패 의미·측정 범위·보존 출처를
  순차 리뷰했다. 요구 범위에 남은 결함은 없다. 제품 코드 변경이 없어 전체 compiler
  matrix와 제품 suite 반복은 하지 않았다.
- 생성 SDK는 검산 후 제거했다. 진단 실행 파일 35,153,220 B도 profile 분석 후 제거했다.
  전용 Go cache 약 3.3 GiB, cache 외 scratch 약 371 MiB로 운영 상한 안이다.

## 재현 근거

[원시 근거 archive](measurements/13-live-package-memory.tar.gz)는 2,222,417 B,
127개 근거 항목과 hash index를 포함한다. SHA-256:
`8419b89b85a763488d0c118b7c6631c030079ec2f8f4a2ca1256423de60b1ee7`.
입력 원본·observer/producer·빌드 출처·두 heap profile·pprof 결과·옵션 검사·전체 파일
hash·정리 검증을 보존하고 모든 archive 항목을 다시 읽어 길이와 hash를 검증했다.
archive의 `snapshot-unit13.py`, `check-snapshot-unit13.py`, `archive-unit13.py`가
수집·검산 절차다. Go 1.27.2와 위 한도로 `preflight:perf`를 통해 실행했다.
실행 파일과 생성 SDK는 포함하지 않는다. 이 기록은 내부 로컬 진단이며 공개 benchmark가 아니다.
