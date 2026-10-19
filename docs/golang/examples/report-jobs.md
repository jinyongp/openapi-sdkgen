# Generate a background report

Request a monthly sales report, watch its progress and download the CSV when it
finishes. [Download the example](/downloads/golang-report-jobs-app.zip) for a
separate Go HTTP server, generated client, order data and run instructions.

The server aggregates `orders.json` by region. The client sends the month and
receives a job ID; it does not compute the report itself.

## Submit and wait {#submit}

The following code runs inside a function returning `error`. Use a client pointed
at `http://127.0.0.1:18796` and a `ctx` with a deadline. The full client uses a
ticker to poll every 100 ms and stops when the context is cancelled.

```go
job, err := client.Operations.CreateReport(api.CreateReportInput{
	Body: api.CreateReportBody{Month: "2026-10"},
}).Do(ctx)
if err != nil {
	return err
}
fmt.Println(job.Status)
```

```text
queued
```

Each poll returns the current status and processed order count:

```go
progress, err := client.Operations.GetReport(api.GetReportInput{
	Path: api.GetReportPath{ReportID: job.ID},
}).Do(ctx)
if err != nil {
	return err
}
fmt.Printf("%s: %g/%g orders\n", progress.Status, progress.Processed, progress.Total)
```

When the job finishes, this prints:

```text
completed: 3/3 orders
```

## Save the result {#download}

Download only after the polled status is `completed`:

```go
file, err := client.Operations.DownloadReport(api.DownloadReportInput{
	Path: api.DownloadReportPath{ReportID: job.ID},
}).Do(ctx)
if err != nil {
	return err
}
if err := os.WriteFile("monthly-sales.csv", file, 0o600); err != nil {
	return err
}
```

The file contains October's totals. The September order is excluded:

```text
region,orders,revenueUSD
APAC,2,180.00
EMEA,1,90.00
```

## Cancel a job {#cancel}

The full client submits a second job and cancels it before completion. Given that
job as `second`:

```go
cancelled, err := client.Operations.CancelReport(api.CancelReportInput{
	Path: api.CancelReportPath{ReportID: second.ID},
}).Do(ctx)
if err != nil {
	return err
}
fmt.Println(cancelled.Status)
```

```text
cancelled
```

Stopping polling or cancelling the HTTP context does not cancel the background
job. Call `CancelReport` explicitly. An unfinished or cancelled report has no
download (HTTP 409); a completed job cannot be cancelled (HTTP 409). Repeating a
cancellation is safe. Unknown job IDs return HTTP 404.

The local server keeps jobs in memory and pauses between records so progress can
be observed. Restarting it clears job state. Edit `orders.json` before restarting
to compute different reports.
