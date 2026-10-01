package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchCorpusDownloadsRootsAndAuxiliaryFilesWithBoundedConcurrency(t *testing.T) {
	data := []byte("openapi: 3.0.4\ninfo: {title: Parallel, version: '1'}\npaths: {}\n")
	checksum := sha256.Sum256(data)
	sha := hex.EncodeToString(checksum[:])
	var active, maximum atomic.Int32
	started := make(chan struct{}, 16)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		count := active.Add(1)
		defer active.Add(-1)
		for previous := maximum.Load(); count > previous; previous = maximum.Load() {
			if maximum.CompareAndSwap(previous, count) {
				break
			}
		}
		started <- struct{}{}
		<-release
		_, _ = response.Write(data)
	}))
	defer server.Close()
	manifest := benchmarkManifest{SchemaVersion: benchmarkSchemaVersion, Pinned: true,
		Corpora: []corpusSpec{{ID: "parallel", Cohort: "regression", Input: "spec/root.yaml",
			SourceURL: server.URL + "/spec/root.yaml", SHA256: sha, OpenAPIVersion: "3.0.4",
			Revision: "fixture-1", Trust: "fixture", EvidenceKind: "real-document"}},
	}
	for i := range 15 {
		manifest.Files = append(manifest.Files, pinnedCorpusFile{Input: fmt.Sprintf("spec/aux-%d.yaml", i), SHA256: sha})
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	manifestPath := filepath.Join(root, "manifest.json")
	if err := os.WriteFile(manifestPath, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	destination := filepath.Join(root, "corpus")
	go func() { finished <- fetchCorpus(manifestPath, destination, false) }()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for range 8 {
		select {
		case <-started:
		case err := <-finished:
			close(release)
			t.Fatalf("fetch returned before concurrent requests: %v", err)
		case <-deadline.C:
			close(release)
			<-finished
			t.Fatal("eight independent requests did not start concurrently")
		}
	}
	close(release)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if maximum.Load() != 8 {
		t.Fatalf("maximum requests = %d, want bounded concurrency of eight", maximum.Load())
	}
	if err := fetchCorpus(manifestPath, destination, true); err != nil {
		t.Fatalf("published roots and auxiliary files failed verification: %v", err)
	}
}

func TestFetchCorpusPublishesVerifiedInputsAndOfflineDetectsTamper(t *testing.T) {
	files := map[string][]byte{
		"APIs/alpha.test/1/openapi.yaml": []byte("openapi: 3.1.0\ninfo: {title: Alpha, version: '1'}\npaths: {}\n"),
		"APIs/beta.test/1/openapi.yaml":  []byte("openapi: 3.0.3\ninfo: {title: Beta, version: '1'}\npaths: {}\n"),
	}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		path := strings.TrimPrefix(request.URL.Path, "/root/")
		data, exists := files[path]
		if !exists {
			http.NotFound(response, request)
			return
		}
		_, _ = response.Write(data)
	}))
	defer server.Close()

	manifest := benchmarkManifest{
		SchemaVersion: benchmarkSchemaVersion,
		Source: &corpusSource{
			Repository: "https://example.test/openapi-directory.git",
			Commit:     strings.Repeat("a", 40),
			RawBaseURL: server.URL + "/root/",
		},
		Corpora: []corpusSpec{
			{
				ID: "alpha", Provider: "alpha.test", Cohort: "holdout",
				Input: "APIs/alpha.test/1/openapi.yaml", GitBlob: gitBlobSHA(files["APIs/alpha.test/1/openapi.yaml"]),
				Bytes: int64(len(files["APIs/alpha.test/1/openapi.yaml"])), SizeClass: "small",
			},
			{
				ID: "beta", Provider: "beta.test", Cohort: "holdout",
				Input: "APIs/beta.test/1/openapi.yaml", GitBlob: gitBlobSHA(files["APIs/beta.test/1/openapi.yaml"]),
				Bytes: int64(len(files["APIs/beta.test/1/openapi.yaml"])), SizeClass: "small",
			},
		},
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "corpus")
	if err := fetchCorpus(manifestPath, destination, false); err != nil {
		t.Fatal(err)
	}
	if err := fetchCorpus(manifestPath, destination, true); err != nil {
		t.Fatalf("offline verification failed: %v", err)
	}
	if err := fetchCorpus(manifestPath, destination, false); err == nil {
		t.Fatal("expected online fetch to refuse existing destination")
	}

	tampered := filepath.Join(destination, "APIs", "alpha.test", "1", "openapi.yaml")
	if err := os.WriteFile(tampered, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fetchCorpus(manifestPath, destination, true); err == nil {
		t.Fatal("expected offline verification to reject tampered corpus")
	}
}

func TestRunBenchmarkRejectsTamperedSourceBackedCorpus(t *testing.T) {
	data := []byte("openapi: 3.1.0\ninfo: {title: Alpha, version: '1'}\npaths: {}\n")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write(data)
	}))
	defer server.Close()

	manifest := benchmarkManifest{
		SchemaVersion: benchmarkSchemaVersion,
		Source: &corpusSource{
			Repository: "https://example.test/openapi-directory.git",
			Commit:     strings.Repeat("a", 40),
			RawBaseURL: server.URL + "/",
		},
		Corpora: []corpusSpec{{
			ID: "alpha", Provider: "alpha.test", Cohort: "holdout",
			Input: "openapi.yaml", GitBlob: gitBlobSHA(data), Bytes: int64(len(data)), SizeClass: "small",
		}},
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
		t.Fatal(err)
	}
	corpusRoot := filepath.Join(t.TempDir(), "corpus")
	if err := fetchCorpus(manifestPath, corpusRoot, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(corpusRoot, "openapi.yaml"), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = runBenchmark(manifestPath, corpusRoot, filepath.Join(t.TempDir(), "report.json"), filepath.Join(t.TempDir(), "typescript"), time.Second)
	if err == nil || !strings.Contains(err.Error(), "verify corpus before benchmark") {
		t.Fatalf("runBenchmark error = %v, want corpus verification failure", err)
	}
}

func TestFetchPinnedDocumentURLsAndAuxiliaryFiles(t *testing.T) {
	rootData := []byte("openapi: 3.0.4\ninfo: {title: Pinned, version: '1'}\npaths: {}\n")
	auxiliaryData := []byte("type: string\n")
	checksum := func(data []byte) string {
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:])
	}
	for _, damaged := range []string{"", "root", "auxiliary"} {
		t.Run("damaged-"+damaged, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case "/revision/spec/openapi.yaml":
					if damaged == "root" {
						_, _ = response.Write([]byte("tampered"))
					} else {
						_, _ = response.Write(rootData)
					}
				case "/revision/spec/components/model #1.yaml":
					if damaged == "auxiliary" {
						_, _ = response.Write([]byte("tampered"))
					} else {
						_, _ = response.Write(auxiliaryData)
					}
				default:
					http.NotFound(response, request)
				}
			}))
			defer server.Close()
			manifest := benchmarkManifest{
				SchemaVersion: benchmarkSchemaVersion,
				Pinned:        true,
				Corpora: []corpusSpec{{
					ID: "pinned", Cohort: "regression", Input: "provider/spec/openapi.yaml",
					SourceURL: server.URL + "/revision/spec/openapi.yaml", Revision: "revision",
					SHA256: checksum(rootData), OpenAPIVersion: "3.0.4", EvidenceKind: "real-document", Trust: "fixture",
				}},
				Files: []pinnedCorpusFile{{Input: "provider/spec/components/model #1.yaml", SHA256: checksum(auxiliaryData)}},
			}
			manifestData, err := json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			manifestPath := filepath.Join(t.TempDir(), "manifest.json")
			if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(t.TempDir(), "corpus")
			err = fetchCorpus(manifestPath, destination, false)
			if damaged != "" {
				if err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
					t.Fatalf("fetch error = %v, want hash mismatch", err)
				}
				if _, err := os.Stat(destination); !os.IsNotExist(err) {
					t.Fatal("failed fetch published the destination")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := fetchCorpus(manifestPath, destination, true); err != nil {
				t.Fatal(err)
			}
			if err := fetchCorpus(manifestPath, destination, false); err == nil {
				t.Fatal("fresh fetch overwrote an existing corpus")
			}
			if err := os.WriteFile(filepath.Join(destination, "provider/spec/components/model #1.yaml"), []byte("tampered"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := fetchCorpus(manifestPath, destination, true); err == nil {
				t.Fatal("offline verification accepted a tampered auxiliary file")
			}
		})
	}
}

func TestCommittedHoldoutResultMatchesManifestIdentity(t *testing.T) {
	manifestPath := filepath.Join("..", "..", "test", "compatibility", "holdout.json")
	resultPath := filepath.Join("..", "..", "test", "compatibility", "holdout-results.json")

	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest benchmarkManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	resultData, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(resultData), "/workspace/") {
		t.Fatal("durable holdout result contains checkout-specific absolute paths")
	}
	var result benchmarkReport
	if err := json.Unmarshal(resultData, &result); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(manifestData)
	if got, want := result.ManifestSHA256, hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("result manifest SHA = %q, want %q", got, want)
	}
	if len(result.Documents) != len(manifest.Corpora) {
		t.Fatalf("result documents = %d, want %d", len(result.Documents), len(manifest.Corpora))
	}
	byID := make(map[string]documentResult, len(result.Documents))
	for _, document := range result.Documents {
		byID[document.ID] = document
	}
	for _, corpus := range manifest.Corpora {
		document, exists := byID[corpus.ID]
		if !exists {
			t.Fatalf("result missing corpus %q", corpus.ID)
		}
		if document.Input != corpus.Input || document.GitBlob != corpus.GitBlob || document.SizeClass != corpus.SizeClass {
			t.Fatalf("result identity mismatch for %q: %#v vs %#v", corpus.ID, document, corpus)
		}
	}
}
