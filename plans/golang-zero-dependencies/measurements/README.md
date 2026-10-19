# Go 외부 런타임 의존 제거 전후의 로컬 비용 비교

외부 의존이 있던 기준 버전과 현재 후보를 같은 입력·Go/Node·옵션에서 순차 측정했다. 반복한 시간·RSS 비교는 5회 중앙값이며 전체 값과 최소/최대는 `comparison.json`에 있다. 디버그 정보를 제거한 바이너리 크기는 별도 단일 빌드 결과다. 준비·심볼·플랫폼 검사는 반복 시간 비교에 포함하지 않는다. 순서를 교차하지 않았고 통계적 유의성을 주장하지 않는다. 공개 성능 수치가 아니다.

| 종류 | 시나리오 | 이전 중앙값 | 현재 중앙값 | 단위 |
| --- | --- | ---: | ---: | --- |
| `features` | `features-small-json-selected0` | 0.04 | 0.09 | 초 |
| `features` | `features-regex-heavy-selected0` | 1.46 | 1.49 | 초 |
| `features` | `features-server-selected0` | 1.7 | 1.76 | 초 |
| `generation` | `generation-10000-selected0` | 15.46 | 15.62 | 초 |
| `generation` | `generation-10000-selected1` | 1.89 | 1.92 | 초 |
| `generation` | `generation-10000-selected10` | 1.85 | 2.1 | 초 |
| `builds` | `build-full-static-cold` | 5.1 | 4.37 | 초 |
| `builds` | `build-full-static-warm` | 0.23 | 0.19 | 초 |
| `builds` | `build-selected-cold` | 4.86 | 4.45 | 초 |
| `builds` | `build-selected-warm` | 0.22 | 0.19 | 초 |
| `builds` | `build-named-cold` | 5.04 | 4.34 | 초 |
| `builds` | `build-named-warm` | 0.23 | 0.2 | 초 |
| `builds` | `build-dynamic-cold` | 5.55 | 4.43 | 초 |
| `builds` | `build-dynamic-warm` | 0.25 | 0.21 | 초 |
| `server` | `build-server-cold` | 4.67 | 4.39 | 초 |
| `server` | `build-server-warm` | 0.18 | 0.18 | 초 |
| `runtime` | `BenchmarkRegex/ascii/execute` | 70.5 | 152.7 | ns/op |
| `runtime` | `BenchmarkRegex/unicode/execute` | 95.76 | 130.4 | ns/op |
| `runtime` | `BenchmarkURL/ascii` | 1,401 | 218.5 | ns/op |
| `runtime` | `BenchmarkURL/long-idn` | 5,847 | 5,056 | ns/op |
| `runtime` | `BenchmarkNormalization/ascii` | 4.72 | 4.932 | ns/op |
| `runtime` | `BenchmarkNormalization/long-combining` | 4.612e+04 | 2.473e+04 | ns/op |

| 제공자 전체 생성 | 이전 시간(초) | 현재 시간(초) | 이전 RSS(MiB) | 현재 RSS(MiB) | 현재 소스(MiB) |
| --- | ---: | ---: | ---: | ---: | ---: |
| cloudflare | 39.44 | 40.97 | 1875.2 | 1849.2 | 734.26 |
| digitalocean | 3.29 | 3.54 | 490.8 | 488.7 | 4.65 |
| github | 8.97 | 10.36 | 478.7 | 552.3 | 157.62 |
| gitlab | 10.87 | 11.14 | 590.7 | 617.6 | 213.05 |
| microsoft-graph-beta | 308.95 | 336.44 | 6522.5 | 6608.8 | 3047.34 |
| stripe | 10.94 | 11.75 | 548.1 | 555.6 | 237.12 |
| twilio | 1.94 | 2.03 | 111.7 | 134.4 | 42.10 |

| 소비자 | 이전 바이너리(MiB) | 현재 바이너리(MiB) |
| --- | ---: | ---: |
| full-static | 6.96 | 6.44 |
| selected | 6.93 | 6.42 |
| named | 6.96 | 6.43 |
| dynamic | 8.03 | 7.19 |
| server | 4.79 | 4.67 |

작은 명세는 자체 URL/Unicode 소스를 준비하는 고정 비용이 남는다. 중복 정렬 제거로 초기 후보보다 줄었지만 이전 버전보다 느린 경우도 유지해서 기록했다. 단순 정규식도 자체 VM 실행 비용이 남으며 모든 시나리오의 개선을 주장하지 않는다. 병적 정규식의 작업 한도 도달 시간은 서로 다른 엔진의 작업 단위 차이가 있으므로 정상 처리 속도 비교로 해석하지 않는다.

반복 패턴 공유·NFC 빠른 경로·임시 상태 캐시 경합 제거의 전후 자료는 초기·중간 후보의 압축 기록과 프로파일에 있다. HTTP 압축 해제 제거 전후의 벤치마크는 책임 범위가 달라 직접 비교하지 않는다. 최초 항목 시간·처리량·할당량·GC 전후 힙 자료는 실행 벤치마크의 원시 출력에 보존했다.

6개 플랫폼 교차 빌드, Linux에서의 네이티브 실행, Playground WASM 빌드 및 전체·선택·독립·계산 조회·서버 소비자 빌드가 성공했다. 서버 빌드 사례는 동적 스키마 컴파일과 검증을 실행한다. 실제 HTTP 핸들러는 별도 통합 검사에서 검증한다. 게시된 npm의 빈 네이티브 캐시 검사는 실제 게시 후 실행할 항목이며 이 로컬 결과에 포함하지 않는다.

## 동일 명령으로 재검증한 동시 실행

이전의 별도 진단 실행은 정확한 명령 메타데이터가 남지 않아 진단 자료로만 보존했다. 모든 CI가 끝난 뒤 동일한 소스·명령으로 다시 측정했다. 각 값은 5회 중앙값이다. 전체 범위·할당량은 `matched-parallel-comparison.json`, 정확한 명령·소스 해시는 `parallel-final.tar.gz`에 있다.

| 패턴·동시 작업 수 | 이전 ns/op | 현재 ns/op |
| --- | ---: | ---: |
| `ascii/workers1` | 67.85 | 143.00 |
| `ascii/workers8` | 10.28 | 28.39 |
| `ascii/workers64` | 9.69 | 23.44 |
| `capture/workers1` | 59.61 | 254.50 |
| `capture/workers8` | 10.16 | 44.09 |
| `capture/workers64` | 7.95 | 35.47 |

## 원시 기록과 재현

`index.json`은 초기·중간·최종·실패·의도적 중단·검증 기록의 압축 파일과 해시를 연결한다. 실패한 명령은 최종 성공 표본에 포함하지 않았다. `devtools run -- python3 plans/golang-zero-dependencies/record_measurements.py --verify plans/golang-zero-dependencies/measurements/*.tar.gz`로 파일·해시·JSON 복원을 확인할 수 있다. JSON 공백은 정규화했으며 원래 바이트 수·해시와 복원한 값의 해시를 함께 보존했다. stdout·stderr·프로파일은 원시 바이트를 보존한다. 생성 바이너리·빌드 캐시는 임시 디렉터리에 남기고 해시와 크기를 기록했다.

각 압축 파일은 별도 디렉터리에 풀어 사용한다. 예를 들어 두 runtime 기록을 푼 디렉터리에 `devtools run -- python3 plans/golang-zero-dependencies/compare_measurements.py BEFORE AFTER comparison.json`을 적용하면 원래 임시 경로 없이 사례별 통계를 재현할 수 있다. 빌드·규모별 생성의 기준 기록은 `../baseline`, 제공자 기준 기록은 `../../golang-target/measurements/2026-10-19/raw/w11-final-providers.tar.gz`를 재사용한다. 제공자 기준의 생산 코드331개가 `cab3cb8` 체크아웃과 일치함을 확인했다. `provenance.json`에 비교 범위와 버전을 기록했다.

`reproduction`에는 실제 기록에 사용한 실행 스크립트를 보존했다. 원래 임시 경로가 포함되므로 새 환경에서는 입력·출력 디렉터리를 바꿔 실행한다. 이전 소비자에만 기존 go.sum을 복사했고, 이전 정규화 벤치마크는 기존 Unicode 정규화 모듈, BodyLease는 이전 Encoded 인자를 사용했다. 소비자 cold 빌드는 각각 빈 GOCACHE에서 시작하며 외부 모듈 다운로드 시간은 측정하지 않았다. 제공자 생성은 포맷·해시하는 sink를 사용해 대규모 소스 쓰기를 제외한다. 최종 생성 모드의 생산 코드는 동일하다. 실행 벤치마크는 이후 생성 대상의 공유·방출 보완보다 앞서 실행했지만 자체 엔진 Go 소스는 최종 코드와 동일하다.

`reachability.json`은 실제 소스 선택과 바이너리 심볼을 대조한다. 전체 생성의 단일 호출과 독립 클라이언트는 matcher 소스를 포함해도 최종 바이너리에서는 제거했다. 계산 조회는 matcher를 유지하고, 동적 서버 스키마는 compiler와 property 자료도 유지한다. 정적 소비자4종에는 compiler/property 소스가 없었다.
