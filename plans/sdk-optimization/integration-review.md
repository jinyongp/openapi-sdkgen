# 통합 리뷰 진행 기록

대상: correctness 기준 `25d5626` 이후 이 workstream의 최적화·측정 도구·결정 기록.
최종 완료 판정 전에는 아래 미완료 항목과 최신 commit의 검사 유효성을 다시 대조한다.
수용 기준은 `plan.md`와 `execution-policy.md`를 따른다.

| 영역 | 현재 검토 | 근거와 남은 확인 |
| --- | --- | --- |
| projected closure 계획·merge | source 검토 | exact incidence는 projection별 부분집합만 공유한다. ordinal heap merge는 Go 순서·중복 거절·descriptor reference를 유지한다. W02의 부분 겹침/특수 키/동일 map·startup 증거와 최종 TS suite를 대조할 것 |
| registry·공개 타입·resource | source 검토 | 준비·Link 연결·decorate·등록 순서와 map identity 유지. 좁은 canonical 계약을 사용한다. 700 public/300 private 및 네 compiler consumer 회귀 통과. 최종 TS generator suite·Graph 전체 source/public/server 검사는 미완료 |
| Go final sink·packing | 단위 완료·통합 compile 미완료 | partition/import binding 뒤 exact import 그룹만 묶는다. 제외 규칙·singleton·8 MiB payload/128 group·sticky failure·경로 충돌·최종 flush를 확인했다. native facade/reflection·publisher 소유권 회귀와 Graph 실제 생성·전체 계약 대조 통과. 실제 두 제공자의 64/128/256 KiB 계약·cold/warm 80회·정확한 warm 40회 및 쓰기 증폭을 검토해 W06 current done. 전체 Graph compile은 W09에 남는다 |
| Go HTTP 반복 후보 E | 단위 행동·실제 AST/계약·HTTP 10쌍 통과 | `Parameter` 재사용, 최대 16개, header 등록 시점·missing·snapshot·첫 오류·취소와 응답 index/동점 유지. 기존 HTTP/media/stream 및 33개 항목의 HTTP/race 검사 통과. 실제 Graph AST 3.65% 감소와 기존 6,434,421개 계약·정확한 추가 8개를 확인했다. 실제 GitHub 두 호출은 paired ratio 95% 상한 1.0282/1.0209, B/op·allocs/op 동일. Graph cold는 시간 초과, 전체 compile은 미완료 |
| TS section 필터 후보 C | 진단·네 compiler·실제 membership 통과 | 동일 10 GiB control은 semantic 120초 미완료, literal membership 후보는 9.14초 계측 구간 안에 오류 0개. canonical InputSections/operation reachability에서 기존 계약 청크에 membership만 생성한다. 이전 mapped-filter oracle·generic/union/never/resource/negative·네 compiler source/isolated/dts와 실제 Graph 3×6 전수 membership은 통과했다. 최종 Graph source/public 검사는 아래 resource 오류의 후속 수정을 포함해 재확인할 것 |
| resource callable Surface·registry | 실제 root 오류 312→158→0·전체 미완료 | canonical Members/상속 interface의 확대 계약과 실제 Graph owner 0–19/root는 통과했다. 6 GiB owner 20은 진단 전환으로 중단했고 새 10 GiB 전체 검사는 host 재시작으로 최종 결과 없이 끝났다. 실제 CLI·client/types API에서 GC·mapped generic 제약 비용을 확인했다. ID view 한 줄 후보는 기존 허용 generic/literal/union·전체 key·네 compiler/세 모듈·독립 declaration/runtime 회귀를 통과했다. 실제 Graph 후보 대조·전체 CLI/public 성공은 미완료 |
| validator identity/runtime | W07 보류 결정 완료 | exact 중복 0개, Program 매개변수화 기회와 비용을 별도 계량. planner/template/runtime 원래 코드 불변. 추가 공유·새 hot-path 호출 없음. P의 최종 runtime 검사는 별도 |
| source ownership·검사 도구 | 도구 회귀 통과 | 각 source의 유일 owner·hash·fresh declaration/build-info·실제 source list·오류 전파와 JS/native 검증. isolatedDeclarations 및 RSS budget 검증. 실제 Graph 최종 consumer 성공은 미완료 |
| Go 계약 비교 도구 | P/E 실제 Graph·정확한 추가 경계 통과 | filename을 제외하고 final package/defined type/public 서명과 import binding을 비교한다. P는 기존 6,434,421개가 모두 같으며 E는 그 계약을 보존하고 검토한 8개만 추가됐다. 양쪽 publisher manifest 전수·큰 파일 supplement도 일치한다. import 문자열 복제를 피한 실제 대조는 약 540 MB RSS다. 기존 변경/삭제·미지 추가·목록 누락/중복을 거절하는 양성/음성도 통과. compiler/runtime는 별도 |
| output·배포·fingerprint·rollback | 일부 회귀 통과 | pack의 no-op/추가·삭제/stale 제거/user edit 보존 통과. 전체 source packaging·provider 혼용·ABI·최종 rollback/복구 경로는 W10 확인 필요 |
| CI·문서·측정 근거 | 미완료 | 기존 generation-only Graph 결과를 compile 성공으로 바꾸지 않는다. local 수치를 public benchmark에 게시하지 않는다. 최종 필수 suite·재현 archive·문서 정합성·task/integration evidence를 확인할 것 |

`check:dev`의 SSOT `scripts/dev/check.sh`는 `ci.sh` 전체 suite와 release cross-build·예제
consumer를 직렬 호출한다. 그 실행에서 동일 최종 source의 suite가 통과하면 개별 suite를
다시 실행하지 않고 해당 하위 로그를 재사용한다. 실제 실행 경로·도구·code basis를
함께 남기며 계획한 검사나 기존 source의 성공을 새 source의 성공으로 기록하지 않는다.
