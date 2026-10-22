# 실제 API 비용 비교의 최종 통합 리뷰

범위는 기존5ada624a와 최종52eab98의 TVMaze·ListenNotes·GitHub 전체 입력을 사용한
TS/Go 생성·최대 RSS·cold/warm 비용과 관련 공개 문서다. 전체 Graph·Cloudflare는
새로 실행하지 않았다. 기존 완료 workstream의 후속 요청이며 제품 source를 바꾸지 않았다.

## 영역별 검토

| 영역 | 상태 | 근거와 처리 |
| --- | --- | --- |
| 버전·입력·도구·옵션 | 완료 | 두 revision의 실제 source archive와 추출 파일, 생성기 hash, 고정 원본3개, Go1.27.2·Node24.22.0·TS5.9.3을 검산했다. Go GOMAXPROCS=4·soft heap4 GiB, Node heap5 GiB와 server/full 범위를 유지했다 |
| 시간·메모리·cache 경계 | 완료 | 200ms recorder 시간을 짧은 작업 시간으로 쓰지 않고 fresh worker 내부 시간을 사용했다. 개별 프로세스 high-water와 session RSS 합을 구분했다. Go 빈 GOCACHE/cached no-change, TS noEmit full/incremental no-change를 다른 경계로 기록했다 |
| 표본·복구·실패 | 완료 | 180개 고유 phase 명령 모두 성공했다. driver의143 종료는 숨기지 않았으며 기존171개 표본은 snapshot과 같다. 변경하지 않은 worker로 빠진3사례만 재개했다. 중단 경계를 걸친 쌍 제외 보조 분석에서도 해석은 같고 primary5쌍은 유지했다 |
| 출력·계약 보존 | 완료 | 12개 조합 각각5회 관리 파일 hash가 같다. 세 Go 입력 모두 전체 AST의 기존 계약 차이0·사전 검토된 내부 추가8개뿐이다. 공개 TS 계약은 기존 SDK에서 동결해 전체 source와 함께 검사했다. 원격 API 호출 증거로 확대하지 않았다 |
| 수치·해석·원시 근거 | 완료 | 중앙값 변화율과 paired geometric ratio/구간을 구분했다. 긴 실행을 제외하지 않았다. 생성/cold/메모리의 전반적 개선을 주장하지 않고 작은 TS의 크기·시간 증가를 기록했다. 65,558,818 B archive의1,727 entry 크기·hash를 다시 읽어 검산했다 |
| 공개 문서·한국어·연결 기록 | 완료 | 관련 비용4·호환성4·문제 해결4페이지를 확인했다. 영문/한국어 비용4페이지의 조건·손해·예제를 보완하고 코드 블록 동일성을 확인했다. 공개 수치의 Actions 정책을 유지하고 로컬 수치는 내부 보고서로 분리했다. 기존 `.a` 전체 집계의 잘못된 export-data 명칭도 정정했다 |
| 최종 검증 | 완료 | 하네스 행동 검사2개·기존 recorder 검사5개 통과. 실제180개 측정·출력/Go 계약 검산과 복구171개 보존 검사 통과. 최종 문구의 `validate:docs`와 `build:docs` 통과. 제품 cmd/internal/go.mod/go.sum은52eab98과 diff0 |

## 결론과 남는 한계

확인된 이득은 Go 관리 파일 수74.8–80.2% 감소, Go 무변경 warm 명령 중앙값
22.3–55.4% 감소, GitHub TS full strict 검사 중앙값10.8% 감소다. 전체 생성 시간·Go
cold 시간·cold 최대 RSS가 일관되게 개선됐다는 근거는 없다. 작은 TS 출력과 일부
생성/검사 비용은 늘었다. 기존 측정에서 확인한 변화와 이번 최종 비교를 섞지 않는다.

OS 파일 cache·CPU 스케줄·전역 swap은 완전히 격리하지 못했다. 개별 프로세스 최대
RSS는 동시 전체 실사용량이 아니다. API 수정 후의 재컴파일·JS/선언 방출·binary 크기·
원격 HTTP·다른 플랫폼/컴파일러 성능은 새로 측정하지 않았다. compiler heap 정책과
복구 간격을 생략한 보편적 개선율로 게시하지 않는다.

하네스는 `c270a38`, 복구 도구는 `38a9f7b`, 결과·원시 근거는 `733989f`에서 단위
리뷰 후 커밋했다. 공개 문서도 별도 단위 리뷰·검증 후 커밋했다. 이번 통합 리뷰의
요청 범위에서 남은 필수 수정·미검토 영역·필수 검증은 없다. 제품 변경이나 새 공개
benchmark/원격 CI 성공을 주장하지 않는다.

전체 수치와 재현 경계는 `representative-comparison.md`, 원본은
`measurements/2026-10-22/representative-final-comparison.tar.gz`와 그 README를 따른다.
