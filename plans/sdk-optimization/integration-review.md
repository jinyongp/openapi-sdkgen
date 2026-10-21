# 통합 리뷰 진행 기록

대상: correctness 기준 `25d5626` 이후 이 workstream의 최적화·측정 도구·결정 기록.
최종 완료 판정 전에는 아래 미완료 항목과 최신 commit의 검사 유효성을 다시 대조한다.
수용 기준은 `plan.md`와 `execution-policy.md`를 따른다.

| 영역 | 현재 검토 | 근거와 남은 확인 |
| --- | --- | --- |
| projected closure 계획·merge | source 검토 | exact incidence는 projection별 부분집합만 공유한다. ordinal heap merge는 Go 순서·중복 거절·descriptor reference를 유지한다. W02의 부분 겹침/특수 키/동일 map·startup 증거와 최종 TS suite를 대조할 것 |
| registry·공개 타입·resource | source 검토 | 준비·Link 연결·decorate·등록 순서와 map identity 유지. 좁은 canonical 계약을 사용한다. 700 public/300 private 및 네 compiler consumer 회귀 통과. 최종 TS generator suite·Graph 전체 source/public/server 검사는 미완료 |
| Go final sink·packing | source 검토·실제 생성/계약 통과 | partition/import binding 뒤 exact import 그룹만 묶는다. 제외 규칙·singleton·8 MiB payload/128 group·sticky failure·경로 충돌·최종 flush를 확인했다. native facade/reflection·publisher 소유권 회귀와 Graph 실제 생성·전체 계약 대조 통과. 전체 compile·파라미터 비교는 미완료 |
| validator identity/runtime | W07 보류 결정 완료 | exact 중복 0개, Program 매개변수화 기회와 비용을 별도 계량. planner/template/runtime 원래 코드 불변. 추가 공유·새 hot-path 호출 없음. P의 최종 runtime 검사는 별도 |
| source ownership·검사 도구 | 도구 회귀 통과 | 각 source의 유일 owner·hash·fresh declaration/build-info·실제 source list·오류 전파와 JS/native 검증. isolatedDeclarations 및 RSS budget 검증. 실제 Graph 최종 consumer 성공은 미완료 |
| Go 계약 비교 도구 | 실제 Graph 대조 통과 | filename을 제외하고 final package/defined type/public 서명과 import binding을 비교한다. publisher manifest 전수·큰 파일 supplement가 일치하며 양쪽 6,434,421개 계약의 차이는 0개. 큰 import 문자열 복제 문제를 수정한 뒤 양성·음성 및 실제 Graph 대조가 541 MB RSS로 통과. compiler/runtime는 별도 |
| output·배포·fingerprint·rollback | 일부 회귀 통과 | pack의 no-op/추가·삭제/stale 제거/user edit 보존 통과. 전체 source packaging·provider 혼용·ABI·최종 rollback/복구 경로는 W10 확인 필요 |
| CI·문서·측정 근거 | 미완료 | 기존 generation-only Graph 결과를 compile 성공으로 바꾸지 않는다. local 수치를 public benchmark에 게시하지 않는다. 최종 필수 suite·재현 archive·문서 정합성·task/integration evidence를 확인할 것 |

`check:dev`의 SSOT `scripts/dev/check.sh`는 `ci.sh` 전체 suite와 release cross-build·예제
consumer를 직렬 호출한다. 그 실행에서 동일 최종 source의 suite가 통과하면 개별 suite를
다시 실행하지 않고 해당 하위 로그를 재사용한다. 실제 실행 경로·도구·code basis를
함께 남기며 계획한 검사나 기존 source의 성공을 새 source의 성공으로 기록하지 않는다.
