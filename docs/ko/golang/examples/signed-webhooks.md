# 서명된 웹훅 받기

별도 발신자가 주문 이벤트를 서명해 보내고, 수신 서버는 서명을 검증한 뒤
생성된 웹훅 처리기로 데이터를 검사하고 주문을 기록합니다.
[예제 ZIP](/downloads/golang-signed-webhook-app.zip)에 두 프로그램과 실행 방법이 들어 있습니다.

## 서명해서 보내기 {#send}

예제의 발신자는 원문 바이트 `body`, 전송 ID `id`, Unix 초 단위 `timestamp`를 준비합니다.
두 터미널에 같은 `WEBHOOK_SECRET`을 설정하세요.
발신자는 `internal/signature` 패키지를 사용합니다. `request`는 JSON 본문과 `Content-Type`을 설정한 POST 요청입니다.

```go
signed := signature.Sign(secret, id, timestamp, body)
request.Header.Set("x-webhook-id", id)
request.Header.Set("x-webhook-timestamp", timestamp)
request.Header.Set("x-webhook-signature", signed)
response, err := http.DefaultClient.Do(request)
if err != nil {
	return err
}
defer response.Body.Close()
fmt.Println(response.StatusCode)
```

새로 실행한 수신 서버를 대상으로 전체 예제를 실행하면 다음 결과를 확인할 수 있습니다.

```text
First delivery: 204
Duplicate delivery: 204
Tampered delivery: 401
Processed orders: 1
Order: order-1001
```

마지막 두 줄은 생성된 SDK로 수신 서버의 `GET /fulfillments`를 조회한 결과입니다.
같은 전송을 두 번 보내도 주문은 한 번 기록됩니다. 발신자를 다시 실행하면 새 전송 ID가 생성됩니다.

## 검증과 중복 처리 {#verify}

이 예제는 `id.timestamp.rawBody`의 HMAC-SHA256 값을 `v1=<소문자 16진수>`로 전달합니다.
서버는 원문 바이트 그대로 서명을 비교하고, 전송 시각이 현재에서 앞뒤 5분 이내인지 확인합니다.
검증이 끝난 뒤 생성된 처리기가 JSON 구조를 검사합니다.

서명 오류나 만료된 시각은 401, 올바르게 서명했지만 명세에 맞지 않는 본문은 400을 반환합니다.
같은 ID와 같은 주문 데이터는 재처리하지 않고 204를 반환합니다.
같은 ID에 다른 주문 데이터를 보내면 409를 반환합니다. 64 KiB를 넘는 본문은 413을 반환합니다.

중복 검사와 주문 기록은 함께 처리하며, 기록은 서버 메모리에 보관합니다.
서버를 다시 시작하면 이력이 사라집니다. 실제 공급자를 연결할 때는 해당 공급자의 서명 규칙을 사용하세요.
