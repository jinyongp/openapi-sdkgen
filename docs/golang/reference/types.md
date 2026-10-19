# Types and model codecs

## Presence and null

Import the generated `types` package. `Optional[T]` separates absence from a value; `Some(value)`
is present, and its zero value is absent. `Get()` returns `(T, bool)`, and `IsSet()` reports presence.
`Nullable[T]` separates null from a non-null value; use `Null[T]()` and `NonNull(value)`, then `Get()`
or `IsNull()`. A field that is both optional and nullable composes both wrappers.

## JSON wire values

`types.Value` is an immutable JSON tree. `ParseValue([]byte)` parses JSON; `ValueFrom(value)`
converts supported Go data with cycle checks. `Kind`, typed scalar accessors, `Get`, `Equal` and
JSON serialization work without opening files or executing streams. Files belong in `Binary` or native media slots.

## Generated models and codecs

The `models` package owns canonical models; root aliases preserve their identities. Operation input/output
and component input/output projections respect `readOnly` and `writeOnly`. Public codec names are generated
from each model: `Decode<Model>(ctx, value)`, `Encode<Model>(ctx, model)`, with media-aware variants where needed.
Use generated codecs for schema validation and wire conversion; `encoding/json` alone does not enforce schemas.
Allowed unknown response properties are retained. Standalone codecs and server output validate their contracts.

## Enums and composition

Scalar enum literals are named constants; structured/nullable literals use generated factories.
`<Model>Values`, `Is<Model>`, `Parse<Model>`, `Equal` expose the declared values.
Object unions expose branch constructors/accessors such as `As<Branch>() (T, bool)` and update helpers.
Discriminators select declared branches; conflicting tags and invalid combinations fail validation.
Tuple/prefix items, additional properties, recursive references and boolean schemas preserve their schema constraints.
Generated declarations are authoritative for exact names and direction projections.

## Dynamic and native media

Dynamic operations use `types.Value` or `types.NativeInput`. `MediaValue[T]` distinguishes schema values,
binary content and multipart slots through `AsValue`, `AsBinary`, `AsMultipart`.
See [streaming API](./streaming.md) for `Binary`, `SourceFactory`, codecs and ownership.
