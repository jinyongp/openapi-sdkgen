# 조사·계획 검증 기록

대상: `5ada624a69ebfd85103248379970f78a8f3d2899`의 관련 제품 코드와 이 디렉터리의 계획.
workstream: `88a65ff3-848d-4fff-a307-325ab4cbb221`, 조사 작업 W00.

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
