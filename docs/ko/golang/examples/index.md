# Go 예제

| 작업 | 예제 | 명세 |
| --- | --- | --- |
| 타입이 있는 첫 HTTP 호출 | [첫 SDK](../guide/getting-started.md) | [Todo](/examples/golang-todo.json) |
| SSE로 AI 출력 받기 | [AI 스트림](./ai-streaming.md) | [Generation API](/examples/golang-ai-stream.json) |
| 타입이 있는 HTTP 수신 | [웹훅 수신](./webhook-server.md) | [Todo](/examples/golang-todo.json) |
| 기능별 이름 있는 클라이언트 | [이름 있는 클라이언트](../guide/named-clients.md) | [주문/카탈로그](/examples/named-clients.json) |

[전체 Go Todo 애플리케이션](https://github.com/jinyongp/openapi-sdkgen/tree/main/examples/golang-todo-app)은
로컬 서버에서 raw 응답·Link·리소스·NDJSON·정적 선택·웹훅을 실행합니다.
각 예제는 자원을 정리하고 실행 시간에 제한을 둡니다. 영어·한국어의 코드는 같습니다.
