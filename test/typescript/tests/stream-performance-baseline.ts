export const streamPerformanceBaseline = {
  sampleCount: 5,
  regressionThresholdPercent: 50,
  catastrophicLimitMilliseconds: 250,
  relativeOverhead: {
    multiplier: 2.5,
    slackMilliseconds: 5,
  },
  workloads: {
    "many-small-ndjson": { wallMilliseconds: 12 },
    "many-small-sse": { wallMilliseconds: 12 },
    "one-byte-chunks": { wallMilliseconds: 18 },
    "near-limit-frames": { wallMilliseconds: 4 },
    "adapter-pass-through": { wallMilliseconds: 12 },
    "operation-stream-readable": { wallMilliseconds: 12 },
  },
} as const;

export type StreamPerformanceMetric = keyof typeof streamPerformanceBaseline.workloads;
