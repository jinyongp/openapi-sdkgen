package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const performanceGateEnv = "OPENAPI_SDKGEN_PERF_GATE"

type performanceBaseline struct {
	Workload                    string         `json:"workload"`
	BenchmarkWallNanoseconds    uint64         `json:"benchmarkWallNanoseconds"`
	ProcessWallNanoseconds      uint64         `json:"processWallNanoseconds"`
	ProcessCPUNanoseconds       uint64         `json:"processCpuNanoseconds"`
	BytesPerOperation           uint64         `json:"bytesPerOperation"`
	PeakHeapBytes               uint64         `json:"peakHeapBytes"`
	MaxRSSBytes                 uint64         `json:"maxRssBytes"`
	RequiredReductionPercentage map[string]int `json:"requiredReductionPercent"`
}

type performanceGateRun struct {
	benchmarkWall uint64
	processWall   uint64
	processCPU    uint64
	bytesPerOp    uint64
	peakHeap      uint64
	maxRSS        uint64
}

var fullBenchmarkPattern = regexp.MustCompile(`(?m)^BenchmarkGeneration/self-contained/full-\d+\s+\d+\s+(\d+) ns/op\s+(\d+) B/op`)

func TestPerformanceAcceptanceGate(t *testing.T) {
	if os.Getenv(performanceGateEnv) != "1" {
		t.Skip("performance acceptance gate is opt-in")
	}
	logDirectory := os.Getenv("OPENAPI_SDKGEN_PERF_GATE_LOG_DIR")
	if logDirectory == "" {
		t.Fatal("OPENAPI_SDKGEN_PERF_GATE_LOG_DIR is required")
	}
	baseline := readPerformanceBaseline(t)
	runs := make([]performanceGateRun, 0, 5)
	for index := 1; index <= 5; index++ {
		runs = append(runs, readPerformanceGateRun(t, logDirectory, index))
	}
	medians := performanceGateRun{
		benchmarkWall: medianPerformanceValue(runs, func(run performanceGateRun) uint64 { return run.benchmarkWall }),
		processWall:   medianPerformanceValue(runs, func(run performanceGateRun) uint64 { return run.processWall }),
		processCPU:    medianPerformanceValue(runs, func(run performanceGateRun) uint64 { return run.processCPU }),
		bytesPerOp:    medianPerformanceValue(runs, func(run performanceGateRun) uint64 { return run.bytesPerOp }),
		peakHeap:      medianPerformanceValue(runs, func(run performanceGateRun) uint64 { return run.peakHeap }),
		maxRSS:        medianPerformanceValue(runs, func(run performanceGateRun) uint64 { return run.maxRSS }),
	}
	checks := []struct {
		name     string
		baseline uint64
		current  uint64
	}{
		{"benchmarkWallNanoseconds", baseline.BenchmarkWallNanoseconds, medians.benchmarkWall},
		{"processWallNanoseconds", baseline.ProcessWallNanoseconds, medians.processWall},
		{"processCpuNanoseconds", baseline.ProcessCPUNanoseconds, medians.processCPU},
		{"bytesPerOperation", baseline.BytesPerOperation, medians.bytesPerOp},
		{"peakHeapBytes", baseline.PeakHeapBytes, medians.peakHeap},
		{"maxRssBytes", baseline.MaxRSSBytes, medians.maxRSS},
	}
	var failed []string
	for _, check := range checks {
		reduction := reductionPercentage(check.baseline, check.current)
		required := float64(baseline.RequiredReductionPercentage[check.name])
		status := "pass"
		if reduction < required {
			status = "fail"
			failed = append(failed, check.name)
		}
		t.Logf("PERF_ACCEPTANCE field=%s baseline=%d median=%d reduction=%.1f%% required=%.1f%% status=%s", check.name, check.baseline, check.current, reduction, required, status)
	}
	if len(failed) != 0 {
		t.Fatalf("performance acceptance failed: %s", strings.Join(failed, ", "))
	}
}

func readPerformanceBaseline(t *testing.T) performanceBaseline {
	t.Helper()
	path := os.Getenv("OPENAPI_SDKGEN_PERF_GATE_BASELINE")
	if path == "" {
		path = filepath.Join("testdata", "performance-baseline.json")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var baseline performanceBaseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		t.Fatal(err)
	}
	return baseline
}

func readPerformanceGateRun(t *testing.T, directory string, index int) performanceGateRun {
	t.Helper()
	bench := readPerformanceLog(t, filepath.Join(directory, fmt.Sprintf("run-%02d-bench.log", index)))
	matches := fullBenchmarkPattern.FindSubmatch(bench)
	if len(matches) != 3 {
		t.Fatalf("run %d benchmark metric not found", index)
	}
	process := readPerformanceLog(t, filepath.Join(directory, fmt.Sprintf("run-%02d-process.log", index)))
	marker := []byte("PERF_METRIC ")
	start := strings.Index(string(process), string(marker))
	if start < 0 {
		t.Fatalf("run %d process metric not found", index)
	}
	line := string(process[start+len(marker):])
	line, _, _ = strings.Cut(line, "\n")
	var metric performanceProcessMetric
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &metric); err != nil {
		t.Fatalf("run %d process metric: %v", index, err)
	}
	return performanceGateRun{
		benchmarkWall: parsePerformanceUint(t, matches[1]),
		bytesPerOp:    parsePerformanceUint(t, matches[2]),
		processWall:   uint64(metric.WallNanosecond),
		processCPU:    uint64(metric.CPUNanosecond),
		peakHeap:      metric.PeakHeapBytes,
		maxRSS:        metric.MaxRSSBytes,
	}
}

func readPerformanceLog(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func parsePerformanceUint(t *testing.T, value []byte) uint64 {
	t.Helper()
	parsed, err := strconv.ParseUint(string(value), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func medianPerformanceValue(runs []performanceGateRun, value func(performanceGateRun) uint64) uint64 {
	values := make([]uint64, 0, len(runs))
	for _, run := range runs {
		values = append(values, value(run))
	}
	sort.Slice(values, func(left, right int) bool { return values[left] < values[right] })
	return values[len(values)/2]
}

func reductionPercentage(baseline, current uint64) float64 {
	if baseline == 0 {
		return 0
	}
	return (float64(baseline) - float64(current)) * 100 / float64(baseline)
}
