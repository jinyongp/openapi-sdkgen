# W06 리뷰 루프 — 검토 완료

기준은 계획 5.1–5.2·6.1·7절, D12·D13·D27·D28과 배정 기능 18개다.
최종 소스는 `w06-final-code.json`의 1259개 파일이며 SHA-256은
`33600463c88cccdb6ee07b624c0126416210d7e7c1db5836869262760ea7b847`다.
앞선 W05·W06 후보의 근거는 역사로 보존한다.

| 영역 | 상태 | 검토·검증 |
| --- | --- | --- |
| Stream 상태·factory | 검토 완료 | 지연 시작·동시 Response·단일 Next·겹친 Next 거부·Close·외부 취소·timeout·nil·시작 실패·one-shot 복사를 생성 race 소비자로 확인했다. 시작 실패는 먼저 취소하고 한 번 닫는다. |
| reader·Unicode | 검토 완료 | per-read 상한·빌린 버퍼·진행 없음·bytes+EOF·BOM·분할 UTF-8·잘못된 prefix 치환과 decoded UTF-8 byte 한계를 확인했다. 취소는 실제 Body를 닫는다. |
| NDJSON·JSON-seq | 검토 완료 | 1/2/7/4096 byte 청크·CRLF·빈 record·RS·마지막 EOF·문법·한계·송신 wire를 확인했다. 전체 응답과 증분 스키마를 분리한다. |
| SSE | 검토 완료 | data 문자열·event/id 상태·빈 id·NUL·retry·BOM·줄바꿈·미완성 EOF·wire 순서·한계를 검토했다. contentSchema는 공개 문자열을 유지한다. |
| multipart framing | 검토 완료 | split boundary·preamble·closing·잘못된 입력·part 소유권·본문/헤더 상한·지연 인코딩을 확인했다. 중첩에는 바깥 payload보다 작은 한계를 다시 넣지 않는다. |
| 파트 계약·binary·nested | 검토 완료 | 전체 prefix/item과 증분 item selector·필수 헤더·native adapter 이전 검증·default Content-Type·XML·중첩 바이트를 실제 HTTP로 확인했다. 재귀 anyOf와 9 KiB 내부 헤더도 양쪽에서 통과했다. |
| protocol/adapter | 검토 완료 | native adapter-only·구체 DTO·portable Value·unknown protocol·map 복사·요청 override·잘못된 타입·nil·생성 실패·원인과 Close를 확인했다. 단계별 context로 상위 EOF 뒤 마지막 item/closing bytes를 유지한다. |
| 응답·선택·union·Raw | 검토 완료 | itemSchema 유무별 Stream 노출·schema-only/both Do·media/status union·원본 metadata·Raw 비소비/닫기·finite 응답 Stream 거부를 확인했다. binary를 JSON view로 버리지 않는다. |
| 요청·wrapper·대형 파일 | 검토 완료 | canonical requestBodyShapes가 예약/모델/송신을 공유한다. complete/stream·reuse·one-shot·nil/제로값·항목 오류·조기 응답·custom Encode를 확인했다. 64 MiB source는 헤더 Read까지 Open/Read 0이다. |
| 양방향·압축·오류 수명 | 검토 완료 | 실제 HTTP/1·HTTP/2 barrier로 업로드 종료 전 첫 응답을 확인했다. gzip metadata·bytes+EOF·실패 단계/원인·취소·EOF·정리 실패를 검토했다. Raw lease와 typed 실행 종료를 분리한다. |
| 비용·생성·이름 | 검토 완료 | pipe·writer goroutine·선읽기 queue가 없다. 현재 frame/part와 bounded 청크만 보유한다. JSON 1천/10만 항목의 buffer를 관찰했다. 직접 module/import closure·모델 정체성·전체 문서 이름과 공유 caller를 확인했다. |
| 영향 단위·근거 현재성 | 검토 완료 | D28 W01–W05 전용 검사와 각 단위 리뷰를 별도 완료했다. 마지막 변경은 codec template와 새 수명 테스트뿐이며 나머지 1257개 해시는 같다. 근거는 아래 경계로 재사용한다. |

확인한 결함과 수정·재검토는 다음과 같다.

1. 네트워크 bytes+EOF가 마지막 항목 검증을 취소했다. TransferStream/FinishStream으로
   논리 source 종료까지 execution을 유지하고 실제 HTTP·gzip·Raw를 재검사했다.
2. complete 파트 코덱을 늦추면 조기 204가 잘못된 source metadata를 우회했다.
   metadata 사전 검증을 복구하고 파일은 pull로 읽는다. 기존 XML/custom/nested와 새
   송신·64 MiB·조기 응답 소비자를 재검사했다.
3. factory 정리가 취소를 EOF 오류로 노출하거나 Close가 취소를 기다릴 수 있었다.
   child context를 먼저 취소하고 MapSource의 종료 오류를 정리 전에 확정했다.
4. 전체 응답에 itemSchema DTO를 적용하면 서로 다른 schema의 유효한 전체 값이 실패했다.
   전체 Data 수집과 최종 schema 검증을 분리하고 두 스키마의 다른 성공/실패를 확인했다.
5. binary/nested part를 JSON DTO로 바꾸면 바이트가 사라졌다. MultipartData 슬롯,
   sealed AsMultipart/AsBinary와 복사 Bytes를 추가하고 JSON view의 손실 변환을 거부했다.
6. 요청용 복합 파트 투영을 응답의 다음 재귀 단계마다 펼치면 계획이 무한히 늘어났다.
   native response 자체 sequence facets와 전체 assertion을 분리했다. 양쪽 실제 재귀
   소비자와 큰 내부 헤더를 검사했다.
7. multipart 전용 SDK와 분리한 stream 파일에 미사용 models/schema import가 생겼다.
   실제 alias/참조에 맞춰 고치고 여러 생성 소비자를 다시 검사했다.
8. protocol/adapter 생성 실패·EOF에서 context가 살아 있는 채 Close를 호출했다.
   네 생성 방향의 수정 전 실패를 재현했다. 단계별 child context를 먼저 취소하되 다음
   단계의 final item/closing bytes는 유지한다. 원인·정리 오류·한 번의 Close와 실제
   송수신을 수정 후 검사했다.

`w06-feature-evidence.json`과 check JSON에 실제 근거를 연결한다. 1258개 후보의 전체
Go race는 391개 통과 이벤트(Go target 225.535초), 영향 TypeScript는 107개·83.233초,
공통 XML/encoding은 8개 통과했다. 마지막 두 파일 수정 후 관련 11개 최상위 검사를
race로 다시 실행해 28.300초 통과했고 vet·format·module·diff도 최종 소스에서 통과했다.
두 manifest를 비교해 바뀌지 않은 영역의 결과를 재사용했다.

이 단위에 미검토 영역·확인된 미수정 결함은 없다. complete 수집과 사용자 adapter의
집계 상태까지 상수 메모리라고 주장하지 않는다. 최대 RSS·전체 성능/동등성은 W11,
실제 inbound handler는 W09, selection·DCE는 W08, 전체 최종 리뷰는 W12에서 완료한다.
현재 TypeScript native 미디어·XML 한계를 새 기능으로 확대하지 않는다.


## W11 통합 변경의 영향 재검토

Stream 자동 Accept를 Begin의 기본/호출 옵션 병합 뒤 fallback으로 옮겼다. typed/dynamic의 자동/client/request/명시 빈 값과 기존 native HTTP·스트림 수명·race를 재검증했다. 최종 공개 typed Stream 성능은 항목 검증을 켜고 수집했다.

최종 근거는 [W11 리뷰](w11-review.md), [기능/통합 증거](w11-feature-evidence.json),
[최종 소스 적용성](w11-verified-applicability.json)에 있다. 위의 이전 단위 소스와 실행 기록은
당시 이력으로 유지하며 최종 코드에서의 재검토와 혼합하지 않는다. W11의 별도 구현/리뷰
record는 `aa4df9e5-0c5d-4462-a282-e3642a77cac4`와
`8eaf5468-7404-4d77-9cd0-808a686a783d`다.
