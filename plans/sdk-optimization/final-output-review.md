# 최종 출력·배포 리뷰

W08/W09의 최종 선택 SDK·corpus 검증 이후 `52eab98`의 실제 생성기로 출력 경계를
재확인했다. 추가 제품 수정은 필요하지 않았다.

- 실제 Todo 명세/server의 두 target에서 old fresh→current incremental→current no-op→
  old incremental을 실행했다. upgrade/downgrade의 모든 managed hash와 manifest가
  각 binary의 fresh 출력과 같고 stale0개·사용자 파일 보존·no-op inode/mtime 불변이다.
  TS185→185파일, Go160→120파일이다. 이는 작은 실제 입력의 복구 검증이다.
- 이전/현재 TS SDK의185개 전체 source를 각각 strict/native compiler로 JS·declaration
  emit했다. 현재 provider는 정상 bind되며 이전 generation reference/provider와 ABI999는
  `IDENTITY` 단계에서 거절됐다. 거절한 경우 client module 로드는0회다. ABI1을 유지한다.
- client artifact의 길이 구분 path/data와 identity-neutral 선택 출력을 해시하는 caller,
  선택적 server를 client identity에서 제외하는 경계를 검토했다. 새 파일 배치·타입 표현이
  실제 fingerprint/manifest에 반영되며 이전 fingerprint를 억지로 고정하지 않는다.
- publisher의 commit 전 소유권 재확인·새 unowned 경로 충돌·stale 제거·실패 시 복구
  caller와 기존22개 출력 계약을 재검토했다. 최종 Go core 및 실제 새 출력 roundtrip이
  통과했다. native pack의 사용자 수정·추가/삭제·no-op 경계도 유효한 기존 근거로 대조했다.
- 최종 SSOT의 npm source packaging·교차 빌드6개·예제가 통과했다. 실제 SDK source
  archive/export/client binary 비용은 `final-go-verification.md`에 기록했다. 모든 측정은
  정확한 입력과 경계로 제한하며 로컬 결과를 공개 benchmark로 게시하지 않았다.

기존 compatibility workflow와 source에 묶인 GitHub Actions 보고서는 변경하지 않았다.
과거 Graph generation-only와 Go38/38 결과를 새 commit의 전체 compile 통과로 바꾸지
않는다. 이번 결과는 승인된37개 전체·Cloudflare/Graph 선택 및 로컬 SSOT 검증이다.
공개 사용 문서·설치 계약은 바뀌지 않았고 TS 설치를 생성의 필수 조건으로 추가하지 않았다.
공개 docs 페이지 변경이 없어 docs build/validate를 새 완료 근거로 주장하지 않는다.

원시 입력·설정·manifest·실행 결과·consumer·producer는 `final-integration.tar.gz`에 있다.
비공개 context·실행 파일·Go build cache의 제외와 이전 실패도 명시했다. 최종 제품 입력
1,841개가 통합 검사 이후 같다는 검산을 다시 통과했고 이 영역의 남은 필수 결함은 없다.
