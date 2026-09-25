package sdkgen

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestCompatibilityIgnoresReservedHeaderFromReferencedLocalSourceBeforeNestedReferences(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.1.1
info: {title: Referenced reserved header, version: "1"}
paths:
  /items:
    get:
      operationId: listItems
      parameters:
        - {$ref: parameter.yaml}
        - {name: X-Trace, in: header, schema: {type: string}}
      responses:
        "204": {description: OK}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "parameter.yaml"), []byte(`name: Authorization
in: header
schema:
  $ref: missing-schema.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}

	document, err := CompileFile(root)
	if err != nil {
		t.Fatal(err)
	}
	parameters := document.Operations[0].Parameters
	if len(parameters) != 1 || parameters[0].Name != "X-Trace" {
		t.Fatalf("parameters = %#v", parameters)
	}
}

func TestCompatibilityIgnoresReservedHeaderFromReferencedRemoteSourceBeforeNestedFetch(t *testing.T) {
	var parameterRequests atomic.Int32
	var nestedRequests atomic.Int32
	remote := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/parameter.yaml":
			parameterRequests.Add(1)
			_, _ = response.Write([]byte(`name: Authorization
in: header
schema:
  $ref: nested.yaml
`))
		case "/nested.yaml":
			nestedRequests.Add(1)
			_, _ = response.Write([]byte("type: string\n"))
		default:
			http.NotFound(response, request)
		}
	}))
	defer remote.Close()

	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.1.1
info: {title: Remote reserved header, version: "1"}
paths:
  /items:
    get:
      operationId: listItems
      parameters:
        - {$ref: "`+remote.URL+`/parameter.yaml"}
        - {name: X-Trace, in: header, schema: {type: string}}
      responses:
        "204": {description: OK}
`), 0o600); err != nil {
		t.Fatal(err)
	}

	document, err := CompileFileWithOptions(root, CompileOptions{
		RemoteRefAllowlist:    []string{remote.URL},
		UpdateRefLock:         true,
		remoteReferenceClient: remote.Client(),
		remoteReferenceLookup: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	parameters := document.Operations[0].Parameters
	if len(parameters) != 1 || parameters[0].Name != "X-Trace" {
		t.Fatalf("parameters = %#v", parameters)
	}
	if got := parameterRequests.Load(); got != 1 {
		t.Fatalf("parameter requests = %d, want 1", got)
	}
	if got := nestedRequests.Load(); got != 0 {
		t.Fatalf("nested requests = %d, want 0", got)
	}
}
