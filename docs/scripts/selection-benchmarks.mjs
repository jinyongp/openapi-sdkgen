import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { gunzipSync } from "node:zlib";
import { benchmarkBars } from "../.vitepress/theme/benchmark-bars.ts";

const directory = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../test/perf/documentation-results",
);
const summary = JSON.parse(fs.readFileSync(path.join(directory, "selection-results.json")));
const raw = JSON.parse(
  gunzipSync(fs.readFileSync(path.join(directory, "selection-measurements.json.gz"))),
);
assert.equal(summary.commit, raw.commit);
assert.equal(summary.generatorSHA256, raw.generatorSHA256);
assert.equal(summary.inputSHA256, raw.inputSHA256);
assert.equal(summary.generation.length, 5);
assert.equal(summary.bundles.length, 6);
for (const row of summary.generation) {
  const original = raw.generation.find((item) => item.name === row.name);
  assert.equal(original.runs.length, 3);
  assert.equal(row.bytes, original.inventory.bytes);
  assert.equal(row.files, original.inventory.files);
  assert.deepEqual(row.summary, original.summary);
}
assert.deepEqual(
  raw.generation.find((row) => row.name === "full").inventory,
  raw.generation.find((row) => row.name === "all").inventory,
);
assert.deepEqual(
  raw.generation.find((row) => row.name === "named").inventory,
  raw.generation.find((row) => row.name === "combined").inventory,
);
const traces = raw.cases[0].runtime[0].traces;
for (const row of summary.bundles) {
  const original = raw.cases.find((item) => item.name === row.name);
  assert.equal(original.buildRuns.length, 3);
  assert.equal(original.runtime.length, 6);
  assert.deepEqual(row.summary, original.summary);
  for (const key of ["files", "bytes", "gzipBytes", "brotliBytes"])
    assert.equal(row.deployed[key], original.deployed[key]);
  for (const run of original.runtime) {
    assert.deepEqual(run.traces, traces);
    assert.equal(run.prepareHTTPRequests, 0);
    for (const stage of ["import", "ready", "link"]) {
      for (const key of ["files", "bytes", "gzipBytes", "brotliBytes"])
        assert.equal(row.loaded[stage][key], run.loaded[stage][key]);
      assert(
        run.loaded[stage].records.every((file) =>
          original.deployed.records.some(
            (deployed) => deployed.file === file.file && deployed.sha256 === file.sha256,
          ),
        ),
      );
    }
  }
}
const labels = {
  en: {
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
  },
  ko: {
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
  },
};
const charts = [
  {
    id: "generation-time",
    unit: "s",
    en: "Generation time",
    ko: "생성 시간",
    rows: summary.generation.map((row) => ({
      id: row.name,
      value: row.summary.seconds.median,
      low: row.summary.seconds.min,
      high: row.summary.seconds.max,
    })),
  },
  {
    id: "generation-source",
    unit: "MiB",
    en: "Generated source",
    ko: "생성 소스",
    rows: summary.generation.map((row) => ({ id: row.name, value: row.bytes / 1024 ** 2 })),
  },
  {
    id: "generation-memory",
    unit: "MiB",
    en: "Peak generation memory",
    ko: "생성 최대 메모리",
    rows: summary.generation.map((row) => ({
      id: row.name,
      value: row.summary.peakRSSKiB.median / 1024,
    })),
  },
  {
    id: "bundle-ready",
    unit: "KiB",
    en: "JS loaded to ready",
    ko: "호출 준비까지 로드한 JS",
    rows: summary.bundles.map((row) => ({ id: row.name, value: row.loaded.ready.bytes / 1024 })),
  },
  {
    id: "bundle-deployed",
    unit: "KiB",
    en: "All deployed JS",
    ko: "전체 배포 JS",
    rows: summary.bundles.map((row) => ({ id: row.name, value: row.deployed.bytes / 1024 })),
  },
  {
    id: "bundle-brotli",
    unit: "KiB",
    en: "Brotli-compressed size",
    ko: "Brotli 압축 크기",
    rows: summary.bundles.map((row) => ({ id: row.name, value: row.deployed.brotliBytes / 1024 })),
  },
  {
    id: "bundle-time",
    unit: "ms",
    en: "Time to ready",
    ko: "준비 시간",
    rows: summary.bundles.map((row) => ({
      id: row.name,
      value: row.summary.readyMS.median,
      low: row.summary.readyMS.min,
      high: row.summary.readyMS.max,
    })),
  },
];
const escape = (text) =>
  String(text).replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll('"', "&quot;");
for (const locale of ["en", "ko"])
  for (const chart of charts) {
    const rows = chart.rows.map((row) => ({ ...row, label: labels[locale][row.id] }));
    const bars = benchmarkBars(rows, 720);
    assert.equal(bars.length, rows.length);
    for (const bar of bars) {
      assert.equal(bar.x, 228);
      assert(bar.width > 0 && bar.x + bar.width <= 624);
      assert.equal(bar.value, rows.find((row) => row.id === bar.id).value);
      const scale = bars[0].width / bars[0].value;
      assert(
        Math.abs(bar.width / bar.value - scale) < 0.002,
        "Bar length must preserve the zero-based linear ratio",
      );
    }
    const number = (value) =>
      new Intl.NumberFormat(locale, { maximumFractionDigits: chart.unit === "s" ? 3 : 2 }).format(
        value,
      );
    const height = rows.length * 36 + 100;
    const title = `${chart[locale]} (${chart.unit})`;
    const maximum = Math.max(...rows.map((row) => row.high ?? row.value)) * 1.08;
    const axis = [0, maximum / 2, maximum].map((value) => {
      const x = 228 + 396 * value / maximum;
      const anchor = value === 0 ? "start" : value === maximum ? "end" : "middle";
      return `<line x1="${x}" y1="12" x2="${x}" y2="${rows.length * 36 + 12}" stroke="#cad0d9"/><text x="${x}" y="${rows.length * 36 + 35}" text-anchor="${anchor}" font-size="11">${number(value)} ${chart.unit}</text>`;
    }).join("");
    const barMarkup = bars.map((bar) => {
      const center = bar.y + bar.height / 2;
      const whisker = bar.low === undefined ? "" : `<line x1="${bar.x + bar.width * bar.low / bar.value}" x2="${bar.x + bar.width * bar.high / bar.value}" y1="${center}" y2="${center}" stroke="#172033" stroke-width="1.5"/>`;
      return `<g><title>${escape(bar.label)}: ${number(bar.value)} ${chart.unit}</title><text x="12" y="${center + 4}">${escape(bar.label)}</text><rect x="${bar.x}" y="${center - 6}" width="${bar.width}" height="12" rx="2" fill="#0f766e"/>${whisker}<text x="712" y="${center + 4}" text-anchor="end">${number(bar.value)} ${chart.unit}</text></g>`;
    }).join("");
    const unitSvg = `<svg xmlns="http://www.w3.org/2000/svg" width="720" height="${height}" viewBox="0 0 720 ${height}" role="img" aria-label="${escape(title)}"><title>${escape(title)}</title><rect width="720" height="${height}" fill="white"/><g font-family="system-ui,sans-serif" font-size="12" fill="#172033"><text x="12" y="26" font-size="16" font-weight="600">${escape(title)}</text><text x="12" y="48" font-size="11">${locale === "ko" ? "API 1,000개 중 10개 · 중앙값, 선은 최소–최대" : "10 of 1,000 APIs · medians, whiskers show min–max"}</text><g transform="translate(0 56)">${axis}${barMarkup}</g></g></svg>\n`;
    const measuredSvg = rows.some((row) => row.low !== undefined)
      ? unitSvg
      : unitSvg
          .replace("medians, whiskers show min–max", "lower is smaller")
          .replace("중앙값, 선은 최소–최대", "낮을수록 작음");
    fs.writeFileSync(path.join(directory, `${chart.id}-${locale}.svg`), measuredSvg);
  }
const features = JSON.parse(fs.readFileSync(path.join(directory, "runtime-features-results.json")));
assert.match(features.generatorSourceSHA256, /^[a-f0-9]{64}$/);
assert.match(features.inputSHA256, /^[a-f0-9]{64}$/);
assert.equal(features.environment.nativeBundleCallsPassed, true);
assert.equal(features.environment.gzipLevel, 6);
assert.equal(features.bundles.length, 4);
for (const row of features.bundles) {
  assert.equal(row.chunks, row.outputs.length);
  assert.equal(row.deployedBytes, row.outputs.reduce((sum, item) => sum + item.bytes, 0));
  assert.equal(row.deployedGzipBytes, row.outputs.reduce((sum, item) => sum + item.gzipBytes, 0));
  const entry = row.outputs.find((item) => item.entry);
  assert.equal(row.entryBytes, entry.bytes);
  assert.equal(row.entryGzipBytes, entry.gzipBytes);
}
assert.equal(new Set(features.featureSizes.map((row) => `${row.group}/${row.name}`)).size, features.featureSizes.length);
for (const row of features.featureSizes) {
  for (const variant of Object.values(row.variants)) {
    assert.ok(variant.source.total.files >= variant.source.runtime.files);
    assert.ok(variant.source.total.lines >= variant.source.runtime.lines);
    assert.ok(variant.native.files > 0 && variant.bundle.chunks > 0);
  }
}
console.log("ok selection benchmark evidence, feature size evidence, six execution cases, and 14 Sectile SVG exports");
