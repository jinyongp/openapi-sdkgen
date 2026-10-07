<script setup>
import { withBase } from "vitepress";
import results from "../../generated/compatibility-results.json";

const props = defineProps({
  locale: { type: String, default: "en" },
  evidence: { type: Boolean, default: false },
});

const copy = {
  en: {
    caption: "Documents with successful SDK generation and typechecking",
    input: "Document set", result: "Generation and typechecking", options: "Options tested", default: "Default options", adjusted: "Required options applied", emitted: "Generated API calls",
    webhookFeature: "Webhooks", callbackFeature: "Callbacks", tested: "tested with",
    docs: "documents", pass: "Succeeded", server: "Use --with server", fail: "Failed",
    generatedOnly: "Generated · typecheck not passed",
    document: "Document", version: "OpenAPI", report: "Results JSON", manifest: "Input manifest",
    details: "Document results",
    receiving: "Generated receiving code", webhooks: "Webhook handlers", callbacks: "Callback handlers",
    duration: "SDK generation time", seconds: "s", environment: "Measured on",
    regression: ["Major API providers", "6 providers · OpenAPI 3.0"],
    holdout: ["Independent holdout", "20 providers · OpenAPI 3.0 / 3.1"],
    production32: ["Provider-published documents", "1 provider · declared OpenAPI 3.2.0"],
    modern: ["Feature examples", "1 real API document + 9 authored examples"],
  },
  ko: {
    caption: "SDK 생성과 타입 검사에 성공한 문서 수",
    input: "문서 모음", result: "생성·타입 검사", options: "검증한 생성 옵션", default: "기본 옵션", adjusted: "필요한 옵션 적용", emitted: "생성된 호출 API 수",
    webhookFeature: "웹훅", callbackFeature: "콜백", tested: "로 검증",
    docs: "개 문서", pass: "성공", server: "--with server 필요", fail: "실패",
    generatedOnly: "생성 성공 · 타입 검사 미통과",
    document: "문서", version: "OpenAPI", report: "결과 JSON", manifest: "입력 목록 JSON",
    details: "문서별 결과",
    receiving: "생성된 수신 코드", webhooks: "웹훅 핸들러", callbacks: "콜백 핸들러",
    duration: "SDK 생성 시간", seconds: "초", environment: "측정 환경",
    regression: ["주요 API 제공자", "6개 제공자 · OpenAPI 3.0"],
    holdout: ["독립 표본", "20개 제공자 · OpenAPI 3.0 / 3.1"],
    production32: ["제공자 공개 문서", "1개 제공자 · OpenAPI 3.2.0 선언"],
    modern: ["기능별 예제 문서", "실문서 1개 + 직접 작성한 예제 9개"],
  },
};
const labels = copy[props.locale];
const number = (value) => new Intl.NumberFormat(props.locale).format(value);
const measuredDate = (measurement) => new Intl.DateTimeFormat(props.locale, { dateStyle: "medium", timeZone: "UTC" }).format(new Date(measurement.measuredAt));
const measuredVersion = (measurement) => `${measurement.sourceCommit.slice(0, 7)}${measurement.sourceDirty ? "+" : ""}`;
const documentCount = (count) => number(count) + (props.locale === "ko" ? "" : " ") + labels.docs;
const successCount = (count, total) => props.locale === "ko"
  ? `${number(total)}개 중 ${number(count)}개 성공`
  : `${number(count)} of ${number(total)} succeeded`;
const serverDocuments = (corpus) => corpus.results.filter((document) =>
  !document.defaultSuccess && document.adjustedSuccess && document.serverGenerated);
const serverDescription = (corpus) => {
  const documents = serverDocuments(corpus);
  const features = [
    documents.some((document) => document.receiving.includes("document.webhooks")) ? labels.webhookFeature : null,
    documents.some((document) => document.receiving.includes("operation.callbacks")) ? labels.callbackFeature : null,
  ].filter(Boolean).join(props.locale === "ko" ? "·" : " and ");
  return props.locale === "ko"
    ? `${features} 포함 문서 ${number(documents.length)}개:`
    : `${number(documents.length)} documents with ${features}:`;
};
const status = (document) => document.defaultSuccess ? labels.pass : document.clientGenerated ? labels.generatedOnly
  : document.serverGenerated ? labels.server : labels.fail;
const adjustedStatus = (document) => document.adjustedSuccess ? labels.pass
  : document.clientGenerated || document.serverGenerated ? labels.generatedOnly : labels.fail;
const receiving = (document) => document.receiving.map((feature) =>
  feature === "document.webhooks" ? labels.webhooks : labels.callbacks).join(", ") || "—";
const duration = (milliseconds) => {
  if (milliseconds === null) return "—";
  const seconds = milliseconds >= 1000;
  return new Intl.NumberFormat(props.locale, { maximumFractionDigits: seconds || milliseconds < 1 ? 2 : 0 }).format(
    seconds ? milliseconds / 1000 : milliseconds,
  ) + ` ${seconds ? labels.seconds : "ms"}`;
};
const environmentKey = (measurement) => JSON.stringify([
  measurement?.provider, measurement?.runner, measurement?.runUrl, measurement?.samples,
]);
const measurement = results.every((corpus) => corpus.measurement && environmentKey(corpus.measurement) === environmentKey(results[0].measurement))
  ? results[0].measurement : null;
</script>

<template>
  <div class="compatibility-results">
    <div v-if="!evidence" class="results-table-scroll" role="region" :aria-label="labels.caption" tabindex="0">
    <table class="results-summary">
      <caption>{{ labels.caption }}</caption>
      <colgroup>
        <col class="results-name-column" />
        <col />
        <col class="results-options-column" />
        <col span="2" />
      </colgroup>
      <thead>
        <tr>
          <th scope="col">{{ labels.input }}</th>
          <th scope="col">{{ labels.result }}</th>
          <th scope="col">{{ labels.options }}</th>
          <th scope="col">{{ labels.emitted }}</th>
          <th scope="col">{{ labels.duration }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="corpus in results" :key="corpus.id">
          <th scope="row">
            {{ labels[corpus.id][0] }}
            <span>{{ documentCount(corpus.documents) }} · {{ labels[corpus.id][1] }}</span>
          </th>
          <td>{{ successCount(corpus.adjustedSuccess, corpus.documents) }}</td>
          <td class="results-options">
            <template v-if="serverDocuments(corpus).length">
              <span class="results-options-line">{{ serverDescription(corpus) }}</span>
              <span class="results-options-line"><template v-if="locale !== 'ko'">{{ labels.tested }} </template><code>--with server</code><template v-if="locale === 'ko'">{{ labels.tested }}</template></span>
            </template>
            <template v-else>{{ labels.default }}</template>
          </td>
          <td>{{ number(corpus.generatedOperations) }}</td>
          <td>{{ duration(corpus.generationDurationMillis) }}</td>
        </tr>
      </tbody>
    </table>
    </div>
    <p v-if="!evidence && measurement" class="results-environment">
      {{ locale === "ko" ? "마지막 측정" : "Last measured" }}: {{ measuredDate(measurement) }} ·
      {{ locale === "ko" ? "버전" : "Version" }}: <code>{{ measuredVersion(measurement) }}</code><br />
      {{ labels.environment }}: <a :href="measurement.runUrl">GitHub Actions</a> · {{ measurement.runner }} ·
      {{ locale === "ko" ? `문서별 ${measurement.samples}회 측정` : `${measurement.samples} measurement per document` }}
    </p>
    <template v-if="evidence">
      <details v-for="corpus in results" :key="corpus.id" :open="corpus.id === 'regression'" class="results-evidence">
        <summary>{{ labels[corpus.id][0] }} — {{ labels.details }}</summary>
        <p v-if="corpus.measurement?.sourceCommit" class="results-environment">
          {{ locale === "ko" ? "마지막 측정" : "Last measured" }}: {{ measuredDate(corpus.measurement) }} ·
          {{ locale === "ko" ? "버전" : "Version" }}: <code>{{ measuredVersion(corpus.measurement) }}</code>
        </p>
        <p class="results-downloads">
          <a :href="withBase(`/compatibility-results/${corpus.id}-results.json`)" download>{{ labels.report }}</a>
          <a :href="withBase(`/compatibility-results/${corpus.id}.json`)" download>{{ labels.manifest }}</a>
        </p>
        <table>
          <thead>
            <tr>
              <th scope="col">{{ labels.document }}</th>
              <th scope="col">{{ labels.version }}</th>
              <th scope="col">{{ labels.default }}</th>
              <th scope="col">{{ labels.adjusted }}</th>
              <th scope="col">{{ labels.emitted }}</th>
              <th scope="col">{{ labels.receiving }}</th>
              <th scope="col">{{ labels.duration }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="document in corpus.results" :key="document.id">
              <th scope="row"><a :href="withBase(document.sourceUrl)">{{ document.name }}</a></th>
              <td>{{ document.version }}</td>
              <td>{{ status(document) }}</td>
              <td>{{ adjustedStatus(document) }}</td>
              <td>{{ document.generatedOperations === null ? "—" : number(document.generatedOperations) }}</td>
              <td>{{ receiving(document) }}</td>
              <td>{{ duration(document.generationDurationMillis) }}</td>
            </tr>
          </tbody>
        </table>
      </details>
    </template>
  </div>
</template>

<style scoped>
.compatibility-results { margin: 24px 0; }
table { font-size: 14px; font-variant-numeric: tabular-nums; }
caption { text-align: left; color: var(--vp-c-text-2); padding-bottom: 12px; }
.results-table-scroll { overflow-x: auto; overscroll-behavior-x: contain; }
.results-table-scroll:focus-visible { outline: 2px solid var(--vp-c-brand-1); outline-offset: 2px; }
.results-summary { display: table; width: 100%; min-width: 48rem; table-layout: auto; }
.results-summary .results-name-column { width: 30%; }
.results-summary .results-options-column { width: 18rem; }
.results-summary :is(th, td) { min-width: 0; }
.results-summary th { white-space: normal; word-break: keep-all; }
.results-summary tbody th { text-align: left; }
.results-summary tbody th span { display: block; margin-top: 4px; font-size: 12px; font-weight: 400; color: var(--vp-c-text-2); }
.results-summary td { text-align: right; white-space: normal; word-break: keep-all; }
.results-summary .results-options { text-align: left; }
.results-options-line { display: block; white-space: nowrap; }
.results-evidence { border-bottom: 1px solid var(--vp-c-divider); padding: 16px 0; }
.results-evidence summary { cursor: pointer; font-weight: 600; }
.results-evidence summary:focus-visible { outline: 2px solid var(--vp-c-brand-1); outline-offset: 4px; }
.results-evidence :is(th, td) { white-space: normal; word-break: keep-all; }
.results-downloads { display: flex; flex-wrap: wrap; gap: 8px 24px; }
.results-environment { font-size: 13px; color: var(--vp-c-text-2); }
@media (max-width: 639px) {
  .results-summary :is(th, td) { padding: 8px; }
}
</style>
