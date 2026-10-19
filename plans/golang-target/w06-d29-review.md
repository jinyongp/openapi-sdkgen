# W06의 D29 영향 재검토

기준은 D29와 `w07-complete-code.json`이다. W06의 원래 최종 근거는 그대로 보존한다.

| 영역 | 검토 | 상태 |
| --- | --- | --- |
| 지연 시작·단일 소비·취소 | 기존 Stream과 source lease 수명은 불변이다. Link factory는 실제 대상 실행 때만 OpenSourceFactory/MapSource로 변환하며 취소 context와 Close를 전달한다. 원본 factory를 Follow에서 열지 않는다. | 검토 완료 |
| native JSON·SSE·MIME | framing·마지막 bytes+EOF·frame 상한·전체/항목 schema와 직접 part selector 분리를 유지한다. typed response item에만 DecodeResponse/응답 context 검증을 적용한다. | 검토 완료 |
| complete·incremental·바이너리 | Link의 native carrier가 슬롯과 factory를 분리해 보존한다. 미디어 전체 Body 또는 value만 교체할 때 버려진 원본 factory는 열지 않는다. 수신 sequential Raw는 header Link만 허용하고 본문을 암묵적으로 소비하지 않는다. | 검토 완료 |
| 코덱·context·실패 | Encode/Decode의 stage별 context와 cancel-before-close·원인 결합을 보존한다. 새 response context는 로컬이며 동시에 진행하는 request codec을 변경하지 않는다. | 검토 완료 |
| protocol·자원·package | 기존 H1/H2 duplex·64 MiB 지연 파일·recursive MIME·binary view 소비자를 포함한 전용 11개를 다시 실행했다. core-link-sources는 필요한 Link/request stream에서만 참조한다. | 검토 완료 |

독립 최상위 11개가 25.496초로 통과했다. 명령·현재 소스와 원본 로그 해시는
`w06-d29-check.json`에 있다. 마지막 affected Link/페이지 race도 통과했다.
이 단위의 미검토 영역이나 확인된 미수정 결함은 없다. W09 수신의 실제 duplex와
W11 전체 성능/플랫폼은 각 단위에서 별도로 완료한다.
