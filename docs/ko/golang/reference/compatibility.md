# Go 호환성

구현된 Go 동작은 [지원 계약](./capabilities.md)에서 확인할 수 있습니다.
[로컬 예제](../examples/index.md)로 모듈·전송·스키마 기능을 먼저 확인하세요.

## 제공자별 측정

이 페이지에는 GitHub Actions에서 측정해 공개한 Go 제공자별 결과가 없습니다. 다른 SDK의
생성·타입 검사 표를 Go 근거로 사용하지 않습니다. 생성 성공만으로 제공자 서버가 모든 요청을
받아들이거나 생성된 모든 호출을 실행했다고 판단할 수도 없습니다.

## 자신의 명세 검증

```sh
openapi-sdkgen generate --input ./openapi.yaml --target golang \
  --go-import-path example.com/consumer/internal/api --output ./internal/api --check
go test ./...
```

출력 디렉터리가 없으면 먼저 생성하세요. 웹훅·콜백에는 `--with server`를 추가합니다.
모의 서버나 제공자의 시험 환경에서 실제 호출과 미디어를 확인하고 입력 버전·선택 API·확장·
Go 버전을 기록하세요. 비용 비교는 [선택과 빌드 비용](../guide/selection-cost.md)을 참고하세요.
