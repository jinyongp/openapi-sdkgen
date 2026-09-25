package sdkgen

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestCompatibilityReferenceSiblingNestedReferencesStayInvisibleToReferenceIO(t *testing.T) {
	t.Run("oas30 local sibling", func(t *testing.T) {
		directory := t.TempDir()
		root := filepath.Join(directory, "openapi.yaml")
		if err := os.WriteFile(root, []byte(`openapi: 3.0.3
info: {title: Reference sibling I/O, version: "1"}
paths:
  /items:
    get:
      operationId: listItems
      responses:
        "200":
          $ref: "#/components/responses/Base"
          content:
            application/json:
              schema:
                $ref: missing.yaml
components:
  responses:
    Base:
      description: OK
`), 0o600); err != nil {
			t.Fatal(err)
		}
		document, err := CompileFile(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(document.Operations) != 1 || len(document.Operations[0].Responses) != 1 {
			t.Fatalf("operations = %#v", document.Operations)
		}
	})

	t.Run("oas31 remote sibling", func(t *testing.T) {
		var requests atomic.Int32
		remote := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			requests.Add(1)
			_, _ = response.Write([]byte("type: string\n"))
		}))
		defer remote.Close()

		directory := t.TempDir()
		root := filepath.Join(directory, "openapi.yaml")
		if err := os.WriteFile(root, []byte(`openapi: 3.1.1
info: {title: Reference sibling I/O, version: "1"}
paths:
  /items:
    get:
      operationId: listItems
      parameters:
        - $ref: "#/components/parameters/Limit"
          schema:
            $ref: "`+remote.URL+`/schema.yaml"
      responses:
        "204": {description: OK}
components:
  parameters:
    Limit:
      name: limit
      in: query
      schema: {type: string}
`), 0o600); err != nil {
			t.Fatal(err)
		}
		document, err := CompileFile(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(document.Operations) != 1 || len(document.Operations[0].Parameters) != 1 {
			t.Fatalf("operations = %#v", document.Operations)
		}
		if got := requests.Load(); got != 0 {
			t.Fatalf("ignored sibling caused %d remote request(s), want 0", got)
		}
	})
}
