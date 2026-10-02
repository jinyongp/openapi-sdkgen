<script setup>
import { withBase } from "vitepress";
import data from "../../generated/runtime-quality.json";

const props = defineProps({ locale: { type: String, default: "en" } });
const ko = props.locale === "ko";
const number = value => new Intl.NumberFormat(props.locale, { maximumFractionDigits: 2 }).format(value);
const seconds = value => `${number(value / 1000)} ${ko ? "초" : "s"}`;
const size = value => `${number(value / 1024 ** 2)} MiB`;
const date = new Intl.DateTimeFormat(props.locale, { dateStyle: "medium", timeZone: "UTC" }).format(new Date(data.measuredAt));
const headers = ko
  ? ["API 수", "생성 소스", "선언 파일", "생성 시간", "타입·선언 검사", "검사 최대 메모리"]
  : ["APIs", "Source", "Declarations", "Generation", "Typecheck + declarations", "Peak checking memory"];
const sse = data.sse.filter(row => row.dataBytes === 262144 && row.chunkBytes === 64);
</script>

<template>
  <div class="runtime-quality">
    <p>{{ ko ? "마지막 측정" : "Last measured" }}: <strong>{{ date }}</strong> · {{ ko ? "버전" : "Version" }}: <strong>{{ data.sourceCommit.slice(0, 7) }}</strong></p>
    <p v-if="ko">TypeScript {{ data.matrix.versions.join(" · ") }}에서 {{ data.matrix.total }}개 생성 조합의 엄격 타입 검사가 통과했습니다. 생성 소스와 선언 파일을 검사한 SDK별 크기·시간은 다음과 같습니다.</p>
    <p v-else>Strict typechecking passed for {{ data.matrix.total }} generation combinations across TypeScript {{ data.matrix.versions.join(" · ") }}. SDK source and declaration sizes and timings are shown below.</p>
    <div class="measurement-table" role="region" :aria-label="ko ? '생성 SDK 크기와 타입 검사 비용' : 'Generated SDK size and checking cost'" tabindex="0">
      <table>
        <thead><tr><th v-for="header in headers" :key="header" scope="col">{{ header }}</th></tr></thead>
        <tbody><tr v-for="row in data.delivery" :key="row.count">
          <th scope="row">{{ number(row.count) }}</th>
          <td>{{ size(row.source.bytes) }}</td><td>{{ size(row.declarations.bytes) }}</td>
          <td>{{ seconds(row.generationMS) }}</td><td>{{ seconds(row.compileMS) }}</td>
          <td>{{ size(row.compileResources.peakRSSKiB * 1024) }}</td>
        </tr></tbody>
      </table>
    </div>
    <p v-if="ko">API 수를 고정한 예제 문서에서 TypeScript {{ data.typescript }}로 전체 생성 파일을 확인한 결과입니다. 시간과 메모리는 이 측정 환경에서의 관측값입니다.</p>
    <p v-else>These example documents use fixed API counts. Every generated file was checked with TypeScript {{ data.typescript }}. Times and memory are observations from this measurement environment.</p>
    <details>
      <summary>{{ ko ? "스트리밍 성능과 측정 환경" : "Streaming performance and environment" }}</summary>
      <p v-if="ko">256 KiB SSE 이벤트를 64바이트씩 나눠 받은 실제 SDK 호출 시간입니다. 예열 후 세 번 측정한 중앙값을 비교했습니다.</p>
      <p v-else>Actual SDK calls receiving a 256 KiB SSE event in 64-byte chunks, compared using the median of three warmed measurements.</p>
      <table>
        <thead><tr><th scope="col">{{ ko ? "버전" : "Version" }}</th><th scope="col">{{ ko ? "호출 시간" : "Call time" }}</th></tr></thead>
        <tbody><tr v-for="row in sse" :key="row.kind"><th scope="row">{{ (row.kind === 'baseline' ? data.baselineCommit : data.sourceCommit).slice(0, 7) }}</th><td>{{ number(row.medianMS) }} ms</td></tr></tbody>
      </table>
      <p>{{ data.environment.cpu }} · {{ data.environment.os }} / {{ data.environment.architecture }} · Node.js {{ data.node }} · {{ size(data.environment.totalMemoryBytes) }} RAM</p>
      <p><a :href="withBase('/compatibility-results/runtime-quality-results.json')">{{ ko ? "컴파일러별 비교·원본 측정 JSON" : "Compiler comparisons and measurement JSON" }}</a></p>
    </details>
  </div>
</template>

<style scoped>
.measurement-table { overflow-x: auto; }
.measurement-table table { display: table; width: 100%; margin: 0; }
.measurement-table th { white-space: normal; }
.measurement-table td { white-space: nowrap; }
.runtime-quality tbody tr { background: transparent; }
</style>
