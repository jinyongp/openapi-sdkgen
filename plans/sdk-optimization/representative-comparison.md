# 최종 버전과 기존 버전의 실제 API 비교

상태: 하네스 리뷰·예비 실행 완료, 본 측정 준비. 전체 Graph·Cloudflare는 제외한다. 기존 workstream 완료와 별개인
후속 실측이며, 이전의 부분 최적화 실험을 최종 버전의 성능으로 대체하지 않는다.

## 고정한 비교 조건

- 기존 제품은 `5ada624a69ebfd85103248379970f78a8f3d2899`, 최종 제품은
  `52eab9882d4a4df33b0131b119f37d87c254344b`다. 두 revision의 실제 `cmd`·`internal`·
  `go.mod`·`go.sum`을 archive로 추출하고 같은 도구·빌드 옵션으로 각각 빌드한다.
- TVMaze(42개 API), ListenNotes(24개 API), GitHub(1,221개 API)의 고정 실제 원본을
  사용한다. 작은 두 문서와 더 큰 문서의 사례 비교이며 전체 제공자의 통계 표본은 아니다.
  입력 경로와 SHA-256은 `compare-apis.py`에서 검증하며 선택 옵션 없이 server를 포함한다.
- Go 1.27.2, Node 24.22.0, TypeScript 5.9.3, CGO=0, CPU thread 4개,
  Go `-p=1 -trimpath -buildvcs=false`, Go soft heap 4 GiB, Node heap 5 GiB를 사용한다.
  phase별 600초·관측 session RSS 6 GiB, 전체 7,200초를 상한으로 한다.
- 각 입력·대상에서 기존/최종 순서를 바꾸며 5쌍을 직렬 실행한다. 모든 생성은 새 출력이다.
  생성기 빌드·입력 및 설정 준비는 생성/컴파일 시간에서 제외한다. 첫 표본도 보존한다.
- Go cold는 매번 빈 전용 `GOCACHE`에서 표준 라이브러리를 포함해 전체 `./api/...`를
  빌드한다. warm은 바로 그 출력·캐시에서 수정 없이 같은 명령을 다시 실행한다.
- TS cold는 build-info 없이 전체 strict source와 기존 SDK에서 동결한 공개 계약을 검사한다.
  warm은 같은 build-info를 재사용하는 무변경 증분 검사다. 둘 다 새 Node 프로세스다.
  Go의 compiler cache와 TS의 증분 검사 cache는 서로 다른 경계다.
- OS·파일시스템 cache는 비우지 않는다. warm은 수정된 package를 다시 컴파일하는
  증분 개발 시간이나 원격 HTTP 응답 시간을 측정하지 않는다.
- 시간은 fresh worker의 subprocess wall time으로 측정한다. Linux wait4 계열의
  RSS high-water는 가장 큰 개별 자식/후손 프로세스의 최대치다. 동시 전체 메모리가 아니다.
  별도로 200ms 간격의 session RSS 합을 기록하며 공유 페이지 중복과 짧은 peak 누락이 있다.
- 알려진 실패를 반복하지 않는다. 첫 실패 쌍을 보존하고 해당 입력·대상의 나머지 쌍은
  중단한다. 실패·timeout·RSS 종료를 성공 표본이나 개선율 계산에 넣지 않는다.

## 문서 반영 범위

현재 공개 문서는 소스 감소를 고정 속도 향상으로 해석하지 말라고 설명하고 있다.
캐시의 정확한 경계, 출력 파일 묶기의 변경 비용, TypeScript 공유의 효과 범위는 보완한다.
공개 숫자는 GitHub Actions 실행에 연결한다는 프로젝트 규칙을 유지한다. 이번 로컬
실측과 원시 자료는 내부 개발 기록에 보존하고 공개 benchmark처럼 게시하지 않는다.

## 리뷰와 완료 조건

1. 하네스: revision/input/tool pin, 동등한 옵션, cache 격리, 시간·RSS 경계, 실패 분모를 검토한다.
2. 실측: 양쪽 생성과 전체 컴파일 성공, 출력 manifest hash, 원시 표본과 집계를 대조한다.
   성공한 작은 입력만 전체 개선으로 확대하지 않는다. 모든 실패와 미측정 범위를 기록한다.
3. 문서: 영어·한국어 TS/Go 비용 페이지의 기존 주장을 확인하고 사용자가 선택할 조건과
   손해를 명시한다. 문서 검증·빌드를 수행한다.
4. 각 작업 단위 리뷰와 커밋 뒤 최종 통합 리뷰에서 코드·결과·문서의 일치를 확인한다.

하네스 리뷰에서는 이전의 200ms 외부 recorder 시간을 짧은 작업의 실행 시간으로 쓰지
않도록 fresh worker의 내부 시간을 추가했다. memory high-water와 session RSS 합도
별도 필드로 구분했다. 실패·비대응 쌍 제외와 실제 실패 종료 코드·메모리 측정 검사2개,
기존 recorder 검사5개가 통과했다. TVMaze 예비 실행의 양쪽 TS/Go 생성·cold/warm도
모두 통과했다. 예비 실행은 도구 검증에만 쓰며 성능 집계에 섞지 않는다. 최초 sandbox의
전역 Go cache 쓰기 실패도 별도 기록으로 남겼고 이후 정상 cache 접근으로 재실행했다.
