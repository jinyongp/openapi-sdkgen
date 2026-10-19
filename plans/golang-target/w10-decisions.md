# W10 현재 구현 경계

W09는 revision1668에서 실제 완료됐으며 W10 claim은1669다. 다음은 D29 범위의 구체 구현이며
W10 완료를 뜻하지 않는다.

- 입력 획득/snapshot과 생성은 check.sh에서 한 번 실행한다. target verifier가 생성물을
  복사한 private directory를 소비한다. TypeScript의 기존 strict/module/checking/runtime
  profile은 별도 verify-typescript.sh로 옮겼다.
- Go verifier는 generated client.go의 자기 import root와 package를 읽고 그 directory
  안에서만 go.mod와 소비자 probe를 만든다. generated 파일의 실제 import에 해당하는
  현재 고정 의존 버전만 넣고 URL parser의 전이 의존도 고정한다. GOWORK=off,
  GOTOOLCHAIN=local과45초 제한으로 compile/run한다. 사용자 go.mod는 변경하지 않는다.
- CI go suite가 Go verifier의 integration test를 소유한다. Node/TypeScript가 없는 PATH에서
  실제 생성물의 NewClient·Value·Binary 실행, compiler17 전파와 임시 정리가 통과했다.
  기존 TS verifier의 실제 compiler/실패/정리 검사도 통과했다.
- in-memory playground Options는 goImportPath/goPackage/addons만 받는다. 파일/네트워크를
  열거나 생성 소비자를 실행하지 않는다. WASM은 기존2인자 호출과 선택3번째 JSON options를
  받는다. Go 타깃과 원래 TS를 같은 registry/공통 pipeline으로 준비한다.
- browser worker가 options를 전달하며 UI는 Go import root/package와 server/metadata를
  선택한다. 출력 대상은 요청 시작 시 복사해 표시/ZIP 이름을 정한다. Go 파일에는 Go grammar를
  적용한다. 실제 WASM+ZIP의 두 target/옵션/오류/모든 파일 byte 검사가 통과했다.
- feature manifest는 기존 무target TS 필드를 보존한 채 targetStates.golang을 추가한다.
  Go Fetch-only method 생략 차이는 generated로 명시한다. 구조/조건/증거와 공개 문서는
  아직 리뷰 중이다. 이 상태를 전체 대응의 최종 통과로 기록하지 않는다.

W10 문서/실행 예제·배포/브라우저·최종 리뷰·검증이 남았다. W11 전체 호환성/성능/플랫폼과
W12 최종 리뷰도 계속 필수다. commit/push/출시는 하지 않았다.


## 통합·리뷰 보완

- Go target/옵션을 예제 URL에 보존하지 않으면 새로고침에서 TS로 생성되는 문제를 브라우저에서
  확인했다. target/import/package/addon을 먼저 읽고 생성하며 변경 내용은 같은 helper로 URL에
  기록한다. mounted 전 SSR 컨트롤도 비활성화한다. EN/KO에서 실제 Go ZIP과 설정 유지·오류를 확인했다.
- 원래 TypeScript manifest의 모든 필드/조건은 JSON 구조로 동일함을 확인했다.302개 Go profile과
  5개 조건의 실제 테스트 참조·addon·실패 scope/effect를 validator가 확인한다. 기존 TS 증거에는
  Go test뿐 아니라 명명된 TypeScript 테스트도 있어 해당 형식을 보존한다.
- Go verifier는 import block에서 실제 외부 module을 찾고 고정 버전이 없는 의존을 거부한다.
  TOML의 custom package/import와 no-Node 실행이 통과했다. 사용자 모듈 대신 private copied module을
  사용한다. Go suite는 이 integration을 한 번 소유하고 TS tools suite는 기존 TS 경로를 유지한다.
- Go Todo 예제는 같은 실제 생성 API를 사용해 Raw/Link/resource/stream/selection/webhook을
  로컬 서버와5초 context에서 실행했다. EN/KO 코드 블록은 같고 standalone native generation은
  PATH가 완전히 빈 환경에서도144파일을 생성했다. npm source pack 경계도 통과했다. 릴리스는 하지 않았다.
- 최종1402개 source manifest와11개 실제 검사 기록을 고정했다. 영향 race206이벤트와 static,
  docs build/30문서도구검사, 실제 WASM/ZIP, privateGo예제/verifier/benchmark명령이 통과했다.
  TypeScript tools CI도 통과했다. Go CI 결과와 최종 validation 기록은 계속 진행 중이다.
