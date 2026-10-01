<script setup>
import { withBase } from "vitepress";
import results from "../../generated/compatibility-results.json";

const props = defineProps({
  locale: { type: String, default: "en" },
  evidence: { type: Boolean, default: false },
});

const copy = {
  en: {
    caption: "Generation and strict TypeScript verification by input set",
    input: "Input set", default: "Default client", adjusted: "Required add-on", emitted: "Emitted operations",
    docs: "documents", pass: "Pass", server: "Needs server", fail: "Failed",
    document: "Document", version: "OpenAPI", report: "Results JSON", manifest: "Input manifest",
    digest: "Results SHA-256", details: "Document results and original JSON",
    holdout: ["Independent holdout", "20 providers · OpenAPI 3.0 / 3.1"],
    production32: ["Provider-published documents", "1 provider · declared OpenAPI 3.2.0"],
    modern: ["Feature and boundary checks", "1 real document + 9 authored fixtures"],
  },
  ko: {
    caption: "검증 문서별 SDK 생성·엄격한 TypeScript 타입 검사 결과",
    input: "검증 문서 모음", default: "기본 클라이언트", adjusted: "필요한 기능 포함", emitted: "생성된 API 작업",
    docs: "개 문서", pass: "통과", server: "서버 기능 필요", fail: "실패",
    document: "문서", version: "OpenAPI", report: "결과 JSON", manifest: "입력 목록 JSON",
    digest: "결과 SHA-256", details: "문서별 결과와 원본 JSON",
    holdout: ["독립 표본", "20개 제공자 · OpenAPI 3.0 / 3.1"],
    production32: ["제공자 공개 문서", "1개 제공자 · OpenAPI 3.2.0 선언"],
    modern: ["기능·지원 경계 검증", "실문서 1개 + 작성한 검증 문서 9개"],
  },
};
const labels = copy[props.locale];
const number = (value) => new Intl.NumberFormat(props.locale).format(value);
const documentCount = (count) => number(count) + (props.locale === "ko" ? "" : " ") + labels.docs;
const status = (document) => document.defaultSuccess ? labels.pass : document.adjustedSuccess ? labels.server : labels.fail;
</script>

<template>
  <div class="compatibility-results">
    <div v-if="!evidence" class="results-table-scroll" role="region" :aria-label="labels.caption" tabindex="0">
    <table class="results-summary">
      <caption>{{ labels.caption }}</caption>
      <colgroup>
        <col class="results-name-column" />
        <col span="3" />
      </colgroup>
      <thead>
        <tr>
          <th scope="col">{{ labels.input }}</th>
          <th scope="col">{{ labels.default }}</th>
          <th scope="col">{{ labels.adjusted }}</th>
          <th scope="col">{{ labels.emitted }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="corpus in results" :key="corpus.id">
          <th scope="row">
            {{ labels[corpus.id][0] }}
            <span>{{ documentCount(corpus.documents) }} · {{ labels[corpus.id][1] }}</span>
          </th>
          <td>{{ corpus.defaultSuccess }} / {{ corpus.documents }}</td>
          <td>{{ corpus.adjustedSuccess }} / {{ corpus.documents }}</td>
          <td>{{ number(corpus.emitted) }}</td>
        </tr>
      </tbody>
    </table>
    </div>
    <template v-else>
      <details v-for="corpus in results" :key="corpus.id" class="results-evidence">
        <summary>{{ labels[corpus.id][0] }} — {{ labels.details }}</summary>
        <p class="results-downloads">
          <a :href="withBase(`/compatibility-results/${corpus.id}-results.json`)" download>{{ labels.report }}</a>
          <a :href="withBase(`/compatibility-results/${corpus.id}.json`)" download>{{ labels.manifest }}</a>
        </p>
        <p class="results-digest">{{ labels.digest }}<code>{{ corpus.reportSha256 }}</code></p>
        <table>
          <thead>
            <tr>
              <th scope="col">{{ labels.document }}</th>
              <th scope="col">{{ labels.version }}</th>
              <th scope="col">{{ labels.default }}</th>
              <th scope="col">{{ labels.adjusted }}</th>
              <th scope="col">{{ labels.emitted }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="document in corpus.results" :key="document.id">
              <th scope="row">{{ document.id }}</th>
              <td>{{ document.version }}</td>
              <td>{{ status(document) }}</td>
              <td>{{ document.adjustedSuccess ? labels.pass : labels.fail }}</td>
              <td>{{ number(document.emitted) }}</td>
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
.results-summary { display: table; width: 100%; min-width: 36rem; table-layout: fixed; }
.results-summary .results-name-column { width: 46%; }
.results-summary :is(th, td) { min-width: 0; }
.results-summary th { white-space: normal; word-break: keep-all; }
.results-summary tbody th { text-align: left; }
.results-summary tbody th span { display: block; margin-top: 4px; font-size: 12px; font-weight: 400; color: var(--vp-c-text-2); }
.results-summary td { text-align: right; }
.results-evidence { border-bottom: 1px solid var(--vp-c-divider); padding: 16px 0; }
.results-evidence summary { cursor: pointer; font-weight: 600; }
.results-evidence summary:focus-visible { outline: 2px solid var(--vp-c-brand-1); outline-offset: 4px; }
.results-downloads { display: flex; flex-wrap: wrap; gap: 8px 24px; }
.results-digest { font-size: 12px; color: var(--vp-c-text-2); }
.results-digest code { display: block; margin-top: 4px; overflow-wrap: anywhere; white-space: normal; }
@media (max-width: 639px) {
  .results-summary { min-width: 32rem; }
  .results-summary :is(th, td) { padding: 8px; }
}
</style>
