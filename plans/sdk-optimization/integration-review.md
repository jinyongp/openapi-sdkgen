# 통합 리뷰 진행 기록

대상: correctness 기준 `25d5626` 이후 이 workstream의 최적화·측정 도구·결정 기록.
최종 완료 판정 전에는 아래 미완료 항목과 최신 commit의 검사 유효성을 다시 대조한다.
수용 기준은 `plan.md`와 `execution-policy.md`를 따른다.

| 영역 | 현재 검토 | 근거와 남은 확인 |
| --- | --- | --- |
| projected closure 계획·merge | source 검토 | exact incidence는 projection별 부분집합만 공유한다. ordinal heap merge는 Go 순서·중복 거절·descriptor reference를 유지한다. W02의 부분 겹침/특수 키/동일 map·startup 증거와 최종 TS suite를 대조할 것 |
| registry·공개 타입·resource | source 검토 | 준비·Link 연결·decorate·등록 순서와 map identity 유지. 좁은 canonical 계약을 사용한다. 700 public/300 private 및 네 compiler consumer 회귀 통과. 최종 TS generator suite·Graph 전체 source/public/server 검사는 미완료 |
| Go final sink·packing | source 검토·실제 생성/계약 통과 | partition/import binding 뒤 exact import 그룹만 묶는다. 제외 규칙·singleton·8 MiB payload/128 group·sticky failure·경로 충돌·최종 flush를 확인했다. native facade/reflection·publisher 소유권 회귀와 Graph 실제 생성·전체 계약 대조 통과. 전체 compile·파라미터 비교는 미완료 |
| Go HTTP 반복 후보 E | 단위 행동 검사 통과·실규모 미완료 | `Parameter` 재사용, 최대 16개, header 등록 시점·missing·snapshot·첫 오류·취소와 응답 index/동점 유지. 기존 HTTP/media/stream 및 33개 항목의 HTTP/race 검사 통과. 새 3 type/2 method의 정확한 추가 계약, 실제 Graph AST/compile과 runtime paired 비용은 확인 필요 |
| TS section 필터 후보 C | 원인 분리 진단 통과·제품 검증 중 | 동일 10 GiB control은 semantic 120초 미완료, literal membership 후보는 9.14초 계측 구간 안에 오류 0개. 실제 제품은 canonical InputSections와 operation emitter와 같은 reachability에서 기존 계약 청크에 membership만 생성. 이전 mapped-filter oracle·generic/union/never/resource/negative·네 compiler/isolated 및 최종 Graph는 아직 미완료 |
| resource callable Surface | 실제 오류 확인·단위 호환 통과 | 기존 root의 312개 TS2859는 소유 Surface의 call signature를 부모에서 다시 intersection으로 복제한 경로다. 부모가 canonical Surface만 참조하도록 수정했다. 22단계 경로·이전 signature/key/Parameters/ReturnType·양방향 generic 및 Function.bind/call의 인수/반환 타입을 네 compiler source/declaration에서 검증했다. 새 전체 Graph 결과는 대기 중 |
| validator identity/runtime | W07 보류 결정 완료 | exact 중복 0개, Program 매개변수화 기회와 비용을 별도 계량. planner/template/runtime 원래 코드 불변. 추가 공유·새 hot-path 호출 없음. P의 최종 runtime 검사는 별도 |
| source ownership·검사 도구 | 도구 회귀 통과 | 각 source의 유일 owner·hash·fresh declaration/build-info·실제 source list·오류 전파와 JS/native 검증. isolatedDeclarations 및 RSS budget 검증. 실제 Graph 최종 consumer 성공은 미완료 |
| Go 계약 비교 도구 | P 실제 Graph·정확한 추가 경계 통과 | filename을 제외하고 final package/defined type/public 서명과 import binding을 비교한다. P는 publisher manifest 전수·큰 파일 supplement가 일치하며 양쪽 6,434,421개 계약의 차이가 0개다. 큰 import 문자열 복제 수정 뒤 실제 대조가 541 MB RSS로 통과. E의 새 내부 helper만 정확한 서명·binding·단일 발생으로 허용하고 기존 변경/삭제·미지 추가·목록 누락/중복을 거절하는 양성/음성도 통과. E 실제 Graph·compiler/runtime는 별도 |
| output·배포·fingerprint·rollback | 일부 회귀 통과 | pack의 no-op/추가·삭제/stale 제거/user edit 보존 통과. 전체 source packaging·provider 혼용·ABI·최종 rollback/복구 경로는 W10 확인 필요 |
| CI·문서·측정 근거 | 미완료 | 기존 generation-only Graph 결과를 compile 성공으로 바꾸지 않는다. local 수치를 public benchmark에 게시하지 않는다. 최종 필수 suite·재현 archive·문서 정합성·task/integration evidence를 확인할 것 |

`check:dev`의 SSOT `scripts/dev/check.sh`는 `ci.sh` 전체 suite와 release cross-build·예제
consumer를 직렬 호출한다. 그 실행에서 동일 최종 source의 suite가 통과하면 개별 suite를
다시 실행하지 않고 해당 하위 로그를 재사용한다. 실제 실행 경로·도구·code basis를
함께 남기며 계획한 검사나 기존 source의 성공을 새 source의 성공으로 기록하지 않는다.
