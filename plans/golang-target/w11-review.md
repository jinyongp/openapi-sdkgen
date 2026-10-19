# W11 통합 검증 리뷰

상태: 검토 완료. 이 단위에 미검토 영역·확인된 미수정 결함은 없다.
최종 1415파일 소스는 [완료 소스](w11-verified-code.json)이며 SHA256은
`1e7c3b09d00dab78f12159ce6082fc1bf4f50eb02d7fd048d865729ef6e187c5`다.

| 영역 | 상태 | 실제 근거 |
| --- | --- | --- |
| 기능 302개·조건 5개·보충 18개 | 검토 완료 | 기존 TS 필드·조건 보존, Go 증거 참조 813개와 실제 함수 연결. catalog 정상18건 전체 생성 패키지 race 소비자·의도한 오류1건. 전체18보충의 현재 통합 근거 연결 |
| 원본 제공자·holdout·3.2 | 검토 완료 | 원본39건 실제 전체 방출·API/inbound 정체성·강화 비교 통과. 제공자7개×선택1/10의 격리 모듈 전체 생성 패키지 compile/run14건과 모든 원본 pin 확인 |
| 책임·소유권·생성 구조 | 검토 완료 | Prepare가 의미와 닫힌 파일/import 의존을 고정하며 EmitTo는 파일별 렌더/format/sink를 소유. 원본 validator·immutable plan·입력 borrow·옵션 컨테이너 복사·재귀 포인터와 cycle 거부를 확인하고 최종 outer race 통과 |
| HTTP·미디어·스트림·자원 | 검토 완료 | 동일 원본 Go/TS 독립 HTTP 기대값8종, typed/dynamic Accept 우선순위와 trusted extension 실제 소비자. 공개 typed Stream의 schema 거부·지연 실행·정리 및 최종 runtime46조건×5표본/forced-GC/경합·동시 race 통과 |
| 성능·생성·빌드·제거 | 검토 완료 | runtime9/build117/generation33/platform10/provider107명령. 각 생성·빌드 조합 n5, 두 진입점 named/static/selected/dynamic의 실제 실행·nm/deps·cold/warm. 105제공자 표본 결정성·원본 full artifact 일치. 실패·framing-only 이력과 최종 수치를 분리 |
| 플랫폼·race·compiler 회귀 | 검토 완료 | Go·TS runtime/tools·네 compiler CI, release6교차 빌드·실제 WASM/ZIP·예제. CGO0 여섯 generated-consumer/server-runtime 교차 빌드와 Linuxamd64 실제 실행. 다른 OS 실제 실행으로 확대하지 않음 |
| 의존성·보안·배포 준비 | 검토 완료 | nested Go module OSV inventory·clean/findings/incomplete 회귀. 실제 root govulncheck/OSV와 fresh full SDK library7외부모듈 symbol scan0findings. 고정 라이선스/하위 고지·root/npm NOTICE·source pack 확인 |
| 문서·예제·증거 현재성 | 검토 완료 | EN/KO 동일 코드, 실제 corrected RequestID 소비자, 최종 docs validation/build·diff 통과. 후보→측정4파일과 측정→최종4파일 적용성을 명시하고 원래 raw/source를 보존. 현재 Go guide는 다음 release 경계를 명시 |

## 발견·원인·수정·영향 재검토

1. Cloudflare inline allOf의 재귀 DTO 무한 확장: 최초300초 timeout/약10GiB live heap을
   보존했다. 활성 객체와 validator 정체성으로 포인터 연결하며 원래 모든 assertion을 유지한다.
   참조 sibling·복제 allOf의 실제 decode/encode/assertion/cycle 거부와 원본 전체 방출을 재검증했다.
2. 일반 Go 호출의 암묵 Accept: 원본 small-json fixture 독립 기대값으로 차이를 재현하고
   삽입을 제거했다. 일반·Raw·Unicode·명시 Accept·잘못된 출력/JSON·선언 오류·금지 원시 인증
   헤더와 전송 횟수를 두 실제 SDK로 확인했다. 빈 경로 문자열은 원래 schema가 허용하므로
   잘못된 기대값만 제거했고 원본을 바꾸지 않았다.
3. Stream 자동 Accept가 client 기본값보다 앞섬: 기대201 대신200을 재현했다. 공통 execute의
   Begin 옵션 병합 뒤 fallback을 적용한다. typed/dynamic의 자동/client/request/명시 빈 값과
   영향 HTTP·스트림·최종 outer race를 재검증했다.
4. OSV root-only inventory: 중첩 Go 예제 모듈도 열거하며 상태 회귀와 실제 DB 검사를 통과했다.
5. 감사 도구의 미선언/중복 경로·inbound 및 Fetch 예외 비교 누락: 정확한 code/target/scope/
   route/effect와 원본 정체성으로 검사한다. 9조건 비교 unit과 최종 원본39건을 다시 확인했다.
6. 제공자 선택의 기존 W140 생략 경로 선택: full의 실제 공개 경로에서 선택한다. DigitalOcean
   기존471개 생략의 원본 hash/진단 code·route·scope·effect·rule·message가 baseline과 같다.
   입력과 거부 정책을 바꾸지 않았다. 새 선택자로14실제 소비자와105반복 생성이 통과했다.
7. 측정 경계: 기본 HTTP도 매 반복 새 요청을 구성하며 named는 두 entry 합집합 후 하나만
   import한다. mutex/block profile은 일반5표본과 분리한다. static/named에서 정규식 실행은
   제거돼도 공유 패키지 초기화18심볼은 남았고 selected는 의존 패키지 자체가 빠졌다.
   dynamic은 registry 실행 경로를 보존한다. 크기만으로 제거를 판정하지 않았다.
8. EN/KO RequestID 예제: SDK 소유 X-Request-ID를 raw Headers로 지정하던 예제를 전용
   RequestID로 수정했다. 실제 생성 소비자가 이전 표현을 전송 전 거부하고 수정 표현은
   정확한 헤더로 한 번 전송하는 것을 확인했다. locale 코드·docs validation/build를 재검사했다.
9. framing-only Stream 측정: 공개 Events().Stream의 요청 구성·항목 검증·DTO 변환으로 바꾸고
   invalid-item 거부와 수명도 확인했다. 첫 보완 스크립트의 helper 중복 compile 실패를 보존했다.
   중복을 제거한 새 소스에서 runtime46×5와9명령, 영향 문서/옵션/검사가 모두 통과했다.

trusted vocabulary를 낮춘 뒤 executable을 삭제해도 생성 SDK가 minLength3을 검사하고
잘못된 입력을 전송하지 않는다. metadata는 원본 document JSON을 보존하며 입력 URL의
자격 정보를 추가하지 않는다. 기본 경로에 unsafe·CGO·pool·무제한 전역 cache를 추가하지 않았다.

## 현재 증거와 완료 경계

[전체 기능/통합 근거](w11-feature-evidence.json), [참조 감사](w11-feature-reference-audit.json),
[성능 원시 연결](w11-performance.json)·[측정 해석](w11-performance.md),
[최종 적용성](w11-verified-applicability.json)·[이전 CI 적용성](w11-ci-applicability.json),
[라이선스](w11-dependency-licenses.json)에 실제 코드/명령/원시 hash를 연결했다.
원래 성공·실패 결과를 새 소스로 재인증하지 않는다. 변경하지 않은 유효 범위만 명시적으로 재사용한다.

원본39건 감사는 전체 소스 렌더·format·hash와 계약 집합의 증거다. 수 GiB full 제공자 전체를
컴파일한 증거와 구분한다. 정상 catalog18건과 제공자 선택14건은 실제 전체 생성 패키지
소비자다. Linux 실제 실행과 다른 OS 교차 빌드를 구분한다. 로컬 수치를 공개 성능 우위로
사용하지 않는다. W12 전체 리뷰와 I01~I04 완료가 남아 있다. commit/push/출시는 하지 않았다.
