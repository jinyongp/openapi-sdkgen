# 첫 두 개선의 실제 CLI 통합 확인

제품 기준: `2fd6bd1`. 첫 두 개선을 합친 실제 CLI를 사용해 TVMaze·ListenNotes·
GitHub를 선택 옵션 없이 `--target golang --with server`로 생성했다.
import root는 첫 단위와 같은 `example.test/matched/api`다.

원래 배포 소스 `6231d70`에서 동결한 첫 단위의 전체 파일별 hash와 직접 대조했다.
세 입력 모두 관리 파일 집합과 모든 파일 내용이 일치했다. 파일 수는 각각
143개, 201개, 6,375개다. observer 조건을 거친 간접 비교에 의존하지 않고,
원래 CLI 조건에서 최종 CLI의 통합 출력도 확인한 것이다.

사용한 최종 CLI SHA-256은
`8d38d3f3fb6348ca42277928a999ce0c98ea34c5aad830be0b05aca282aceb90`다.
각 한 번의 정확성 실행이며 새 성능 개선율이나 Go cold/warm 컴파일 비용으로 집계하지 않는다.
앞 두 단위의 조건과 개선율은 각 보고서 범위를 유지한다. 관련 제품 소스는 두 번째
단위의 빌드·검사 이후 바뀌지 않았다. 원본 비교 목록·검산·생산 스크립트·실행 로그를
보존한 뒤 생성 SDK는 제거했다. 전용 cache는 약 2.1 GiB, Go 임시 경로는 비어 있다.

[근거 archive](measurements/integration-01-02.tar.gz)는 597,832 B,
근거 항목 32개와 색인 1개다. 모든 항목을 다시 읽어 크기·hash를 확인했다.
SHA-256: `c082b69af8bd13a45df03cd98f09f1428b43eff68eca81921c18e9718ad8ffbc`.
전체 Graph·Cloudflare와 전체 compiler/corpus 매트릭스는 이 확인에 포함하지 않는다.
