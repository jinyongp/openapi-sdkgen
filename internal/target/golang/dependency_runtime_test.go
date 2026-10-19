package golang

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestGeneratedHTTPDependencyBoundaries(t *testing.T) {
	_, plan := healthPlan(t)
	root := testGeneratedModule(t, plan, map[string]string{"consumer_test.go": consumerTest})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "list", "-mod=readonly", "-deps", "-json", "./api/...")
	command.Dir = root
	command.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("generated package graph: %v\n%s", err, output)
	}
	type packageInfo struct {
		ImportPath string
		Imports    []string
		Standard   bool
	}
	packages := make(map[string]packageInfo)
	decoder := json.NewDecoder(bytes.NewReader(output))
	for {
		var item packageInfo
		err := decoder.Decode(&item)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		packages[item.ImportPath] = item
	}
	const prefix = "example.test/consumer/api"
	layer := func(name string) string {
		if name == prefix {
			return "root"
		}
		return strings.TrimPrefix(name, prefix+"/")
	}
	allowed := map[string]map[string]bool{
		"root":                {"internal/core": true, "internal/operations": true, "internal/resources": true, "models": true, "types": true},
		"internal/resources":  {"internal/core": true, "internal/operations": true, "models": true, "types": true},
		"internal/operations": {"internal/core": true, "internal/schema": true, "models": true, "types": true},
		"internal/core":       {"internal/media": true, "internal/security": true, "internal/transport": true, "types": true},
		"models":              {"internal/schema": true, "internal/wire": true, "types": true},
		"internal/media":      {"internal/schema": true, "internal/wire": true, "internal/stream": true, "types": true},
		"internal/schema":     {"internal/wire": true}, "internal/transport": {"types": true}, "types": {"internal/wire": true}, "internal/wire": {},
		"internal/security": {"types": true},
		"operations":        {"internal/core": true, "internal/operations": true, "models": true, "types": true},
		"selective":         {"internal/core": true, "internal/registry": true, "operations": true, "types": true},
		"internal/registry": {"operations": true},
	}
	for name, item := range packages {
		if name != prefix && !strings.HasPrefix(name, prefix+"/") {
			continue
		}
		owner := layer(name)
		edges, present := allowed[owner]
		if !present {
			t.Fatalf("unreviewed generated package %s", name)
		}
		for _, imported := range item.Imports {
			if imported == prefix || strings.HasPrefix(imported, prefix+"/") {
				if !edges[layer(imported)] {
					t.Errorf("forbidden generated edge %s -> %s", owner, layer(imported))
				}
			} else if owner == "internal/wire" && !packages[imported].Standard {
				t.Errorf("wire imports a non-standard dependency %s", imported)
			}
		}
	}
}
