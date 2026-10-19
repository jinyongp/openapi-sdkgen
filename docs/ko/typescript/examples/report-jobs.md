# 백그라운드 보고서 만들기

월별 매출 보고서를 요청하고 진행 상황을 조회한 뒤 CSV를 내려받습니다.
[예제 ZIP](/downloads/typescript-report-jobs-app.zip)에 별도 HTTP 서버,
생성된 클라이언트, 주문 데이터와 실행 방법이 들어 있습니다.

서버가 `orders.json`을 지역별로 집계합니다. 클라이언트는 조회할 월을 보내고
작업 ID를 받아 진행 상황과 결과를 요청합니다.

## 작업 제출과 완료 대기 {#submit}

예제 서버를 실행하고 `http://127.0.0.1:18795`로 클라이언트를 준비합니다.
실행 예제의 `signal`에는 취소 신호와 10초 제한 시간이 설정되어 있습니다.

```ts
import { setTimeout as sleep } from "node:timers/promises";

let job = await api.$operations.createReport(
  { body: { month: "2026-10" } },
  { signal },
);
while (job.status === "queued" || job.status === "running") {
  await sleep(100, undefined, { signal });
  job = await api.$operations.getReport(
    { path: { reportID: job.id } },
    { signal },
  );
}
if (job.status !== "completed")
  throw new Error(`Report ended as ${job.status}`);
console.log(`Completed: ${job.processed}/${job.total} orders`);
```

```text
Completed: 3/3 orders
```

## 결과 파일 저장 {#download}

다운로드 호출은 바이트 스트림을 반환합니다. 로컬 파일로 연결해 저장합니다.

```ts
import { createWriteStream } from "node:fs";
import { pipeline } from "node:stream/promises";

const file = await api.$operations.downloadReport(
  { path: { reportID: job.id } },
  { signal },
);
await pipeline(file, createWriteStream("monthly-sales.csv"), { signal });
```

파일에는 10월의 지역별 합계가 들어 있습니다. 9월 주문은 집계에서 빠집니다.

```text
region,orders,revenueUSD
APAC,2,180.00
EMEA,1,90.00
```

## 작업 취소 {#cancel}

두 번째 작업을 제출하고 완료 전에 취소합니다.

```ts
const second = await api.$operations.createReport({
  body: { month: "2026-10" },
});
const cancelled = await api.$operations.cancelReport({
  path: { reportID: second.id },
});
console.log(cancelled.status);
```

```text
cancelled
```

진행 상황 조회를 멈추거나 HTTP 요청을 중단해도 백그라운드 작업은 계속됩니다.
작업을 멈추려면 취소 API를 호출하세요. 아직 완료되지 않았거나 취소된 작업의 파일을
요청하면 HTTP 409를 반환합니다. 완료된 작업을 취소해도 HTTP 409를 반환합니다.
취소는 반복해서 요청해도 괜찮습니다. 존재하지 않는 작업 ID에는 HTTP 404를 반환합니다.

로컬 서버는 작업을 메모리에 보관하고, 진행 상황을 확인할 수 있도록 주문 처리 사이에
잠시 대기합니다. 서버를 다시 시작하면 작업 기록이 사라집니다.
다른 보고서를 만들려면 `orders.json`을 수정한 뒤 서버를 다시 시작하세요.
