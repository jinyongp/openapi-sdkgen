# 문제 해결

SDK 패키지 경로를 빠뜨리면 생성 오류입니다. 빠진 의존성은 사용자 모듈에서 설치해야 하며
생성기가 설치하지 않습니다. 증분 갱신에는 출력 디렉터리의 관리 매니페스트를 함께 유지하세요.
`errors.As`와 안정된 오류 코드로 `types.APIError`를 검사합니다. 오류 문자열에는 비공개 요청·응답
데이터를 넣지 않습니다.

```go
var failure *types.APIError
if errors.As(err, &failure) {
	fmt.Println(failure.Code)
}
```

XML은 문서화된 네임스페이스·속성·CDATA·배열 감싸기 및 타입 합성 경로를 지원합니다.
임의 추가 속성과 위치별 배열 항목의 XML 매핑은 기존 제한을 유지합니다. XML로 표현할 수
없는 스키마에는 JSON이나 명시적 코덱을 선택하세요. 전체 import와 로컬 서버, 실행 가능한
오류 처리·정리는 [Go Todo 예제](https://github.com/jinyongp/openapi-sdkgen/tree/main/examples/golang-todo-app)에서 확인할 수 있습니다.

## 생성 오류 찾기

`--diagnostic-mode collect`로 진단 코드·위치·해결 안내를 확인하세요.
`--go-import-path`, 스키마·미디어 매핑 제약과 [원격 참조 권한](./remote-inputs.md)을 점검하세요.
`--check`는 생성과 관리 출력을 확인합니다. `go test ./...`는 별도로 실행하세요.
