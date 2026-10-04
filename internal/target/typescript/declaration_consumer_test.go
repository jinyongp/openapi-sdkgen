package typescript

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"openapi-sdkgen/internal/generator"
	"openapi-sdkgen/internal/tscheck"
)

// compileDeclarationConsumers checks strict source and isolated declaration-only
// consumers on each supported compiler. No generated ts-nocheck survives.
func compileDeclarationConsumers(t *testing.T, artifacts []generator.Artifact, probe string) {
	t.Helper()
	for _, compiler := range []string{"typescript-5-7", "typescript-5-9", "typescript-6", "typescript"} {
		t.Run(compiler, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source")
			declarations := filepath.Join(root, "declarations")
			checked := make([]generator.Artifact, len(artifacts))
			for index, artifact := range artifacts {
				checked[index] = generator.Artifact{Path: artifact.Path, Data: bytes.ReplaceAll(artifact.Data, []byte("// @ts-nocheck\n"), nil)}
			}
			writeTargetArtifacts(t, source, checked)
			options := tscheck.CompilerOptions()
			for key, value := range map[string]any{"target": "ES2022", "module": "NodeNext", "moduleResolution": "NodeNext", "lib": []string{"ES2022", "DOM", "DOM.Iterable"}, "rootDir": ".", "outDir": "../declarations", "declaration": true, "emitDeclarationOnly": true} {
				options[key] = value
			}
			writeConfig := func(directory string, settings map[string]any) {
				t.Helper()
				data, err := json.Marshal(map[string]any{"compilerOptions": settings, "include": []string{"**/*.ts"}})
				if err != nil {
					t.Fatal(err)
				}
				for name, value := range map[string]string{"tsconfig.json": string(data), "package.json": `{"type":"module"}`, "consumer.ts": probe} {
					if err := os.WriteFile(filepath.Join(directory, name), []byte(value), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			writeConfig(source, options)
			tsc := filepath.Join("..", "..", "..", "test", "typescript", "node_modules", compiler, "lib", "tsc.js")
			if _, err := os.Stat(tsc); err != nil {
				t.Fatalf("required compiler %s unavailable: %v", compiler, err)
			}
			if output, err := exec.Command("node", tsc, "--project", filepath.Join(source, "tsconfig.json")).CombinedOutput(); err != nil {
				t.Fatalf("strict source/declaration emit: %v\n%s", err, output)
			}
			// This directory has only .d.ts artifacts and the fresh consumer. Its
			// imports cannot fall back to the original TypeScript implementation.
			delete(options, "outDir")
			delete(options, "declaration")
			delete(options, "emitDeclarationOnly")
			options["noEmit"] = true
			writeConfig(declarations, options)
			if output, err := exec.Command("node", tsc, "--project", filepath.Join(declarations, "tsconfig.json")).CombinedOutput(); err != nil {
				t.Fatalf("isolated declaration consumer: %v\n%s", err, output)
			}
		})
	}
}
