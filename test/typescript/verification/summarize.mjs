import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { repositoryRoot } from "./catalog.mjs";
import { median } from "./contracts.mjs";
const [file] = process.argv.slice(2);
assert.ok(file, "provide a report.json path");
const report = JSON.parse(readFileSync(resolve(repositoryRoot, file), "utf8"));
const summary = report.fixtures.map((f) => ({
  id: f.id,
  contract: f.contracts,
  timing: Object.fromEntries(
    Object.entries(f.timingSummary ?? {}).map(([field, result]) => [
      field,
      {
        baselineMs: result.ab.baseline,
        candidateMs: result.ab.candidate,
        pairedDeltaMs: result.ab.pairedDelta,
        pairedPercent: result.ab.pairedPercent,
        aaPairedPercent: result.aa.pairedPercent,
        noiseFloorMs: result.noiseFloorMs,
        review: result.review,
      },
    ]),
  ),
  memory: f.memory?.length
    ? {
        pairs: f.memory.length,
        baselineLateGrowthBytes: median(f.memory.map((p) => p.baseline.lateGrowthBytes)),
        candidateLateGrowthBytes: median(f.memory.map((p) => p.candidate.lateGrowthBytes)),
        candidateRequestHelperCalls: f.memory.map((p) => p.candidate.requestHelperCalls),
        candidateConstructionCalls: f.memory.map((p) => p.candidate.constructionCalls),
        clientsDiscardedPerProcess: f.memory[0].candidate.clientsDiscarded,
        review: f.memoryReview,
      }
    : undefined,
  leaf: f.leaf,
}));
console.log(
  JSON.stringify(
    {
      status: report.status,
      correctness: report.correctness,
      startedAt: report.startedAt,
      finishedAt: report.finishedAt,
      binarySha256: report.binarySha256,
      summary,
    },
    null,
    2,
  ),
);
