package sdkgen

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCompileInputAcceptsPathFileURLHTTPAndStandardInput(t *testing.T) {
	directory := t.TempDir()
	input := filepath.Join(directory, "contract.yaml")
	contents := []byte(`openapi: 3.2.0
info:
  title: Source inputs
  version: "1"
paths: {}
`)
	if err := os.WriteFile(input, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	fileURL := (&url.URL{Scheme: "file", Path: input}).String()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/openapi" {
			http.NotFound(response, request)
			return
		}
		response.Header().Set("Content-Type", "text/plain")
		_, _ = response.Write(contents)
	}))
	defer server.Close()
	var expected any
	for _, test := range []struct {
		name    string
		input   string
		options CompileOptions
	}{
		{name: "path", input: input},
		{name: "file URL", input: fileURL},
		{name: "HTTP URL", input: server.URL + "/openapi"},
		{name: "standard input", input: "-", options: CompileOptions{InputReader: strings.NewReader(string(contents))}},
	} {
		t.Run(test.name, func(t *testing.T) {
			document, err := CompileInputWithOptions(test.input, test.options)
			if err != nil {
				t.Fatal(err)
			}
			if document.OpenAPIVersion != "3.2.0" {
				t.Fatalf("version = %q", document.OpenAPIVersion)
			}
			if source := string(document.SourceMetadataJSON); !strings.Contains(source, `"title":"Source inputs"`) || !strings.Contains(source, `"paths":{}`) {
				t.Fatalf("source metadata = %s", source)
			}
			document.Provenance = nil
			document.ProvenanceIndex = nil
			if expected == nil {
				expected = document
				return
			}
			if !reflect.DeepEqual(expected, document) {
				t.Fatal("equivalent input sources produced different compiler documents")
			}
		})
	}
}

func TestCompileCapturesDecodedEntrySourceMetadata(t *testing.T) {
	document, err := Compile([]byte(`{
	  "openapi": "3.1.2",
	  "info": {"title": "In memory", "version": "1", "x-prototype": {"__proto__": "safe"}},
	  "paths": {}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	metadata := string(document.SourceMetadataJSON)
	for _, expected := range []string{`"title":"In memory"`, `"__proto__":"safe"`} {
		if !strings.Contains(metadata, expected) {
			t.Fatalf("source metadata missing %q: %s", expected, metadata)
		}
	}
}

func TestInputLocatorDoesNotTreatFilesystemPathsAsURLs(t *testing.T) {
	for _, value := range []string{"C:\\work\\openapi.yaml", "schema:openapi.yaml", "./schema:openapi.yaml"} {
		if isURLInput(value) {
			t.Fatalf("filesystem input %q was classified as a URL", value)
		}
	}
	if _, err := parseInputBase("C:\\work\\openapi.yaml"); err != nil {
		t.Fatalf("Windows input base classification failed: %v", err)
	}
	for _, value := range []string{"file:///workspace/openapi.yaml", "http://localhost:4010/openapi.yaml", "https://api.example.test/openapi.yaml"} {
		if !isURLInput(value) {
			t.Fatalf("URL input %q was not classified as a URL", value)
		}
	}
}

func TestCompileInputResolvesRelativeReferencesFromURLAndStdinBase(t *testing.T) {
	directory := t.TempDir()
	schema := []byte(`Thing:
  type: object
  required: [id]
  properties:
    id: {type: string}
`)
	input := []byte(`openapi: 3.2.0
info: {title: Relative reference, version: "1"}
paths:
  /things:
    get:
      operationId: listThings
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema: {$ref: schemas.yaml#/Thing}
`)
	if err := os.WriteFile(filepath.Join(directory, "schemas.yaml"), schema, 0o600); err != nil {
		t.Fatal(err)
	}
	fileInput := filepath.Join(directory, "openapi.yaml")
	if err := os.WriteFile(fileInput, input, 0o600); err != nil {
		t.Fatal(err)
	}
	compiled, err := CompileInputWithOptions((&url.URL{Scheme: "file", Path: fileInput}).String(), CompileOptions{})
	if err != nil || len(compiled.Operations) != 1 {
		t.Fatalf("file URL compilation = %#v, %v", compiled, err)
	}
	if metadata := string(compiled.SourceMetadataJSON); !strings.Contains(metadata, `"schemas.yaml#/Thing"`) || strings.Contains(metadata, `"components"`) {
		t.Fatalf("file URL source metadata was normalized or bundled: %s", metadata)
	}
	if _, err := CompileInputWithOptions("-", CompileOptions{InputReader: strings.NewReader(string(input))}); err == nil || !strings.Contains(err.Error(), "--input-base") {
		t.Fatalf("stdin relative reference error = %v", err)
	}
	compiled, err = CompileInputWithOptions("-", CompileOptions{
		InputReader: strings.NewReader(string(input)),
		InputBase:   filepath.Join(directory, "openapi.yaml"),
	})
	if err != nil || len(compiled.Operations) != 1 {
		t.Fatalf("stdin base compilation = %#v, %v", compiled, err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/openapi.yaml":
			_, _ = response.Write(input)
		case "/schemas.yaml":
			_, _ = response.Write(schema)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	if _, err := CompileInputWithOptions(server.URL+"/openapi.yaml", CompileOptions{}); err == nil || !strings.Contains(err.Error(), "--ref-lock") {
		t.Fatalf("URL relative reference without lock error = %v", err)
	}
	compiled, err = CompileInputWithOptions(server.URL+"/openapi.yaml", CompileOptions{
		RefLockPath:   filepath.Join(directory, "remote.lock"),
		UpdateRefLock: true,
	})
	if err != nil || len(compiled.Operations) != 1 {
		t.Fatalf("URL base compilation = %#v, %v", compiled, err)
	}
	if metadata := string(compiled.SourceMetadataJSON); !strings.Contains(metadata, `"schemas.yaml#/Thing"`) || strings.Contains(metadata, server.URL+"/schemas.yaml") {
		t.Fatalf("remote root source metadata was absolutized: %s", metadata)
	}
	compiled, err = CompileInputWithOptions(server.URL+"/openapi.yaml", CompileOptions{
		RefLockPath: filepath.Join(directory, "remote.lock"),
	})
	if err != nil || len(compiled.Operations) != 1 {
		t.Fatalf("URL base locked compilation = %#v, %v", compiled, err)
	}

	crossOrigin := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write(schema)
	}))
	defer crossOrigin.Close()
	crossDocument := strings.Replace(string(input), "schemas.yaml", crossOrigin.URL+"/schemas.yaml", 1)
	root := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write([]byte(crossDocument))
	}))
	defer root.Close()
	if _, err := CompileInputWithOptions(root.URL+"/openapi.yaml", CompileOptions{RefLockPath: filepath.Join(directory, "cross.lock"), UpdateRefLock: true}); err == nil {
		t.Fatalf("cross-origin reference error = %v", err)
	}
	compiled, err = CompileInputWithOptions(root.URL+"/openapi.yaml", CompileOptions{
		RemoteRefAllowlist:    []string{crossOrigin.URL},
		RefLockPath:           filepath.Join(directory, "cross.lock"),
		UpdateRefLock:         true,
		remoteReferenceClient: crossOrigin.Client(),
		remoteReferenceLookup: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		},
	})
	if err != nil || len(compiled.Operations) != 1 {
		t.Fatalf("allowlisted cross-origin compilation = %#v, %v", compiled, err)
	}
}

func TestCompileFileCombinesCachedLocalAndAllowlistedRemoteReferences(t *testing.T) {
	directory := t.TempDir()
	remote := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/schema.yaml" {
			http.NotFound(response, request)
			return
		}
		_, _ = response.Write([]byte("Thing:\n  type: object\n  properties:\n    id: {type: string}\n"))
	}))
	defer remote.Close()

	if err := os.MkdirAll(filepath.Join(directory, "paths"), 0o755); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(directory, "openapi.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.1.2
info: {title: Mixed references, version: "1"}
paths:
  /things:
    $ref: paths/things.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "paths", "things.yaml"), []byte(`get:
  operationId: listThings
  responses:
    "200":
      description: OK
      content:
        application/json:
          schema:
            $ref: "`+remote.URL+`/schema.yaml#/Thing"
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
	if len(document.Operations) != 1 || document.Operations[0].OperationID != "listThings" {
		t.Fatalf("operations = %#v", document.Operations)
	}
	if metadata := string(document.SourceMetadataJSON); !strings.Contains(metadata, `"paths/things.yaml"`) || strings.Contains(metadata, remote.URL) {
		t.Fatalf("entry source metadata changed: %s", metadata)
	}
}

func TestCompileFileRemoteReferenceViewSkipsOpaqueURLsAndLocksExactSources(t *testing.T) {
	var pathRequests atomic.Int32
	var schemaRequests atomic.Int32
	var opaqueRequests atomic.Int32
	var pathBody []byte
	schemaBody := []byte("Thing:\n  type: object\n  properties:\n    id: {type: string}\n")

	remote := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/path.yaml":
			pathRequests.Add(1)
			_, _ = response.Write(pathBody)
		case "/schema.yaml":
			schemaRequests.Add(1)
			_, _ = response.Write(schemaBody)
		case "/opaque-missing.yaml":
			opaqueRequests.Add(1)
			http.NotFound(response, request)
		default:
			http.NotFound(response, request)
		}
	}))
	defer remote.Close()

	pathBody = []byte(`get:
  operationId: listRemoteThings
  x-codeSamples:
    - lang: curl
      $ref: "` + remote.URL + `/opaque-missing.yaml"
  responses:
    "200":
      description: OK
      content:
        application/json:
          schema:
            $ref: "` + remote.URL + `/schema.yaml#/Thing"
`)

	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.1.1
info: {title: Remote opaque references, version: "1"}
paths:
  /things:
    $ref: "`+remote.URL+`/path.yaml"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(directory, "references.lock")
	metrics := &compilationMetrics{}
	document, err := CompileFileWithOptions(root, CompileOptions{
		RemoteRefAllowlist:    []string{remote.URL},
		RefLockPath:           lockPath,
		UpdateRefLock:         true,
		remoteReferenceClient: remote.Client(),
		remoteReferenceLookup: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		},
		metrics: metrics,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pathRequests.Load() != 1 || schemaRequests.Load() != 1 || opaqueRequests.Load() != 0 {
		t.Fatalf("remote requests = path:%d schema:%d opaque:%d", pathRequests.Load(), schemaRequests.Load(), opaqueRequests.Load())
	}
	if metrics.RemoteReferenceSourceDecodes != 2 {
		t.Fatalf("remote source decodes = %d, want 2", metrics.RemoteReferenceSourceDecodes)
	}
	if len(document.Operations) != 1 || document.Operations[0].OperationID != "listRemoteThings" {
		t.Fatalf("operations = %#v", document.Operations)
	}
	samples, _ := document.Operations[0].Raw["x-codeSamples"].([]any)
	if len(samples) != 1 {
		t.Fatalf("x-codeSamples = %#v", document.Operations[0].Raw["x-codeSamples"])
	}
	sample, _ := samples[0].(map[string]any)
	if sample["$ref"] != remote.URL+"/opaque-missing.yaml" {
		t.Fatalf("remote opaque reference = %#v", sample)
	}
	rawJSON, err := json.Marshal(document.Raw)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rawJSON), opaqueReferenceMarkerPrefix) {
		t.Fatalf("transient opaque marker escaped into canonical Raw: %s", rawJSON)
	}
	schemaPointer := document.Operations[0].Pointer + "/responses/200/content/application~1json/schema"
	if provenance, found := document.LookupProvenance(schemaPointer); !found || provenance.Primary.Source != remote.URL+"/schema.yaml" {
		t.Fatalf("remote schema provenance = %#v, found=%v", provenance, found)
	}

	lock, err := loadReferenceLock(lockPath, false)
	if err != nil {
		t.Fatal(err)
	}
	for source, body := range map[string][]byte{
		remote.URL + "/path.yaml":   pathBody,
		remote.URL + "/schema.yaml": schemaBody,
	} {
		digest := sha256.Sum256(body)
		if got := lock.References[source]; got != hex.EncodeToString(digest[:]) {
			t.Fatalf("lock digest for %s = %q", source, got)
		}
	}
	if _, exists := lock.References[remote.URL+"/opaque-missing.yaml"]; exists {
		t.Fatalf("opaque reference entered lock: %#v", lock.References)
	}
}

func TestProtectedHTTPSInputSettingsApplyOnlyToSameOriginReferences(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows fails protected same-origin reference caching before persistence")
	}
	const token = "credential-sentinel"
	t.Setenv("SDKGEN_HTTP_TOKEN", token)
	schema := []byte(`Thing:
  type: object
  required: [id]
  properties:
    id: {type: string}
`)
	document := []byte(`openapi: 3.2.0
info: {title: Protected reference, version: "1"}
paths:
  /things:
    get:
      operationId: listThings
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema: {$ref: schemas.yaml#/Thing}
`)
	root := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/openapi.yaml":
			_, _ = response.Write(document)
		case "/schemas.yaml":
			if got := request.Header.Get("Authorization"); got != token {
				t.Errorf("same-origin Authorization = %q", got)
			}
			if got := request.Header.Get("Accept"); got != token {
				t.Errorf("same-origin Accept = %q", got)
			}
			_, _ = response.Write(schema)
		default:
			http.NotFound(response, request)
		}
	}))
	defer root.Close()
	rootCAPath, _, _ := writeTLSServerCredentials(t, root)
	directory := t.TempDir()
	compiled, err := CompileInputWithOptions(root.URL+"/openapi.yaml", CompileOptions{
		HTTPHeaderEnv: []string{
			"Authorization=SDKGEN_HTTP_TOKEN",
			"Accept=SDKGEN_HTTP_TOKEN",
		},
		TLSCAFile:     rootCAPath,
		RefLockPath:   filepath.Join(directory, "same-origin.lock"),
		UpdateRefLock: true,
	})
	if err != nil || len(compiled.Operations) != 1 {
		t.Fatalf("same-origin compilation = %#v, %v", compiled, err)
	}

	crossOrigin := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "" {
			t.Errorf("cross-origin Authorization = %q", got)
		}
		_, _ = response.Write(schema)
	}))
	defer crossOrigin.Close()
	crossDocument := strings.Replace(string(document), "schemas.yaml", crossOrigin.URL+"/schemas.yaml", 1)
	crossRoot := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(crossDocument))
	}))
	defer crossRoot.Close()
	crossRootCAPath, _, _ := writeTLSServerCredentials(t, crossRoot)
	compiled, err = CompileInputWithOptions(crossRoot.URL+"/openapi.yaml", CompileOptions{
		HTTPHeaderEnv:         []string{"Authorization=SDKGEN_HTTP_TOKEN"},
		TLSCAFile:             crossRootCAPath,
		RemoteRefAllowlist:    []string{crossOrigin.URL},
		RefLockPath:           filepath.Join(directory, "cross-origin.lock"),
		UpdateRefLock:         true,
		remoteReferenceClient: crossOrigin.Client(),
		remoteReferenceLookup: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		},
	})
	if err != nil || len(compiled.Operations) != 1 {
		t.Fatalf("cross-origin compilation = %#v, %v", compiled, err)
	}
}

func TestUnprotectedSameOriginReferenceCannotRedirectCrossOrigin(t *testing.T) {
	crossOriginCalled := false
	crossOrigin := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		crossOriginCalled = true
	}))
	defer crossOrigin.Close()
	root := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/openapi.yaml":
			_, _ = response.Write([]byte(`openapi: 3.2.0
info: {title: Redirect, version: "1"}
paths:
  /things:
    get:
      operationId: listThings
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema: {$ref: schemas.yaml#/Thing}
`))
		case "/schemas.yaml":
			http.Redirect(response, request, crossOrigin.URL+"/schema.yaml", http.StatusFound)
		default:
			http.NotFound(response, request)
		}
	}))
	defer root.Close()
	_, err := CompileInputWithOptions(root.URL+"/openapi.yaml", CompileOptions{
		RefLockPath:   filepath.Join(t.TempDir(), "refs.lock"),
		UpdateRefLock: true,
	})
	if err == nil || !strings.Contains(err.Error(), "leaves the OpenAPI input origin") {
		t.Fatalf("cross-origin same-origin reference redirect error = %v", err)
	}
	if crossOriginCalled {
		t.Fatal("same-origin reference redirect opened a cross-origin request")
	}
}

func TestCompileFileIgnoresUnusedSiblingReferenceLock(t *testing.T) {
	directory := t.TempDir()
	input := filepath.Join(directory, "openapi.json")
	if err := os.WriteFile(input, []byte(`{"openapi":"3.2.0","info":{"title":"Input","version":"1"},"paths":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input+".openapi-sdkgen.lock", []byte("not JSON"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileFile(input); err != nil {
		t.Fatalf("self-contained file read an unused lock: %v", err)
	}
}

func TestCompileInputRejectsOfflineHTTPAndUnexpectedInputBase(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("offline HTTP input opened a request")
	}))
	defer server.Close()
	if _, err := CompileInputWithOptions(server.URL, CompileOptions{Offline: true}); err == nil || !strings.Contains(err.Error(), "--offline") {
		t.Fatalf("offline error = %v", err)
	}
	if _, err := CompileInputWithOptions("-", CompileOptions{InputReader: strings.NewReader("")}); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty stdin error = %v", err)
	}
	directory := t.TempDir()
	input := filepath.Join(directory, "openapi.json")
	if err := os.WriteFile(input, []byte(`{"openapi":"3.2.0","info":{"title":"Input","version":"1"},"paths":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileInputWithOptions(input, CompileOptions{InputBase: input}); err == nil || !strings.Contains(err.Error(), "only valid") {
		t.Fatalf("input base error = %v", err)
	}
}

func TestReadInputRejectsOversizedDocument(t *testing.T) {
	if _, err := readInput(strings.NewReader(strings.Repeat("x", inputMaxBytes+1)), "test input"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized input error = %v", err)
	}
}

func TestCompileBuildsValidatedIR(t *testing.T) {
	input := []byte(`{
  "openapi": "3.2.0",
  "info": {"title": "Example", "version": "0.1.0"},
  "servers": [{"url": "/v1"}],
  "paths": {
    "/healthz": {
      "servers": [{"url": "/"}],
      "get": {
        "operationId": "getHealth",
		"security": [],
        "responses": {"200": {"description": "OK"}},
        "x-envelope": "none",
        "x-concurrency": "none",
        "x-idempotency": "unsupported",
        "x-sdk-visibility": "public"
      }
    }
  }
}`)
	document, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Operations) != 1 || document.Operations[0].OperationID != "getHealth" {
		t.Fatalf("operations = %#v", document.Operations)
	}
}

func TestCompileAcceptsGenericOpenAPIWithoutProjectProfile(t *testing.T) {
	input := []byte(`{"openapi":"3.2.0","info":{"title":"Generic","version":"1"},"paths":{}}`)
	if _, err := Compile(input); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileProject(input); err != nil {
		t.Fatalf("compatibility compiler applied a project profile: %v", err)
	}
}

func TestCompileFileBundlesInDirectoryReferencesForEverySupportedVersionLine(t *testing.T) {
	for _, version := range []string{"3.0.3", "3.1.1", "3.2.0"} {
		t.Run(version, func(t *testing.T) {
			directory := t.TempDir()
			main := `{
  "openapi": "` + version + `",
  "info": {"title": "External", "version": "1"},
  "paths": {
    "/things": {
      "get": {
        "operationId": "listThings",
        "responses": {
          "200": {
            "description": "OK",
            "content": {"application/json": {"schema": {"$ref": "schemas.json#/Thing"}}}
          }
        }
      }
    }
  }
}`
			mainPath := filepath.Join(directory, "openapi.json")
			if err := os.WriteFile(mainPath, []byte(main), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "schemas.json"), []byte(`{"Thing":{"type":"object","properties":{"id":{"type":"string"}}}}`), 0o600); err != nil {
				t.Fatal(err)
			}

			document, err := CompileFile(mainPath)
			if err != nil {
				t.Fatal(err)
			}
			if document.OpenAPIVersion != version {
				t.Fatalf("version = %q, want %q", document.OpenAPIVersion, version)
			}
			if len(document.Operations) != 1 || document.Operations[0].OperationID != "listThings" {
				t.Fatalf("operations = %#v", document.Operations)
			}
		})
	}
}

func TestCompileFileIgnoresOpaqueReferencesInsideReferencedExtensions(t *testing.T) {
	directory := t.TempDir()
	pathsDirectory := filepath.Join(directory, "paths")
	if err := os.Mkdir(pathsDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(directory, "openapi.yaml")
	if err := os.WriteFile(input, []byte(`openapi: 3.0.3
info: {title: Opaque references, version: "1"}
paths:
  /things:
    $ref: paths/things.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pathsDirectory, "things.yaml"), []byte(`get:
  operationId: listThings
  x-codeSamples:
    - lang: curl
      $ref: examples/missing.yml
  responses:
    "200":
      description: OK
      content:
        application/json:
          schema:
            $ref: ../schemas.yaml#/Thing
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "schemas.yaml"), []byte(`Thing:
  type: object
  properties:
    id: {type: string}
`), 0o600); err != nil {
		t.Fatal(err)
	}

	metrics := &compilationMetrics{}
	document, err := CompileFileWithOptions(input, CompileOptions{metrics: metrics})
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Operations) != 1 || document.Operations[0].OperationID != "listThings" {
		t.Fatalf("operations = %#v", document.Operations)
	}
	samples, _ := document.Operations[0].Raw["x-codeSamples"].([]any)
	if len(samples) != 1 {
		t.Fatalf("x-codeSamples = %#v", document.Operations[0].Raw["x-codeSamples"])
	}
	sample, _ := samples[0].(map[string]any)
	if sample["$ref"] != "examples/missing.yml" {
		t.Fatalf("opaque extension reference = %#v", sample)
	}
	rawJSON, err := json.Marshal(document.Raw)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rawJSON), opaqueReferenceMarkerPrefix) {
		t.Fatalf("transient opaque marker escaped into canonical Raw: %s", rawJSON)
	}
	schemaPointer := document.Operations[0].Pointer + "/responses/200/content/application~1json/schema"
	if provenance, found := document.LookupProvenance(schemaPointer); !found ||
		!strings.HasSuffix(filepath.ToSlash(provenance.Primary.Source), "/schemas.yaml") {
		t.Fatalf("local schema provenance = %#v, found=%v", provenance, found)
	}
	wantFilter := []string{"openapi.yaml", "paths/things.yaml", "schemas.yaml"}
	if !reflect.DeepEqual(metrics.FileFilter, wantFilter) {
		t.Fatalf("reference file filter = %#v, want %#v", metrics.FileFilter, wantFilter)
	}
	if metrics.ReferenceSourceDecodes != 2 {
		t.Fatalf("reference source decodes = %d, want 2", metrics.ReferenceSourceDecodes)
	}
}

func TestSelfContainedCompileUsesSingleDecodeWithoutBundler(t *testing.T) {
	directory := t.TempDir()
	input := filepath.Join(directory, "openapi.json")
	contents := `{"openapi":"3.1.0","info":{"title":"Fast path","version":"1"},"paths":{},"components":{"schemas":{"Thing":{"type":"object"}}}}`
	if err := os.WriteFile(input, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	metrics := &compilationMetrics{}
	if _, err := CompileFileWithOptions(input, CompileOptions{metrics: metrics}); err != nil {
		t.Fatal(err)
	}
	if metrics.SourceDecodes != 1 || metrics.Bundles != 0 || metrics.ModelBuilds != 1 {
		t.Fatalf("structural metrics = %#v, want decode=1 bundle=0 model=1", metrics)
	}
}

func TestJSONInputDecodePreservesScalarTypesAndRejectsDuplicateKeys(t *testing.T) {
	value, err := decodeInputValue([]byte(`{"integer":3,"decimal":1.5,"unsigned":9223372036854775808,"array":[true,null]}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	object := value.(map[string]any)
	if object["integer"] != 3 || object["decimal"] != 1.5 || object["unsigned"] != uint64(9223372036854775808) {
		t.Fatalf("decoded scalars = %#v", object)
	}
	if _, err := decodeInputValue([]byte(`{"openapi":"3.1.0","openapi":"3.2.0"}`), nil); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate JSON key error = %v", err)
	}
	flow, err := decodeInputValue([]byte(`{openapi: "3.1.0", info: {title: Flow, version: "1"}, paths: {}}`), nil)
	if err != nil || flow.(map[string]any)["openapi"] != "3.1.0" {
		t.Fatalf("YAML flow fallback = %#v, %v", flow, err)
	}
}

func TestExternalCompileUsesExactReferenceClosureAndSingleModelBuild(t *testing.T) {
	directory := t.TempDir()
	schemas := filepath.Join(directory, "schemas")
	if err := os.Mkdir(schemas, 0o700); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(directory, "openapi.yaml")
	contents := `openapi: 3.1.0
info: {title: Bounded references, version: "1"}
paths:
  /things:
    get:
      operationId: listThings
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema: {$ref: schemas/thing.yaml#/Thing}
`
	files := map[string]string{
		input:                                   contents,
		filepath.Join(schemas, "thing.yaml"):    "Thing:\n  type: object\n  properties:\n    name: {$ref: common.yaml#/Name}\n",
		filepath.Join(schemas, "common.yaml"):   "Name: {type: string}\n",
		filepath.Join(directory, "unused.yaml"): "this: [is: not: valid",
	}
	for path, contents := range files {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	metrics := &compilationMetrics{}
	document, err := CompileFileWithOptions(input, CompileOptions{metrics: metrics})
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Operations) != 1 || document.Operations[0].OperationID != "listThings" {
		t.Fatalf("operations = %#v", document.Operations)
	}
	wantFilter := []string{"openapi.yaml", "schemas/common.yaml", "schemas/thing.yaml"}
	if metrics.SourceDecodes != 1 || metrics.Bundles != 1 || metrics.ModelBuilds != 1 || !reflect.DeepEqual(metrics.FileFilter, wantFilter) {
		t.Fatalf("structural metrics = %#v, want decode=1 bundle=1 model=1 filter=%v", metrics, wantFilter)
	}
}

func TestCompileProjectFileResolvesExternalReferences(t *testing.T) {
	directory := t.TempDir()
	main := `{"openapi":"3.2.0","info":{"title":"External","version":"1"},"servers":[{"url":"/v1"}],"paths":{"/things":{"get":{"operationId":"listThings","security":[],"responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"$ref":"schemas.json#/Thing"}}}}},"x-envelope":"none","x-concurrency":"none","x-idempotency":"unsupported","x-sdk-visibility":"public"}}}}`
	external := `{"Thing":{"type":"object","properties":{"requestId":{"type":"string"}}}}`
	mainPath := filepath.Join(directory, "openapi.json")
	if err := os.WriteFile(mainPath, []byte(main), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "schemas.json"), []byte(external), 0o600); err != nil {
		t.Fatal(err)
	}
	document, err := CompileFile(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Operations) != 1 {
		t.Fatalf("operations = %#v", document.Operations)
	}
	if projectDocument, err := CompileProjectFile(mainPath); err != nil || len(projectDocument.Operations) != 1 {
		t.Fatalf("compatibility compiler did not use general reference behavior: %#v, %v", projectDocument, err)
	}
}

func TestCompileFileRejectsReferenceOutsideInputDirectory(t *testing.T) {
	root := t.TempDir()
	inputDirectory := filepath.Join(root, "input")
	if err := os.Mkdir(inputDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "outside.json"), []byte(`{"Thing":{"type":"object"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(inputDirectory, "openapi.json")
	input := `{"openapi":"3.2.0","info":{"title":"External","version":"1"},"paths":{"/things":{"get":{"operationId":"listThings","responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"$ref":"../outside.json#/Thing"}}}}}}}}}`
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileFile(path); err == nil || !strings.Contains(err.Error(), "escapes the input directory") {
		t.Fatalf("CompileFile error = %v", err)
	}
}

func TestCompileFileRejectsEscapingReferenceBelowPropertyNamedDefault(t *testing.T) {
	root := t.TempDir()
	inputDirectory := filepath.Join(root, "input")
	if err := os.Mkdir(inputDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "outside.json"), []byte(`{"type":"string"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(inputDirectory, "openapi.json")
	input := `{
  "openapi":"3.1.0",
  "info":{"title":"Named default","version":"1"},
  "paths":{},
  "components":{"schemas":{"Payload":{
    "type":"object",
    "properties":{"default":{"$ref":"../outside.json"}}
  }}}
}`
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileFile(path); err == nil || !strings.Contains(err.Error(), "escapes the input directory") {
		t.Fatalf("CompileFile error = %v", err)
	}
}

func TestCompileFileRejectsTransitiveReferenceOutsideInputDirectory(t *testing.T) {
	root := t.TempDir()
	inputDirectory := filepath.Join(root, "input")
	if err := os.MkdirAll(filepath.Join(inputDirectory, "schemas"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "outside.json"), []byte(`{"Thing":{"type":"object"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDirectory, "schemas", "first.json"), []byte(`{"Thing":{"$ref":"../../outside.json#/Thing"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(inputDirectory, "openapi.json")
	input := `{"openapi":"3.2.0","info":{"title":"External","version":"1"},"paths":{"/things":{"get":{"operationId":"listThings","responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"$ref":"schemas/first.json#/Thing"}}}}}}}}}`
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileFile(path); err == nil || !strings.Contains(err.Error(), "escapes the input directory") {
		t.Fatalf("CompileFile error = %v", err)
	}
}

func TestCompileFileAcceptsYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "openapi.yaml")
	input := "openapi: 3.2.0\ninfo:\n  title: YAML\n  version: 1\npaths: {}\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileFile(path); err != nil {
		t.Fatal(err)
	}
}

func TestCompileFilePreservesOpenAPI32AdditionalOperations(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "openapi.json")
	if err := os.WriteFile(path, []byte(`{
  "openapi": "3.2.0",
  "info": {"title": "Additional operations", "version": "1"},
  "paths": {
    "/records": {
      "additionalOperations": {
        "PURGE": {"operationId": "purgeRecords", "responses": {"204": {"description": "Deleted"}}}
      }
    }
  }
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	document, err := CompileFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Operations) != 1 || document.Operations[0].OperationID != "purgeRecords" || document.Operations[0].Method != "PURGE" {
		t.Fatalf("compiled operations = %#v", document.Operations)
	}
}

func TestCompileNormalizesNestedComponentSchemaReferences(t *testing.T) {
	document, err := Compile([]byte(`{
  "openapi":"3.1.0",
  "info":{"title":"Nested","version":"1"},
  "paths":{"/holder":{"get":{"operationId":"getHolder","responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Holder"}}}}}}}},
  "components":{"schemas":{
    "Thing":{"type":"object","properties":{"id":{"type":"string"}}},
    "Holder":{"type":"object","properties":{"id":{"$ref":"#/components/schemas/Thing/properties/id"}}}
  }}
}`))
	if err != nil {
		t.Fatal(err)
	}
	holder := document.ComponentSchemas["Holder"]
	properties, _ := holder["properties"].(map[string]any)
	id, _ := properties["id"].(map[string]any)
	reference, _ := id["$ref"].(string)
	name := strings.TrimPrefix(reference, "#/components/schemas/")
	if reference == "" || document.ComponentSchemas[name]["type"] != "string" {
		t.Fatalf("normalized nested schema = %#v", id)
	}
}

func TestCompileNormalizesLocalSchemaAnchorReferences(t *testing.T) {
	document, err := Compile([]byte(`{
  "openapi":"3.1.0",
  "info":{"title":"Anchors","version":"1"},
  "paths":{},
  "components":{"schemas":{
    "Node":{
      "$id":"https://schemas.example.test/node",
      "$anchor":"node",
      "type":"object",
      "properties":{"next":{"$ref":"#node"}}
    },
    "Envelope":{
      "type":"object",
      "properties":{"node":{"$ref":"https://schemas.example.test/node#node"}}
    }
  }}
}`))
	if err != nil {
		t.Fatal(err)
	}
	node := document.ComponentSchemas["Node"]
	if _, exists := node["$anchor"]; exists {
		t.Fatalf("anchor was not lowered: %#v", node)
	}
	properties, _ := node["properties"].(map[string]any)
	next, _ := properties["next"].(map[string]any)
	if next["$ref"] != "#/components/schemas/Node" {
		t.Fatalf("anchored reference = %#v", next)
	}
	envelope := document.ComponentSchemas["Envelope"]
	envelopeProperties, _ := envelope["properties"].(map[string]any)
	child, _ := envelopeProperties["node"].(map[string]any)
	if child["$ref"] != "#/components/schemas/Node" {
		t.Fatalf("anchored reference = %#v", child)
	}
}

func TestCompileUsesOpenAPI32SelfAsSchemaResourceBase(t *testing.T) {
	document, err := Compile([]byte(`{
  "openapi":"3.2.0", "$self":"https://schemas.example.test/openapi.json",
  "info":{"title":"Self base","version":"1"}, "paths":{},
  "components":{"schemas":{
    "Node":{"$id":"schemas/node","$anchor":"node","type":"object"},
    "Envelope":{"type":"object","properties":{"node":{"$ref":"https://schemas.example.test/schemas/node#node"}}}
  }}
}`))
	if err != nil {
		t.Fatal(err)
	}
	envelope := document.ComponentSchemas["Envelope"]
	properties, _ := envelope["properties"].(map[string]any)
	node, _ := properties["node"].(map[string]any)
	if node["$ref"] != "#/components/schemas/Node" {
		t.Fatalf("$self-relative anchor reference = %#v", node)
	}
}

func TestCompileLowersDynamicSchemaReferenceMetadata(t *testing.T) {
	document, err := Compile([]byte(`{
  "openapi":"3.1.0", "info":{"title":"Anchors","version":"1"}, "paths":{},
  "components":{"schemas":{"Node":{"$dynamicAnchor":"node","type":"object","properties":{"child":{"$dynamicRef":"#node"}}}}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	node := document.ComponentSchemas["Node"]
	if node[dynamicAnchorMetadataKey] != "node" {
		t.Fatalf("dynamic anchor metadata = %#v", node)
	}
	properties, _ := node["properties"].(map[string]any)
	child, _ := properties["child"].(map[string]any)
	dynamic, _ := child[dynamicReferenceMetadataKey].(map[string]any)
	if dynamic["anchor"] != "node" || dynamic["reference"] != "#/components/schemas/Node" {
		t.Fatalf("dynamic reference metadata = %#v", child)
	}
}

func TestCompileDoesNotInterpretExampleReferenceLikeValuesAsSchemas(t *testing.T) {
	document, err := Compile([]byte(`{
  "openapi":"3.1.0", "info":{"title":"Examples","version":"1"}, "paths":{},
  "components":{"schemas":{"Thing":{"type":"object","properties":{"id":{"type":"string"}}},"Example":{"type":"object","examples":[{"$ref":"#/components/schemas/Thing/properties/id"}]}}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	example := document.ComponentSchemas["Example"]
	examples, _ := example["examples"].([]any)
	value, _ := examples[0].(map[string]any)
	if value["$ref"] != "#/components/schemas/Thing/properties/id" {
		t.Fatalf("example value changed: %#v", value)
	}
}

func TestCompileProjectValidatesResolvedPathItemParameters(t *testing.T) {
	input := []byte(`{
		"openapi":"3.2.0",
		"info":{"title":"Refs","version":"1"},
		"servers":[{"url":"/v1"}],
		"paths":{"/things/{thingID}":{"$ref":"#/components/pathItems/Thing"}},
		"components":{"pathItems":{"Thing":{
			"parameters":[{"name":"thingID","in":"path","required":true,"schema":{"type":"string"}}],
			"get":{"operationId":"getThing","security":[],"responses":{"204":{"description":"OK"}},"x-envelope":"none","x-concurrency":"none","x-idempotency":"unsupported","x-sdk-visibility":"public"}
		}}}
	}`)
	if _, err := CompileProject(input); err != nil {
		t.Fatal(err)
	}
}
