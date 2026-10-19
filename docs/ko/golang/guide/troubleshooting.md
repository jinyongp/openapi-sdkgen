# 문제 해결



## 생성이 실패할 때

진단의 코드·위치·해결 안내를 확인하세요. `--diagnostic-mode collect`로 여러 진단을 볼 수 있습니다.
`--go-import-path`, 스키마·미디어 지원 범위와 [원격 참조 설정](./remote-inputs.md)을 점검하세요.
출력이 충돌한다면 [재생성 조건](./generate.md)을 확인하세요.

## import나 빌드가 실패할 때

출력 위치에서 가장 가까운 `go.mod`를 확인하세요. `--go-import-path`를 직접 지정했다면
대상 모듈 이름과 출력 디렉터리에 맞는지 확인하세요.
`--check`는 생성 결과를 검사하므로, Go 빌드는 `go test ./...`로 따로 확인합니다.
전체 모듈 설정은 [Todo 예제 ZIP](/downloads/golang-todo-app.zip)을 참고하세요.

## 요청이나 스트림이 실패할 때

생성된 `types` 패키지의 오류를 `errors.As`로 확인합니다.

```go
var failure *types.APIError
if errors.As(err, &failure) {
	fmt.Println(failure.Code)
}
```

예를 들어 context로 취소된 호출이라면 `REQUEST_ABORTED`가 출력됩니다. `err`가 `nil`이거나 SDK의 `APIError`가 아니면 이 분기에서는 출력하지 않습니다.

[인증과 전송](./transport.md)을 점검하고, 스트림을 중단할 때는 닫기와 오류 확인을 수행하세요.
XML 매핑의 지원 범위는 [OpenAPI 지원](../reference/capabilities.md)에 있습니다.
