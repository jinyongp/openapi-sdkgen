package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"openapi-sdkgen/internal/tscheck"
)

func TestStrictTypecheckRunsNativeCompilerSeriallyWithoutWeakeningChecks(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node unavailable")
	}
	for _, version := range []string{"7.0.2", "5.9.3"} {
		t.Run(version, func(t *testing.T) {
			root := t.TempDir()
			compiler := filepath.Join(root, "node_modules", "typescript")
			if err := os.MkdirAll(filepath.Join(compiler, "lib"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(compiler, "package.json"), []byte(`{"version":"`+version+`"}`), 0o644); err != nil {
				t.Fatal(err)
			}
			profile, err := json.Marshal(tscheck.CompilerOptions())
			if err != nil {
				t.Fatal(err)
			}
			probe := `const expected = ` + string(profile) + `;
const fs = require('node:fs');
const args = process.argv.slice(2);
const options = JSON.parse(fs.readFileSync(args[1], 'utf8')).compilerOptions;
if (args[0] !== '--project' || !options.strict || !options.noUncheckedIndexedAccess || options.skipLibCheck || !options.noEmit) process.exit(1);
for (const [flag, value] of Object.entries(expected)) if (options[flag] !== value) process.exit(3);
const native = JSON.parse(fs.readFileSync(require('node:path').join(__dirname, '../package.json'), 'utf8')).version.startsWith('7.');
if (native !== args.includes('--singleThreaded') || args.length !== (native ? 3 : 2)) process.exit(2);
`
			if err := os.WriteFile(filepath.Join(compiler, "lib", "tsc.js"), []byte(probe), 0o644); err != nil {
				t.Fatal(err)
			}
			result := strictTypecheck(t.TempDir(), root, time.Minute)
			if result.Status != "pass" {
				t.Fatalf("typecheck = %#v", result)
			}
		})
	}
}
