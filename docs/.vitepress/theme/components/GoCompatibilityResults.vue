<script setup>
import { withBase } from "vitepress";
import results from "../../generated/golang-compatibility.json";
const props = defineProps({ locale: { type: String, default: "en" } });
const labels = props.locale === "ko" ? {
  missing: "공개된 GitHub Actions 기반 Go 제공자 측정 결과가 아직 없습니다.",
  source: "측정 커밋", run: "측정 실행", report: "결과 JSON", document: "명세", generation: "생성된 API 수", receiving: "수신 핸들러 수", compilation: "Go 컴파일", prepared: "전처리 입력", pass: "성공", fail: "실패", blocked: "생성 차단",
} : {
  missing: "No GitHub Actions provider measurements for Go have been published yet.",
  source: "Measured commit", run: "Measurement run", report: "Results JSON", document: "Document", generation: "Generated API calls", receiving: "Receiving handlers", compilation: "Go compilation", prepared: "Prepared input", pass: "Passed", fail: "Failed", blocked: "Generation blocked",
};
</script>
<template>
  <p v-if="!results">{{ labels.missing }}</p>
  <template v-else>
    <p>{{ labels.source }}: <code>{{ results.sourceCommit.slice(0, 7) }}</code> · <a :href="results.runUrl">{{ labels.run }}</a> · <a :href="withBase('/compatibility-results/golang-results.json')">{{ labels.report }}</a></p>
    <table>
      <thead><tr><th>{{ labels.document }}</th><th>{{ labels.generation }}</th><th>{{ labels.receiving }}</th><th>{{ labels.compilation }}</th></tr></thead>
      <tbody><tr v-for="item in results.documents" :key="`${item.corpus}/${item.id}`">
        <td>{{ item.id }}<span v-if="item.preparation"> · {{ labels.prepared }}</span></td>
        <td>{{ item.routes ?? '—' }}</td><td>{{ item.handlers ?? '—' }}</td><td>{{ labels[item.compilation] }}<details v-if="item.failure"><summary>{{ labels.fail }}</summary><pre>{{ item.failure }}</pre></details></td>
      </tr></tbody>
    </table>
  </template>
</template>
