# W10 리뷰 루프

기준은 D29 계획 1·3·10·13–14절과 W10 task다. task는
`9e26ac75-5b14-4fcb-9cd6-ee08a17602fc`, run은
`faeb0c70-9c94-4cc3-be06-869f46c34a1f`, claim revision은 `1669`다.

| 영역 | 상태 | 현재 근거·남은 검토 |
| --- | --- | --- |
| 입력 snapshot·Go verifier·정리 | 검토 완료 | snapshot·한 번 생성·기존 입력 소유 경계는 유지했다. 실제 client.go의 자기 package/import root와 import 목록으로 private module·고정 의존을 구성한다. 알 수 없는 외부 의존은 고정 버전 없이 설치하지 않는다. Node/TS 없는 PATH, TOML package/import, compiler17 전파, 성공/실패 정리와 sdk:generate 추가 옵션의 실제 예제 생성이 통과했다. 사용자 모듈은 변경하지 않는다. |
| 기존 TypeScript 검증 호출자 | 검토 완료 | 기존 checker를 target 전용 파일로 옮기고 strict/module/checking/runtime profile과 실패 코드를 보존했다. 실제 TS 생성/컴파일·compiler17·정리와 TypeScript tools CI의 선언/검증 도구·npm 소스·release simulation이 통과했다. 성공 요약은 부모 check.sh가 소유한다. |
| 기능 manifest·조건·증거 | 검토 완료 | 모든302개 기존 ID와 무target TS 필드·5조건이 원래 Git 문서와 JSON 구조상 완전히 같음을 확인했다. targetStates.golang에는 상태·조건·필요 addon·failure scope/effect와 W별 실제 테스트 참조를 넣었다. Fetch-only 메서드 생략은 Go generated 차이로 명시했다. validator가 모든 소스/함수 참조·조건/addon·실패 범위를 확인한다.18보충 계약의 단위별 근거와 W11 통합 대기는 별도로 남긴다. |
| Go suite·CI·성능 명령 | 검토 완료 | Go CI suite는 기존 패키지 검사를 한 번 실행하고 Go verifier의 독립 integration을 소유한다. TS tool suite에는 Go verifier를 중복 등록하지 않았다. golang:verify의 test/race/선택coverage와 golang:perf의 고정100작업 prepare/emit 반복·할당 측정을 연결했다. 성능 명령의 실제5회 smoke가 통과했다. 전체 성능/플랫폼 측정은 W11이며 smoke를 성능 결론으로 쓰지 않는다. |
| Playground·WASM·ZIP·브라우저 | 검토 완료 | 실제 WASM 두 target·Go options·기존2인자 호출·오류와 ZIP 모든 경로/바이트가 통과했다. EN/KO 브라우저에서 사용자 import/package, server/metadata·실제파일/Go강조·ZIP이름·새로고침·잘못된 import 경로를 확인했다. 예제 URL이 target/options를 잃던 경로를 수정했고 SSR 초기 조작은 mounted 전 차단했다. 옵션 URL 갱신을 하나로 모았다. 공유 worker 타입은 canonical 계약을 import한다. privacy 문구도 현재 addon 경계로 고쳤다. 관리 서버는 준비 확인 뒤 사용하고 해당 실행을 stopped로 정리했다. |
| 배포 binary·npm 경로 | 검토 완료 | 실제 배포 형태의 native binary가 PATH가 완전히 빈 환경에서 Go server SDK144파일을 생성했다. Go/Node/TypeScript를 호출하지 않았다. npm source pack은 실제 package/README/LICENSE/NOTICE와 Releaseway 소유 launcher 경계를 통과했다. 출시는 하지 않았으며 배포 launcher와 native version/cache는 기존 Releaseway 소유다. 배포 준비의 전체 플랫폼/릴리스 경로는 W11에서 통합 확인한다. |
| EN/KO 문서·실행 예제 | 검토 완료 | 실제 Go API로 생성·고정 의존·presence·Raw/Link/stream/selection/security/transport/server/generic schema·오류/제한을 문서화했다. Go source revision/다음 release 경계를 명시했다. runnable Go 예제는 로컬 HTTP에서 typed입력/Raw/Link/resource/stream/static선택/webhook까지 실행하고 readonly private module로도 통과했다. locale의 모든 코드 블록이 같고 한국어 일반 개념은 자연스러운 한국어로 검토했다. docs build·locale/기능 검증·기존 문서 도구30검사가 통과했다. |
| 최종 검증·별도 구현/리뷰 증거 | 검토 완료 | 최종1402개 소스 SHA25614f373a2b238361053707821989e3e251abad8cb452a69ce0daadc6b23306458에서 영향 Go race206이벤트, vet/fmt/module/diff, docs build/test, WASM/ZIP, Go예제, verifier, 성능 명령 smoke의11개 검사가 통과했다. Go CI 전체와 TypeScript tools CI도 통과했다.302기능/5조건·18보충 계약을 연결했고 W10 소유2보충 계약의 실제 근거를 남겼다. 전체 통합 성능/플랫폼은 W11 대기라고 구분한다. 별도 validation 등록 식별자는 구현 기록에 남긴다. |


이 단위에 미검토 영역·확인된 미수정 결함은 없다. 전체 기능 대응의 최종 통합 통과나
성능·플랫폼 완료로 확대하지 않는다. W11과 W12는 별도 필수 작업이다. 고정 소스에서
검사마다 before/after 해시를 확인했고, 이전 시작 CI와 브라우저 검증은 해당 경로가
변하지 않았음을 확인해 재사용했다. 기록은 `w10-feature-evidence.json`에 있다.


## W11 통합 변경의 영향 재검토

EN/KO Go 가이드의 raw X-Request-ID 예제를 전용 RequestID로 고쳤다. 실제 생성 소비자가 이전 표현을 전송 전에 거부하고 수정 표현은 정확히 한 번 전송했다. locale 코드·문서 검증/build를 재실행했다. 선택 성능 명령의 Stream을 공개 typed 경로로 보완했고 helper 중복 실패를 보존한 뒤 전체 runtime46×5를 새 소스에서 통과했다.

최종 근거는 [W11 리뷰](w11-review.md), [기능/통합 증거](w11-feature-evidence.json),
[최종 소스 적용성](w11-verified-applicability.json)에 있다. 위의 이전 단위 소스와 실행 기록은
당시 이력으로 유지하며 최종 코드에서의 재검토와 혼합하지 않는다. W11의 별도 구현/리뷰
record는 `aa4df9e5-0c5d-4462-a282-e3642a77cac4`와
`8eaf5468-7404-4d77-9cd0-808a686a783d`다.
