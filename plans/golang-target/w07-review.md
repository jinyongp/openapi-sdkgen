# W07 리뷰 루프

기준은 계획 5·7·7.1·7.2절, D29와 배정 기능 6개·보충 계약이다.
run은 `8148f069-cf80-43c7-96ee-05ac35d54411`이다. W06 이전의 근거는 당시
manifest와 함께 보존한다. W07의 현재 소스는 `w07-complete-code.json`이다.

| 영역 | 상태 | 검토 범위와 보완 |
| --- | --- | --- |
| 확장 의미·진단·소유권 | 검토 완료 | 공통 extension planner·소스 소유권·Link 대상/server 계획을 추출했다. TypeScript synthetic IR·타입 렌더링 어댑터를 유지했다. TypeScript 관련 133개와 공통 HTTP 계획 검사가 통과했다. Go→TypeScript 의존이 없다. |
| resource·route·selector | 검토 완료 | public 리소스·구체 Routes·전체 문서 명명·매개변수 snapshot·실행 작업의 제약 검사와 복합 인자를 확인했다. incompatible tuple/sealed 인자는 Routes fallback을 실제 HTTP로 검사했다. 무단어 literal 경로가 생성을 막는 오류를 재현해 메서드 terminal로 수정했다. |
| Link 계획·외부 closure | 검토 완료 | component Link·component response·외부 파일·오프라인 잠금 closure·상대 server 기준을 실제 Go 소비자로 확인했다. 공통 소유권의 정확한 source identity·읽은 closure 제한·capability quarantine을 유지한다. hidden 대상 오류와 internal 직접 호출의 경계를 확인했다. |
| Link 표현식·merge·source | 검토 완료 | sealed Raw/HTTP-error source·private metadata snapshot·명시 SourceInput·미선언 응답 속성·원래 envelope를 확인했다. false/0/null/missing·escaped/큰/잘못된 배열 Pointer·한 단계 병합과 native binary/MIME/factory 전달을 실제 HTTP로 검사했다. SourceInput에는 원본 root 제약을 다시 적용하지 않고 대상 실행이 검증한다. |
| 상태별 Link API·옵션 | 검토 완료 | exact/range/default/강제 ByStatus·다른 target의 tagged input/options/request/output·필수 security·잘못된 tag 차단을 확인했다. typed stream·지연 시작·header Link의 Raw 비소모·native media의 전체/부분 교체를 검증했다. 다른 작업 source와 대상 override는 실제 Go 컴파일러가 거부했다. |
| 페이지·envelope·sort·오류 범주 | 검토 완료 | cursor/offset/both·빈 page·반복 상태·취소·Close 해제·선택 envelope fallback·sort enum 쌍·오류 범주와 typed details를 검토했다. 복수 응답 항목은 전체 계약 union이다. 마지막 조합 리뷰에서 Link 페이지 입력 재설정을 재현했고 최초 Next에서 한 번 준비하도록 수정했다. 마지막 affected race7개/21.340초가 통과했다. |
| 생성 책임·공개/내부 경계 | 검토 완료 | immutable Prepare/Emit·정적 직접 호출·선택 전 이름 예약·필요한 파일 dependency를 확인했다. public/internal/hidden 경계를 유지한다. W08 생성 선택·독립 client와 W11 실제 DCE 측정은 해당 단위의 완료 조건으로 남긴다. |
| 현재 검증·기능 배정·완료 근거 | 검토 완료 | 후보 전체 race414 이벤트/269.599초·TS133/33.012초 통과 후 pagination template/test 2개만 변경했다. 최종 affected race7개/21.340초와 vet/fmt/mod/diff가 통과했다. 1307개 소스 manifest와 배정6개·보충3개 연결은 w07-feature-evidence.json에 있다. 선행 W01–W06의 독립 검사와 각 단위 전체 리뷰도 완료했다. |

리뷰에서 확인한 원인과 재검증 경계:

1. HTTP 응답 미선언 속성: 기존 TypeScript는 unknownProperties preserve였다.
   Go에 응답 전용 context와 strict-first 조합 판정을 추가하고 Raw/Link에는 원래 전체
   값을 유지했다. standalone/request는 strict다. response-properties 공유 fixture,
   typed stream·페이지·오류 details 소비자와 전체 회귀를 검사했다.
2. body 없는 선언에 실제 Content-Type이 있으면 선택 실패: body 없는 미디어 선언을
   fallback으로 인정한다. typed Do는 NoContent, Raw는 실제 유한 본문을 보존한다.
   외부 잠금 Link 소비자와 기존 응답 전체 회귀로 확인했다.
3. Link native body: JSON만으로 binary·MIME 파일·SourceFactory를 옮길 수 없다.
   내부 carrier가 타입 있는 슬롯을 보관하고 대상 검증/프레이밍으로 연결한다. Follow는
   열지 않는다. 전체/항목 schema가 다른 같은 미디어·부분 media 교체를 실제 HTTP에서
   검증했다. Nullable required 누락은 null과 구분해 요청 전에 거부한다.
4. 리소스 인자: 첫 작업 제약으로 selector를 검증하면 공유 가능한 다른 작업을 막는다.
   표현만 snapshot하고 Do에서 구체 제약을 검사한다. tuple 필수 prefix와 sealed
   생성자 제약처럼 Go 표현이 달라지는 경우는 Routes로 보존했다.
5. 페이지 항목: 첫 응답 모델만 사용하면 다른 응답의 합법적인 값과 tuple 직렬화가
   실패한다. 전체 응답의 item 계약을 수집해 union으로 보존했다. 같은 fixture의 실제
   TypeScript 기대값을 확인했다. 객체 property 제약 차이는 TS E653을 그대로 유지한다.
6. Link 페이지 순회: 매 Raw에서 prepareLink를 반복해 cursor를 초기화하던 오류를
   `[a,b]` 기대값이 `[a,a]`로 실패하는 소비자로 재현했다. 최초 Next의 initial callback에서
   준비하고 제거하여 이후 query가 유지되게 했다. 지연 시작·취소·기존 Link도 재검증한다.

실패했던 전체 후보의 2개 scaffolding 오류는 새로운 resource dependency를 검사
whitelist에 반영하고 이름 테스트의 synthetic IR에 Method/Path를 채워 수정했다.
제품에 불완전 IR 추정 로직을 추가하지 않았다. 이후 전체 race가 통과했다.

브라우저가 제어하는 Origin 같은 헤더는 Go에서 명세의 명시 입력으로 전달한다.
Go 전송에 Fetch 금지 규칙이나 browser 자동값 공급을 약속하지 않는다. 배정표의 기존
transport adaptation에 따른다. 방문 cursor/offset 집합은 반복 종료에 필요해 순회 동안
유지하며 Close에서 해제한다. 외부 closure를 추가 fetch하거나 순차 Raw를 Link가
암묵적으로 materialize하지 않는다. contentSchema는 공개 문자열을 유지한다.

이 단위에 미검토 영역·확인된 미수정 결함은 없다. W08–W12와 전체 기능·성능 및
최종 통합 리뷰는 여전히 별도 필수 범위다. W07 구현과 리뷰 validation은 devtools의
현재 정의에 별도로 기록하며 이 문서만으로 후속 작업 완료를 선언하지 않는다.
