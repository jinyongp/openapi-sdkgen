package golang

import (
	"fmt"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/generator"
	"strconv"
)

func appendMetadataFile(files []filePlan, document *ir.Document) ([]filePlan, error) {
	data, err := generator.SourceMetadata(document)
	if err != nil {
		return nil, err
	}
	prepared := fmt.Sprintf("const Version = %s\nconst VersionLine = %s\nconst documentJSON = %s\n// Document returns an independent copy of the original entry document.\nfunc Document()json.RawMessage{return json.RawMessage(documentJSON)}\n", strconv.Quote(document.OpenAPIVersion), strconv.Quote(document.OpenAPIVersionLine), strconv.Quote(string(data)))
	return append(files, filePlan{Path: "metadata/metadata.go", Package: "metadata", Template: "templates/prepared.go.tmpl", Imports: []importPlan{{Path: "encoding/json"}}, Prepared: prepared}), nil
}
