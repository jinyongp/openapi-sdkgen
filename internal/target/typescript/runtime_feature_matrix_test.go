package typescript

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type runtimeMatrixFixture struct {
	Name       string   `json:"name"`
	Group      string   `json:"group"`
	Input      string   `json:"input"`
	Operations []string `json:"operations"`
	Addons     []string `json:"addons"`
}

func TestRuntimeFeatureDeclarationMatrixRegression(t *testing.T) {
	fixtureRoot := filepath.Join("..", "..", "..", "test", "typescript", "fixtures")
	data, err := os.ReadFile(filepath.Join(fixtureRoot, "runtime-features", "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var catalog runtimeMatrixCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	registry, err := generator.NewAddonRegistry(generator.AddonServer, generator.AddonMetadata)
	if err != nil {
		t.Fatal(err)
	}
	var combined []Artifact
	var probe strings.Builder
	for index, fixture := range catalog.Fixtures {
		input, err := os.ReadFile(filepath.Join(fixtureRoot, fixture.Input))
		if err != nil {
			t.Fatal(err)
		}
		document, err := sdkgen.Compile(input)
		if err != nil {
			t.Fatal(err)
		}
		options, err := registry.Resolve(fixture.Addons)
		if err != nil {
			t.Fatal(err)
		}
		if len(fixture.Operations) > 0 {
			options.Selection = &generator.Selection{Operations: fixture.Operations}
		}
		artifacts, err := (Generator{}).Generate(document, options)
		if err != nil {
			t.Fatal(err)
		}
		prefix := fmt.Sprintf("sdk%d/", index)
		for _, artifact := range artifacts {
			combined = append(combined, Artifact{Path: prefix + artifact.Path, Data: artifact.Data})
		}
		fmt.Fprintf(&probe, "import { createClient as create%d, isOperationHTTPError as guard%d } from './%sindex.js';\nimport type { OperationHTTPError as Failure%d } from './%sindex.js';\nconst api%d = create%d({baseURL:'https://matrix.test'});\ntype Calls%d = typeof api%d.$routes[keyof typeof api%d.$routes];\ndeclare const failure%d: Failure%d<Calls%d>;\nvoid [api%d,guard%d,failure%d];\n", index, index, prefix, index, prefix, index, index, index, index, index, index, index, index, index, index, index)
		if fixture.Group == "server" {
			fmt.Fprintf(&probe, "import {createWebhookRouter as router%d} from './%sserver/webhooks.js';\nimport {decodeInboundBody as decode%d} from './%sserver/runtime.js';\nvoid [router%d,decode%d];\n", index, prefix, index, prefix, index, index)
		}
	}
	compileDeclarationConsumers(t, combined, probe.String())
}

type runtimeMatrixCatalog struct {
	Version  int                    `json:"version"`
	Fixtures []runtimeMatrixFixture `json:"fixtures"`
}

// The full-capability control is composed from exactly the tested source basis.
// Only test preparation changes; no AST rewriting or historical baseline runs.
func fullCapabilityMatrixPlan(plan *sourcePlan) {
	// The control executes the canonical arbitrary-schema implementation. Keep
	// generated programs out of this test-only graph so the size comparison does
	// not charge the control for two independent execution algorithms.
	plan.schemaPrograms = nil
	plan.modules.schemaPrograms = nil
	plan.serverSchemaPrograms = nil
	if plan.root != nil {
		plan.root.schemaPrograms = nil
		plan.root.modules.schemaPrograms = nil
	}
	for _, client := range plan.clients {
		client.view.schemaPrograms = nil
		client.view.modules.schemaPrograms = nil
	}
	plan.callbacks, _ = collectCallbacksDiagnostics(plan.document, plan.omittedOperations)
	plan.webhooks, _ = collectWebhooksDiagnostics(plan.document)
	full := runtimeComposition{imports: []runtimeHandlerImport{
		{path: "internal/runtime/compatibility/http-codecs.ts", names: []string{"fullRequestServices"}},
		{path: "internal/runtime/http/request/http-request-core.ts", names: []string{"createRequestCore"}},
	}, httpTypes: []string{"StreamingRequestExecutionServices"}, servicesType: "StreamingRequestExecutionServices", declarations: []string{"const services: StreamingRequestExecutionServices = fullRequestServices"}}
	root := plan
	if plan.root != nil {
		root = plan.root
	}
	root.modules.runtimeComposition = full
	for route, execution := range plan.executions {
		execution.composition = full
		if execution.inputBundle == "" && len(execution.inputSchemas) > 0 || execution.outputBundle == "" && len(execution.outputSchemas) > 0 {
			execution.composition.wireTypes = []string{"WireSchemas"}
		}
		plan.executions[route] = execution
	}
	plan.runtimeArtifacts = append([]runtimeTemplateArtifact(nil), runtimeTemplateArtifacts...)
	plan.serverRuntimeArtifacts = nil
	for kind := range plan.serverCompositions {
		plan.serverCompositions[kind] = []byte("export * from './runtime.js'\n")
	}
}

func copyRuntimeMatrixOutput(t *testing.T, source, target string) {
	t.Helper()
	// Each target is an owned fixture output. A previous program identity must
	// not remain in a fresh source/native inventory after repeated measurements.
	if err := os.RemoveAll(target); err != nil {
		t.Fatal(err)
	}
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0700)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeFeatureNativeMatrixRegression(t *testing.T) {
	fixtureRoot := filepath.Join("..", "..", "..", "test", "typescript", "fixtures")
	data, err := os.ReadFile(filepath.Join(fixtureRoot, "runtime-features", "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var catalog runtimeMatrixCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	output := os.Getenv("SDKGEN_RUNTIME_MATRIX_DIR")
	if output == "" {
		output = t.TempDir()
	}
	output, err = filepath.Abs(output)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := generator.NewAddonRegistry(generator.AddonServer, generator.AddonMetadata)
	if err != nil {
		t.Fatal(err)
	}
	serverCatalog := runtimeMatrixCatalog{Version: 1}
	for _, fixture := range catalog.Fixtures {
		t.Run(fixture.Group+"/"+fixture.Name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join(fixtureRoot, fixture.Input))
			if err != nil {
				t.Fatal(err)
			}
			document, err := sdkgen.Compile(input)
			if err != nil {
				t.Fatal(err)
			}
			options, err := registry.Resolve(fixture.Addons)
			if err != nil {
				t.Fatal(err)
			}
			if len(fixture.Operations) > 0 {
				options.Selection = &generator.Selection{Operations: fixture.Operations}
			}
			for _, variant := range []string{"selected", "full"} {
				plan, diagnostics, err := (Generator{}).Prepare(document, options)
				if err != nil || diagnostic.HasErrors(diagnostics) {
					t.Fatalf("prepare %s: %v %v", variant, err, diagnostics)
				}
				if variant == "full" {
					value, _ := plan.Value("typescript")
					fullCapabilityMatrixPlan(value.(*sourcePlan))
				}
				artifacts, err := (Generator{}).Emit(plan)
				if err != nil {
					t.Fatal(err)
				}
				// Keep strict source beside JS for optional reproducible bundle measurement.
				base := output
				if fixture.Group == "server" {
					base = filepath.Join(base, "server")
				}
				checked := make([]Artifact, len(artifacts))
				for index, artifact := range artifacts {
					checked[index] = Artifact{Path: artifact.Path, Data: bytes.ReplaceAll(artifact.Data, []byte("// @ts-nocheck\n"), nil)}
				}
				writeTargetArtifacts(t, filepath.Join(base, variant, fixture.Name), checked)
				compiled := compileTypeScriptArtifactSet(t, artifacts, "consumer.ts", `import {createClient} from './index.js'; void createClient;`)
				copyRuntimeMatrixOutput(t, compiled, filepath.Join(base, variant+"-js", fixture.Name))
			}
		})
		if fixture.Group == "server" {
			serverCatalog.Fixtures = append(serverCatalog.Fixtures, fixture)
		}
	}
	if t.Failed() {
		return
	}
	report, err := json.Marshal(serverCatalog)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "server", "report.json"), report, 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"native-basic", "native-features", "native-extra", "native-server", "graph"} {
		script := filepath.Join("..", "..", "..", "test", "typescript", "verification", "runtime-feature-"+name+".mjs")
		command := exec.Command("node", script, output)
		if result, err := command.CombinedOutput(); err != nil {
			t.Fatalf("runtime feature %s: %v\n%s", name, err, result)
		} else {
			t.Logf("%s", result)
		}
	}
}
