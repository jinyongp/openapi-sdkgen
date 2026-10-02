# Runtime quality regression inputs

These fixtures reproduce the runtime audit performed on 2026-10-02 at
`b37f9fd982b4b0e08e65fafb0bbb6df52b28d75f`. They use synthetic APIs and injected
responses; provider credentials and network API calls are unnecessary.

`runtime-quality.cases.json` records independent request values and expected
response bodies. The three OpenAPI inputs preserve the applicable version rules:
3.0 binary and nullable, 3.1 schema composition and evaluation, and 3.2 XML nodes
and SSE. Runtime tests should activate each regression with its corresponding
fix, and compare actual wire bodies and decoded values with these expectations.

Coverage includes XML composition, text/CDATA, namespaces, quoted attributes and
character references; required nullable content parameters; binary responses;
pattern/additional properties; conditional and nested evaluation; large numeric
multiples; and SSE fragmentation. Each fix also includes invalid-input controls.

Reference semantics: [XML 1.0](https://www.w3.org/TR/xml/),
[OpenAPI 3.2 XML](https://spec.openapis.org/oas/v3.2.0.html#xml-object), and
[JSON Schema 2020-12](https://json-schema.org/draft/2020-12/json-schema-core).
