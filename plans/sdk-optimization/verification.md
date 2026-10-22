# 조사·계획 검증 기록

대상: `5ada624a69ebfd85103248379970f78a8f3d2899`의 관련 제품 코드와 이 디렉터리의 계획.
workstream: `88a65ff3-848d-4fff-a307-325ab4cbb221`, 조사 작업 W00.

현재 수용 범위는 2026-10-22 사용자가 승인한 `scope-decision-2026-10-22.md`를 따른다.
전체 Graph·Cloudflare는 참고 스트레스 사례로 전환하고, corpus 37개 전체·Cloudflare 선택 SDK·대표 SDK·Graph 선택 SDK의
정확성·호환성·배포와 최종 통합을 검증한다. 아래 초기 조사·기존 실패 근거는 보존한다.

첫 Graph 범위 변경은 devtools definition revision 2105에 원자적으로 반영했다. 로컬 spec/plan과
15개 task·16개 validation·14개 현재 acceptance 및 전체 plan 참조를 전수 대조했고
dependency 변경과 구조 오류는 없었다. `scope-validation-2026-10-22.json`에 hash와
검증 결과를 보관했다. 정의 변경으로 기존 완료 11개가 stale이 된 것은 테스트 실패가
아니며, 현재 범위에 유효한 실제 근거를 검토·수용하기 전 완료로 복구하지 않는다.

후속 Cloudflare 결정은 definition 2163에서 17개 연산으로 반영했다. spec/plan 본문과
모든 plan 참조·task/validation 의존성·acceptance 연결을 대조했고 구조 오류는 없다.
`cloudflare-scope-validation-2026-10-22.json`이 이 시점의 정합성 결과다.
이 정의에서도 기존 완료 11개는 유효한 근거를 개별 재검토한 뒤 복구한다.

## 실행 결과

| 확인 | 실제 결과 | 근거·범위 |
| --- | --- | --- |
| 첨부 전체 검사 | README·스크립트 8개·원시 JSON 3개·별도 결정 보고서 검토 | 각 수치의 실측/합성/추정/누락을 `investigation.md`에 분리 |
| 원시 결과 중앙값 | 첨부된 3종 JSON의 전체 표본 검산, 보고서 숫자와 일치 | registry/section/runtime 원시는 미첨부 |
| Graph 원본 SHA | 고정 `46bead9a…daec` 일치 | 69,965,060 B의 기존 원본, 문서 수정 없음 |
| 저장 CI 보고서 | Graph·Go 각각 해제 내용 hash가 provenance digest와 일치 | Graph는 generation-only, Go는 38/38 컴파일 pass라는 과거 근거 |
| 실제 GitHub Graph job | generation-only gate 성공, TS 설치·Go compiler skipped | Actions 39456862841, Graph job 118746888987. job 목록은 첫 페이지 30개만 조회 |
| 관련 제품 코드 | `212adb9..5ada624`의 조사 대상 경로 차이 없음 | TS/Go target, compatibility benchmark, TS package manifest |
| TypeScript 생성기 회귀 | **515/515 pass**, fail 0, skip 0 | `GOTOOLCHAIN=go1.27.2 devtools run ci:dev typescript-generator`; `.tmp/ci-tests/typescript-1.json`, exit 0. native/declaration 별도 matrix는 이 묶음에서 제외 |
| Go 소비자 회귀 | **157/157 pass**, fail 0, skip 0 | `GOTOOLCHAIN=go1.27.2 devtools run ci:dev go-consumers 1 1`; `.tmp/ci-tests/golang-1.json`, exit 0 |
| JS 의미 재현 | pass | Node 24.22.0, ordinal merge의 keys·own descriptor·object reference·prototype 보존, assign prototype 변경, UTF-8/UTF-16 순서 차이 |
| Go 초기화 재현 | 위험 재현 | Go 1.27.2, 파일별 `[A Z] 1 2`, 순서를 바꾼 병합 `[Z A] 2 1` |
| devtools 구조 | revision 1967, `valid=true`, `issues=[]` | 원자적 edit dry-run과 저장, 12개 task·13개 validation·완전한 plan 참조 확인 |
| 등록·문서 정합성 | revision 1969, pass | spec/plan 본문, 12개 task·13개 validation·7개 requirement·12개 acceptance·15개 dependency edge와 전체 plan 참조를 누락 없이 비교 |

두 회귀 묶음은 기존 코드의 실행 가능성과 계획에서 재사용할 계약 검사를 확인한 결과다. 새 최적화의 정확성이나 성능 결과가 아니다. 실행 중 source를 바꾸지 않았다. 전체 source compiler matrix·runtime suite·check:dev·문서 빌드·Graph 대형 재측정은 이번 문서 전용 변경의 검사가 아니며 W01~W11에 남겨 두었다.

설치된 devtools는 0.29.0/protocol 8이다. 기본 Go 1.27.3과 별개로 조사 검사는 `GOTOOLCHAIN=go1.27.2`를 명시했다. 잠긴 vault로 `test:dev`와 기본 직접 실행이 거절되어 해당 시도에서는 테스트가 실행되지 않았다. 기존 `inject=false`인 `ci:dev` 경로로 검사했고 vault·프로필 보안 설정·프로젝트 명령을 변경하지 않았다.

## 의미 재현의 입력과 관측

JS 재현은 `['', '10', '2', '__proto__', 'constructor', 'z', '\ue000', '\u{10000}']`에 기존 ordinal을 부여하고 짝수·홀수 block으로 나눴다. 두 block을 ordinal로 병합하여 `Object.fromEntries`로 만든 map과 원래 map을 비교했다. 키별 `getOwnPropertyDescriptor`와 값의 `===`, `Reflect.ownKeys`, prototype을 검사했다.

```txt
checks: pass
keys: ["2", "10", "", "__proto__", "constructor", "z", "\ue000", "\u{10000}"]
UTF-16 sorting: ["\u{10000}", "\ue000"]
UTF-8 sorting: ["\ue000", "\u{10000}"]
Object.assign changes prototype: true
```

Go 재현은 package 변수 `seen`, `mark(value)` 함수, `var A=mark("A")`, `var Z=mark("Z")`를 사용했다. 원래 파일은 `a.go`의 A 선언, `z.go`의 Z 선언 순서다. 병합 파일은 두 선언을 Z→A 순서로 바꿨다. `fmt.Println(seen,A,Z)` 결과가 달랐다. compiler가 생성 파일만 처리하더라도 package 변수·초기화 순서를 안전하다고 생략할 수 없다는 반례다. 임시 재현 위치는 `/tmp/sdkgen-optimization-probes`이며 제품 코드에 통합하지 않았다.

## 범위 리뷰루프

| 영역 | 상태 | 검토 결과·계획 보완 |
| --- | --- | --- |
| 첨부 근거·현재 CI 상태 | reviewed | 857 MB 모델 입력과 일부 원시 결과 부재, Graph generation-only 정책, Go 38개 실제 근거를 구분 |
| TS closure·registry·helper | reviewed | 정확한 closure뿐 아니라 import graph subset·반복 map identity·stage 순서·cross-chunk Link·generic 의미를 명시 |
| Go pack·validator·type identity | reviewed | partition 함수 안의 packer만으로는 비분할/operations 누락. 공통 최종 sink 설계로 보완. 후속 validator 공유의 byte 기반 partition 이동 위험도 추가 |
| 측정·실패·채택 정책 | reviewed | 실제 process tree RSS·native compiler·실패 분모·cold/warm·stripped binary 구분. 5%는 제안이며 사전 정책 확정으로 연결 |
| 구조적 fallback·보류 | reviewed | references의 largest SCC·전체 declaration consumer 비용을 남김. API/workflow 변경을 자동 승인하지 않음 |
| 증분 출력·CI·문서 | reviewed | ownership/collision/rollback, mixed-generation provider 거절, TS optional install, 실제 Actions 근거를 포함 |
| 작업·의존성·종료 | reviewed | W05는 진단 완료와 full Graph 성공을 분리. 조건부 보류가 최종 수용 기준을 약화하지 않음. W11 통합 리뷰와 root validation을 유지 |

첫 패스에서 pack 위치 누락, validator 공유의 재분할 영향, Graph 기존 명령의 검사 미실행, references 성공의 불확실성을 찾아 계획에 반영했다. 두 번째 패스는 변경된 설계·의존성·수용 기준을 함께 대조했다. 구현 전에 측정으로 확정할 D03/D04와 조건부 채택을 계획에 명시했으며 이를 검증 완료나 사용자 확정 SLA로 바꾸지 않았다.

마지막 문서·등록 정합성 검사에서 저장된 spec/plan 본문, task·validation ID, 요구사항·수용 기준·의존성을 대조해 통과했다. 문서 inventory·ID shape·credential 미포함 검사도 통과했다. Git 범위와 commit 후 재확인 결과는 W00의 현재 devtools validation record에 남긴다. 미래 제품 작업과 workstream 전체는 완료 처리하지 않는다.

## 후속 선행 조사와 계획 보완

최신 결과는 `preflight.md`, 원시 근거는
`measurements/2026-10-21/preflight.tar.gz`다. 최초 W00 기록과 새 실행 결과를 구분한다.
`4251698`은 내부 측정·audit 도구 12파일의 단위 리뷰·회귀 검사 후 만든 커밋이다.
제품 `cmd`/`internal`은 바꾸지 않았다.

| 확인 | 실제 결과 | 한계 |
| --- | --- | --- |
| v2 재검산 | manifest 5,921파일·완료 235명령·98,204 source reference hash 일치 | 합성 결과이며 제품 86 matrix는 blocked |
| recorder 행동 | 최종 5개 검사 pass | 정상·오류·signal·timeout·자식 RSS·별도 process group 중단·blocked nonzero |
| Go audit / TS inventory | 무시/init/변수/directive 등 7 Go subcase, AST/config/SCC/no-check 거절 pass | audit는 pack 구현이 아님 |
| JS 의미 | own keys·descriptor·reference·prototype·UTF-8 반례 pass | 후보 제품 구현의 parity 검사는 별도 |
| Graph TS 소스 | 186,448파일·2,358,735,553 B, manifest/hash 대조 누락·추가·불일치 0 | helper resource omission 1,208개는 기존 계약 |
| Graph TS strict 네 compiler | JS 세 버전 heap 부족, native RSS 보호 종료 | 완료 0/4, public consumer 전체 검사 미완료 |
| 실제 작은 TS consumer | todo positive 4/4, negative TS2339 거절 4/4 | Graph/전체 named·선택 계약의 대체 근거가 아님 |
| 기존 재귀 타입 오류 | 원래 fixture와 축소 입력 모두 TS2456 4/4 재현 | 별도 W01R 선행 수정, 현재 제품 코드 미수정 |
| Go corpus compile 기준 | 과거 CI 38/38의 제품 소스·manifest·현재 입력 pin 일치 | correctness만 재사용, candidate 성능 증거 아님 |
| Graph Go 전체 생성 | 900.68초 timeout, peak 6,411,071,488 B | 완성 출력 없음, full compile·전체 pack 후보율 not-run |
| 실제 작은 Go SDK | server 포함 생성·전체 package compile·AST audit pass | 173파일·21 package의 국소 결과 |
| 선행 보완 통합 검사 | `GOTOOLCHAIN=go1.27.2 GOMAXPROCS=4 devtools run ci:dev go-core` exit 0 | 534 pass·4 기존 opt-in 성능 skip. fmt/vet/mod·partition·Go generation-check 포함 |
| 원시 보관 | 281파일·53 trial, 포함 파일 SHA-256 전부 일치 | 대형 생성 트리·closure/file inventory는 로컬 경로와 digest만 보존 |

이번 범위의 통합 리뷰는 다음 전체 영역을 대조했다.

| 영역 | 상태 | 확인·후속 조건 |
| --- | --- | --- |
| v1/v2 주장·원시·기존 CI 재사용 | reviewed | 소급 복원·제품 수치 승격 없음, 38곳 표현 수정 |
| runner·실패·tool identity | reviewed | FieldList 출력·자식 중단·입력 부재·SCC 최대값 수정 후 caller와 회귀 재검증 |
| 실제 TS 구조·계획 | reviewed | closure 비용 실측, 큰 공개 타입 파일 추가, W01R의 correctness 기준 갱신 의존성 |
| Go 영향·실험 경계 | reviewed | EmitTo와 실제 게시 구분, full 생성 timeout을 compile 실패로 오인하지 않음 |
| 기록·검사·소스 범위 | reviewed | 원시 archive 검산·core 검사·제품 소스 동일성 확인. 변경은 내부 도구·계획·재현 입력 |
| 제품 구현 준비도 | incomplete | W01의 Go 단계/GC 진단·최종 inventory·계약 경로·runtime 정책, W01R 수정 필요 |

이 통합 리뷰는 선행 조사·도구·계획 보완 범위다. 제품 최종 W11 통합 리뷰를 실행했거나
W01 이후 수용 기준을 통과했다고 기록하지 않는다. W01은 남은 조건과 다음 동작을
현재 정의에 checkpoint하고 미완료로 유지한다. W00 계획 검증만 새 정의에서 갱신한다.

보완 정의는 revision 1974에 원자적 preview→저장했다. 새 W01R과 전용 validation을
추가해 13 task·14 validation·7 requirement·13 acceptance가 됐다. TS W02/W03/W04의
선행 조건은 W01R로 연결했고 Go W06/W07은 W01을 유지했다. revision 1976에서
spec/plan 본문·전체 ID·의존성·문서 inventory·원시 archive와 결과 해석을 다시 대조해
pass였다. workstream check는 `valid=true`, `issues=[]`, `closable=false`다. 마지막
변경 범위·staged/unstaged `diff --check`와 commit 결과는 현재 validation record에 남긴다.

## W09의 실제 CI 입력 계약 대조

후보의 38문서 검증에서는 원본 SHA뿐 아니라 CI가 사용하는 **보정 후 입력 계약**을
확인한다. `test/compatibility/ci/*-manifest.json.gz`와 원래 catalog를 전수 대조한 결과,
원본 pin은 모두 같았지만 `spoonacular.com` 한 건의 catalog에는 CI의 `preparation`이
없었다. CI는 `spoonacular-parameter-bodies-v1`로 중복 parameter body 99개를 보정하며,
보정 후 SHA-256은 `f08a9540575e201b4d740d6252d21336435a74dddd58b0f91efcbf641976d60a`다.

첫 로컬 검증은 그 보정 없이 실행되어 빈 request-body media type을 거절했다. 해당
검증의 실패 원본을 보존한다. 제품의 해당 거절 동작은 기준 코드에도 있어 최적화 회귀로
분류하지 않는다. CI manifest를 그대로 동결한 별도 입력으로 Spoonacular만 다시 실행한다.
나머지 결과는 관련 제품·auditor source와 binary hash가 그대로이고 유효 입력이 같은
경우에만 재사용한다. 진행 중인 기존 검증 스크립트나 제품의 유효성 검사는 변경하지 않는다.

최종 집계는 다음을 모두 확인한 뒤 작성한다.

- 38개 문서의 원본 SHA 및 보정 유무·kind·최종 SHA·operation 수·`contractsVerified`.
- Go 1.27.2, server 포함 전체 생성과 전체 package compile, 고정 compilation policy.
- Spoonacular·Loket·Stripe·Cloudflare·DigitalOcean의 기존 HTTP 계약 검사 실행 증거.
- 각 recorder의 성공·source manifest·실제 auditor binary와 현재 관련 source의 동일성.

원본 catalog 실패와 CI 입력 재검증은 별도 경로로 남긴다. 최종 38/38 성공 전에는
완료로 기록하지 않으며, 이 결과를 Graph 전체 compile·성능이나 새 공개 CI 근거로
확대 해석하지 않는다.
