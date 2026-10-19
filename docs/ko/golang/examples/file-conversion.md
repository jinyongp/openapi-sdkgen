# 파일 변환과 다운로드

JSON 배열 파일을 업로드하고, 서버가 변환한 NDJSON 파일을 저장합니다.
[예제 ZIP](/downloads/golang-file-conversion-app.zip)에 서버, 클라이언트와 입력 파일이 들어 있습니다.
README에 따라 두 터미널에서 서버와 클라이언트를 실행하세요.

## 파일 보내고 저장하기 {#convert}

`input`은 `os.ReadFile`로 읽은 파일 바이트이며, 다음 코드는 `error`를 반환하는 함수 안에서 실행합니다.
예제 서버 주소는 `http://127.0.0.1:18798`입니다.

```go
file, err := client.Operations.ConvertContacts(api.ConvertContactsInput{
	Body: types.BinaryFromBytes(input),
}).Do(ctx)
if err != nil {
	return err
}
if err := os.WriteFile("contacts.ndjson", file, 0o600); err != nil {
	return err
}
```

저장된 `contacts.ndjson`의 내용입니다.

```text
{"name":"Ada Lovelace","email":"ada@example.test"}
{"name":"Grace Hopper","email":"grace@example.test"}
```

서버는 업로드한 각 항목의 `name`과 `email`을 한 줄씩 기록합니다.
다른 필드는 제외합니다. 입력 파일을 수정하고 클라이언트를 다시 실행하면 결과도 달라집니다.
빈 배열은 빈 파일로 변환합니다. 잘못된 JSON이나 이름·이메일이 없는 항목은 HTTP 400,
64 KiB를 넘는 입력은 HTTP 413을 반환합니다. 이 예제는 작은 파일을 메모리에서 변환합니다.
