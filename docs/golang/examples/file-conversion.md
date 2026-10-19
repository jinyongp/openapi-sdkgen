# Convert and download a file

Upload a JSON array and save the NDJSON produced by a separate server.
[Download the example](/downloads/golang-file-conversion-app.zip) for the server, client and input file.
Follow its README to run the server and client in separate terminals.

## Send and save the file {#convert}

`input` contains bytes read with `os.ReadFile`. This snippet runs in a function returning `error`.
Point the client at `http://127.0.0.1:18798`.

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

The saved `contacts.ndjson` contains:

```text
{"name":"Ada Lovelace","email":"ada@example.test"}
{"name":"Grace Hopper","email":"grace@example.test"}
```

The server writes each uploaded contact's `name` and `email` on a separate line,
omitting other fields. Edit the input and run the client again to change the result.
An empty array produces an empty file. Invalid JSON or missing names/emails return
HTTP 400; inputs above 64 KiB return HTTP 413. This example buffers small files in memory.
