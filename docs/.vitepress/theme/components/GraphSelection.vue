<script setup>
import { withBase } from "vitepress";
import data from "../../generated/graph-selection.json";

const props = defineProps({ locale: { type: String, default: "en" } });
const ko = props.locale === "ko";
const number = value => new Intl.NumberFormat(props.locale, { maximumFractionDigits: 2 }).format(value);
const size = bytes => bytes >= 1024 ** 3 ? `${number(bytes / 1024 ** 3)} GiB` : `${number(bytes / 1024 ** 2)} MiB`;
const time = value => `${number(value / 1000)} ${ko ? "초" : "s"}`;
const selected = data.selected;
const labels = ko
  ? ["생성 범위", "호출 API", "파일", "용량", "생성 시간"]
  : ["Generated APIs", "API calls", "Files", "Size", "Generation time"];
const date = new Intl.DateTimeFormat(props.locale, { dateStyle: "medium", timeZone: "UTC" }).format(new Date(data.measurement.measuredAt));
</script>

<template>
  <div class="graph-selection">
    <p>{{ ko ? "선택 SDK 마지막 측정" : "Selected SDK last measured" }}: <strong>{{ date }}</strong> · {{ ko ? "버전" : "Version" }}: <strong>{{ data.measurement.sourceCommit.slice(0, 7) }}</strong></p>
    <div class="graph-table" role="region" :aria-label="ko ? 'Graph 생성 범위 비교' : 'Graph generation scope comparison'" tabindex="0">
      <table>
        <thead><tr><th v-for="label in labels" :key="label" scope="col">{{ label }}</th></tr></thead>
        <tbody>
          <tr><th scope="row">{{ ko ? "전체 API · 원문 포함" : "All APIs · source included" }}</th><td>{{ number(data.full.operationEmission.count) }}</td><td>{{ number(data.full.generation.artifactCount) }}</td><td>{{ size(data.full.generation.artifactBytes) }}</td><td>{{ time(data.full.generation.durationMillis) }}</td></tr>
          <tr><th scope="row">{{ ko ? "선택 API" : "Selected APIs" }}</th><td>{{ number(selected.operationEmission.count) }}</td><td>{{ number(selected.generation.artifactCount) }}</td><td>{{ size(selected.generation.artifactBytes) }}</td><td>{{ time(selected.generation.durationMillis) }}</td></tr>
        </tbody>
      </table>
    </div>
    <p v-if="ko">선택 SDK는 전체 파일 타입 검사를 {{ time(selected.typecheck.durationMillis) }}에 통과했습니다. <code>loadOperations</code>로 준비한 클라이언트의 경로·쿼리·본문·응답 처리도 모의 호출로 확인했습니다.</p>
    <p v-else>The selected SDK passed typechecking of every generated file in {{ time(selected.typecheck.durationMillis) }}. Mock calls also verified path, query, body, and response handling through a client prepared with <code>loadOperations</code>.</p>
    <p v-if="ko">원문이 필요한 경우 <code>--with metadata</code>로 포함할 수 있습니다. 아래는 같은 API를 생성했을 때의 용량과 생성 시간입니다. 기본 설정에서도 API 호출과 필요한 타입은 모두 제공됩니다. Graph API 9개는 서로 연결된 스키마 {{ number(selected.generation.schemaArtifactCount) }}개를 사용합니다.</p>
    <p v-else>Include the original document with <code>--with metadata</code> when you need it. The comparison below shows size and generation time for the same selected APIs. Default generation includes all API calls and required types. The nine Graph APIs use {{ number(selected.generation.schemaArtifactCount) }} connected schemas.</p>
    <div class="graph-table" role="region" :aria-label="ko ? '원문 포함 여부에 따른 크기와 시간' : 'Size and time with optional source metadata'" tabindex="0">
      <table>
        <thead><tr><th scope="col">{{ ko ? "선택한 API" : "Selected APIs" }}</th><th scope="col">{{ ko ? "기본 · 용량 / 시간" : "Default · size / time" }}</th><th scope="col">{{ ko ? "원문 포함 · 용량 / 시간" : "Source included · size / time" }}</th></tr></thead>
        <tbody><tr v-for="item in data.comparison.cases" :key="item.name"><th scope="row">{{ item.displayName }} · {{ number(item.routes.length) }} {{ ko ? "개" : "APIs" }}</th><td>{{ size(item.default.generation.artifactBytes) }} / {{ time(item.default.generation.durationMillis) }}</td><td>{{ size(item.metadata.generation.artifactBytes) }} / {{ time(item.metadata.generation.durationMillis) }}</td></tr></tbody>
      </table>
    </div>
    <p v-if="ko">네 선택 SDK 모두 타입 검사를 통과했습니다. Graph 1개 표본은 <code>GET /users/$count</code>를 생성한 결과입니다. 전체 API 행은 기존 전체 생성 측정값입니다.</p>
    <p v-else>All four selections passed typechecking. The single Graph API sample generates <code>GET /users/$count</code>. The all-API row retains the earlier full-generation measurement.</p>
    <details>
      <summary>{{ ko ? "선택한 경로와 측정 자료" : "Selected routes and measurement data" }}</summary>
      <p><a :href="data.sourceUrl">{{ ko ? "OpenAPI 원문" : "OpenAPI document" }}</a> · <a :href="withBase('/compatibility-results/graph-selected-results.json')">{{ ko ? "선택 SDK 결과 JSON" : "Selected SDK results JSON" }}</a></p>
      <p><a :href="withBase('/compatibility-results/graph-metadata-results.json')">{{ ko ? "원문 포함 결과 JSON" : "Source-included results JSON" }}</a> · <a :href="withBase('/compatibility-results/metadata-comparison-results.json')">{{ ko ? "네 표본 비교 JSON" : "Four-sample comparison JSON" }}</a></p>
      <ul><li v-for="route in selected.generationSelection.routes" :key="route"><code>{{ route }}</code></li></ul>
      <p>{{ data.measurement.cpu }} · {{ data.measurement.os }}/{{ data.measurement.architecture }} · {{ ko ? "검증 최대 메모리" : "Verification peak RSS" }} {{ size(data.resources.peakRssBytes) }}</p>
      <p v-if="ko"><a :href="data.ci.ciRunUrl">GitHub Actions 검증</a> · 버전 <strong>{{ data.ci.measurement.sourceCommit.slice(0, 7) }}</strong>: 생성·타입 검사·모의 호출 통과. 생성 {{ time(data.ci.selected.generation.durationMillis) }}, 타입 검사 {{ time(data.ci.selected.typecheck.durationMillis) }} · <a :href="withBase('/compatibility-results/graph-selected-ci-results.json')">Actions 결과 JSON</a></p>
      <p v-else><a :href="data.ci.ciRunUrl">GitHub Actions verification</a> · Version <strong>{{ data.ci.measurement.sourceCommit.slice(0, 7) }}</strong>: generation, typechecking, and mock calls passed. Generation {{ time(data.ci.selected.generation.durationMillis) }}, typecheck {{ time(data.ci.selected.typecheck.durationMillis) }} · <a :href="withBase('/compatibility-results/graph-selected-ci-results.json')">Actions results JSON</a></p>
    </details>
  </div>
</template>

<style scoped>
.graph-table { overflow-x: auto; }
.graph-table table { display: table; width: 100%; margin: 0; }
.graph-table th { white-space: normal; }
.graph-table td { white-space: nowrap; }
.graph-table tbody tr { background: transparent; }
</style>
