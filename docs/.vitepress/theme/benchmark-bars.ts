import { createChartDefinition } from "@sectile/chart/definition";
import { createChartProjection } from "@sectile/chart/projection";

export interface BenchmarkDatum {
  readonly id: string;
  readonly label: string;
  readonly value: number;
  readonly low?: number;
  readonly high?: number;
}

export interface BenchmarkBar extends BenchmarkDatum {
  readonly x: number;
  readonly y: number;
  readonly width: number;
  readonly height: number;
}

export function benchmarkBars(
  data: readonly BenchmarkDatum[],
  width: number,
): readonly BenchmarkBar[] {
  const maximum: number =
    Math.max(...data.map((row: BenchmarkDatum): number => row.high ?? row.value)) * 1.08;
  const definition: ReturnType<typeof createChartDefinition> = createChartDefinition<
    BenchmarkDatum,
    string
  >({
    coordinate: {
      kind: "cartesian",
      axes: [
        {
          id: "value",
          orientation: "x",
          scale: "linear",
          field: "value",
          domain: { kind: "numeric", minimum: 0, maximum },
        },
        {
          id: "category",
          orientation: "y",
          scale: "categorical",
          field: "id",
          domain: {
            kind: "categorical",
            values: data.map((row: BenchmarkDatum): string => row.id).reverse(),
          },
        },
      ],
    },
    layers: [
      {
        id: "benchmarks",
        kind: "bar",
        orientation: "horizontal",
        xAxis: "value",
        yAxis: "category",
        data,
      },
    ],
  });
  const projection: ReturnType<typeof createChartProjection> = createChartProjection(definition, {
    viewport: { width, height: data.length * 54 + 44 },
    insets: { left: 228, right: 96, top: 12, bottom: 32 },
  });
  const source: Map<string, BenchmarkDatum> = new Map(
    data.map((row: BenchmarkDatum): [string, BenchmarkDatum] => [row.id, row]),
  );
  const bars: BenchmarkBar[] = [];
  for (const batch of projection.batches) {
    if (batch.type !== "rectangle") continue;
    for (let index: number = 0; index < batch.identityIndices.length; index++) {
      const row: BenchmarkDatum | undefined = source.get(
        String(projection.identities[batch.identityIndices[index]!]),
      );
      if (row === undefined) throw new Error("Benchmark projection lost a row identity");
      const offset: number = index * 4;
      bars.push({
        ...row,
        x: batch.rectangles[offset]!,
        y: batch.rectangles[offset + 1]!,
        width: batch.rectangles[offset + 2]!,
        height: batch.rectangles[offset + 3]!,
      });
    }
  }
  return bars;
}
