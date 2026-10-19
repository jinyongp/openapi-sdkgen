# W11 최종 리뷰

검토 기준은 사용자 결정, 이 계획의 N01–N12, 원래 TypeScript/Go 기능 계약이다.
제품 코드는 W10 `806144e`에서 검증한 상태다. W11은 문서·검증·인계 기록을 정리한다.
기존 단위의 성공을 최종 코드에서 다시 실행한 것으로 표현하지 않는다.

| 영역 | 검토·재확인한 내용 | 증거 | 상태 |
| --- | --- | --- | --- |
| 책임·경계 | 생성기 입력 처리 모듈과 생성 SDK를 구분. 전송은 HTTP 계층, 검증은 자체 원본, 방출은 Go 대상의 책임 | 계획 N01–N12, architecture, W02/W07/W08 | reviewed |
| 정규식 | 문법·역참조·역방향 assertion·캡처 초기화·고립 surrogate·공유 Program·취소·작업 한도 | W04/W05/W07, 최종 Go CI/race/fuzz | reviewed |
| Unicode·URL | 공식 자료 해시·재생성·정규화·IDNA·origin·불변 record·파싱/직렬화의 일치 | W03/W06, 최종 Go CI/race/fuzz | reviewed |
| HTTP·수명 | 압축을 이유로 client/pool/header를 수정하지 않음. actual headers, 구조화 Raw 검증, close1/EOF/취소/stream 소유권 | W02, 최종 Go CI/Go 예제/Playground | reviewed |
| 준비·선택·방출 | 명시적 source 목록·AST import·동일 pattern 공유·성공한 program 수집·불변 source와 fresh bytes | W07/W10, 소비자/결정성/수정 후 재방출 검사 | reviewed |
| 생성 표면·의존 | 전체/선택/독립/계산 조회/server/callback/webhook. 빈 module cache·offline·CGO0 실제 소비자, source/deps/symbol 도달성 | W08/W10, 제공자21사례 계약 동일, reachability.json | reviewed |
| 문서·출시 | EN/KO·README·Go 예제·Playground ZIP·Unicode 고지·배포 smoke. 미출시 제품 안내나 외부 모듈 설치 명령 없음 | W09/W10, 최종 docs/Playground/6개 배포 교차 빌드 | reviewed |
| 성능·재현 | 같은 입력/도구/조건, 반복 범위·실패·중단·프로파일, cold/warm·5종 소비자, 실제 심볼, 관측된 증가도 보존 | W10, 측정 기록23개·해시 검증·비교/재현 도구 | reviewed |
| TypeScript 영향 | 공유 matcher 보완 후 실행·선언 매트릭스를 새로 실행해 이전 성공의 적용 범위를 재확인 | 새 실행312.63초·성공, w11-evidence.json | reviewed |

## 수정과 재검증

W10 전체 회귀에서 발견한 header pattern 방출, 의존 경계 allowlist, 이전 compiler symbol 전제를
수정했다. 초기 결과를 지우지 않았으며 최종 전체 Go CI에서 해당 패키지를 통과했다.
성능 측정으로 발견한 NFC 할당·전역 channel 경합·동일 pattern 중복·source 중복 정렬도 수정하고
공식 사례·독립 경계·race/fuzz·실제 생성 소비자 및 같은 입력의 후속 측정을 수행했다.

최종 문서 리뷰에서 정적 추출 manifest의 동적 검증이 아직 통합 전이라는 오래된 설명을 고쳤다.
정적 문법 추출과 별도 동적 trace manifest를 구분한다. 계획의 최초 기준 상태도 현재 상태로
오해하지 않도록 실행 결과에 연결했다. 이 수정은 생산 Go 소스나 생성 산출물을 바꾸지 않는다.

## 증거의 범위

- 고정 구문3,930개·독립 boolean95개, 보충 trace1,919개와 전체 소스2,119개의 결과를 구분한다.
  Test262 JS 객체 API 전체의 적합성 통과를 주장하지 않는다. 불완전 JS 실행22개·준비 크기로
  추적하지 않은1,863호출은 manifest에 남아 있다. 이 준비 제한을 SDK 입력 제한으로 쓰지 않았다.
- Unicode17 공식 정규화·IDNA6,391개와 고정 WPT constructor 결과를 검사했다.
  Unicode17에서 금지한 두 코드포인트가 포함된 WPT7사례는 원본·해시·거부 근거를 별도 보존한다.
  이를 다른 Unicode 버전의 원인으로 단정하거나 전체 브라우저 API 적합성으로 확대하지 않는다.
- 6개 OS/architecture의 소비자와 배포 바이너리를 교차 빌드했다. 네이티브 실행은 Linux에서 했다.
  다른 운영체제에서 실제 실행한 것으로 표현하지 않는다. Playground는 실제 WASM 생성과
  ZIP을 거쳐 격리한 Go 소비자의 로컬 HTTP 호출까지 검사했다.
- 로컬 반복 측정은 공개 성능 수치나 통계적 유의성 주장이 아니다. RSS·할당·GC 뒤 힙을 구분한다.
  압축 해제 전후의 서로 다른 벤치마크를 해제 속도 비교로 쓰지 않는다. 제공자 생성은 포맷·해시
  sink까지 측정했으며 모든 대규모 제공자 SDK를 컴파일했다고 주장하지 않는다.
- 추적 계획의 설명 정리로 이전 완료 기록이 stale 처리됐다. 원래 단위 증거와 최종 코드의 통합
  결과를 다시 검토해 새 검증 기록을 연결했다. 이전 검사를 새로 실행한 것으로 꾸미거나 waiver를
  사용하지 않았다. 원래 단위별 구현 커밋·증거도 유지한다.

## 버전·재사용 판정

`reusableGeneratorIdentity`는 release/module 버전 또는 깨끗한 VCS revision을 사용하며 dirty
소스는 빠른 재사용을 허용하지 않는다. 출시 버전/새 revision이 기존 생성기의 결과를 구분하므로
별도 임의 fingerprint 상수를 만들지 않는다. 정규식 정적 프로그램에는 version1을 도입했고
matcher가 확인한다. 진단 보고서의 version4와 generation fingerprint를 갖는 출력 manifest의
version2는 필드 형식이 바뀌지 않았으므로 유지한다. fingerprint가 없는 기존 version1도 유지한다.

## 출시 인계

생성 Go 패키지는 외부 runtime 모듈이 없다. 정규식·URL·Unicode 구현과 고지는 소스에 포함된다.
HTTP 압축 협상·해제는 주입한 `http.Client`/`RoundTripper`가 제어한다. 제거된 `ResponseBodyMode`,
`TransportCapabilities.ResponseBody` 및 외부 엔진 오류 cause의 변화와 실제 응답 헤더 검증을
출시 설명에 포함한다. binary·순차 Raw는 실제 받은 바이트를 보존하고 구조화 Raw의 검증은 유지한다.
기존 소비자 go.mod의 사용하지 않는 모듈 정리는 사용자가 수행하며 생성기가 자동 변경하지 않는다.

실제로 게시된 npm의 빈 네이티브 캐시 검사는 게시 후 release workflow에서 실행해야 한다.
Native 후보·npm source pack·release 호출 연결은 검사했지만 아직 게시된 후보가 없으므로
게시 후 검사를 통과로 기록하지 않는다. push·게시·원격 workflow dispatch는 수행하지 않았다.
이 항목은 출시 시점의 실행 절차이며 구현 중 미결 설계 선택이 아니다.

최종 판정: 모든 검토 영역은 reviewed이며 확인된 구현 결함과 미결 설계 선택은 없다.
공유 경로 보완 뒤 TypeScript 실행·선언 매트릭스도 `-count=1`로 새로 통과했다.
생산 Go/template/module/auditor 소스356개의 해시가 W10 최종 측정 코드와 같음을 재확인했다.
