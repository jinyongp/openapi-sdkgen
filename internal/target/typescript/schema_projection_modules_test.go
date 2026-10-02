package typescript

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

const namedProjectionFixture = `{"openapi":"3.2.1","info":{"title":"Directed models","version":"1"},"paths":{
"/write":{"post":{"operationId":"write","requestBody":{"required":true,"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Shared"}}}},"responses":{"204":{"description":"OK"}}}},
"/read":{"get":{"operationId":"read","responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Shared"}}}}}}}
},"components":{"schemas":{
"Shared":{"type":"object","properties":{"secret":{"writeOnly":true,"$ref":"#/components/schemas/InputOnly"},"result":{"readOnly":true,"$ref":"#/components/schemas/OutputOnly"},"next":{"$ref":"#/components/schemas/Shared"}}},
"InputOnly":{"type":"string","enum":["A_B_INPUT_ONLY"]},"OutputOnly":{"type":"string","enum":["C_OUTPUT_ONLY"]}
}}}`

func TestNamedSchemaProjectionsDoNotLoadOppositeDirection(t *testing.T) {
	document, err := sdkgen.Compile([]byte(namedProjectionFixture))
	if err != nil {
		t.Fatal(err)
	}
	options := generator.Options{
		Selection: &generator.Selection{Operations: []string{"read"}},
		Clients: map[string]generator.Client{
			"a": {Selection: &generator.Selection{Operations: []string{"write"}}},
			"c": {Selection: &generator.Selection{Operations: []string{"read"}}},
		},
	}
	output := compileSelectedTypeScriptArtifacts(t, document, options, "")
	if err := filepath.WalkDir(output, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".js") {
			return err
		}
		relative, _ := filepath.Rel(output, path)
		file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = file.WriteString("\nglobalThis.evaluations.push(" + quoteTS(filepath.ToSlash(relative)) + ");\n")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	script := `import {pathToFileURL} from 'node:url';
globalThis.evaluations=[];
await import(pathToFileURL(process.argv[1]+'/internal/executions/read/get.js'));
if(globalThis.evaluations.some(path=>path.includes('schema-projections/input/')||path.includes('input-only')||path.includes('inputonly')||path.startsWith('internal/schemas/')))throw Error('opposite projection loaded: '+JSON.stringify(globalThis.evaluations));
if(!globalThis.evaluations.includes('internal/schema-projections/output/output-only.js')&&!globalThis.evaluations.includes('internal/schema-projections/output/outputonly.js'))throw Error('required output dependency missing');
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, output).CombinedOutput(); err != nil {
		t.Fatalf("native projection imports: %v\n%s", err, result)
	}
}
