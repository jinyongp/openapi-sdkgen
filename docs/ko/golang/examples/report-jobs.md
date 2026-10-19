# 백그라운드 보고서 만들기

월별 매출 보고서를 요청하고 진행 상황을 조회한 뒤 CSV를 내려받습니다.
[예제 ZIP](/downloads/golang-report-jobs-app.zip)에 별도 Go HTTP 서버,
생성된 클라이언트, 주문 데이터와 실행 방법이 들어 있습니다.

서버가 `orders.json`을 지역별로 집계합니다. 클라이언트는 조회할 월을 보내고
작업 ID를 받아 진행 상황과 결과를 요청합니다.

## 작업 제출과 완료 대기 {#submit}

다음 코드는 `error`를 반환하는 함수 안에서 실행합니다.
클라이언트 주소는 `http://127.0.0.1:18796`으로 지정하고 `ctx`에 제한 시간을 설정합니다.
전체 실행 코드에서는 타이커로 100ms마다 진행 상황을 조회하고 컨텍스트가 취소되면 중단합니다.

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

진행 상황을 조회하면 현재 상태와 처리한 주문 수를 받습니다.

```go
progress, err := client.Operations.GetReport(api.GetReportInput{
	Path: api.GetReportPath{ReportID: job.ID},
}).Do(ctx)
if err != nil {
	return err
}
fmt.Printf("%s: %g/%g orders\n", progress.Status, progress.Processed, progress.Total)
```

작업이 완료되면 다음과 같이 출력됩니다.

```text
completed: 3/3 orders
```

## 결과 파일 저장 {#download}

조회한 상태가 `completed`일 때 파일을 내려받습니다.

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

파일에는 10월의 지역별 합계가 들어 있습니다. 9월 주문은 집계에서 빠집니다.

```text
region,orders,revenueUSD
APAC,2,180.00
EMEA,1,90.00
```

## 작업 취소 {#cancel}

전체 실행 코드에서는 두 번째 작업을 제출하고 완료 전에 취소합니다.
이 작업을 `second`로 받았을 때 호출하는 코드입니다.

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

진행 상황 조회를 멈추거나 HTTP 컨텍스트를 취소해도 백그라운드 작업은 계속됩니다.
작업을 멈추려면 `CancelReport`를 호출하세요. 아직 완료되지 않았거나 취소된 작업의 파일을
요청하면 HTTP 409를 반환합니다. 완료된 작업을 취소해도 HTTP 409를 반환합니다.
취소는 반복해서 요청해도 괜찮습니다. 존재하지 않는 작업 ID에는 HTTP 404를 반환합니다.

로컬 서버는 작업을 메모리에 보관하고, 진행 상황을 확인할 수 있도록 주문 처리 사이에
잠시 대기합니다. 서버를 다시 시작하면 작업 기록이 사라집니다.
다른 보고서를 만들려면 `orders.json`을 수정한 뒤 서버를 다시 시작하세요.
