# Files, Links, and streams

Use this guide when your API offers file transfers, follow-up calls, or sequential
data. Start with [basic client usage](/typescript/guide/client) for ordinary JSON calls.

These snippets need more than the two operations in Getting started. Declare each
section's inputs, outputs, and parameters in your document and regenerate the SDK
before using them. `api` is a client configured for your real API server.

| Task | Required document contract |
| --- | --- |
| Send binary data or text | `uploadTodoAttachment` with both request media types and a `todoID` path parameter |
| Upload a file | A multipart `file` property on `uploadAttachment` |
| Retrieve the created item | A `getTodo` Link on the creation response and its target operation |
| Receive events sequentially | Response `itemSchema` and a `cursor` query on `watchTodos` |
| Send events sequentially | NDJSON request `itemSchema` on `importTodos` |

For a streaming document and its consumer code, see the
[AI streaming example](/typescript/examples/ai-streaming).

## Choose a request media type

If one request body declares several media types, the generated body input is a
discriminated value. Its `contentType` field selects the representation.

For a Todo attachment operation that accepts binary data or text:

```ts
await api.$operations.uploadTodoAttachment({
  path: { todoID: "todo-1" },
  body: {
    contentType: "application/octet-stream",
    value: new Uint8Array([1, 2, 3]),
  },
});
```

This sends bytes `01 02 03` with `Content-Type: application/octet-stream` and path value `todo-1`. Its return value follows the upload operation's response schema; the request media type does not define it.

Use a content type declared by the operation. Custom media codecs are selected by
the same media type.

## Upload a multipart file

Raw file parts accept `File`, `Blob`, `ArrayBuffer`, `ArrayBufferView`, or a text
string. Strings are encoded as UTF-8. A `File` carries its filename; byte views
send only their selected byte range. Base64-encoded schema values remain strings.

For an operation whose multipart body declares a `file` property:

```ts
await api.$operations.uploadAttachment({
  body: { file: new File([new Uint8Array([0, 127, 255])], "attachment.bin") },
});
```

The server receives a multipart `file` part named `attachment.bin` containing bytes `00 7f ff`. Fetch supplies the multipart boundary. The response value depends on the operation's declared response.

An Encoding Object's `contentType` takes precedence over the part's
`contentMediaType`. Raw file parts default to `application/octet-stream`.
The same schema used in a JSON body keeps its JSON input type.

## Follow OpenAPI Links

An OpenAPI Link describes a follow-up operation using values from a response.
When a Todo creation response defines a Link named `getTodo`, the generated
[`$links`](/typescript/reference/client-api#links) helper carries the source response context into that follow-up call:

```ts
const created = await api.$operations.createTodo.raw({
  body: {
    title: "Write documentation",
  },
});

const todo = await api.$links.createTodo.getTodo(created);
```

For a creation response with ID `todo-1`, the Link can resolve that ID into the target path parameter. An example follow-up `todo` value is:

```json
{
  "id": "todo-1",
  "title": "Write documentation",
  "completed": false
}
```

Values passed to the Link call take precedence over values derived from the Link
Object.
If a required runtime expression cannot be resolved, the call fails.

## Consume streaming responses

An operation with an OpenAPI 3.2
[`itemSchema`](https://spec.openapis.org/oas/v3.2.0.html#media-type-object)
exposes [`.stream(...)`](/typescript/reference/streaming#response-streams) on its
operation, exact-route, and generated resource call surfaces. See
[OpenAPI support](/typescript/reference/capabilities#supported-openapi-versions) for
the 3.0/3.1/3.2 version split.

```ts
const stream = api.$operations.watchTodos.stream({
  query: { cursor: "0" },
});

for await (const event of stream) {
  console.log(event.todoID, event.completed);
}
```

For example, if the server sends NDJSON lines `{"todoID":"todo-1","completed":false}` and `{"todoID":"todo-1","completed":true}`, the loop prints:

```text
todo-1 false
todo-1 true
```

Each `event` is a decoded object; the loop does not wait for the complete response body before processing the first item.

`.stream()` returns an
[`OperationStream<T>`](/typescript/reference/streaming#operationstream).
It starts lazily, preserves Fetch backpressure, and owns one response body. Use `stream.response` when status,
headers, content type, or request metadata are needed after the response opens.

```ts
const metadata = await stream.response;
console.log(metadata.status, metadata.contentType);
```

For an NDJSON response with status `200`, this prints `200 application/x-ndjson`. Metadata is available after the headers arrive, before all items have been read.

Call `stream.abort()`, pass an `AbortSignal`, or use a request timeout to stop
the operation. `stream.toReadableStream()` adapts the same single-consumer
source to the Web Streams API.

For the unconsumed Fetch response body, make a separate [`.raw()`](/typescript/reference/client-api#raw) call.
Server-Sent Events preserve `data`, `event`, `id`, and `retry`; replay and
reconnect policy stays in application code.

## Send streaming request bodies

Import a file from the client into a server. The client uploads records; the
server replies with the number it read. [Download the complete example](/downloads/typescript-request-streaming-app.zip)
for a separately running HTTP server, client, and [import contract](/examples/todo-import-stream.json).
This API is separate from the Getting started Todo contract.

The client's `todos.ndjson` file contains one record per line:

```text
{"title":"Write documentation"}
{"title":"Ship the SDK"}
```

**Client:** read the file line by line and send one `POST /todos/import` request.
`readTodos` is an application function included in the example. It reads file
lines and yields parsed JSON objects; the SDK does not generate that function.
The SDK validates each object against `itemSchema` and encodes it as NDJSON.

```ts
const result = await api.$operations.importTodos({
  body: readTodos("./todos.ndjson"),
});
console.log(`Imported ${result.imported} todos`);
```

**Server:** a separate process reads and processes the request body line by line.
This excerpt is the processing portion of the example's Node HTTP server.
`request` and `response` are the server's HTTP request/response objects;
`createInterface` comes from `node:readline`.

```ts
let imported = 0;
for await (const line of createInterface({ input: request })) {
  const todo = JSON.parse(line);
  console.log("Received:", todo.title);
  imported++;
}
response.writeHead(200, { "content-type": "application/json" });
response.end(JSON.stringify({ imported }));
```

The server terminal prints the records below. This example logs records rather
than storing them; put your import persistence in that processing step.

```text
Received: Write documentation
Received: Ship the SDK
```

After the file upload ends, the server replies with HTTP `200` and JSON
`{"imported":2}`. The client's decoded `result` is `{ imported: 2 }`, so it prints:

```text
Imported 2 todos
```

The response reports server processing; it does not echo the uploaded records.
The generator creates SDK call code from the contract. It does not create the
file or events, or run the server.

Records can be sent in order without collecting the whole file into an array.
This is one request, not one request per record; network chunk boundaries need
not match record boundaries. Use a Fetch environment that supports streaming
uploads. A failed upload may already have processed earlier records, so define
retry and deduplication policy on the server.

If the sequential media type declares only `schema`, pass the complete schema
value instead. When it declares both `schema` and `itemSchema`, the generated
request body accepts either the complete value or a [`StreamSource<T>`](/typescript/reference/streaming#streaming-request-bodies).
