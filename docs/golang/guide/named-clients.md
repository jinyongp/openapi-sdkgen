# Named clients

Use the [named-client document](/examples/named-clients.json), saved as `openapi.yaml`.
Each named client has an independent import package and the selected operation API.

```toml
source = "./openapi.yaml"
target = "golang"
output = "./internal/api"

[golang]
import_path = "example.com/consumer/internal/api"

[clients.orders.selection]
operations = ["listOrders", "createOrder"]

[clients.catalog.selection]
routes = ["GET /products/{id}"]
```

```sh
openapi-sdkgen generate --config ./openapi-sdkgen.toml
```

Import `example.com/consumer/internal/api/clients/orders` or
`example.com/consumer/internal/api/clients/catalog` and use `NewClient` with that package's
`ClientOptions`. Calls use `context.Context` like the root client. The application owns dependencies.

Client names start with a lowercase letter and use lowercase ASCII letters, digits or hyphens,
up to 64 characters. Reserved Windows device names are rejected. Selections must be nonempty;
unknown clients' fields, unknown operations and hidden APIs fail generation.

## Root selection

| Configuration | Root SDK |
| --- | --- |
| No root selection, named clients exist | Union of named selections |
| Explicit `[selection]` | Root selection only |
| No selections or named clients | Full document |

CLI `--operation` and `--route` override root selection. Generation includes the union of root and
named selections plus required Link dependencies. Link-only targets are internal. Named clients
have independent URLs, credentials and request state; common source packages remain shared.
Go imports are static. There is no runtime dynamic-import or JavaScript chunk configuration.
See [selection cost](./selection-cost.md) and [generation modes](./generate.md).
