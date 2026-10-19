# Generate a monthly sales report

Submit a background job, watch progress and save a CSV report using a generated
Go client. A second job demonstrates cancellation. The server reads `orders.json`
and aggregates October orders by region.

Install Go 1.27+, Bash and the `openapi-sdkgen` CLI. From this directory:

```sh
bash ./setup.sh
go run ./cmd/server
```

In another terminal, from this directory:

```sh
go run .
```

The client prints progress and writes `monthly-sales.csv`. Intermediate progress
lines depend on polling timing. The result includes:

```text
Report: completed (3/3)
Saved: monthly-sales.csv
region,orders,revenueUSD
APAC,2,180.00
EMEA,1,90.00
Second report: cancelled
```

Edit `orders.json` before restarting the server to change the report. Amounts are
integer cents; September's order is excluded from the October report. The server
pauses 200 ms between records so progress and cancellation are observable.

`cmd/server/main.go` owns job state and computation. `main.go` uses the generated
SDK for all four operations in `openapi.json`. Requests return queued, running,
completed or cancelled jobs. Downloading an unfinished/cancelled report returns
409. Cancelling a completed report returns 409; repeating a cancellation is safe.
Unknown report IDs return 404. The client has a ten-second context deadline and
handles Ctrl+C; stopping polling does not cancel server work. Call `CancelReport`
explicitly to cancel the job.

Job state lives in memory and is lost on server restart.
Run both terminals from this directory. The server binds
loopback on port `18796`; override `REPORT_API_PORT` and the client's
`REPORT_API_BASE_URL` together. Set `SDKGEN_BIN` to use a particular CLI executable.
