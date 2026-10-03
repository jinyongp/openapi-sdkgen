package typescript

import (
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
)

func TestOperationSchemaQualificationDoesNotRewriteContractNamedResourceExample(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi": "3.0.3",
  "info": {"title": "Contract resource", "version": "1"},
  "paths": {
    "/tax-contract": {
      "delete": {
        "operationId": "deleteTaxContract",
        "responses": {"204": {"description": "Deleted"}}
      }
    }
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := SourceArtifacts(document)
	if err != nil {
		t.Fatal(err)
	}
	source := string(artifactByPath(t, artifacts, "internal/operations/tax-contract/delete.ts"))
	if !strings.Contains(source, "await api.taxContract.delete()") {
		t.Fatalf("operation example lost contract-named resource call:\n%s", source)
	}
	if strings.Contains(source, "taxContractSchemas.delete") {
		t.Fatalf("schema namespace qualification rewrote resource call:\n%s", source)
	}
}
