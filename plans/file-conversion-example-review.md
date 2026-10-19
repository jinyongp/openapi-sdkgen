# File conversion applications

Both SDK targets upload a real contacts JSON file to a separate HTTP server and
save the returned NDJSON. The server emits each record's name/email; other fields
are omitted. Input is limited to 64 KiB, buffered, validated before responding and
converted on demand. Empty input arrays produce empty files. Invalid files return
400 and oversized uploads return 413. Both use standard platform libraries.

Review covers binary request/response types, actual saved data, UTF-8/newline
escaping, empty input, malformed/invalid records, size limits, separate server
processes, README setup, paired docs, navigation and ZIP completeness. The Go
server explicitly rejects JSON null as an array. The TypeScript build caught an
error-handler callback returning a ServerResponse despite its declared void return;
the callback now has a statement body and the archive was rebuilt and rechecked.

Passed: `devtools run build:docs`, `devtools run verify:docs`,
`devtools run fmt-check:dev`, and `git diff --check`. The archive verification now
runs twelve examples. Shared HTTP checks exercise both converters with Korean
text, escaped newlines, empty arrays, invalid records and oversized bodies; saved
files are parsed and compared with independently specified expected contacts.
No external service or runtime library was added. Tests ran in WSL.
