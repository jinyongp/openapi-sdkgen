package typescript

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
)

func supportGapFixture(t *testing.T, name string) []byte {
	t.Helper()
	input, err := os.ReadFile(filepath.Join("../../../test/fixtures/support-gaps", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return input
}

func TestSupportGapReferenceFixtures(t *testing.T) {
	for _, name := range []string{"schema-locations", "schema-array", "schema-percent"} {
		t.Run(name, func(t *testing.T) {
			for _, version := range []string{"3.0.3", "3.1.1", "3.2.0"} {
				input := supportGapFixture(t, name)
				input = []byte(strings.ReplaceAll(strings.ReplaceAll(string(input), "3.2.0", version), "3.1.1", version))
				document, err := sdkgen.Compile(input)
				if err == nil {
					_, err = SourceArtifacts(document)
				}
				if err != nil {
					t.Fatalf("%s %s: %v", name, version, err)
				}
			}
		})
	}
}

func TestSupportGapReferencesValidateGeneratedRuntime(t *testing.T) {
	for _, name := range []string{"schema-locations", "schema-array", "schema-percent"} {
		t.Run(name, func(t *testing.T) {
			for _, version := range []string{"3.0.3", "3.1.1", "3.2.0"} {
				input := strings.ReplaceAll(strings.ReplaceAll(string(supportGapFixture(t, name)), "3.2.0", version), "3.1.1", version)
				document, err := sdkgen.Compile([]byte(input))
				if err != nil {
					t.Fatal(err)
				}
				output := compileTypeScriptArtifacts(t, document)
				script := `
import { pathToFileURL } from "node:url";
const { createClient } = await import(pathToFileURL(process.argv[1]).href);
let body = '"valid"';
const api = createClient({baseURL:"https://example.test",fetch:async()=>new Response(body,{headers:{"content-type":"application/json"}})});
if (await api.$operations.use() !== "valid") throw new Error("reference changed output");
for (const invalid of ['"x"','7']) {
  body=invalid;
  try { await api.$operations.use(); throw new Error("invalid target value accepted"); }
  catch (error) { if (String(error).includes("invalid target value accepted")) throw error; }
}
`
				if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
					t.Fatalf("%s: %v\n%s", version, err, result)
				}
			}
		})
	}
}
