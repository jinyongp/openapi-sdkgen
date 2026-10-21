# 선행 조사 원시 근거

`preflight.tar.gz` SHA-256:
`dc310e0f3825e7cb8ba046c0305d526b520a78ebf074ad90a8d81e75282fb5fa`

크기 5,460,138 B, 파일 281개, trial 53개다. 성공한 최종 검사뿐 아니라 중단·실패·
초기 도구 오류·negative consumer·기존 TS2456 재현을 모두 보존한다. 일부 command의
실패는 검사 대상의 실제 결과이며 archive 전체를 제품 통과 근거로 해석하지 않는다.

- `trials/`: 실행별 started/result/source hash manifest, redacted stdout/stderr.
- `inputs/`: 정상/negative consumer, 원래 fixture, 축소 재현 원본, 작은 Go AST inventory.
- `ci/core-1.json`: 534 pass·4 opt-in skip, exit 0인 새 Go core 검사 기록.
- `inventory/summary.json`: 실제 Graph TS 분포. 초기 `largestSccFiles` 필드 해석은
  `index.json`의 `sccCorrection`과 현재 inventory 도구를 따른다.
- `index.json`: 포함 파일의 bytes/SHA-256, 53개 outcome, Graph source coverage,
  SCC 수정 집계, archive에 넣지 않은 대형 로컬 artifact의 경로·bytes/hash.

모든 포함 파일을 다시 읽어 index의 SHA-256과 대조했다. 생성 SDK 전체·Graph 원본·
대형 closure/file/import inventory는 포함하지 않았다. 로컬 원시 artifact는
`.tmp/sdk-optimization-preflight/`에 남아 있다. 원본 입력 pin, 제품 소스, 재현 가능한
진입점은 `preflight.md`와 `scripts/perf/sdk-optimization/README.md`에 있다.
대형 artifact를 다시 만들 때 저장된 source/input/옵션/도구 identity와 대조해야 한다.
실행별 절대 경로는 원래 provenance이며 archive 내부 경로 대응은 index를 사용한다.

이는 내부 로컬 조사 근거다. 실제 최적화 후보 측정이나 공개 성능 수치가 아니다.
