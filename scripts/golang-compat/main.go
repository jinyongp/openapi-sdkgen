// Command golang-compat audits the same pinned documents against both targets.
package main

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
	"time"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
	"openapi-sdkgen/internal/target/golang"
	"openapi-sdkgen/internal/target/typescript"
)

type corpus struct {
	ID, Input, SHA256, GitBlob string
	Bytes                      int64
}
type manifest struct {
	Corpora []corpus
	Files   []struct{ Input, SHA256 string }
}
type profile struct {
	Blocked        bool                                     `json:"blocked"`
	Diagnostics    []diagnostic.Diagnostic                  `json:"diagnostics,omitempty"`
	Routes         []string                                 `json:"routes,omitempty"`
	Inbound        []generator.InboundOperationInspection   `json:"inbound,omitempty"`
	Files          int                                      `json:"files"`
	Bytes          int64                                    `json:"bytes"`
	ArtifactSHA256 string                                   `json:"artifactSha256,omitempty"`
	Surfaces       map[string]generator.OperationInspection `json:"surfaces,omitempty"`
}
type report struct {
	Mode          string   `json:"mode"`
	Selection     []string `json:"selection,omitempty"`
	ID            string   `json:"id"`
	Input         string   `json:"input"`
	InputSHA256   string   `json:"inputSha256"`
	Addons        []string `json:"addons"`
	GoVersion     string   `json:"goVersion"`
	TypeScript    profile  `json:"typescript"`
	Go            profile  `json:"golang"`
	GoExtraRoutes []string `json:"goExtraRoutes,omitempty"`
	Pass          bool     `json:"pass"`
	Failure       string   `json:"failure,omitempty"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	mode := flag.String("mode", "parity", "parity or golang-generation; generation mode proves no cross-target parity")
	selectCount := flag.Int("select-count", 0, "select first N exact original route keys, sorted; zero selects all")
	partialOutput := flag.String("partial-output", "", "optional partial report before Go; pass is false until both targets finish")
	manifestPath := flag.String("manifest", "", "pinned corpus manifest")
	root := flag.String("corpus-root", "", "directory containing original input paths")
	document := flag.String("document", "", "one original corpus ID")
	output := flag.String("output", "", "JSON report path")
	goOnly := flag.Bool("diagnostic-go-only", false, "profile Go preparation only; never a passing parity audit")
	server := flag.Bool("server", false, "enable the server addon for both targets")
	profilePrefix := flag.String("profile", "", "optional private CPU/heap profile prefix; intrusive diagnostic run")
	deadline := flag.Duration("deadline", 0, "optional diagnostic process deadline; timeout is never a passing result")
	var exactRoutes []string
	flag.Func("route", "repeat an exact original route key for generation selection", func(value string) error { exactRoutes = append(exactRoutes, value); return nil })
	flag.Parse()
	if *deadline > 0 && *profilePrefix == "" {
		return fmt.Errorf("deadline requires a diagnostic profile")
	}
	if *goOnly && *mode != "parity" {
		return fmt.Errorf("diagnostic-go-only cannot be combined with generation measurement mode")
	}
	if *profilePrefix != "" {
		output, err := os.OpenFile(*profilePrefix+".cpu", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		if err := pprof.StartCPUProfile(output); err != nil {
			output.Close()
			return err
		}
		var once sync.Once
		stop := func() { once.Do(func() { pprof.StopCPUProfile(); output.Close() }) }
		defer stop()
		if *deadline > 0 {
			timer := time.AfterFunc(*deadline, func() {
				phase("diagnostic-timeout")
				stop()
				heap, err := os.OpenFile(*profilePrefix+".heap", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
				if err == nil {
					pprof.WriteHeapProfile(heap)
					heap.Close()
				}
				os.Exit(124)
			})
			defer timer.Stop()
		}
	}

	if *mode != "parity" && *mode != "golang-generation" {
		return fmt.Errorf("unknown mode %q", *mode)
	}
	if *selectCount < 0 {
		return fmt.Errorf("select-count must be nonnegative")
	}
	if *manifestPath == "" || *root == "" || *document == "" || *output == "" {
		return fmt.Errorf("manifest, corpus-root, document and output are required")
	}
	data, err := os.ReadFile(*manifestPath)
	if err != nil {
		return err
	}
	var entries manifest
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	var selected *corpus
	for index := range entries.Corpora {
		if entries.Corpora[index].ID == *document {
			selected = &entries.Corpora[index]
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("unknown pinned document %q", *document)
	}
	for _, file := range entries.Files {
		if _, err := verifiedInput(*root, corpus{Input: file.Input, SHA256: file.SHA256}); err != nil {
			return err
		}
	}
	input, err := verifiedInput(*root, *selected)
	if err != nil {
		return err
	}
	absolute, err := filepath.Abs(filepath.Join(*root, filepath.FromSlash(selected.Input)))
	if err != nil {
		return err
	}
	result := report{Mode: *mode, ID: selected.ID, Input: selected.Input, InputSHA256: digest(input), GoVersion: runtime.Version()}
	if *server {
		result.Addons = []string{"server"}
	}
	phase("compile-start")
	compiled, err := compiler.CompileFileResultWithOptions(absolute, compiler.CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		return err
	}
	phase("compile-finished")
	registry, _ := generator.NewAddonRegistry(generator.AddonServer, generator.AddonMetadata)
	options, err := registry.Resolve(result.Addons)
	if err != nil {
		return err
	}
	options.DiagnosticMode = diagnostic.ModeCollect
	if len(exactRoutes) != 0 {
		if *selectCount != 0 {
			return fmt.Errorf("route and select-count cannot be combined")
		}
		selection, err := (&generator.Selection{Routes: exactRoutes}).Canonical()
		if err != nil {
			return err
		}
		result.Selection = selection.Routes
		options.Selection = selection
	}
	if *selectCount > 0 {
		if compiled.Document == nil {
			return fmt.Errorf("cannot select operations from a blocked compilation")
		}
		routes := make([]string, 0, len(compiled.Document.Operations))
		for _, operation := range compiled.Document.Operations {
			routes = append(routes, operation.RouteKey)
		}
		sort.Strings(routes)
		if len(routes) < *selectCount {
			return fmt.Errorf("document has fewer than %d operations", *selectCount)
		}
		result.Selection = append([]string(nil), routes[:*selectCount]...)
		options.Selection = &generator.Selection{Routes: result.Selection}
	}
	if *goOnly && (*profilePrefix == "" || *deadline <= 0) {
		return fmt.Errorf("diagnostic-go-only requires a profile and bounded deadline")
	}
	if !*goOnly && *mode == "parity" {
		result.TypeScript, err = prepareAndEmit(typescript.Generator{}, compiled, options)
		if err != nil {
			return fmt.Errorf("TypeScript: %w", err)
		}
	}
	if *partialOutput != "" {
		partial, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(*partialOutput, append(partial, '\n'), 0600); err != nil {
			return err
		}
	}
	runtime.GC()
	options.Golang = &generator.GoOptions{ImportPath: "example.test/corpus/api"}
	result.Go, err = prepareAndEmit(golang.Generator{}, compiled, options)
	if err != nil {
		return fmt.Errorf("Go: %w", err)
	}
	if *goOnly {
		result.Failure = "Go-only diagnostic; not a parity audit"
	} else if *mode == "golang-generation" {
		result.Pass = !result.Go.Blocked
		if !result.Pass {
			result.Failure = "Go generation was blocked"
		}
	} else if err := compare(&result); err != nil {
		result.Failure = err.Error()
	} else {
		result.Pass = true
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(*output, append(encoded, '\n'), 0600); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%s server=%t: TS routes=%d/inbound=%d, Go routes=%d/inbound=%d, pass=%t\n", result.ID, *server, len(result.TypeScript.Routes), len(result.TypeScript.Inbound), len(result.Go.Routes), len(result.Go.Inbound), result.Pass)
	if !result.Pass {
		return fmt.Errorf("%s: %s", selected.ID, result.Failure)
	}
	return nil
}
func digest(data []byte) string { value := sha256.Sum256(data); return hex.EncodeToString(value[:]) }
func verifiedInput(root string, item corpus) ([]byte, error) {
	clean := filepath.Clean(filepath.FromSlash(item.Input))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("unsafe input path %q", item.Input)
	}
	data, err := os.ReadFile(filepath.Join(root, clean))
	if err != nil {
		return nil, err
	}
	if item.Bytes > 0 && int64(len(data)) != item.Bytes {
		return nil, fmt.Errorf("input size mismatch: %s", item.Input)
	}
	if item.SHA256 != "" {
		if digest(data) != item.SHA256 {
			return nil, fmt.Errorf("input SHA256 mismatch: %s", item.Input)
		}
	} else if item.GitBlob != "" {
		sum := sha1.New()
		fmt.Fprintf(sum, "blob %d%c", len(data), byte(0))
		sum.Write(data)
		if hex.EncodeToString(sum.Sum(nil)) != item.GitBlob {
			return nil, fmt.Errorf("input git blob mismatch: %s", item.Input)
		}
	} else {
		return nil, fmt.Errorf("input has no immutable pin: %s", item.Input)
	}
	return data, nil
}
func prepareAndEmit(target generator.Target, compiled compiler.Result, options generator.Options) (profile, error) {
	phase(target.Name() + "-prepare-start")
	prepared, err := generator.PrepareCompilation(target, compiled, options)
	if err != nil {
		return profile{}, err
	}
	phase(target.Name() + "-prepare-finished")
	result := profile{Diagnostics: prepared.Diagnostics, Blocked: diagnostic.HasErrors(prepared.Diagnostics)}
	if result.Blocked {
		return result, nil
	}
	inspector, ok := target.(generator.PlanInspector)
	if !ok {
		return result, fmt.Errorf("target has no plan inspector")
	}
	result.Surfaces, err = inspector.InspectPlan(prepared.Plan)
	if err != nil {
		return result, err
	}
	hashes := make(map[string]string)
	phase(target.Name() + "-emit-start")
	err = generator.EmitTo(target, prepared.Plan, generator.ArtifactSinkFunc(func(artifact generator.Artifact) error {
		result.Files++
		result.Bytes += int64(len(artifact.Data))
		hashes[artifact.Path] = digest(artifact.Data)
		return nil
	}))
	if err != nil {
		return result, err
	}
	phase(target.Name() + "-emit-finished")
	names := make([]string, 0, len(hashes))
	for name := range hashes {
		names = append(names, name)
	}
	sort.Strings(names)
	hash := sha256.New()
	for _, name := range names {
		fmt.Fprintf(hash, "%s\x00%s\n", name, hashes[name])
	}
	result.ArtifactSHA256 = hex.EncodeToString(hash.Sum(nil))
	if target.Name() == "golang" {
		result.Routes, err = golang.EmissionRoutes(prepared.Plan)
		if err == nil {
			result.Inbound, err = golang.EmissionInbound(prepared.Plan)
		}
	} else {
		result.Routes, err = typescript.EmissionRoutes(prepared.Plan)
		if err == nil {
			result.Inbound, err = typescript.EmissionInbound(prepared.Plan)
		}
	}
	sort.Strings(result.Routes)
	sort.Slice(result.Inbound, func(i, j int) bool { return result.Inbound[i].Identity < result.Inbound[j].Identity })
	return result, err
}
func compare(result *report) error {
	result.GoExtraRoutes = nil
	if result.Go.Blocked && !result.TypeScript.Blocked {
		return fmt.Errorf("new Go document block")
	}
	if result.Go.Blocked {
		return nil
	}
	goRoutes := make(map[string]bool)
	tsRoutes := make(map[string]bool)
	for _, route := range result.Go.Routes {
		if goRoutes[route] {
			return fmt.Errorf("Go emitted duplicate route %s", route)
		}
		goRoutes[route] = true
	}
	for _, route := range result.TypeScript.Routes {
		tsRoutes[route] = true
		if !goRoutes[route] {
			return fmt.Errorf("Go did not emit TS route %s", route)
		}
	}
	for route := range goRoutes {
		if _, present := result.TypeScript.Surfaces[route]; !present {
			return fmt.Errorf("Go emitted an undeclared extra route %s", route)
		}
	}
	for route, want := range result.TypeScript.Surfaces {
		got, present := result.Go.Surfaces[route]
		if !present {
			return fmt.Errorf("Go lost declared route %s", route)
		}
		if want.Status == "hidden" && got.Status != "hidden" {
			return fmt.Errorf("Go exposed hidden route %s", route)
		}
		if goRoutes[route] && !tsRoutes[route] {
			if want.Status != "omitted" || !fetchOnlyOmission(result.TypeScript.Diagnostics, route) {
				return fmt.Errorf("unexplained Go extra route %s", route)
			}
			result.GoExtraRoutes = append(result.GoExtraRoutes, route)
		}
	}
	received := make(map[string]generator.InboundOperationInspection)
	for _, entry := range result.Go.Inbound {
		if _, exists := received[entry.Identity]; exists {
			return fmt.Errorf("Go emitted duplicate inbound identity %s", entry.Identity)
		}
		received[entry.Identity] = entry
	}
	for _, entry := range result.TypeScript.Inbound {
		if got, present := received[entry.Identity]; !present || got != entry {
			return fmt.Errorf("Go inbound identity differs: %s", entry.Identity)
		}
	}
	for _, entry := range result.Go.Inbound {
		found := false
		for _, expected := range result.TypeScript.Inbound {
			if entry.Identity == expected.Identity {
				found = true
				break
			}
		}
		if !found {
			nativeSource := false
			for _, route := range result.GoExtraRoutes {
				if entry.SourceRoute == route {
					nativeSource = true
					break
				}
			}
			if !nativeSource {
				return fmt.Errorf("unexplained Go inbound identity %s", entry.Identity)
			}
		}
	}
	sort.Strings(result.GoExtraRoutes)
	return nil
}

func fetchOnlyOmission(findings []diagnostic.Diagnostic, route string) bool {
	for _, finding := range findings {
		if finding.Code == "SDKGEN-W511" && finding.Target == "typescript" && finding.Scope == "operation" && finding.Route == route && finding.Effect == "omit-operation" && strings.Contains(strings.ToLower(finding.Message), "fetch") {
			return true
		}
	}
	return false
}

func phase(name string) {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	fmt.Fprintf(os.Stderr, "phase=%s at=%s liveHeap=%d heapSys=%d GC=%d\n", name, time.Now().UTC().Format(time.RFC3339), memory.HeapAlloc, memory.HeapSys, memory.NumGC)
}
