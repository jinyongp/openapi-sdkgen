# 로컬 검증 원본

## 출력·복구

`distribution.tar.gz`: 2,595,313바이트, 2,160개 항목.
SHA-256: `df46712754d65a5b668365d9050d4074de8affa3614fead69d8d183c0dc8e177`.
index에 기록한 자료 파일의 크기·SHA와 후보 patch를 archive에서 다시 읽어 대조했다.
index 자체를 포함한 archive 전체는 위 SHA로 확인할 수 있다.

- `trials/`: 출력 22개 회귀, 현재 CLI build, TS·Go 재생성 복구, 실제 provider 혼용/ABI,
  npm 소스 패키징의 완료한 recorder 원본과 redacted stdout/stderr.
- `inputs/`: Todo 원본, 이전·현재 전체 생성 SDK, 복구 결과, 실제 native strict 검사에
  쓴 복제본·canonical config·probe와 JS/declaration 출력.
- `producers/`: 실제 실행한 검증·archive 스크립트.
- `baseline/`: 이전 generator의 source hash·binary hash·build 정책과 해당 실행의 Git
  revision·dirty 경로·source manifest. runtime benchmark stdout은 이 단위 범위가 아니다.
- `candidate.patch`: 당시 현재 CLI build의 변경 두 파일. 아직 Graph 검증·채택 전인
  TypeScript 후보이며, 이 archive의 작은 입력 통과로 최종 채택을 주장하지 않는다.
- `index.json`: 전체 포함 파일의 크기·SHA, rebuild revision·도구와 제외 항목.

generator·compiler 실행 파일은 넣지 않았다. 원본은 로컬 측정 디렉터리에 남기고 hash와
build/source 근거를 보존했다. baseline은 `e0e10df6bf002397f1add51afba51c88b2fee649`,
현재 build는 `c70a476699cd69c8fce00115138b76dc42a090b3`에 candidate patch를 적용한
소스다. Go 1.27.2·CGO=0으로 기록된 build 명령을 사용한다. baseline build의 유일한
dirty 파일은 `scripts/perf/golang/runtime_bench.go.tmpl`이었다. 해당 파일은
`56fbfe4` 버전이 보존된 basis SHA와 같으므로, baseline checkout에 그 버전을 적용하면
원래 dirty 상태도 재현할 수 있다. clean checkout build와 동일하다고 취급하지 않는다.

개인 CPU 모델·private task context·관련 없는 helper·진행 중 corpus/Graph 결과는
선택하지 않았다. 이 archive는 내부 로컬 단위 검증 원본이며 공개 성능 근거,
전체 Graph 성공이나 W10/W11 완료를 뜻하지 않는다.

## Go 38문서의 첫 전체 실행과 입력 대조

`go-corpus-attempt-1.tar.gz`: 9,271,246바이트, 266개 항목, recorder trial 40개.
SHA-256: `2e371349f2d1f4b76c6ce8016fa3f5363ded9dabad5f57787d52ca234295b102`.
index와 모든 자료 파일을 archive에서 다시 읽어 크기·SHA를 대조했다.

원본 38개 문서 실행은 36개 통과다. Spoonacular의 누락된 CI 입력 보정을 적용한 별도
실행이 통과해, 정확한 CI 입력·현재 관련 source·auditor binary·Go 1.27.2·server·전체
compile policy·필수 HTTP 검사를 대조한 집계는 **37/38**이다. 남은 Cloudflare는
RSS 보호 종료이며 전체 compile·HTTP 통과가 아니다. workstream W09는 미완료다.

- `trials/`: auditor build, 원래 38개 실행, Spoonacular 보정 실행의 원본. 실패 1개와
  RSS 보호 종료 1개도 그대로 포함한다.
- `reports/`: 원래 문서별 report·집계와 별도 보정 report·전수 대조·CI 입력 manifest.
  Cloudflare의 auditor 최종 report는 저장되지 않았으므로 만들지 않았다.
- `cloudflare-input/`: 종료 후 보존한 SDK의 54,542개 파일 inventory·전수 재계산 hash,
  `go.mod`, 실제 HTTP fixture와 중단 분류. HTTP fixture 보존은 실행 성공과 다르다.
- `catalog/`, `existing-ci/`: 원본 catalog와 비교한 CI manifest·기존 CI report 원본.
- `producers/`: 실제 실행한 검증·보정 대조·원시 보존·archive 스크립트와 입력 보정 코드.

Cloudflare의 약 976 MB 기계 생성 소스와 auditor 실행 파일은 로컬에 보존하고 archive에는
파일 inventory·생성물 hash·binary/build/source 근거를 넣었다. 외부 원본 입력은 catalog의
고정 SHA·Git blob·원래 경로와 정확한 CI 보정으로 다시 확인할 수 있다. 실행 파일과
기계 생성 소스의 경로·제외 이유는 index에 명시했다. 당시 auditor build의 TypeScript
후보 patch는 위 `distribution.tar.gz`에도 보존돼 있다.

이 묶음은 통과·실패를 함께 보존한 첫 실행 원본이다. 새 실패를 원본에 덮어쓰거나
미완료를 성공으로 바꾸지 않는다. 별도 Graph, 최종 반복 측정·통합 검사는 포함하지 않으며
진행 중 TS 실행·private task context는 선택하지 않았다.

## TypeScript operation ID 계약 진단

`operation-contract-probes.tar.gz`: 20,056,928바이트, 117개 항목.
SHA-256: `68389333850ac6d5e0ed2c57f0a5ab49050a8f7bc422d3de07da1d73505b6c50`.
완료된 API 진단 8건과 공개 계약 단위 검사 1건의 원본·CPU/heap 프로파일·후보 patch·
재현 스크립트를 포함한다. index·patch·모든 자료 파일을 다시 읽어 hash를 검산했다.

앞선 원본·ID view·key bound의 미완료 semantic과 전체 API RSS 보호 종료도 보존했다.
bounded canonical 계약 후보의 세 파일 진단은 오류 없이 끝났고, 단위 검사는 네 compiler와
모듈·declaration·generic/Link 계약을 통과했다. 완료된 진단 수집과 semantic 완료는
index에서 구분하며, 전체 Graph strict·server 성공이나 공개 성능 수치로 집계하지 않는다.

복제본 네 개의 변경 대상 TS 입력·config·ownership plan과 각 진단의 전체 program-file
목록을 포함한다. 기계 생성 SDK/declaration 전체 트리와 실행 파일은 로컬에 남기고,
이 archive에는 모든 원본 compiler 입력 바이트를 넣지는 않았다. 정확한 경로·도구 hash와
생성 근거, 제외 이유는 index에 명시했다. 단위 검사 뒤 추가한 변경은 기존 `Operations`
JSDoc 복구뿐이며, 해당 분기를 제거한 source hash가 실제 단위 검사 기록과 일치함을
확인했다. 진행 중 새 전체 Graph 검사와 private task context는 포함하지 않았다.

# 최신 선택 SDK·대형 참고 자료

`selected-sdk-verification.tar.gz`: 7,052,323 B, SHA-256
`6eb992b9ef002cec95054c7a2646ef76b2ba35d581abe2c99462ef6cecd0e531`.
2,140개 entry의 크기·SHA를 전수 검산했다. 작은 Graph/Cloudflare 선택 SDK 전체 출력,
네 compiler·Go 전체 compile·실제 HTTP와 처음 잘못된 HTTP fixture의 실패를 포함한다.
정확한 결과와 초기 큰 선택의 생성 전용 한계는 `../../selected-sdk-verification.md`를 따른다.

`full-stress-limits.tar.gz`: 66,129,639 B, SHA-256
`59395966618b312acfe8f3b8334edf22f77b2462bb7675e888d79f0240180a9e`.
204개 entry를 전수 검산했다. 이미 종료된 전체 Graph/Cloudflare 원시 결과·ownership·
manifest·producer를 보존하며 전체 SDK/임시 선언/실행 파일은 중복 포함하지 않는다.
`../../full-stress-limits.md`의 실패 분류와 제외 범위를 따른다.

## 대표 성능 실험 원본

`representative-performance.tar.gz`: 35,652,082 B, SHA-256
`2893050c5805366949b0189cd4ff4ea1fc56a684f53b24718e394ba930b0e515`.
8,156개 entry와199개 trial을 검산했다. 두 실제 제공자의 P-only 묶기 크기·80 build·
40개 정밀 warm, 실제 선택 GitHub SDK의 P/E HTTP10쌍, W02 closure 초기화 표본을
보존했다. 작은 SDK·입력·fixture·producer도 포함한다. compiler cache와 실행 파일은
제외했고,36개 benchmark 로그의 CPU 모델 header 값만 생략해 원본 hash와 변환을
index에 명시했다. 수치·오류·이상 표본은 바꾸지 않았다. 현재 전체 생성기 timing이나
전체 Graph/Cloudflare 성공을 증명하는 자료가 아니다.
