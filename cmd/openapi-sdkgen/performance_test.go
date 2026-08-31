package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
	"openapi-sdkgen/internal/target/typescript"
)

const performanceEnabledEnv = "OPENAPI_SDKGEN_PERF"

type performanceWorkload struct {
	name        string
	root        string
	files       map[string][]byte
	expectError bool
}

type performanceProcessMetric struct {
	Workload       string `json:"workload"`
	InputBytes     int    `json:"inputBytes"`
	ArtifactCount  int    `json:"artifactCount"`
	ArtifactBytes  int    `json:"artifactBytes"`
	WallNanosecond int64  `json:"wallNanoseconds"`
	CPUNanosecond  int64  `json:"cpuNanoseconds"`
	PeakHeapBytes  uint64 `json:"peakHeapBytes"`
	MaxRSSBytes    uint64 `json:"maxRSSBytes"`
}

func BenchmarkGeneration(b *testing.B) {
	for _, workload := range performanceWorkloads() {
		workload := workload
		b.Run(workload.name, func(b *testing.B) {
			root := materializePerformanceWorkload(b, workload)
			inputBytes := performanceInputBytes(workload)

			b.Run("compile", func(b *testing.B) {
				b.ReportAllocs()
				b.ReportMetric(float64(inputBytes), "input-B/op")
				for index := 0; index < b.N; index++ {
					compiled := compilePerformanceWorkload(b, root, workload.expectError)
					if compiled.Document == nil && !workload.expectError {
						b.Fatal("compiler returned no document")
					}
				}
			})

			if workload.expectError {
				b.Run("full", func(b *testing.B) {
					b.ReportAllocs()
					for index := 0; index < b.N; index++ {
						_ = compilePerformanceWorkload(b, root, true)
					}
				})
				return
			}

			compiled := compilePerformanceWorkload(b, root, false)
			target := typescript.Generator{}
			prepared := preparePerformanceWorkload(b, target, compiled)
			artifacts := emitPerformanceWorkload(b, target, prepared)
			artifactBytes := performanceArtifactBytes(artifacts)

			b.Run("prepare", func(b *testing.B) {
				b.ReportAllocs()
				for index := 0; index < b.N; index++ {
					_ = preparePerformanceWorkload(b, target, compiled)
				}
			})
			b.Run("emit", func(b *testing.B) {
				b.ReportAllocs()
				b.ReportMetric(float64(len(artifacts)), "artifacts/op")
				b.ReportMetric(float64(artifactBytes), "artifact-B/op")
				for index := 0; index < b.N; index++ {
					_ = emitPerformanceWorkload(b, target, prepared)
				}
			})
			b.Run("publish", func(b *testing.B) {
				b.ReportAllocs()
				outputRoot := b.TempDir()
				b.ResetTimer()
				for index := 0; index < b.N; index++ {
					output := filepath.Join(outputRoot, fmt.Sprintf("output-%06d", index))
					if err := writeArtifacts(output, append([]generator.Artifact(nil), artifacts...)); err != nil {
						b.Fatal(err)
					}
					b.StopTimer()
					if err := os.RemoveAll(output); err != nil {
						b.Fatal(err)
					}
					b.StartTimer()
				}
			})
			b.Run("full", func(b *testing.B) {
				b.ReportAllocs()
				outputRoot := b.TempDir()
				b.ResetTimer()
				for index := 0; index < b.N; index++ {
					output := filepath.Join(outputRoot, fmt.Sprintf("output-%06d", index))
					runPerformanceGeneration(b, root, output)
					b.StopTimer()
					if err := os.RemoveAll(output); err != nil {
						b.Fatal(err)
					}
					b.StartTimer()
				}
			})
		})
	}
}

func TestPerformanceWorkloadsDeterministic(t *testing.T) {
	if os.Getenv(performanceEnabledEnv) != "1" {
		t.Skip("performance workload verification is opt-in")
	}
	first := performanceWorkloads()
	second := performanceWorkloads()
	if len(first) != len(second) {
		t.Fatalf("workload counts differ: %d != %d", len(first), len(second))
	}
	for index := range first {
		left, right := first[index], second[index]
		leftHash := performanceWorkloadHash(left)
		rightHash := performanceWorkloadHash(right)
		if left.name != right.name || leftHash != rightHash {
			t.Fatalf("workload %d is not deterministic: %s/%s != %s/%s", index, left.name, leftHash, right.name, rightHash)
		}
		t.Logf("PERF_WORKLOAD name=%s files=%d bytes=%d sha256=%s", left.name, len(left.files), performanceInputBytes(left), leftHash)
	}
}

func TestPerformanceProcessMetrics(t *testing.T) {
	if os.Getenv(performanceEnabledEnv) != "1" {
		t.Skip("performance process metrics are opt-in")
	}
	selected := os.Getenv("OPENAPI_SDKGEN_PERF_WORKLOAD")
	if selected == "" {
		selected = "self-contained"
	}
	var workload *performanceWorkload
	for _, candidate := range performanceWorkloads() {
		if candidate.name == selected {
			value := candidate
			workload = &value
			break
		}
	}
	if workload == nil || workload.expectError {
		t.Fatalf("unknown successful performance workload %q", selected)
	}
	root := materializePerformanceWorkload(t, *workload)
	output := filepath.Join(t.TempDir(), "output")

	runtime.GC()
	peakHeap, stopSampling := samplePeakHeap()
	beforeCPU := processCPUTime(t)
	started := time.Now()
	artifacts := runPerformanceGeneration(t, root, output)
	wall := time.Since(started)
	afterCPU := processCPUTime(t)
	stopSampling()

	metric := performanceProcessMetric{
		Workload:       workload.name,
		InputBytes:     performanceInputBytes(*workload),
		ArtifactCount:  artifacts.count,
		ArtifactBytes:  artifacts.bytes,
		WallNanosecond: wall.Nanoseconds(),
		CPUNanosecond:  (afterCPU - beforeCPU).Nanoseconds(),
		PeakHeapBytes:  peakHeap.Load(),
		MaxRSSBytes:    processMaxRSSBytes(t),
	}
	encoded, err := json.Marshal(metric)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("PERF_METRIC %s", encoded)
}

type performanceTesting interface {
	Helper()
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	TempDir() string
}

func compilePerformanceWorkload(tb performanceTesting, root string, expectError bool) compiler.Result {
	tb.Helper()
	compiled, err := compiler.CompileInputResultWithOptions(root, compiler.CompileOptions{})
	if err != nil {
		tb.Fatal(err)
	}
	hasErrors := diagnostic.HasErrors(compiled.Diagnostics)
	if hasErrors != expectError {
		tb.Fatalf("compiler error state = %v, want %v: %s", hasErrors, expectError, diagnostic.RenderHuman(compiled.Diagnostics, compiled.SkippedPhases))
	}
	return compiled
}

func preparePerformanceWorkload(tb performanceTesting, target generator.Target, compiled compiler.Result) generator.Preparation {
	tb.Helper()
	prepared, err := generator.PrepareCompilation(target, compiled, generator.Options{})
	if err != nil {
		tb.Fatal(err)
	}
	if diagnostic.HasErrors(prepared.Diagnostics) {
		tb.Fatalf("target preparation failed: %s", diagnostic.RenderHuman(prepared.Diagnostics, prepared.SkippedPhases))
	}
	return prepared
}

func emitPerformanceWorkload(tb performanceTesting, target generator.Target, prepared generator.Preparation) []generator.Artifact {
	tb.Helper()
	artifacts, err := target.Emit(prepared.Plan)
	if err != nil {
		tb.Fatal(err)
	}
	return artifacts
}

type performanceArtifactStats struct {
	count int
	bytes int
}

func runPerformanceGeneration(tb performanceTesting, root, output string) performanceArtifactStats {
	tb.Helper()
	compiled := compilePerformanceWorkload(tb, root, false)
	target := typescript.Generator{}
	prepared := preparePerformanceWorkload(tb, target, compiled)
	publisher, err := newArtifactPublisher(output)
	if err != nil {
		tb.Fatal(err)
	}
	defer publisher.Rollback()
	stats := performanceArtifactStats{}
	err = generator.EmitTo(target, prepared.Plan, generator.ArtifactSinkFunc(func(artifact generator.Artifact) error {
		stats.count++
		stats.bytes += len(artifact.Data)
		return publisher.WriteArtifact(artifact)
	}))
	if err != nil {
		tb.Fatal(err)
	}
	if err := publisher.Commit(); err != nil {
		tb.Fatal(err)
	}
	return stats
}

func materializePerformanceWorkload(tb performanceTesting, workload performanceWorkload) string {
	tb.Helper()
	directory := tb.TempDir()
	for name, data := range workload.files {
		path := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			tb.Fatal(err)
		}
	}
	return filepath.Join(directory, filepath.FromSlash(workload.root))
}

func performanceWorkloads() []performanceWorkload {
	return []performanceWorkload{
		selfContainedPerformanceWorkload("self-contained", 384, 384),
		externalPerformanceWorkload(96, 192),
		diagnosticPerformanceWorkload(),
		selfContainedPerformanceWorkload("high-artifact", 48, 768),
	}
}

func selfContainedPerformanceWorkload(name string, schemaCount, operationCount int) performanceWorkload {
	schemas := make(map[string]any, schemaCount)
	for index := 0; index < schemaCount; index++ {
		schemaName := fmt.Sprintf("Model%04d", index)
		nextName := fmt.Sprintf("Model%04d", (index+1)%schemaCount)
		schemas[schemaName] = map[string]any{
			"type":     "object",
			"required": []any{"id", "payload"},
			"properties": map[string]any{
				"id":      map[string]any{"type": "string", "format": "uuid"},
				"payload": map[string]any{"type": "string", "minLength": 1},
				"next":    map[string]any{"$ref": "#/components/schemas/" + nextName},
				"labels":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			},
		}
	}
	paths := make(map[string]any, operationCount)
	for index := 0; index < operationCount; index++ {
		schemaName := fmt.Sprintf("Model%04d", index%schemaCount)
		paths[fmt.Sprintf("/resources/%04d", index)] = performancePathItem(index, "#/components/schemas/"+schemaName)
	}
	document := performanceDocument(name, paths, schemas)
	return performanceWorkload{name: name, root: "openapi.json", files: map[string][]byte{"openapi.json": mustMarshalPerformance(document)}}
}

func externalPerformanceWorkload(schemaCount, operationCount int) performanceWorkload {
	files := make(map[string][]byte, schemaCount+1)
	paths := make(map[string]any, operationCount)
	for index := 0; index < schemaCount; index++ {
		name := fmt.Sprintf("Thing%03d", index)
		fileName := fmt.Sprintf("schemas/schema-%03d.json", index)
		schema := map[string]any{
			name: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":      map[string]any{"type": "string"},
					"payload": map[string]any{"type": "string"},
				},
			},
		}
		files[fileName] = mustMarshalPerformance(schema)
	}
	for index := 0; index < operationCount; index++ {
		schemaIndex := index % schemaCount
		name := fmt.Sprintf("Thing%03d", schemaIndex)
		reference := fmt.Sprintf("schemas/schema-%03d.json#/%s", schemaIndex, name)
		paths[fmt.Sprintf("/external/%04d", index)] = performancePathItem(index, reference)
	}
	files["openapi.json"] = mustMarshalPerformance(performanceDocument("external-reference", paths, nil))
	return performanceWorkload{name: "external-reference", root: "openapi.json", files: files}
}

func diagnosticPerformanceWorkload() performanceWorkload {
	document := performanceDocument("diagnostic-error", map[string]any{}, nil)
	document["x-sdkgen-invalid"] = true
	return performanceWorkload{
		name:        "diagnostic-error",
		root:        "openapi.json",
		files:       map[string][]byte{"openapi.json": mustMarshalPerformance(document)},
		expectError: true,
	}
}

func performanceDocument(title string, paths, schemas map[string]any) map[string]any {
	document := map[string]any{
		"openapi": "3.1.0",
		"info":    map[string]any{"title": title, "version": "1.0.0"},
		"paths":   paths,
	}
	if len(schemas) != 0 {
		document["components"] = map[string]any{"schemas": schemas}
	}
	return document
}

func performancePathItem(index int, reference string) map[string]any {
	return map[string]any{
		"get": map[string]any{
			"operationId": fmt.Sprintf("getResource%04d", index),
			"tags":        []any{"resources"},
			"responses": map[string]any{
				"200": map[string]any{
					"description": "OK",
					"content": map[string]any{
						"application/json": map[string]any{"schema": map[string]any{"$ref": reference}},
					},
				},
			},
		},
	}
}

func mustMarshalPerformance(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func performanceInputBytes(workload performanceWorkload) int {
	total := 0
	for _, data := range workload.files {
		total += len(data)
	}
	return total
}

func performanceArtifactBytes(artifacts []generator.Artifact) int {
	total := 0
	for _, artifact := range artifacts {
		total += len(artifact.Data)
	}
	return total
}

func performanceWorkloadHash(workload performanceWorkload) string {
	hash := sha256.New()
	names := make([]string, 0, len(workload.files))
	for name := range workload.files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		hash.Write([]byte(name))
		hash.Write([]byte{0})
		hash.Write(workload.files[name])
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func samplePeakHeap() (*atomic.Uint64, func()) {
	peak := &atomic.Uint64{}
	stop := make(chan struct{})
	done := make(chan struct{})
	sample := func() {
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		for current := peak.Load(); stats.HeapAlloc > current && !peak.CompareAndSwap(current, stats.HeapAlloc); current = peak.Load() {
		}
	}
	sample()
	go func() {
		defer close(done)
		ticker := time.NewTicker(2 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sample()
			case <-stop:
				sample()
				return
			}
		}
	}()
	return peak, func() {
		close(stop)
		<-done
	}
}

func processCPUTime(tb performanceTesting) time.Duration {
	tb.Helper()
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		tb.Fatal(err)
	}
	return timevalDuration(usage.Utime) + timevalDuration(usage.Stime)
}

func processMaxRSSBytes(tb performanceTesting) uint64 {
	tb.Helper()
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		tb.Fatal(err)
	}
	value := uint64(usage.Maxrss)
	if runtime.GOOS != "darwin" {
		value *= 1024
	}
	return value
}

func timevalDuration(value syscall.Timeval) time.Duration {
	return time.Duration(value.Sec)*time.Second + time.Duration(value.Usec)*time.Microsecond
}
