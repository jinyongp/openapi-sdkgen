// Command golang-compat audits the same pinned documents against both targets.
package main

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"openapi-sdkgen/internal/compatinput"
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
	"openapi-sdkgen/internal/target/golang"
	"openapi-sdkgen/internal/target/typescript"
	"openapi-sdkgen/internal/tscheck"
)

type corpus struct {
	Preparation                *compatinput.Spec `json:"preparation,omitempty"`
	ID, Input, SHA256, GitBlob string
	Bytes                      int64
}
type manifest struct {
	Corpora []corpus
	Files   []struct{ Input, SHA256 string }
}
type profile struct {
	GenerationError string                                   `json:"generationError,omitempty"`
	Compilation     *compilationResult                       `json:"compilation,omitempty"`
	Blocked         bool                                     `json:"blocked"`
	Diagnostics     []diagnostic.Diagnostic                  `json:"diagnostics,omitempty"`
	Routes          []string                                 `json:"routes,omitempty"`
	Inbound         []generator.InboundOperationInspection   `json:"inbound,omitempty"`
	Files           int                                      `json:"files"`
	Bytes           int64                                    `json:"bytes"`
	ArtifactSHA256  string                                   `json:"artifactSha256,omitempty"`
	Surfaces        map[string]generator.OperationInspection `json:"surfaces,omitempty"`
}
type compilationResult struct {
	Status        string   `json:"status"`
	Detail        string   `json:"detail,omitempty"`
	ContractTests []string `json:"contractTests,omitempty"`
}

const compilerTimeout = 20 * time.Minute
const goCompilerMemoryLimit = "3GiB"

type compilationPolicy struct {
	TimeoutSeconds   int    `json:"timeoutSeconds"`
	BuildParallelism int    `json:"buildParallelism"`
	CPUConcurrency   int    `json:"cpuConcurrency"`
	MemoryLimit      string `json:"memoryLimit"`
}

func goCompilationPolicy() compilationPolicy {
	return compilationPolicy{TimeoutSeconds: int(compilerTimeout / time.Second), BuildParallelism: 1, CPUConcurrency: 2, MemoryLimit: goCompilerMemoryLimit}
}

type report struct {
	GenerationCheckpoint  bool                  `json:"generationCheckpoint,omitempty"`
	GoCompilationPolicy   *compilationPolicy    `json:"goCompilationPolicy,omitempty"`
	VerificationRequested bool                  `json:"verificationRequested"`
	SourceCommit          string                `json:"sourceCommit"`
	SourceDirty           bool                  `json:"sourceDirty"`
	InputPreparation      *compatinput.Evidence `json:"inputPreparation,omitempty"`
	Mode                  string                `json:"mode"`
	Selection             []string              `json:"selection,omitempty"`
	ID                    string                `json:"id"`
	Input                 string                `json:"input"`
	InputSHA256           string                `json:"inputSha256"`
	Addons                []string              `json:"addons"`
	GoVersion             string                `json:"goVersion"`
	TypeScript            profile               `json:"typescript"`
	Go                    profile               `json:"golang"`
	GoExtraRoutes         []string              `json:"goExtraRoutes,omitempty"`
	Pass                  bool                  `json:"pass"`
	Failure               string                `json:"failure,omitempty"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	verify := flag.Bool("verify", false, "compile emitted modules; generation alone is not compilation evidence")
	checkpoint := flag.Bool("verification-checkpoint", false, "save generation evidence before a separate compiler attempt; never a compiler pass")
	typescriptRoot := flag.String("typescript-root", "test/typescript", "installed consumer TypeScript compiler directory")
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
	if *checkpoint && (*verify || *mode != "golang-generation" || *goOnly) {
		return fmt.Errorf("verification-checkpoint requires golang-generation without verify or diagnostic-go-only")
	}
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
	result := report{GenerationCheckpoint: *checkpoint, VerificationRequested: *verify || *checkpoint, Mode: *mode, ID: selected.ID, Input: selected.Input, InputSHA256: digest(input), GoVersion: runtime.Version()}
	if *verify || *checkpoint {
		policy := goCompilationPolicy()
		result.GoCompilationPolicy = &policy
	}
	if *server {
		result.Addons = []string{"server"}
	}
	head, headErr := exec.Command("git", "rev-parse", "HEAD").Output()
	if headErr != nil {
		return headErr
	}
	result.SourceCommit = strings.TrimSpace(string(head))
	status, statusErr := exec.Command("git", "status", "--porcelain").Output()
	if statusErr != nil {
		return statusErr
	}
	result.SourceDirty = len(status) != 0
	preparedInput, err := compatinput.Prepare(absolute, selected.Preparation)
	if err != nil {
		return err
	}
	defer preparedInput.Close()
	absolute = preparedInput.Path
	result.InputPreparation = preparedInput.Evidence
	phase("compile-start")
	compiled, err := compiler.CompileFileResultWithOptions(absolute, compiler.CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		return err
	}
	phase("compile-finished")
	if compiled.Document != nil {
		if err := preparedInput.Verify(compiled.Document); err != nil {
			return err
		}
	}

	registry, _ := generator.NewAddonRegistry(generator.AddonServer, generator.AddonMetadata)
	options, err := registry.Resolve(result.Addons)
	if err != nil {
		return err
	}
	options.DiagnosticMode = diagnostic.ModeCollect
	options.TypeScriptTypeCheck = verify

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
		result.TypeScript, err = prepareAndEmitVerified(typescript.Generator{}, compiled, options, *verify, *typescriptRoot)
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
	options.TypeScriptTypeCheck = nil
	options.Golang = &generator.GoOptions{ImportPath: "example.test/corpus/api"}
	var contracts []string
	if *verify {
		contractRoute := ""
		switch selected.ID {
		case "loket.nl":
			contracts = []string{"loket_http_test.go.txt"}
			contractRoute = "PATCH /providers/employers/conceptemployees/import/{payrollAdministrationId}"
		case "stripe":
			contracts = []string{"stripe_http_test.go.txt"}
			contractRoute = "GET /v1/balance"
		case "cloudflare":
			contracts = []string{"cloudflare_http_test.go.txt"}
			contractRoute = "GET /user/tokens/verify"
		}
		if len(result.Selection) != 0 && !slices.Contains(result.Selection, contractRoute) {
			contracts = nil
		}
	}
	if *verify && len(result.Selection) == 0 && selected.Preparation != nil {
		switch selected.Preparation.Kind {
		case compatinput.DigitalOceanTagsV1:
			contracts = []string{"digitalocean_http_test.go.txt"}
		case compatinput.SpoonacularParametersV1:
			contracts = []string{"spoonacular_http_test.go.txt"}
		}
	}
	result.Go, err = prepareAndEmitVerified(golang.Generator{}, compiled, options, *verify, *typescriptRoot, contracts...)
	if err != nil {
		return fmt.Errorf("Go: %w", err)
	}
	normalizeDiagnostics(&result.TypeScript, *root, absolute, selected.Input)
	normalizeDiagnostics(&result.Go, *root, absolute, selected.Input)
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
	if *verify && (result.Go.Compilation == nil || result.Go.Compilation.Status != "pass" || *mode == "parity" && (result.TypeScript.Compilation == nil || result.TypeScript.Compilation.Status != "pass")) {
		result.Pass = false
		if result.Failure == "" {
			result.Failure = "generated module compilation failed"
		}
	}
	if *checkpoint && !result.Go.Blocked {
		result.Go.Compilation = &compilationResult{Status: "not-run"}
		result.Pass = false
		result.Failure = "compiler verification has not run"
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
	if *checkpoint {
		fmt.Fprintf(os.Stderr, "%s: Go generation checkpoint saved; blocked=%t; compiler not run\n", result.ID, result.Go.Blocked)
		return nil
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
	return prepareAndEmitVerified(target, compiled, options, false, "")
}
func prepareAndEmitVerified(target generator.Target, compiled compiler.Result, options generator.Options, verify bool, typescriptRoot string, contracts ...string) (profile, error) {
	phase(target.Name() + "-prepare-start")
	prepared, err := generator.PrepareCompilation(target, compiled, options)
	if err != nil {
		return profile{Blocked: true, GenerationError: err.Error()}, nil
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
	work := ""
	if verify {
		work, err = os.MkdirTemp("", "sdkgen-compat-*")
		if err != nil {
			return result, err
		}
		defer os.RemoveAll(work)
	}
	hashes := make(map[string]string)
	phase(target.Name() + "-emit-start")
	err = generator.EmitTo(target, prepared.Plan, generator.ArtifactSinkFunc(func(artifact generator.Artifact) error {
		if verify {
			clean := filepath.Clean(filepath.FromSlash(artifact.Path))
			if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				return fmt.Errorf("unsafe generated artifact path")
			}
			path := filepath.Join(work, clean)
			if target.Name() == "golang" {
				path = filepath.Join(work, "api", filepath.FromSlash(artifact.Path))
			}
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(path, artifact.Data, 0600); err != nil {
				return err
			}
		}
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
	if verify && err == nil {
		// Inspect all plan-owned metadata before releasing its source strings.
		// The separate compiler processes need only the emitted artifacts.
		prepared.Plan = generator.Plan{}
		runtime.GC()
		result.Compilation = verifyModule(target.Name(), work, typescriptRoot, contracts...)
	}
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

func verifyModule(target, directory, typescriptRoot string, contracts ...string) *compilationResult {
	ctx, cancel := context.WithTimeout(context.Background(), compilerTimeout)
	defer cancel()
	var command *exec.Cmd
	if target == "golang" {
		for _, name := range contracts {
			if filepath.Base(name) != name || !strings.HasSuffix(name, "_test.go.txt") {
				return &compilationResult{Status: "fail", Detail: "invalid provider contract fixture"}
			}
			data, err := os.ReadFile(filepath.Join("test", "compatibility", "conformance", name))
			if err != nil {
				return &compilationResult{Status: "fail", Detail: err.Error()}
			}
			if err := os.WriteFile(filepath.Join(directory, strings.TrimSuffix(name, ".txt")), data, 0600); err != nil {
				return &compilationResult{Status: "fail", Detail: err.Error()}
			}
		}
		if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module example.test/corpus\n\ngo 1.27.0\n"), 0600); err != nil {
			return &compilationResult{Status: "fail", Detail: err.Error()}
		}
		policy := goCompilationPolicy()
		args := []string{"test", fmt.Sprintf("-p=%d", policy.BuildParallelism), "-mod=readonly"}
		if len(contracts) != 0 {
			args = append(args, "-json", "-count=1")
		}
		command = exec.CommandContext(ctx, "go", append(args, "./...")...)
		// Bound each compiler independently of the generator's inherited heap
		// budget. Serial package builds avoid multiplying that memory budget.
		command.Env = append(os.Environ(), "GOMEMLIMIT="+policy.MemoryLimit, "GOMAXPROCS="+fmt.Sprint(policy.CPUConcurrency), "GOWORK=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOFLAGS=")
	} else {
		options := tscheck.CompilerOptions()
		for key, value := range map[string]any{"target": "ES2022", "module": "NodeNext", "moduleResolution": "NodeNext", "lib": []string{"ES2022", "DOM", "DOM.Iterable"}, "noEmit": true} {
			options[key] = value
		}
		config, err := json.Marshal(map[string]any{"compilerOptions": options, "include": []string{"**/*.ts"}})
		if err != nil {
			return &compilationResult{Status: "fail", Detail: err.Error()}
		}
		for name, data := range map[string][]byte{"tsconfig.json": config, "package.json": []byte("{\"type\":\"module\",\"private\":true}")} {
			if err := os.WriteFile(filepath.Join(directory, name), data, 0600); err != nil {
				return &compilationResult{Status: "fail", Detail: err.Error()}
			}
		}
		root, err := filepath.Abs(typescriptRoot)
		if err != nil {
			return &compilationResult{Status: "fail", Detail: err.Error()}
		}
		args := []string{filepath.Join(root, "node_modules/typescript/lib/tsc.js"), "--project", filepath.Join(directory, "tsconfig.json")}
		var metadata struct{ Version string }
		raw, err := os.ReadFile(filepath.Join(root, "node_modules/typescript/package.json"))
		if err != nil {
			return &compilationResult{Status: "fail", Detail: err.Error()}
		}
		if err := json.Unmarshal(raw, &metadata); err != nil {
			return &compilationResult{Status: "fail", Detail: err.Error()}
		}
		if strings.HasPrefix(metadata.Version, "7.") {
			args = append(args, "--singleThreaded")
		}
		command = exec.CommandContext(ctx, "node", args...)
	}
	command.Dir = directory
	command.WaitDelay = 5 * time.Second
	output, err := command.CombinedOutput()
	if err != nil {
		detail := strings.ReplaceAll(strings.TrimSpace(string(output)), directory, "<generated>")
		if ctx.Err() != nil {
			detail = "compiler time limit exceeded (20 minutes)"
		}
		if detail == "" {
			detail = err.Error()
		}
		if len(detail) > 4000 {
			detail = detail[:4000]
		}
		return &compilationResult{Status: "fail", Detail: detail}
	}
	result := &compilationResult{Status: "pass"}
	if len(contracts) != 0 {
		for _, line := range strings.Split(string(output), "\n") {
			var event struct{ Action, Test string }
			if json.Unmarshal([]byte(line), &event) == nil && event.Action == "pass" && strings.HasSuffix(event.Test, "HTTPContract") {
				result.ContractTests = append(result.ContractTests, event.Test)
			}
		}
		if len(result.ContractTests) != len(contracts) {
			return &compilationResult{Status: "fail", Detail: "provider HTTP contract tests did not complete"}
		}
	}
	return result
}

func normalizeDiagnostics(profile *profile, root, input, logicalInput string) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return
	}
	source := func(value string) string {
		if value == input {
			return filepath.ToSlash(logicalInput)
		}
		if strings.HasPrefix(value, absolute+string(filepath.Separator)) {
			return filepath.ToSlash(strings.TrimPrefix(value, absolute+string(filepath.Separator)))
		}
		return value
	}
	for index := range profile.Diagnostics {
		value := &profile.Diagnostics[index]
		value.Location.Source = source(value.Location.Source)
		for related := range value.Related {
			value.Related[related].Source = source(value.Related[related].Source)
		}
	}
}
