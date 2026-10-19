# Models and validation

Generated request and response types follow the selected operation's schema. Use
`OperationInput` and `RouteInput` for requests, and `ComponentInput` / `ComponentOutput`
for named component schemas. Input/output projections respect `readOnly` and `writeOnly`.

## Optional and nullable values

An optional field may be absent; a nullable field may contain `null`. These contracts
are independent. Do not replace missing properties with `null` unless the schema permits it.
Generated validation checks exchanged data against the declared schema at runtime.
TypeScript checking is optional during generation and uses your application compiler.

## Choose a type

See [generated types](../reference/types.md) for component and operation helpers,
[client calls](./client.md) for request inputs and failures, and
[streaming](../reference/streaming.md) for validated stream items.
