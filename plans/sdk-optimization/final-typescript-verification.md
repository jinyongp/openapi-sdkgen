# 최종 TypeScript 검증

제품 기준은 `52eab98`다. 최종 SSOT 계속 실행에서 TS 생성기522개, 런타임1,172개,
native matrix, 도구 검사, 지원 compiler 네 버전의 consumer/declaration matrix,
교차 빌드6개와 예제 클라이언트가 통과했다. 런타임 suite의 기존25개 skip은 유지했다.
최종 실행은1,120.99초·최대 session RSS4,638,683,136 B이며 비교 성능 표본이 아니다.

최종 생성기로 Graph 선택5개를 다시 생성하고275개 전체 source·server export·isolated
declaration·전체 공개 선언·원본 key/legacy 호출형·양성/음성 사용례를 네 compiler에서
strict 검사했다. 실제 HTTP7건도 통과했다. 변경된18개 파일을 이전 compiler 통과로
대체하지 않았다. 원본 pin과 선택 범위는 `probes/graph-selection-bounded.json`을 따른다.
새 manifest SHA는 `58dc0c8c08d2bc759ad0dba7e6bec281f6feec844442cd9519fbff7f3efd785f`다.

통합 검사에서 발견한 canonical 계약 중복·inline import identity 오인과 selector hover
회귀를 수정했다. 기존 catalog·선언 옵션16개·checker self-test, resource의 네 compiler
소비 검사·LSP24개를 재검증한 뒤 전체 suite를 다시 통과했다.
수정·중간 실패와 초기 진단의 한계는 `canonical-declaration-fix.md`에 보존했다.
전체 Graph 검사는 사용자 결정에 따라 참고 실패로 유지한다.

`check:dev`의 포트 준비는 vault 잠금으로 차단됐다. 비밀값 없는 기존 devtools 측정
명령이 준비한 환경에서 동일 SSOT 스크립트와 계속 실행 경로를 사용했다. 첫 CGO=0
시도는 race 검사를 수행할 수 없어 실패했고, 이후 검사 자식에 CGO=1을 명시했다.
최종 계속 실행은 영향받은 전체 suite를 재실행했으며, Go 소비자166개 통과만 실제
Go 생성 의존269파일의 동일성을 검증해 재사용했다. 원격 GitHub CI 성공을 주장하지 않는다.

최종 검사 이후 제품·설정·fixture·도구1,841개 입력의 hash가 그대로임을 확인했다.
recorder/계약 비교 Python self-test와 inventory/SCC 계획 Node self-test도 통과했다.
전체 영역의 최종 리뷰는 `final-integration-review.md`에서 기록한다.

원시 `measurements/2026-10-22/final-integration.tar.gz`는12,109,087 B·8,517 entry다.
SHA-256은 `df997b51b481bfb37e9bad69a03ce4e2c5a5b9909330df0e19e75fe2d0053583`이며
모든 자료의 크기·hash를 archive에서 다시 대조했다. 종료된 실패·최종 검사·producer·
작은 SDK source/선언/JS·실제 배포 자료와 재사용 근거를 포함한다. Go build cache·
실행 파일·private task context는 제외했고, 초기 scratch hover 진단은 정상 SDK 검사와
분리해 한계를 명시했다. 큰 외부 원본은 고정 pin/hash로 식별한다.
