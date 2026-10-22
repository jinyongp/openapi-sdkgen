# 측정 캐시 소유권과 회수

상태: 재현·구현·최종 검사·리뷰 완료. 기준은 `fe41cba`다. 제품 generator runtime은 바꾸지 않았다.

API 비교/회복 도구는 cold/warm 사례마다 전용 cache를 만들고 그대로 남겼다.
일반 Go 측정은 성공한 쌍만 cache를 지우며, 기본 generator/runtime/platform 빌드는
명시적 `GOCACHE`가 없으면 전역 cache에 쌓였다. 기록과 재생성 가능한 cache의 수명이
묶여 실행/사례 수에 따라 디스크 사용이 증가하는 것이 원인이다.

기본 정책은 실행 범위의 임시 Go cache, cold/warm 한 쌍 범위의 별도 임시 cache다.
caller가 지정한 `GOCACHE`는 빌려 쓰고 삭제하지 않는다. cold는 비어 있는 전용
cache로 시작하고 warm은 같은 cache를 사용한다. 다음 cold는 새 cache다.
소유한 프로세스가 종료된 뒤 소유한 cache를 회수한다. 성공·실패·timeout·협조적
SIGINT/SIGTERM에서 같은 소유권 경계를 유지한다. SIGKILL/host 종료처럼 cleanup을
실행할 수 없는 경우까지 제거를 보장하지 않는다.

`golang/measure.py`, `compare-apis.py`, `resume-apis.py`가 같은 private resource
도우미를 사용한다. 새 helper도 provenance에 포함해 회복 시 hash guard를 유지한다.
이전 기록의 worker/recorder/resource policy를 바꾸거나 현재 정책으로 재해석하지 않는다.
SDK·로그·결과·input/binary hash 보존은 유지한다. 이 단위는 캐시 수명 개선이며
전체 SDK tree 보존 정책이나 생성 속도 개선율로 확대하지 않는다.

검증은 실패 시 cache 잔존 재현, cold/warm 격리·상속, caller cache 보호,
중단 시 writer 종료 선행, 세 호출자 확인과 작은 실제 Go cold/warm 빌드다.
전용 기존 cache 4 GiB·scratch 2 GiB, RSS 3 GiB·개별 180초 상한 안에서 진행한다.
전체 Graph/Cloudflare나 대표 API 전체 compile matrix는 다시 실행하지 않는다.

## 검증과 리뷰

기준 버전의 cold 실패와 warm 실패에서 각각 전용 cache가 남는 것을 재현했다.
최초 재현에는 fixture가 stripped 실행 파일을 만들지 않는 별도 오류도 있었으므로,
이를 고친 두 번째 재현을 채택 근거로 사용한다. 두 번의 원시 기록과 기준 source,
두 번째 재현에 사용한 정확한 테스트 파일을 함께 보존했다.

| 영역 | 완료 근거 |
| --- | --- |
| 소유권 | 미지정·빈 `GOCACHE`는 임시 소유 cache, 명시적 경로·`off`는 caller 소유로 처리한다. caller sentinel이 유지되고 임시 cache는 예외에서도 제거됐다. |
| 실행·쌍 경계 | 실제 Go 측정 controller에서 5쌍의 cold는 빈 cache, warm은 직전 cold의 cache를 사용했다. 성공·cold 실패·warm 실패 후 모두 회수됐다. 기본 실행은 run cache와 현재 pair cache 최대 2개, caller cache 사용 시 별도 owned pair 최대 1개다. 이는 동시 디렉터리 수이며 cache 한 개의 byte 상한은 아니다. |
| 중단·timeout | 실제 cache writer와 별도 session writer에 SIGINT/SIGTERM을 보내 종료 후 cache 제거 순서를 확인했다. GNU time 경로의 timeout은 비통과 결과를 기록하고 writer가 끝난 뒤 cache를 회수했다. |
| API 비교 | 작은 compiler fixture로 비교 controller의 두 variant·두 pair·TS/Go 분기를 실행했다. cold/warm 상속, 다음 pair 격리, 실패 시 회수, caller cache 보존을 확인했다. |
| 회복 | resource policy hash 불일치는 실행 전에 거절한다. 완료된 기록은 재실행하지 않고 missing Go case만 실행하며 성공·cold 실패·warm 실패 모두 cache를 회수했다. |
| 최종 검사 | shared resources 3개, Go 측정 5개, API 비교 3개, 회복 3개, recorder 8개, Go 계약 비교 3개로 총 25개를 통과했다. 테스트 source와 로그 hash를 검산했다. |

실제 고정 TVMaze 원본과 현재 `b97d32f` 제품 binary로 생성·cold/warm Go 빌드를
직렬 실행했다. 생성된 143개 managed 파일은 06 단위의 전체 파일 hash와 같았고
빌드 후에도 유지됐다. 별도 빈 cache에서 cold 후 139,280,672 B,
warm 후 139,333,177 B를 관측했으며 쌍 종료 후 디렉터리가 사라졌다.
Go 1.27.2, CGO off, `-mod=readonly -p=1 -trimpath -buildvcs=false`, 네트워크 의존성
조회 없이 cold 11.787초·warm 0.081초였다. 이 시간은 cache 정책 smoke의 원시 결과이며
전후 제품 개선율로 해석하지 않는다. compiler fixture의 수치도 성능 근거로 쓰지 않는다.

실제 smoke와 최종 검사는 같은 resource policy SHA-256
`aeb3b7a028f86166db9dc4bb732e7628062dcf303021cd83057401fc3d4c18be`를 사용했다.
초기 실제 smoke 뒤 발견한 빈 `GOCACHE` 처리 차이는 공통 소유권 경계에서 수정했고,
최종 실제 smoke를 새 경로에서 다시 실행했다. 중간 source와 결과는 별도로 보존한다.

API 비교의 `measure_child` AST는 기준 버전과 동일하다. 비교 시간·최대 process RSS의
측정 경계를 바꾸지 않았다. 일반 Go 측정의 GNU time은 command 범위이며 driver의
`wallSeconds`는 process finalization도 포함한다. cache 회수는 cold/warm command의
측정 밖이다. lifecycle 비용을 제품 속도 차이로 계산하지 않는다.

전용 기존 Go cache 약 3.0 GiB, cache 외 scratch 약 299 MiB, Go 임시 경로 비어 있음을
확인했다. 실제 smoke의 139 MB cache와 SDK tree는 검산 후 제거했다. SDK·실행 파일·
로그 보존 정책 자체나 직접 실행하는 `micro`의 기본 Go cache 동작은 이 단위에서
변경하지 않았다. SIGKILL·호스트 종료의 회수 보장도 남은 한계다.

[원시 근거](measurements/08-measurement-caches.tar.gz)는 902,845 B,
근거 항목 130개와 색인 1개다. SHA-256:
`1f3bc9b9ddbbe76128fe6794e923a59a1bc40f80b888edfe7b148cd0728a60de`.
모든 항목의 크기·hash를 다시 확인했다. 기준/중간 source, 실패 재현, 최종 검사,
실제 입력·파일 inventory·compiler 기록과 producer를 포함한다. Go cache·생성 SDK
tree·실행 파일은 포함하지 않는다. 최종 리뷰에서 세 호출자의 자원 소유권,
실패/취소 순서, 기존 완료 기록·측정 경계 보존과 문서 범위를 확인했다.
