package golang

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedStandardLibraryOnlyModuleGraph(t *testing.T) {
	var selected, inbound map[string]any
	if err := json.Unmarshal([]byte(selectionFixture), &selected); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(inboundScopeFixture), &inbound); err != nil {
		t.Fatal(err)
	}
	selected["openapi"] = inbound["openapi"]
	selected["webhooks"] = inbound["webhooks"]
	for route, value := range inbound["paths"].(map[string]any) {
		selected["paths"].(map[string]any)[route] = value
	}
	input, err := json.Marshal(selected)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := compiler.Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	addons, _ := generator.NewAddonRegistry(generator.AddonServer)
	options, err := addons.Resolve([]string{"server"})
	if err != nil {
		t.Fatal(err)
	}
	options.Golang = &generator.GoOptions{ImportPath: "example.test/consumer/api"}
	options.Clients = map[string]generator.Client{"source-only": {Selection: &generator.Selection{Operations: []string{"source"}}}, "other": {Selection: &generator.Selection{Operations: []string{"unused"}}}}
	plan, diagnostics, err := (Generator{}).Prepare(doc, options)
	if err != nil || len(diagnostics) > 0 {
		t.Fatal(err, diagnostics)
	}
	root := testGeneratedModule(t, plan, map[string]string{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cache := t.TempDir()
	env := append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "CGO_ENABLED=0", "GOMODCACHE="+cache)
	run := func(args ...string) []byte {
		command := exec.CommandContext(ctx, "go", args...)
		command.Dir = root
		command.Env = env
		data, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("go %v: %v\n%s", args, err, data)
		}
		return data
	}
	moduleData := run("list", "-m", "-json", "all")
	var module struct {
		Path    string
		Main    bool
		Replace any
	}
	if err := json.Unmarshal(moduleData, &module); err != nil || !module.Main || module.Path != "example.test/consumer" || module.Replace != nil {
		t.Fatal("external/replaced modules", string(moduleData), err)
	}
	data := run("list", "-mod=readonly", "-deps", "-json", "./...")
	decoder := json.NewDecoder(bytes.NewReader(data))
	standard, owned := 0, 0
	for {
		var pkg struct {
			ImportPath string
			Standard   bool
			Module     *struct {
				Main    bool
				Replace any
			}
		}
		err := decoder.Decode(&pkg)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if pkg.Standard {
			standard++
			continue
		}
		if pkg.Module == nil || !pkg.Module.Main || pkg.Module.Replace != nil || !strings.HasPrefix(pkg.ImportPath, "example.test/consumer/") {
			t.Fatal("external package", pkg.ImportPath)
		}
		owned++
	}
	run("test", "-mod=readonly", "-count=1", "./...")
	if _, err := os.Stat(filepath.Join(root, "go.sum")); !os.IsNotExist(err) {
		t.Fatal("standard-library-only module produced go.sum", err)
	}
	entries, err := os.ReadDir(cache)
	if err != nil || len(entries) > 0 {
		t.Fatal("consumer downloaded module cache contents", err, len(entries))
	}
	t.Logf("main modules=1 external modules=0 standard packages=%d owned packages=%d; CGO0 all modes/server/callbacks/webhooks compile", standard, owned)
}
