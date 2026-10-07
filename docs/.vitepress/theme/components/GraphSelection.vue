<script setup>
import { withBase } from "vitepress";
import data from "../../generated/graph-selection.json";

const props = defineProps({ locale: { type: String, default: "en" } });
const ko = props.locale === "ko";
const number = value => new Intl.NumberFormat(props.locale, { maximumFractionDigits: 2 }).format(value);
const size = bytes => `${number(bytes / 1024 ** 2)} MiB`;
const time = value => `${number(value / 1000)} ${ko ? "초" : "s"}`;
const selected = data.selected;
const date = new Intl.DateTimeFormat(props.locale, { dateStyle: "medium", timeZone: "UTC" }).format(new Date(data.measurement.measuredAt));
const labels = ko ? ["생성 범위", "호출 API", "파일", "용량", "생성 시간"] : ["Scope", "API calls", "Files", "Size", "Generation time"];
</script>

<template>
  <div class="graph-selection">
    <p v-if="ko">Microsoft Graph beta의 GitHub Actions 생성 결과입니다. 타입 검사 결과는 포함하지 않습니다.</p>
    <p v-else>GitHub Actions generation results for Microsoft Graph beta. Typechecking is excluded.</p>
    <p>{{ ko ? "측정일" : "Measured" }}: {{ date }} · {{ ko ? "버전" : "Version" }}: <code>{{ data.measurement.sourceCommit.slice(0, 7) }}</code></p>
    <div class="graph-table" role="region" :aria-label="ko ? 'Graph 선택 API 생성 결과' : 'Selected Graph API generation results'" tabindex="0">
      <table>
        <thead><tr><th v-for="label in labels" :key="label" scope="col">{{ label }}</th></tr></thead>
        <tbody>
        <tr v-if="data.full">
          <th scope="row">{{ ko ? "전체 API" : "All APIs" }}</th>
          <td>{{ number(data.full.operationEmission.count) }}</td>
          <td>{{ number(data.full.generation.artifactCount) }}</td>
          <td>{{ size(data.full.generation.artifactBytes) }}</td>
          <td>{{ time(data.full.generation.durationMillis) }}</td>
        </tr>
        <tr>
          <th scope="row">{{ ko ? "선택 API" : "Selected APIs" }}</th>
          <td>{{ number(selected.operationEmission.count) }}</td>
          <td>{{ number(selected.generation.artifactCount) }}</td>
          <td>{{ size(selected.generation.artifactBytes) }}</td>
          <td>{{ time(selected.generation.durationMillis) }}</td>
        </tr></tbody>
      </table>
    </div>
    <details>
      <summary>{{ ko ? "선택한 경로와 측정 자료" : "Selected routes and measurement data" }}</summary>
      <p><a :href="data.sourceUrl">{{ ko ? "OpenAPI 원문" : "OpenAPI document" }}</a> · <a :href="data.ciRunUrl">GitHub Actions</a> · <a :href="withBase('/compatibility-results/graph-selected-ci-results.json')">{{ ko ? "결과 JSON" : "Results JSON" }}</a></p>
      <p v-if="data.full"><a :href="withBase('/compatibility-results/graph-full-ci-results.json')">{{ ko ? "전체 생성 결과 JSON" : "Full generation results JSON" }}</a></p>
      <ul><li v-for="route in selected.generationSelection.routes" :key="route"><code>{{ route }}</code></li></ul>
    </details>
  </div>
</template>

<style scoped>
.graph-table { overflow-x: auto; }
.graph-table table { display: table; width: 100%; margin: 0; }
.graph-table th { white-space: normal; }
.graph-table td { white-space: nowrap; }
</style>
