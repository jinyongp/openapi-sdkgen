# W04 구현 리뷰

범위는 W04와 계획 5.3–5.4·6절, D11/D12/D24/D26이다. W03에서 구현한 전송 능력·
HTTPClient 복사·쿠키 정책·압축 계약을 포함해 생성 SDK의 실제 실행을 다시 검토한다.

| 영역 | 검토 대상 | 상태 |
| --- | --- | --- |
| 서버 | root/path/operation 상속, 선택 ID·변수·enum, BaseURL 우선순위, 절대/상대 URL | 검토 완료 |
| 보안 선택 | AND/OR·익명·단일 자동 선택·다중 명시 선택, 작업별 타입·이름 예약 | 검토 완료 |
| 인증 정보 | 전용 옵션 충족·제공자 생략·부분 충족·동일 값·충돌·누락·잘못된 scheme | 검토 완료 |
| 제공자 | 최종 origin·작업·요구 metadata, 취소·timeout·원인·비재시도·동시 호출 | 검토 완료 |
| 전송 | CookieHeaders/Jar/mTLS 능력, 리다이렉트, 공유 client/Jar/header 불변·압축 | 검토 완료 |
| 공유 의미 | 보안 대안의 ID 충돌, URI 참조·컴파일러 작업 생략, 영향받은 TypeScript 호출자 | 검토 완료 |
| URL·쿼리 | IDNA·포트·IPv4·dot·역슬래시, form 이스케이프·순서·중복 쿼리 | 검토 완료 |
| 현재성 | 최종 전체 회귀·race·정적 검사와 정확한 소스 basis | 검토 완료 |

확인하고 고친 결함:

- HTTP URL과 origin을 net/url 표기만으로 비교하면 IDNA·기본 포트·IPv4 표기가 같은 origin을
  다르게 만들 수 있었다. D25의 고정 파서로 먼저 정규화한 뒤 실제 net/http 타입으로 변환한다.
- 라이브러리의 URL 파싱 검증 결과를 SearchParams API에 확대 적용할 수 없었다.
  `a+b&c`가 구분자로 변하는 실패를 실제 HTTP에서 재현했다. 순서가 있는 pair를 개별 해제하고
  URLSearchParams의 form 집합으로 다시 인코딩한다. `+`, `&`, `~`, `*`, 세미콜론과 중복 키를 검사한다.
- 상대 URL과 절대 URL에 같은 base를 바로 적용하면 같은 scheme의 역슬래시 URL 해석이
  TypeScript와 달라졌다. 절대 URL을 먼저 해석하고 실패한 상대 URL만 Origin으로 해석한다.
- 인증이 적용한 Host가 native Request.Host에 반영되기 전에 복사되던 순서를 수정했다.
- 인증 선택·정보 오류의 기존 코드를 APIError에 보존하고 외부 취소·timeout의 우선순위를 유지한다.
- 이름 예약 중 보안 lowerer 오류를 반환하면 컴파일러가 생략한 작업을 다시 문서 오류로
  승격할 수 있었다. 보존한 작업만 실제로 검증하며 안전한 이웃 작업의 생성·컴파일을 확인한다.
- `key`의 두 번째 대안과 실제 `key__2` scheme, 실제 `anonymous` scheme과 익명 대안이
  같은 선택 ID를 만들 수 있었다. 점유한 전체 ID를 검사하는 공통 의미 함수를 양 타깃에 적용했다.

최종 소스의 Go target/generator/output/CLI 회귀가 통과했다(Go target 103.649초, CLI 1.525초).
Go target/generator/output race는 114.523초로 통과했고 생성 SDK 소비자도 각각 race로 실행했다.
TypeScript의 전체 인증 관련 검사·URI·작업별 타입·서버 선택 회귀는 5.421초로 통과했다.
공통 ID 계획과 Go/TypeScript의 초기 영향 검사는 6.610초/0.857초로 통과했다.
vet/format/module/diff 검사도 통과했다. 소스 1186개를 고정한 `w04-final-code.json`의
SHA-256은 `36a7abdc3bc5affa862b2b3abc5d0a7e8dd56a2f9b7ece5be2d29e7304f38b33`다.

W01–W03의 변경하지 않은 호출자와 생성·rollback·schema/DTO·DCE·HTTP/압축 경로를
이 전체 회귀에서 다시 확인했다. D26의 계획/정의 변경으로 이전 완료 근거가 오래된 상태가
되므로 해당 작업들의 구현·리뷰 근거도 같은 최종 소스로 갱신한다. 기존 기록은 보존한다.
W05–W12의 미디어·스트림·리소스·선택·수신·도구·전체 동등성·
성능·최종 리뷰는 이 단위의 완료 범위에 포함하지 않는다.
