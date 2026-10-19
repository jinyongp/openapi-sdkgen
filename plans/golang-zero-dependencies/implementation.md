# Go 무의존성 구현 기록

현재 실행: [계획](plan.md), [devtools ID](workstream.json).
계획 단위별로 리뷰·재검증·커밋한다. W01 완료. W02 압축 경계 통합 검증과 독립적인 W03 Unicode 구현을 진행했다.

## W01 — 기준과 적합성 입력

- 제품 기준 `cab3cb8`(계획만 추가, 제품은 `72e679c`와 동일).
- 고정 Test262에서 정규식 후보 2,119개(리터럴 구문 240개 포함)의 원본과 harness를 확보했다.
  fixture manifest는 path·SHA256·features·flags를 보존한다. adapter 상태는 pending이며
  통과 결과나 제외 판정으로 취급하지 않는다. 원본과 라이선스로 이후 판정을 감사할 수 있다.
- WPT URL·IDNA·percent encoding 원본 JSON을 보존했다. setter/DOM API를 구현 범위로 확대하지 않는다.
- 변경 전 `golang:verify test -run TestGenerated(RegexNodeOracleAndExecutionErrors|Format|HTTPTransportIntegration|Compression) -count=1` 통과.
- `golang:perf runtime`의 생성·컴파일·강제 GC 수명·benchmark 5회·동시성 profile·race 통과.
  원시 자료 `.tmp/golang-zero-baseline-runtime`에 보존했다. 비교 후보는 아직 없다.
- 소비자 4가지 방식의 cold/warm 각 5회와 실행·의존·심볼 검사가 모두 통과했다(117회 invocation).
- 1천/1만 작업 × 전체/1개/10개 선택 생성 각 5회와 소비자 컴파일이 통과했다(33회 invocation).
- 원시 결과·basis·stdout/stderr·GNU time metrics와 해시는 `baseline/{runtime,builds,generation}`에
  모두 보존했다. 생성 바이너리·profile·작업 디렉터리는 대응 `.tmp/golang-zero-baseline-*`에 남겼다.
- HTTP 변경 계약: 압축·Accept-Encoding·자동 해제 정책은 HTTP 계층 소유, 필수 헤더 검증은
  실제 반환된 헤더 기준, Raw는 실제 반환 바이트 기준, 본문 수명·취소는 SDK가 계속 관리한다.

리뷰: 입력 pin·다운로드 경계·licensing·manifest 상태·기존 회귀 확인 완료.
baseline 결과와 SHA, 적용 범위·독립 기대값·재현 경로를 재확인했다.
새 엔진용 upstream adapter는 W04–W07에서 구현·실행할 대상이며 현재 상태는 pending이다.

W01 최종 확인: 본문 수명 포함 명시적 회귀 5개 집합 재실행 통과. 모든 baseline 실행(9+117+33)의
exit code와 raw 출력 해시를 검사했다. 생성 artifact hash map은 중복 제거한 `artifact-trees.json.gz`에
보관하고 각 실행에서 참조한다. 원래 결과의 모든 map을 복원·대조해 일치함을 확인했다.
W01 범위의 남은 결함은 없다. 제품 엔진 적합성은 이후 단위의 pending 검사로 유지한다.

## W03 — 자체 Unicode17 자료와 기본 함수

단일 Go 원본 `internal/goruntime/unicode17`에 property·simple case folding·NFC와 IDNA mapping/
joining/bidi 조회를 구현했다. 표준 라이브러리만 import하며 외부 엔진 코드를 복사하지 않았다.
공식 입력 15개를 SHA256으로 확인해 결정적으로 Go 표를 생성하고 라이선스·고지·원문 fixture를 보존했다.
441개 property, 867개 alias, 23,045개 range를 제공한다. runtime은 map 초기화 대신 고정 배열을 사용한다.

공식 NormalizationTest의 NFC 전체 열, simple folding, IDNA mapping/Joiner의 범위 끝,
Unicode17 새 script·alias 대소문자·script extensions·서로게이트·속성 경계 검사 통과.
Node Unicode17과 전체 alias/범위·빈 구간 137,353개를 대조해 차이0개였다.
보조 oracle이 Hrkt/Katakana_Or_Hiragana를 실제 Script 값으로 받아들이는 오류를 발견했다.
Script/Script_Extensions에 실제 배정된 값만 alias로 채택하도록 고치고 재검사했다.
필수 공식 fixture 검사는 Node 없이도 실행하며 oracle만 선택적으로 skip한다.

일반·race 검사 통과, NFC 멱등성 fuzz 311,848회에 실패 없음(마지막 alias-only 보완 이전).
마지막 데이터 재생성의 byte 비교도 4개 파일 모두 일치했다. 오류·불변 view·기본값·출력 결정성·데이터
도달성 구조를 리뷰했고 고친 alias 영역과 전체 공식 검증을 재검토했다. 세부 해시는 `w03-evidence.json`.
정규식·URL 엔진 자체의 적합성이나 URL의 UTS46 정책을 완료했다고 주장하지 않는다.

## W02 — 압축을 사용자 HTTP 계층에 위임

SDK의 압축 decoder·기본 Accept-Encoding 삽입·DisableCompression 변경과 이미 해제된 응답의
거부를 제거했다. `ResponseBodyMode`/capability 선언도 제거했고 모든 생성 소비자 호출부를 갱신했다.
일반/순차 본문 lease는 지연 읽기·닫기1회·취소·오류·연결 재사용을 유지한다.
고정 br/zstd/gzip/deflate 바이트 fixture로 binary Raw와 custom codec의 전달을 확인하며 정상 테스트
소비자에서 외부 압축 라이브러리를 설치하지 않는다. gzip 자동 해제는 net/http가 수행한다.

리뷰 중 일반 JSON Raw도 기존 미디어 decode·schema 검증을 수행함을 확인했다. 이를 생략하는 API로
확대하지 않고 계획의 Raw 설명을 명확하게 했다. Binary/순차 Raw는 반환 바이트, JSON Raw는 기존
검증 계약을 유지한다. 필수 헤더가 HTTP 계층에서 사라지면 실제 헤더를 기준으로 오류다.

집중 검사에서 HTTP 설정·명시 encoding/Range·연결 풀·자동 gzip·원래 헤더 유지 사용자 transport·
opaque binary Raw·custom codec·필수 헤더 실패·close/cancel/race 통과.
전체 Go 타깃 회귀를 두 번 실행했고 이전 SDK 해제를 전제로 하던 순차 Raw·XML·stream 헤더 기대값을
찾아 새 계약에 맞췄다. 두 번째 전체 검사에서 stream metadata assertion 1개만 실패했으며,
제품 코드는 그대로 두고 해당 기대값을 수정했다. 최종 코드의 HTTP/압축 lease/순차 Raw/XML/실제 stream
영향 집합 6개를 재실행해 모두 통과했다. 나머지 전체 검사 결과는 동일 제품 코드에서의 결과를 재사용한다.
전체 단일 green run은 W11에서 다시 확인한다. 실패 기록을 통과 기록으로 바꾸지 않는다.

공개 lease API를 소비하는 성능 도구도 함께 수정했다. decoder 측정을 SDK lease 측정으로 오인하지 않도록
이름·범위를 바꿨고 생성·컴파일·수명 진단·benchmark 1회 호출이 통과했다
(`.tmp/golang-zero-w02-perf-check`). 이는 성능 비교용 표본이 아니다.
사용자 HTTP 계층/SDK 의미 경계·수명·보안 capability·기존 호출부·측정 도구 리뷰와 영향 재검사를 마쳤다.
실제 모듈·문서의 예전 설치 안내 정리는 W08/W09에서 수행한다. 정규식·URL 외부 의존성은 아직 남아 있다.
