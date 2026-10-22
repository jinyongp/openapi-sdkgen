# Go 파일 묶기 크기 비교

상태: 원본과 64/128/256 KiB의 두 실제 제공자 계약·80개 전체 build는 통과했다.
정밀 warm 시간도 40개 표본을 추가해 확인했다. 기본 128 KiB를 유지한다.
후속 사용자 승인에 따라 최종 범위는 corpus37개전체·Cloudflare/Graph선택SDK이며
전체 Graph·Cloudflare 성공·반복은 요구하지 않는다. 파일 수 감소를 cold compiler 시간·메모리
개선으로 바꾸어 주장하지 않는다.

고정 holdout의 `tvmaze.com`(42개 route)과 `listennotes.com`(24개 route)을 사용했다.
입력 Git blob/SHA-256과 P 이전 generator의 source basis/binary hash를 확인했다.
E·V는 제외했다. 같은 원본 publisher manifest를 정렬 순서로 replay했으므로 native
emitter 순서·생성 시간이나 Graph 파일 수를 대체하지 않는다. 실제 native P Graph의
별도 생성·전수 계약 증거는 `execution.md`에 있다.

| 제공자 | 크기 | 파일 | 바이트 | cold 중앙값 [최소, 최대], 초 | route 하나 추가의 쓰기 바이트 |
| --- | --- | ---: | ---: | --- | ---: |
| tvmaze | 원본 | 567 | 3,254,060 | 12.0282 [12.0216, 12.4374] | 104,577 |
| tvmaze | 64 KiB | 153 | 3,176,543 | 12.0281 [11.8142, 12.0302] | 902,993 |
| tvmaze | 128 KiB | 142 | 3,174,200 | 12.0271 [11.8150, 12.0318] | 800,482 |
| tvmaze | 256 KiB | 135 | 3,172,712 | 12.0279 [11.8232, 12.4435] | 1,179,055 |
| listennotes | 원본 | 1,014 | 7,708,762 | 16.2929 [16.0889, 16.5217] | 266,374 |
| listennotes | 64 KiB | 249 | 7,572,813 | 16.2925 [16.0941, 16.2979] | 1,689,324 |
| listennotes | 128 KiB | 200 | 7,563,173 | 16.1029 [16.0887, 16.3094] | 3,557,144 |
| listennotes | 256 KiB | 177 | 7,558,653 | 16.1009 [16.0916, 16.2936] | 3,666,290 |

각 조합은 fresh empty GOCACHE의 전체 build와 같은 cache의 warm build를 각각 5회
수행했다. Go 1.27.2·CGO=0·GOMAXPROCS=4·`-p=1`·soft heap 6 GiB와 사전 고정한
순서·RSS 보호를 사용했다. 40개 cold와 40개 warm 모두 성공했다. cold는 표준 라이브러리
빌드도 포함하며 범위가 겹친다. 원래 warm 외부 시간은 대부분 recorder의 200 ms 관측
간격에 묶여 비교 지표로 쓰지 않는다. 같은 5개 cache에서 새 Go process를 실행한
내부 monotonic 시간은 다음과 같다. 각 칸은 중앙값 [최소, 최대], 단위는 초다.

| 제공자 | 원본 | 64 KiB | 128 KiB | 256 KiB |
| --- | --- | --- | --- | --- |
| tvmaze | 0.2994 [0.2839, 0.6831] | 0.2710 [0.2615, 0.3244] | 0.2712 [0.2646, 0.3080] | 0.2681 [0.2628, 0.3120] |
| listennotes | 0.3284 [0.3096, 0.5279] | 0.2738 [0.2703, 0.4131] | 0.2773 [0.2602, 0.3588] | 0.2783 [0.2655, 0.3446] |

128 KiB의 paired geometric ratio와 전체 5^5개 resample의 percentile 95% 구간은
각각 0.7882 [0.5915, 0.9385], 0.7920 [0.7183, 0.8647]이다. 첫 표본의 긴 시간도
제외하지 않았다. warm cache-hit build 명령의 비용이며 파일시스템 cache는 통제하지
않았다. cold compile·생성·HTTP 성능으로 확대하지 않는다. 세 묶기 크기 사이에는
뚜렷한 우위를 주장하지 않는다. 분석 producer와 입력 hash도 함께 보존했다.

크기마다 전체/한 route 제외 출력의 정의 package·type·공개 서명·import binding을
원본과 전수 대조했다. 12개 비교가 모두 통과했다. 기본 전체 생성과 모든 route를 명시한
대조군도 두 제공자에서 managed 파일 hash가 완전히 같다. 따라서 한 route의 추가·삭제
delta에 선택 모드 차이가 섞이지 않았다. 제거 방향의 파일/쓰기/삭제 바이트도 원시에
모두 보존한다. 실제 publisher의 stale 제거·충돌·복구 검사는 별도 native 회귀를 따른다.

128→256 KiB의 추가 감소는 7/23개 파일과 1,488/4,520바이트인 반면 추가 operation의
쓰기 바이트는 두 표본 모두 늘었다. 64 KiB는 중간 제공자의 변경 폭을 줄이지만 파일 수가
늘고 작은 제공자에서는 128 KiB보다 변경 폭이 컸다. 어느 크기도 모든 지표에서 우세하지
않다. 현재는 128 KiB를 중간값으로 유지하되, 파일 묶기 때문에 한 operation 변경의 쓰기
증폭이 발생한다는 비용을 수용 판단에 포함한다. package 전체를 다시 검사하는 Go cache
경계와 파일 쓰기 바이트를 구분하며 증분 compile 개선으로 해석하지 않는다.

원시 자료는 `.tmp/sdk-optimization-execution/pack-provider-matrix/summary.json`,
`pack-study-*`의 recorder 결과·source hash·로그, `pack-provider-selection-controls/summary.json`,
`pack-provider-warm-timing/summary.json`·`analysis.json`과 `pack-warm-inner-*`,
실행 producer `pack-provider-matrix.py`와 그 동결 `producer-versions/pack-provider-matrix-v1.py`다.
공유 archive에는 입력 pin과 재현 경로·원시 결과를 포함하고 개인 장비 정보와 private
task context는 넣지 않는다. 최종 검증 전의 후보 판단을 제품 전체 완료로 기록하지 않는다.

실제 작은 입력·모든 표본과 producer는
`measurements/2026-10-22/representative-performance.tar.gz`에도 보존했다.
compiler cache·실행 파일은 제외하고 로그의 개인 CPU 모델 값만 명시적으로 생략했다.
