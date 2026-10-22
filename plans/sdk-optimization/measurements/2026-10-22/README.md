# 출력·복구 단위 검증 원본

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
