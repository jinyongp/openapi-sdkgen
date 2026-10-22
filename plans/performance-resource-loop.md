# 지속 성능·자원 개선

사용자 요청: 중단 요청 전까지 성능과 자원 사용을 함께 개선한다. 구조 변경이
필요하면 원인을 확인하고 결정한다. 임시 조건이나 제공자별 예외로 수치를 맞추지 않는다.

상태: 2026-10-23 사용자 요청으로 종료. 마지막 압축 후보는 기각하고 검증된
`5b9312d` 제품 runtime을 유지한다. 추가 조사/실험은 진행하지 않는다.
[작업 전후 비교와 한계](performance-resource-loop/final-comparison.md)에 최종 결과를 정리했다.
근거 archive 15개를 보존·검산한 뒤 전용 cache/임시 파일 약 3.78 GiB도 정리했다.

## 유지할 계약

- 시작점: `6231d70af9d162ac7fd50099c028ed9860f36c0c` (`v1.4.0`).
- 기존 공개 API·진단·출력 소유권·결정성·실행 동작을 유지한다.
- 전체 Graph·Cloudflare는 필수 검증에 다시 넣지 않는다. 고정 TVMaze,
  ListenNotes, GitHub 원본과 관련 경계 사례로 검증한다.
- 공개 수치에는 GitHub Actions 근거가 필요하다. 로컬 실측은 내부 기록으로 남긴다.
- 작업 단위마다 병목 근거, 설계 결정, 정확성 검사, 동일 조건 전후 표본,
  리뷰 결과를 남기고 커밋한다. 게시·재배포는 이 작업 범위에 포함하지 않는다.

## 자원 운영

- `.tmp/perf-resource-loop/`에 전용 `GOCACHE` 하나와 `GOTMPDIR` 하나를 둔다.
  기존 전역 캐시를 쓰거나 다른 작업의 캐시를 정리하지 않는다.
- 캐시 4 GiB, 생성 출력·임시 파일 2 GiB를 운영 상한으로 삼는다.
  실행 전후 확인하고 넘으면 다음 실행 전 이 작업의 재생성 가능한 파일만 정리한다.
  실행 중인 Go 캐시는 삭제하지 않는다. 비교마다 cold 캐시를 복제하지 않는다.
- 완료한 실험의 실행 파일은 배포 기준 CLI·현재 제품·직전 제품만 유지한다.
  원시 측정/입력/빌드 출처를 보존한 뒤 그 밖의 task-owned 중간 실행 파일은 정리한다.
- 생성·프로파일은 직렬로 실행한다. 기본 `GOMAXPROCS=4`, `GOMEMLIMIT=2GiB`,
  `GOFLAGS=-p=2`, session RSS 보호 3 GiB와 개별 180초를 사용한다.
  다른 제한이 필요한 검사는 근거와 새 상한을 먼저 기록한다.
- 전체 제공자/전체 컴파일 매트릭스보다 관련 패키지와 대표 실제 API를 먼저 확인한다.
  생성 출력은 manifest와 필요한 원시 결과를 보존한 뒤 재사용하거나 제거한다.

## 첫 조사

직전 비교에서 GitHub Go 전체 생성은 약 56초, TypeScript는 약 4.7초였다.
Go 생성 시간·peak RSS의 일관된 개선은 입증되지 않았다. 먼저 실제 Go 생성의
compile/prepare/emit/publish 비용과 CPU 분포를 확인한다. 특히 변환 단계 간
중복 파싱·포맷팅·출력 보유 여부를 조사하되, 프로파일 확인 전 원인으로 단정하지 않는다.

조사 예산: 첫 병목 프로파일 1회와 필요한 보조 실험, 후보별 최대 5쌍.
측정이 불명확하거나 손해가 더 크면 후보를 채택하지 않고 다른 원인을 조사한다.

## 작업 기록

- [01: Go 분할 출력의 중복 포맷 제거](performance-resource-loop/01-partition-format.md):
  채택. 실제 GitHub Go 생성 중앙값 13.83%, CPU 13.81% 감소.
  세 실제 API 전체 출력 동일. peak RSS 절감은 입증하지 못했다.
  관련 검사·실측 검산·최종 리뷰 완료.
- [02: 할당과 중복 파싱](performance-resource-loop/02-partition-references.md):
  채택. 참조 분석용 두 번째 파싱을 제거했다. 같은 관측기 3쌍에서 GitHub 누적
  할당 6.44%, 시간 2.99% 감소. RSS 중앙값 감소는 관측했으나 표본·범위의 한계를 유지한다.
  세 실제 API 전체 출력과 관련 검사·리뷰를 통과했다. 전용 cache 약 2.1 GiB.
- [01+02 통합 확인](performance-resource-loop/integration-01-02.md):
  최종 실제 CLI의 세 입력 전체 파일 hash를 원래 배포 버전과 직접 대조해 일치했다.
- [03: AST 노드 판별과 복사](performance-resource-loop/03-ast-node-boxing.md):
  채택. 파서 AST의 입력 계약을 명확히 하고 비노드 필드 박싱을 제거했다.
  실제 GitHub 누적 할당 5.12% 감소. 생성 시간의 일관된 이득은 입증하지 못했다.
  세 실제 API 출력과 관련 검사·표준 AST oracle·리뷰 완료. cache 약 2.2 GiB.
- [04: facade 원본 소유권](performance-resource-loop/04-facade-source.md):
  채택. 잘못된 위치 집합으로 중간 출력하던 경로를 없애고 공개 문서 손실을 수정했다.
  직전 제품 대비 GitHub 시간 중앙값 10.66%·누적 할당 9.54% 감소. RSS 0.75%·
  출력 0.69% 증가는 비용으로 기록한다. 세 실제 API의 전체 코드·초기화 순서,
  최종 CLI·관련 소비자/게시 검사와 리뷰를 통과했다. cache 약 2.3 GiB.
- [05: 최신 병목 재확인](performance-resource-loop/05-post-facade-profile.md):
  완료. 최종 정규화 제거 후보는 재작성 AST의 위치로 인해 비정규 출력이 나와 기각했다.
  제품 runtime은 유지하고 작은 재현과 정규 서명 회귀 검사를 추가했다.
  최신 profile에서 확인한 template 반복 파싱을 다음 후보로 조사한다.
- [06: emission 템플릿 파싱 공유](performance-resource-loop/06-emission-templates.md):
  채택. emission마다 source별로 한 번 파싱한다. 직전 제품 대비 GitHub observer 시간
  중앙값 4.00%·누적 할당 6.55% 감소, RSS 1.32% 증가는 비용으로 기록한다.
  세 실제 API 전체 출력·최종 CLI·독립성/오류/소비자/race 검사와 리뷰를 통과했다.
  Go cache 약 3.0 GiB, 중간 실행 파일 약 608 MB 제거 후 scratch 약 290 MiB.
- [07: 측정 중단 수명](performance-resource-loop/07-trial-cancellation.md):
  완료. 부모/foreground 취소에서 중첩 trial session과 관측된 고아 자식까지 회수한다.
  controller 종료 전 snapshot 경합도 회귀 검사로 막았다. 중단 결과·가린 로그를
  보존하고 원래 RSS 지표 범위를 유지한다. 13개 최종 검사 통과.
  사례별 cache 수명과 기본 전역 cache 격리는 다음 단위로 이어간다.
- [08: 측정 cache 수명](performance-resource-loop/08-measurement-caches.md):
  완료. 기본 run cache와 cold/warm pair cache의 소유권·수명을 분리했다.
  실패·timeout·협조적 취소에서도 writer 종료 후 cache를 회수하며 caller cache는 유지한다.
  25개 검사와 실제 TVMaze cold/warm 빌드, 약 139 MB 임시 cache 제거를 확인했다.
  전용 기존 cache 약 3.0 GiB를 유지한다. 제품 속도 개선율로 해석하지 않는다.
- [09: Facade AST의 부분 복사](performance-resource-loop/09-current-profile.md):
  채택. 수정하는 공개 표면만 복사해 같은 FileSet으로 먼저 출력하고, 버릴 구현 body의
  재파싱을 제거했다. 직전 제품 대비 GitHub 누적 할당 중앙값 5.15% 감소.
  생성 시간·peak RSS의 일관된 이득은 입증하지 못했다. 세 실제 API 전체 출력·
  소비자/소유권/race·실제 CLI·리뷰를 통과했다. 전용 cache 약 3.1 GiB를 유지하고
  오래된 실행 파일 약 74 MB를 제거했다. raw-source 정규화 생략 후보는 기각했다.
- [10: 중간 AST 출력의 열 정렬](performance-resource-loop/10-intermediate-alignment.md):
  완료·후보 기각. 전체 출력은 같았지만 할당 −0.45%에 비해 생성 시간/CPU 이득을
  입증하지 못해 원래 runtime을 유지했다. stdlib oracle 회귀 검사와 전체 비교의
  원시 근거를 보존했다. 후보 실행 파일 약 74 MB를 제거하고 cache 약 3.2 GiB를
  유지한다. 다음은 AST 순회의 반복 타입 분석과 교체 cursor 구조를 조사한다.
- [11: AST 자식 필드 구조의 분리](performance-resource-loop/11-ast-child-fields.md):
  채택. partition별 walker가 타입별 child field 구조를 재사용하고 scalar 재귀를 제거한다.
  GitHub 생성 시간 중앙값 2.06%·CPU 1.82% 감소, 최대 RSS 1.06% 증가는 비용으로 기록한다.
  세 실제 API 전체 출력·metadata/교체/소비자/race·실제 CLI 검증과 리뷰를 통과했다.
  Go cache 약 3.2 GiB를 유지하고 오래된/benchmark 실행 파일 약 156 MB를 제거했다.
- [12: 재작성 임시 저장소의 수명](performance-resource-loop/12-identifier-rewrites.md):
  채택. binding 통합 후보는 주석 이동으로 기각하고 기존 형식을 유지했다.
  emission 소유·1 MiB 보관 상한의 직렬화 scratch로 GitHub 누적 할당 4.55% 감소.
  시간 중앙값 −0.27%·RSS +0.65%를 구분해 기록한다. 세 실제 API 전체 출력·
  소유권/오류/소비자/race·실제 CLI 검증과 리뷰 통과. cache 약 3.3 GiB 유지,
  오래된 실행 파일 약 74 MB 제거.
- [13: 생성 중 살아 있는 패키지 메모리](performance-resource-loop/13-live-package-memory.md):
  완료. 생성 중 선택 관측점의 heap에서 렌더링 source 보유가 약 절반임을 확인했다.
  optional observer·옵션 실패 경로·작은 기본 동작·GitHub 전체 출력 동일성을 검증했다.
  제품 runtime은 유지하며 다음 단위에서 lossless 보관의 RAM/CPU 비용을 비교한다.
  진단 실행 파일 약 35 MB 제거, 전용 cache 약 3.3 GiB 유지.
- [14: 분할 대기 소스의 저장 수명](performance-resource-loop/14-partition-source-storage.md):
  완료·후보 기각. 실제 CLI 3쌍에서 RSS 중앙값 −7.62%보다 시간 +11.17%·CPU +12.58%의
  손해가 커 기준 runtime을 복구했다. 관련 31개 검사·race·실제 입력 전체 출력·
  복구/원시 근거 보존을 확인했다. 실험 실행 파일 약 184 MB 제거 후 전체 작업을 종료한다.
