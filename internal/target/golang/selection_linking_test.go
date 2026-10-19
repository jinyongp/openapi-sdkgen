package golang

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestGeneratedStaticRegistryLinking(t *testing.T) {
	document, err := compiler.Compile([]byte(selectionFixture))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatal(err, diagnostics)
	}
	files := map[string]string{
		"cmd/static/main.go": `package main
import("context";"fmt";"example.test/consumer/api/operations";"example.test/consumer/api/selective")
func main(){ref:=operations.Source();prepared,err:=selective.Prepare(ref);if err!=nil{panic(err)};client,err:=selective.NewClient(selective.ClientOptions{BaseURL:"https://example.test"},prepared);if err!=nil{panic(err)};operation,err:=selective.Bind(client,ref);if err!=nil{panic(err)};ctx,cancel:=context.WithCancel(context.Background());cancel();_,err=operation.Call().Do(ctx);fmt.Println(err)}
`,
		"cmd/computed/main.go": `package main
import("context";"fmt";"example.test/consumer/api/selective";"example.test/consumer/api/types")
func main(){ref,ok:=selective.LookupOperation("source");if !ok{panic("missing operation")};prepared,err:=selective.Prepare(ref);if err!=nil{panic(err)};client,err:=selective.NewClient(selective.ClientOptions{BaseURL:"https://example.test"},prepared);if err!=nil{panic(err)};operation,err:=selective.Bind(client,ref);if err!=nil{panic(err)};ctx,cancel:=context.WithCancel(context.Background());cancel();_,err=operation.Call(types.Value{}).Do(ctx);fmt.Println(err)}
`,
	}
	root := testGeneratedModule(t, plan, files)
	selected, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}, Selection: &generator.Selection{Operations: []string{"source"}}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatal(err, diagnostics)
	}
	files["cmd/selected/main.go"] = files["cmd/static/main.go"]
	selectedRoot := testGeneratedModule(t, selected, map[string]string{"cmd/selected/main.go": files["cmd/selected/main.go"]})
	roots := map[string]string{"static": root, "computed": root, "selected": selectedRoot}
	result := selectionBuildMeasurements{GoVersion: selectionGoVersion(t), Flags: []string{"-mod=readonly", "-buildvcs=false", "-trimpath"}, CGOEnabled: false, FixtureSHA256: selectionDigest([]byte(selectionFixture))}
	var observed []byte
	for _, variant := range []string{"static", "computed", "selected"} {
		root := roots[variant]
		binary := filepath.Join(root, variant)
		command := exec.Command("go", "build", "-mod=readonly", "-buildvcs=false", "-trimpath", "-o", binary, "./cmd/"+variant)
		command.Dir = root
		command.Env = selectionBuildEnv("")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", variant, err, output)
		}
		nm := exec.Command("go", "tool", "nm", binary)
		symbols, err := nm.CombinedOutput()
		if err != nil {
			t.Fatalf("symbols: %v\n%s", err, symbols)
		}
		registry := strings.Contains(string(symbols), "example.test/consumer/api/internal/registry.load")
		unused := strings.Contains(string(symbols), "example.test/consumer/api/internal/operations.UnusedCall.execute")
		target := strings.Contains(string(symbols), "example.test/consumer/api/internal/operations.TargetCall.execute")
		compiler := strings.Contains(string(symbols), "go-quickjs/go-quickjs/internal/regexp.Compile")
		if registry != (variant == "computed") || unused != (variant == "computed") || target != (variant == "computed") || compiler != (variant == "computed") {
			t.Fatalf("%s registry=%t unused=%t target=%t regexp_compiler=%t", variant, registry, unused, target, compiler)
		}
		info, err := os.Stat(binary)
		if err != nil {
			t.Fatal(err)
		}
		quickJS := 0
		for _, symbol := range strings.Split(string(symbols), "\n") {
			if strings.Contains(symbol, "go-quickjs") {
				quickJS++
			}
		}
		if variant == "selected" && quickJS != 0 {
			t.Fatal("generation selection retained unused regex package")
		}
		output, err := exec.Command(binary).CombinedOutput()
		if err != nil {
			t.Fatalf("execute %s: %v\n%s", variant, err, output)
		}
		if observed == nil {
			observed = output
		} else if string(output) != string(observed) {
			t.Fatalf("required result differs: %s=%s baseline=%s", variant, output, observed)
		}
		artifactPlan := plan
		if variant == "selected" {
			artifactPlan = selected
		}
		digest, count, sourceBytes := selectionArtifactDigest(t, artifactPlan)
		result.Variants = append(result.Variants, selectionBuildVariant{Name: variant, BinaryBytes: info.Size(), Registry: registry, Unused: unused, LinkTarget: target, RegexCompiler: compiler, QuickJSSymbols: quickJS, ArtifactsSHA256: digest, GeneratedFiles: count, GeneratedBytes: sourceBytes, ProgramSHA256: selectionDigest([]byte(files["cmd/"+variant+"/main.go"]))})
		t.Logf("%s binary_bytes=%d registry=%t unused=%t quickjs_symbols=%d", variant, info.Size(), registry, unused, quickJS)
		if directory := os.Getenv("SDKGEN_GO_SELECTION_ARTIFACT_DIR"); directory != "" {
			if err := os.MkdirAll(directory, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, variant+".nm"), symbols, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	directory := os.Getenv("SDKGEN_GO_SELECTION_ARTIFACT_DIR")
	if directory != "" {
		selectionWriteMeasurements(t, directory, result)
	}
	if value := os.Getenv("SDKGEN_GO_SELECTION_BUILD_SAMPLES"); value != "" {
		samples, err := strconv.Atoi(value)
		if err != nil || samples < 1 || samples > 6 || directory == "" {
			t.Fatal("optional measurements require 1-6 samples and an artifact directory")
		}
		started := time.Now()
		for index := 0; index < samples; index++ {
			order := []string{"static", "computed", "selected"}
			if index%2 == 1 {
				order = []string{"selected", "computed", "static"}
			}
			for _, variant := range order {
				if time.Since(started) > 5*time.Minute {
					t.Fatal("build measurement budget exhausted; completed samples were preserved")
				}
				cache := filepath.Join(t.TempDir(), "cache")
				measuredBinary := filepath.Join(roots[variant], "measured-"+variant+"-"+strconv.Itoa(index))
				if _, err := os.Stat(measuredBinary); !os.IsNotExist(err) {
					t.Fatal("cold build output already exists", err)
				}
				if _, err := os.Stat(cache); !os.IsNotExist(err) {
					t.Fatal("cold build cache already exists", err)
				}
				for _, phase := range []string{"cold", "warm"} {
					ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
					command := exec.CommandContext(ctx, "go", "build", "-mod=readonly", "-buildvcs=false", "-trimpath", "-o", measuredBinary, "./cmd/"+variant)
					command.Dir = roots[variant]
					command.Env = selectionBuildEnv(cache)
					begin := time.Now()
					output, err := command.CombinedOutput()
					elapsed := time.Since(begin)
					cancel()
					if err != nil {
						os.WriteFile(filepath.Join(directory, variant+"-"+phase+"-failure.log"), output, 0600)
						t.Fatalf("%s %s failed: %v\n%s", variant, phase, err, output)
					}
					result.Samples = append(result.Samples, selectionBuildSample{Variant: variant, Phase: phase, Index: index, StartedAt: begin.UTC().Format(time.RFC3339Nano), Seconds: elapsed.Seconds()})
					selectionWriteMeasurements(t, directory, result)
				}
			}
		}
	}
}

type selectionBuildVariant struct {
	Name                                        string
	BinaryBytes                                 int64
	Registry, Unused, LinkTarget, RegexCompiler bool
	QuickJSSymbols                              int
	ArtifactsSHA256, ProgramSHA256              string
	GeneratedFiles                              int
	GeneratedBytes                              int64
}
type selectionBuildSample struct {
	Variant, Phase string
	Index          int
	StartedAt      string
	Seconds        float64
}
type selectionBuildMeasurements struct {
	GoVersion     string
	Flags         []string
	CGOEnabled    bool
	FixtureSHA256 string
	Variants      []selectionBuildVariant
	Samples       []selectionBuildSample
}

func selectionDigest(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
func selectionGoVersion(t *testing.T) string {
	t.Helper()
	value, err := exec.Command("go", "version").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(value))
}

func selectionBuildEnv(cache string) []string {
	result := append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local", "CGO_ENABLED=0", "GOPROXY=off", "GOFLAGS=", "GOCACHEPROG=")
	if cache != "" {
		result = append(result, "GOCACHE="+cache)
	}
	return result
}
func selectionWriteMeasurements(t *testing.T, directory string, value selectionBuildMeasurements) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "build-measurements.json"), append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
func selectionArtifactDigest(t *testing.T, plan generator.Plan) (string, int, int64) {
	t.Helper()
	artifacts, err := (Generator{}).Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	hashes := make(map[string]string, len(artifacts))
	var bytes int64
	for _, artifact := range artifacts {
		hashes[artifact.Path] = selectionDigest(artifact.Data)
		bytes += int64(len(artifact.Data))
	}
	data, err := json.Marshal(hashes)
	if err != nil {
		t.Fatal(err)
	}
	return selectionDigest(data), len(artifacts), bytes
}
