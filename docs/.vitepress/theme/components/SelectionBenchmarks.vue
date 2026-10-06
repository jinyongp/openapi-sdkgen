<script setup lang="ts">
import { computed, ref, type ComputedRef, type Ref } from "vue";
import { withBase } from "vitepress";
import snapshot from "../../../public/benchmarks/selection-results.json";
import { benchmarkBars, type BenchmarkBar, type BenchmarkDatum } from "../benchmark-bars";

interface BenchmarkProps {
  readonly locale?: string;
  readonly group: "generation" | "bundle";
}
const props: Readonly<BenchmarkProps> = defineProps<BenchmarkProps>();
const ko: boolean = props.locale === "ko";
type Generation = (typeof snapshot.generation)[number];
type Bundle = (typeof snapshot.bundles)[number];
interface Metric {
  readonly id: string;
  readonly label: string;
  readonly unit: string;
}
const metrics: readonly Metric[] =
  props.group === "generation"
    ? [
        { id: "seconds", label: ko ? "생성 시간" : "Generation time", unit: "s" },
        { id: "bytes", label: ko ? "생성 소스" : "Generated source", unit: "MiB" },
        { id: "memory", label: ko ? "최대 메모리" : "Peak memory", unit: "MiB" },
      ]
    : [
        { id: "ready", label: ko ? "호출 준비까지 로드한 JS" : "JS loaded to ready", unit: "KiB" },
        { id: "deployed", label: ko ? "전체 배포 JS" : "All deployed JS", unit: "KiB" },
        { id: "brotli", label: ko ? "Brotli 압축 크기" : "Brotli-compressed size", unit: "KiB" },
        { id: "readyMS", label: ko ? "준비 시간" : "Time to ready", unit: "ms" },
      ];
const selected: Ref<string> = ref(metrics[0]!.id);
const metric: ComputedRef<Metric> = computed(
  (): Metric => metrics.find((item: Metric): boolean => item.id === selected.value) ?? metrics[0]!,
);
const labels: Readonly<Record<string, string>> = ko
  ? {
      full: "전체 SDK",
      all: "모든 API 선택",
      selected: "API 선택",
      named: "이름 있는 클라이언트",
      combined: "API 선택 + 이름 있는 클라이언트",
      selection: "API 선택",
      "named-selection": "이름 있는 클라이언트",
      "full-static": "전체 SDK · 정적 로딩",
      "selected-static": "API 선택 · 정적 로딩",
      "full-lookup": "전체 SDK · 동적 조회",
      "selected-lookup": "API 선택 · 동적 조회",
    }
  : {
      full: "Full SDK",
      all: "All APIs selected",
      selected: "Root selection",
      named: "Named selection",
      combined: "Selection + named",
      selection: "Root selection",
      "named-selection": "Named selection",
      "full-static": "Full + static loader",
      "selected-static": "Selection + static loader",
      "full-lookup": "Full + dynamic lookup",
      "selected-lookup": "Selection + dynamic lookup",
    };
const rows: ComputedRef<readonly BenchmarkDatum[]> = computed((): readonly BenchmarkDatum[] => {
  if (props.group === "generation")
    return snapshot.generation.map((row: Generation): BenchmarkDatum => {
      if (selected.value === "seconds")
        return {
          id: row.name,
          label: labels[row.name] ?? row.name,
          value: row.summary.seconds.median,
          low: row.summary.seconds.min,
          high: row.summary.seconds.max,
        };
      return {
        id: row.name,
        label: labels[row.name] ?? row.name,
        value:
          selected.value === "bytes" ? row.bytes / 1024 ** 2 : row.summary.peakRSSKiB.median / 1024,
      };
    });
  return snapshot.bundles.map((row: Bundle): BenchmarkDatum => {
    if (selected.value === "readyMS")
      return {
        id: row.name,
        label: labels[row.name] ?? row.name,
        value: row.summary.readyMS.median,
        low: row.summary.readyMS.min,
        high: row.summary.readyMS.max,
      };
    const bytes: number =
      selected.value === "ready"
        ? row.loaded.ready.bytes
        : selected.value === "brotli"
          ? row.deployed.brotliBytes
          : row.deployed.bytes;
    return { id: row.name, label: labels[row.name] ?? row.name, value: bytes / 1024 };
  });
});
const bars: ComputedRef<readonly BenchmarkBar[]> = computed((): readonly BenchmarkBar[] =>
  benchmarkBars(rows.value, 720),
);
const maximum: ComputedRef<number> = computed((): number =>
  Math.max(...rows.value.map((row: BenchmarkDatum): number => row.high ?? row.value)) * 1.08,
);
const ticks: ComputedRef<readonly number[]> = computed((): readonly number[] =>
  [0, maximum.value / 2, maximum.value],
);
const timed: ComputedRef<boolean> = computed((): boolean =>
  rows.value.some((row: BenchmarkDatum): boolean => row.low !== undefined),
);
const exportName: ComputedRef<string> = computed((): string => {
  const names: Readonly<Record<string, string>> =
    props.group === "generation"
      ? { seconds: "generation-time", bytes: "generation-source", memory: "generation-memory" }
      : {
          ready: "bundle-ready",
          deployed: "bundle-deployed",
          brotli: "bundle-brotli",
          readyMS: "bundle-time",
        };
  return `${names[selected.value]}-${ko ? "ko" : "en"}.svg`;
});
function number(value: number): string {
  return new Intl.NumberFormat(ko ? "ko" : "en", {
    maximumFractionDigits: metric.value.unit === "s" ? 3 : 2,
  }).format(value);
}
function range(row: BenchmarkDatum): string {
  return row.low === undefined ? "" : `${number(row.low)}–${number(row.high ?? row.low)}`;
}
</script>

<template>
  <section
    class="selection-benchmark"
    :aria-label="ko ? '선택 방식별 벤치마크' : 'Selection benchmarks'"
  >
    <div class="metric-controls" role="group" :aria-label="ko ? '비교 지표' : 'Comparison metric'">
      <button
        v-for="item in metrics"
        :key="item.id"
        type="button"
        :aria-pressed="selected === item.id"
        @click="selected = item.id"
      >
        {{ item.label }} <span class="control-unit">({{ item.unit }})</span>
      </button>
    </div>
    <p class="chart-caption">
      {{
        timed
          ? (ko ? "짧을수록 빠릅니다. 막대는 중앙값, 가로선은 최소–최대 시간입니다." : "Shorter is faster. Bars show medians; lines show minimum–maximum times.")
          : (ko ? "짧을수록 용량이 작습니다." : "Shorter means a smaller size.")
      }}
      {{ metric.unit === "KiB" ? (ko ? "1 KiB = 1,024바이트." : "1 KiB = 1,024 bytes.") : metric.unit === "MiB" ? (ko ? "1 MiB = 1,024 KiB." : "1 MiB = 1,024 KiB.") : metric.unit === "ms" ? (ko ? "1 ms = 0.001초." : "1 ms = 0.001 seconds.") : (ko ? "s는 초 단위입니다." : "s denotes seconds.") }}
    </p>
    <div class="chart-scroll" tabindex="0" role="region" :aria-label="metric.label">
      <svg
        :viewBox="`0 0 720 ${rows.length * 36 + 44}`"
        role="img"
        :aria-label="`${metric.label}: ${rows.map((row) => `${row.label} ${number(row.value)} ${metric.unit}`).join('; ')}`"
      >
        <title>{{ metric.label }}</title>
        <g v-for="tick in ticks" :key="tick">
          <line :x1="228 + 396 * tick / maximum" y1="12" :x2="228 + 396 * tick / maximum" :y2="rows.length * 36 + 12" class="axis" />
          <text :x="228 + 396 * tick / maximum" :y="rows.length * 36 + 35" :text-anchor="tick === 0 ? 'start' : tick === maximum ? 'end' : 'middle'" class="origin">{{ number(tick) }} {{ metric.unit }}</text>
        </g>
        <g v-for="bar in bars" :key="bar.id">
          <title>{{ bar.label }}: {{ number(bar.value) }} {{ metric.unit }} {{ range(bar) }}</title>
          <text x="12" :y="bar.y + bar.height / 2 + 4" class="row-label">{{ bar.label }}</text>
          <rect
            :x="bar.x"
            :y="bar.y + bar.height / 2 - 6"
            :width="bar.width"
            height="12"
            rx="2"
            class="bar"
          />
          <line
            v-if="bar.low !== undefined"
            :x1="bar.x + (bar.width * bar.low) / bar.value"
            :x2="bar.x + (bar.width * (bar.high ?? bar.low)) / bar.value"
            :y1="bar.y + bar.height / 2"
            :y2="bar.y + bar.height / 2"
            class="whisker"
          />
          <text x="712" :y="bar.y + bar.height / 2 + 4" text-anchor="end" class="value">
            {{ number(bar.value) }} {{ metric.unit }}
          </text>
        </g>
      </svg>
    </div>
    <details>
      <summary>{{ ko ? "정확한 수치와 측정 범위" : "Exact values and observed ranges" }}</summary>
      <table>
        <thead>
          <tr>
            <th scope="col">{{ ko ? "방식" : "Method" }}</th>
            <th scope="col">{{ timed ? (ko ? "중앙값" : "Median") : metric.label }} ({{ metric.unit }})</th>
            <th v-if="timed" scope="col">{{ ko ? "최소–최대" : "Min–max" }} ({{ metric.unit }})</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id">
            <th scope="row">{{ row.label }}</th>
            <td>{{ number(row.value) }}</td>
            <td v-if="timed">{{ range(row) }}</td>
          </tr>
        </tbody>
      </table>
    </details>
    <p class="source-link">
      <a :href="withBase(`/benchmarks/${exportName}`)" download>{{
        ko ? "그래프 SVG 다운로드" : "Download chart SVG"
      }}</a>
      ·
      <a :href="withBase('/benchmarks/selection-results.json')">{{
        ko ? "측정 수치 JSON" : "Measurement JSON"
      }}</a>
    </p>
  </section>
</template>

<style scoped>
.selection-benchmark {
  margin: 24px 0 32px;
}
.metric-controls {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.metric-controls button {
  padding: 6px 12px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 6px;
  font: inherit;
  font-size: 13px;
  cursor: pointer;
}
.metric-controls button[aria-pressed="true"] {
  background: var(--vp-c-brand-soft);
  border-color: var(--vp-c-brand-1);
  color: var(--vp-c-brand-1);
}
.metric-controls button:focus-visible,
.chart-scroll:focus-visible {
  outline: 2px solid var(--vp-c-brand-1);
  outline-offset: 3px;
}
.chart-caption,
.source-link {
  font-size: 13px;
  color: var(--vp-c-text-2);
}
.chart-scroll {
  overflow-x: auto;
}
svg {
  display: block;
  width: 100%;
  max-width: 720px;
  min-width: 600px;
  font-family: var(--vp-font-family-base);
}
.bar {
  fill: var(--vp-c-brand-1);
}
.axis {
  stroke: var(--vp-c-divider);
}
.whisker {
  stroke: var(--vp-c-text-1);
  stroke-width: 1.5;
}
text {
  fill: var(--vp-c-text-1);
  font-size: 12px;
}
.value {
  font-variant-numeric: tabular-nums;
}
.origin {
  fill: var(--vp-c-text-2);
  font-size: 12px;
}
details {
  margin-top: 16px;
}
summary {
  cursor: pointer;
  font-size: 14px;
}
@media print {
  .metric-controls,
  .source-link {
    display: none;
  }
  svg {
    min-width: 0;
  }
}
</style>
