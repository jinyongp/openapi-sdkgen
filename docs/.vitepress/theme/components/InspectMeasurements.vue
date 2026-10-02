<script setup>
import { withBase } from "vitepress";
import data from "../../generated/inspect-measurements.json";

const props = defineProps({ locale: { type: String, default: "en" } });
const ko = props.locale === "ko";
const number = value => new Intl.NumberFormat(props.locale, { maximumFractionDigits: 3 }).format(value);
const time = value => `${number(value / 1000)} ${ko ? "초" : "s"}`;
const memory = bytes => `${number(bytes / 1024 ** (bytes >= 1024 ** 3 ? 3 : 2))} ${bytes >= 1024 ** 3 ? "GiB" : "MiB"}`;
const names = {
  "microsoft-graph-beta": "Microsoft Graph (beta)",
  github: "GitHub", stripe: "Stripe", digitalocean: "DigitalOcean",
  "selection-fixture": ko ? "OpenAPI 3.2.1 예제" : "OpenAPI 3.2.1 example",
};
const rows = Object.keys(names).map(id => data.cases.find(item => item.id === id));
const date = new Intl.DateTimeFormat(props.locale, { dateStyle: "medium", timeZone: "UTC" }).format(new Date(data.measurement.measuredAt));
const versionUrl = `https://github.com/jinyongp/openapi-sdkgen/commit/${data.measurement.sourceCommit}`;
const sourceUrl = item => item.sourceUrl ?? `https://github.com/jinyongp/openapi-sdkgen/blob/${data.measurement.sourceCommit}/${item.input}`;
</script>

<template>
  <p>{{ ko ? "마지막 측정" : "Last measured" }}: <strong>{{ date }}</strong> · {{ ko ? "버전" : "Version" }}: <a :href="versionUrl">{{ data.measurement.sourceCommit.slice(0, 7) }}</a></p>
  <div class="inspect-table" role="region" :aria-label="ko ? 'API 목록 조회와 호출 경로 분석 시간' : 'API lookup and call analysis times'" tabindex="0">
    <table>
      <thead><tr><th scope="col">{{ ko ? "OpenAPI 문서" : "OpenAPI document" }}</th><th scope="col">{{ ko ? "API 수" : "APIs" }}</th><th scope="col">{{ ko ? "목록 조회 / 메모리" : "API lookup / memory" }}</th><th scope="col">{{ ko ? "호출 경로 분석 / 메모리" : "Call analysis / memory" }}</th></tr></thead>
      <tbody><tr v-for="item in rows" :key="item.id"><th scope="row"><a :href="sourceUrl(item)">{{ names[item.id] }}</a></th><td>{{ number(item.operationCount) }}</td><td>{{ time(item.inventory.wallMedianMillis) }} / {{ memory(item.inventory.peakRssBytes) }}</td><td>{{ time(item.typescriptAnalysis.wallMedianMillis) }} / {{ memory(item.typescriptAnalysis.peakRssBytes) }}</td></tr></tbody>
    </table>
  </div>
  <p v-if="ko"><a :href="withBase('/compatibility-results/inspect-results.json')">측정 결과 JSON</a>에서 반복별 시간·CPU 사용 시간·최대 메모리와 문서 버전을 확인할 수 있습니다.</p>
  <p v-else>The <a :href="withBase('/compatibility-results/inspect-results.json')">measurement JSON</a> includes each run's time, CPU time, peak memory, and document revision.</p>
</template>

<style scoped>
.inspect-table { overflow-x: auto; }
.inspect-table table { display: table; width: 100%; margin: 0; }
.inspect-table th { white-space: normal; }
.inspect-table td { white-space: nowrap; }
</style>
