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

var performanceGateWorkloads = []string{
	"self-contained",
	"templated-resource",
	"parameter-heavy",
	"external-reference",
	"high-artifact",
	"server-addon",
	"links-heavy",
}

var performanceGatePhases = []string{"compile", "prepare", "emit", "publish", "full"}

type performanceBaseline struct {
	Historical performanceHistoricalBaseline        `json:"historical"`
	Current    map[string]performanceWorkloadBudget `json:"current"`
	Thresholds performanceRegressionThresholds      `json:"regressionThresholdPercent"`
}

type performanceHistoricalBaseline struct {
	BenchmarkWallNanoseconds    uint64         `json:"benchmarkWallNanoseconds"`
	ProcessWallNanoseconds      uint64         `json:"processWallNanoseconds"`
	ProcessCPUNanoseconds       uint64         `json:"processCpuNanoseconds"`
	BytesPerOperation           uint64         `json:"bytesPerOperation"`
	PeakHeapBytes               uint64         `json:"peakHeapBytes"`
	MaxRSSBytes                 uint64         `json:"maxRssBytes"`
	RequiredReductionPercentage map[string]int `json:"requiredReductionPercent"`
}

type performanceWorkloadBudget struct {
	Phases  map[string]performancePhaseBudget `json:"phases"`
	Process performanceProcessBudget          `json:"process"`
}

type performancePhaseBudget struct {
	WallNanoseconds   uint64 `json:"wallNanoseconds"`
	BytesPerOperation uint64 `json:"bytesPerOperation"`
}

type performanceProcessBudget struct {
	WallNanoseconds uint64 `json:"wallNanoseconds"`
	CPUNanoseconds  uint64 `json:"cpuNanoseconds"`
	PeakHeapBytes   uint64 `json:"peakHeapBytes"`
	MaxRSSBytes     uint64 `json:"maxRssBytes"`
}

type performanceRegressionThresholds struct {
	BenchmarkWall int `json:"benchmarkWall"`
	Allocation    int `json:"allocation"`
	Process       int `json:"process"`
	Memory        int `json:"memory"`
}

type performancePhaseRun struct {
	wall       uint64
	bytesPerOp uint64
}

type performanceGateRun struct {
	phases  map[string]map[string]performancePhaseRun
	process map[string]performanceProcessMetric
}

var benchmarkMetricPattern = regexp.MustCompile(`(?m)^BenchmarkGeneration/([^/]+)/([^\s-]+)-\d+\s+\d+\s+(\d+) ns/op.*?\s+(\d+) B/op`)

func TestPerformanceBaselineCoversRegressionWorkloads(t *testing.T) {
	baseline := readPerformanceBaseline(t)
	for _, workload := range performanceGateWorkloads {
		budget, exists := baseline.Current[workload]
		if !exists {
			t.Errorf("performance baseline missing workload %q", workload)
			continue
		}
		for _, phase := range performanceGatePhases {
			value, exists := budget.Phases[phase]
			if !exists || value.WallNanoseconds == 0 || value.BytesPerOperation == 0 {
				t.Errorf("performance baseline missing %s/%s phase budget", workload, phase)
			}
		}
		if budget.Process.WallNanoseconds == 0 || budget.Process.CPUNanoseconds == 0 || budget.Process.PeakHeapBytes == 0 || budget.Process.MaxRSSBytes == 0 {
			t.Errorf("performance baseline missing %s process budget", workload)
		}
	}
}

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

	var failed []string
	for _, workload := range performanceGateWorkloads {
		budget := baseline.Current[workload]
		for _, phase := range performanceGatePhases {
			phaseBudget := budget.Phases[phase]
			wall := medianPerformanceValue(runs, func(run performanceGateRun) uint64 { return run.phases[workload][phase].wall })
			allocation := medianPerformanceValue(runs, func(run performanceGateRun) uint64 { return run.phases[workload][phase].bytesPerOp })
			failed = appendRegressionCheck(t, failed, workload+"/"+phase+"/wall", phaseBudget.WallNanoseconds, wall, baseline.Thresholds.BenchmarkWall)
			failed = appendRegressionCheck(t, failed, workload+"/"+phase+"/allocation", phaseBudget.BytesPerOperation, allocation, baseline.Thresholds.Allocation)
		}
		process := performanceProcessBudget{
			WallNanoseconds: medianPerformanceValue(runs, func(run performanceGateRun) uint64 { return uint64(run.process[workload].WallNanosecond) }),
			CPUNanoseconds:  medianPerformanceValue(runs, func(run performanceGateRun) uint64 { return uint64(run.process[workload].CPUNanosecond) }),
			PeakHeapBytes:   medianPerformanceValue(runs, func(run performanceGateRun) uint64 { return run.process[workload].PeakHeapBytes }),
			MaxRSSBytes:     medianPerformanceValue(runs, func(run performanceGateRun) uint64 { return run.process[workload].MaxRSSBytes }),
		}
		failed = appendRegressionCheck(t, failed, workload+"/process/wall", budget.Process.WallNanoseconds, process.WallNanoseconds, baseline.Thresholds.Process)
		failed = appendRegressionCheck(t, failed, workload+"/process/cpu", budget.Process.CPUNanoseconds, process.CPUNanoseconds, baseline.Thresholds.Process)
		failed = appendRegressionCheck(t, failed, workload+"/process/heap", budget.Process.PeakHeapBytes, process.PeakHeapBytes, baseline.Thresholds.Memory)
		failed = appendRegressionCheck(t, failed, workload+"/process/rss", budget.Process.MaxRSSBytes, process.MaxRSSBytes, baseline.Thresholds.Memory)
	}
	failed = append(failed, historicalPerformanceFailures(t, baseline.Historical, runs)...)
	if len(failed) != 0 {
		t.Fatalf("performance acceptance failed: %s", strings.Join(failed, ", "))
	}
}

func appendRegressionCheck(t *testing.T, failed []string, name string, baseline, current uint64, threshold int) []string {
	t.Helper()
	limit := baseline + baseline*uint64(threshold)/100
	status := "pass"
	if current > limit {
		status = "fail"
		failed = append(failed, name)
	}
	t.Logf("PERF_REGRESSION field=%s baseline=%d median=%d limit=%d status=%s", name, baseline, current, limit, status)
	return failed
}

func historicalPerformanceFailures(t *testing.T, baseline performanceHistoricalBaseline, runs []performanceGateRun) []string {
	benchmark := medianPerformanceValue(runs, func(run performanceGateRun) uint64 { return run.phases["self-contained"]["full"].wall })
	allocation := medianPerformanceValue(runs, func(run performanceGateRun) uint64 { return run.phases["self-contained"]["full"].bytesPerOp })
	processWall := medianPerformanceValue(runs, func(run performanceGateRun) uint64 { return uint64(run.process["self-contained"].WallNanosecond) })
	processCPU := medianPerformanceValue(runs, func(run performanceGateRun) uint64 { return uint64(run.process["self-contained"].CPUNanosecond) })
	peakHeap := medianPerformanceValue(runs, func(run performanceGateRun) uint64 { return run.process["self-contained"].PeakHeapBytes })
	maxRSS := medianPerformanceValue(runs, func(run performanceGateRun) uint64 { return run.process["self-contained"].MaxRSSBytes })
	checks := []struct {
		name     string
		baseline uint64
		current  uint64
	}{
		{"benchmarkWallNanoseconds", baseline.BenchmarkWallNanoseconds, benchmark},
		{"processWallNanoseconds", baseline.ProcessWallNanoseconds, processWall},
		{"processCpuNanoseconds", baseline.ProcessCPUNanoseconds, processCPU},
		{"bytesPerOperation", baseline.BytesPerOperation, allocation},
		{"peakHeapBytes", baseline.PeakHeapBytes, peakHeap},
		{"maxRssBytes", baseline.MaxRSSBytes, maxRSS},
	}
	var failed []string
	for _, check := range checks {
		reduction := reductionPercentage(check.baseline, check.current)
		required := float64(baseline.RequiredReductionPercentage[check.name])
		status := "pass"
		if reduction < required {
			status = "fail"
			failed = append(failed, "historical/"+check.name)
		}
		t.Logf("PERF_HISTORICAL field=%s baseline=%d median=%d reduction=%.1f%% required=%.1f%% status=%s", check.name, check.baseline, check.current, reduction, required, status)
	}
	return failed
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
	result := performanceGateRun{phases: make(map[string]map[string]performancePhaseRun), process: make(map[string]performanceProcessMetric)}
	for _, match := range benchmarkMetricPattern.FindAllSubmatch(bench, -1) {
		workload, phase := string(match[1]), string(match[2])
		if result.phases[workload] == nil {
			result.phases[workload] = make(map[string]performancePhaseRun)
		}
		result.phases[workload][phase] = performancePhaseRun{wall: parsePerformanceUint(t, match[3]), bytesPerOp: parsePerformanceUint(t, match[4])}
	}
	for _, workload := range performanceGateWorkloads {
		for _, phase := range performanceGatePhases {
			if result.phases[workload][phase].wall == 0 {
				t.Fatalf("run %d benchmark metric not found for %s/%s", index, workload, phase)
			}
		}
		process := readPerformanceLog(t, filepath.Join(directory, fmt.Sprintf("run-%02d-%s-process.log", index, workload)))
		marker := []byte("PERF_METRIC ")
		start := strings.Index(string(process), string(marker))
		if start < 0 {
			t.Fatalf("run %d process metric not found for %s", index, workload)
		}
		line := string(process[start+len(marker):])
		line, _, _ = strings.Cut(line, "\n")
		var metric performanceProcessMetric
		if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &metric); err != nil {
			t.Fatalf("run %d process metric for %s: %v", index, workload, err)
		}
		result.process[workload] = metric
	}
	return result
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

func medianPerformanceValue[T any](runs []T, value func(T) uint64) uint64 {
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
