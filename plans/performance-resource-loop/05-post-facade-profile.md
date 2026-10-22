# Facade 개선 이후의 병목 재확인

상태: 최신 profile·한 번 출력 후보 조사·회귀 검사·리뷰 완료.
한 번 출력 후보는 기각했고 runtime 제품은 `5ddb3a3`과 같다.

앞 네 단위가 같은 출력 경로의 비용을 바꿨으므로 과거 profile의 순위를 현재
병목으로 재사용하지 않는다. 최신 observer로 고정 GitHub Go+server의 CPU와
게시 완료 후 heap/누적 할당 profile을 한 번에 수집한다. 정확성은 직전 단위 후보의
전체 파일 inventory와 대조한다. intrusive profile 비용은 성능 비교 표본에 섞지 않는다.

조사 예산은 CPU+heap 수집 1회와 필요한 함수별 요약·코드 조사다. 이후 원인이
확인된 후보만 별도 설계·관련 검사·동일 조건 비교로 진행한다. GOMAXPROCS=4,
GOMEMLIMIT=2GiB, session RSS 보호 3 GiB·180초, 단일 전용 Go cache 4 GiB와
cache 외 scratch 2 GiB 상한을 유지한다. 시작 cache 약 2.3 GiB, scratch 약
704 MiB다. 전체 Graph/Cloudflare나 SDK cold compile을 추가하지 않는다.

## 다음 후보

최신 profile의 출력은 직전 확정 후보와 6,377개·204,611,637 B 전체 hash가 같았다.
`formatPartitionFile`의 첫 AST 출력은 sampled CPU 7.29초·누적 할당 2.02 GiB,
이어지는 `format.Source`는 9.96초·4.46 GiB였다. attribution 결과이며 개선율이 아니다.
게시 후 GC를 실행한 heap은 약 3.87 MiB로 작았다. peak RSS와 다른 관측점이므로
잔존 cache가 최대 RSS의 원인이라고 결론 내리지 않는다.

실제 호출은 binding 재작성·implementation 재작성·facade의 세 경로다. 입력은
renderer의 `format.Source` 또는 owned runtime의 `format.Node`를 거친 정규 Go
소스다. 재작성은 이름·선택자·import 문자열·전달 호출을 바꾸며 숫자 리터럴은
만들거나 바꾸지 않는다. Go 1.27.2의 `go/format`과 `go/printer` 설정 차이 중 숫자
정규화는 이 입력/변환 계약으로 유지한다. 비공개 printer mode 비트는 사용하지 않는다.

후보는 이 함수가 소유한 AST에서 `ast.SortImports`를 실행하고 한 번만 출력한다.
이름 해석·사용 import 제거·주석 위치·최종 파일 내용이 유지돼야 한다. 기존 관련
검사와 정규 포맷 검사, GitHub 예비 한 쌍의 전체 byte/hash가 같아야 진행한다.
통과 시 같은 observer의 최대 3쌍과 두 작은 입력을 비교하고 실제 CLI를 확인한다.
차이가 나면 정규화 단계를 무조건 없애거나 출력 차이를 예외 처리하지 않고 원인을
확인해 후보를 보완/기각한다. 전용 cache와 scratch 상한은 그대로 유지한다.

## 후보 기각과 원인

관련 검사에서는 차이를 찾지 못했지만 GitHub 전체 byte 대조는 실패했다.
생성 프로세스는 모두 성공했고, 기준 41.8812초·22,006,531,832 B 누적 할당,
후보 30.3326초·17,182,485,840 B였다. 실패한 예비 쌍이므로 채택 개선율로 쓰지 않는다.
파일 수는 양쪽 6,377개, 출력은 204,611,637→204,758,954 B였다.
공통 경로 4,365개가 달랐고 packed 경로 각 7개가 이동했다.

차이는 숫자가 아니라 AST 위치였다. 이름/선택자를 바꾸면 노드의 새 길이·`NoPos`와
원본 위치의 관계가 달라진다. printer가 아래처럼 receiver/인자에 쉼표를 붙였다.
이는 유효한 Go지만 정규 gofmt 출력과 다르며 이후 표준 정규화가 필요했다.

```go
func (state *SdkgenPrivate__modelState,) Child(key string) *SdkgenPrivate__modelState
```

입력이 이미 정규 소스라는 조건만으로 재작성 후 AST를 한 번 출력해도 정규
소스가 된다고 보장할 수 없었다. 출력을 예외 처리하거나 쉼표만 지우는 우회책을
추가하지 않는다. 원래 formatter를 그대로 유지했다. 이 경로를 줄이려면 새 AST의
위치/출력 표현을 일관되게 설계해야 하며 현재 작은 변경의 범위로 진행하지 않는다.

새 `TestPartitionRewrittenSignaturesRemainCanonical`은 두 파일의 private type을
분할해 이름과 선택자를 바꾸고, 모든 구현/외관 소스가 표준 `format.Source`와
같은지 독립적으로 확인한다. 후보에서 두 implementation 모두 실패했다.
원래 runtime 복원 후 이 검사와 기존 `TestPartition*`·`TestPack*`가 모두 통과했다.
변경된 제품 runtime은 없으며 새 회귀 검사만 남긴다.

후보 source·회귀 검사·실제 비정규 파일 예시는 inner archive
`one-print-prototype.tar.gz`에 3항목으로 보존했다(10,604 B,
SHA-256 `09412f6dcfbe753e998639c56f8c0fd6e7ac8ba17cd24ad2bb47d0e2168920d2`).
실패한 byte 대조와 gofmt 진단, 복원 후 검사도 보존하고 전체 SDK는 제거했다.

## 다음 방향과 검증 범위

같은 최신 profile에서 renderer는 파일마다 같은 template를 다시 파싱했다.
`Parse`에 sampled CPU 1.28초·누적 할당 약 1.19 GiB, template 읽기에 약
122 MiB가 귀속됐다. 요청/발행 단위에서 immutable embedded template의 파싱을
공유하는 방안을 다음 후보로 검토한다. 전역 cache, 누적된 파일 이름별 cache,
오류 정보 손실을 피하고 출력·진단·동시 emission의 독립성을 함께 확인해야 한다.

리뷰한 영역은 profile provenance·AST 출력 계약·기존 검사 공백·회귀 검사와
자원 범위다. 고정 원본 SHA-256은
`d39842ee4d43d701e8a8c5483b4afa7c23218518f943dea2e42d36a3798cbdcc`이며
profile 결과의 전체 출력은 직전 확정 제품의 inventory와 직접 일치했다.
잔존 heap은 게시 후/GC 후 관측이며 peak나 실제 CLI benchmark로 쓰지 않는다.
전체 Graph/Cloudflare·SDK compile·다른 플랫폼·전체 corpus 검증은 추가하지 않았다.

## 보존과 자원

[원시 근거](measurements/05-post-facade-profile.tar.gz)는 1,956,321 B,
근거 항목 104개와 색인 1개다. SHA-256:
`8be8bebd008194690721cf506018d0f557faf6566e42991162f185c75c49da2b`.
모든 항목의 크기·hash를 다시 검증했다. profile 원본·함수별 attribution·실패한
후보 source/예시·실측·inventory·실패/복원 검사를 포함한다. 실행 파일·SDK 전체·
Go cache는 제외했다. 최종 전용 cache 약 2.4 GiB, cache 외 scratch 약 784 MiB,
Go 임시 경로는 비어 있다. 기존 상한 안이며 전체 생성물은 검산 후 제거했다.
