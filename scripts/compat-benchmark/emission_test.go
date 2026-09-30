package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmittedOperationsMatchGeneratedRoutesAndKeepLegacyRetention(t *testing.T) {
	for _, test := range []struct {
		name, extra                         string
		emitted, retained, omitted, helpers int
	}{
		{"full", ``, 1, 1, 0, 0},
		{"operation-omission", `,"/bad":{"trace":{"operationId":"bad","responses":{"204":{"description":"OK"}}}}`, 1, 1, 1, 0},
		{"helper-omission", ``, 1, 1, 0, 1},
		{"blocked", ``, 0, 1, 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := `{"description":"OK"}`
			if test.name == "helper-omission" {
				response = `{"description":"OK","links":{"next":{"operationRef":"#/paths/~1missing/get"}}}`
			}
			document := `{"openapi":"3.2.0","info":{"title":"Emission","version":"1"},"paths":{"/items":{"get":{"operationId":"items","responses":{"204":` + response + `}}}` + test.extra + `}`
			if test.name == "blocked" {
				document += `,"webhooks":{"event":{"post":{"responses":{"204":{"description":"OK"}}}}}`
			}
			document += `}`
			path := filepath.Join(t.TempDir(), "openapi.json")
			if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
				t.Fatal(err)
			}
			result, err := benchmarkDocument(corpusSpec{ID: test.name, Cohort: "fixture", Input: "openapi.json"}, path, func(directory string) verificationResult {
				data, err := os.ReadFile(filepath.Join(directory, "internal/routes/index.ts"))
				if err != nil {
					t.Fatal(err)
				}
				routes, _, _ := strings.Cut(strings.SplitN(string(data), "export interface Routes {", 2)[1], "}\n")
				count := strings.Count(routes, ".Contract")
				// A blocked base profile may run the independently supported server
				// profile. Its route contract still contains the original operation.
				expected := test.emitted
				if test.name == "blocked" {
					expected = 1
				}
				if count != expected {
					t.Fatalf("generated route count %d, want %d", count, expected)
				}
				return verificationResult{Status: "pass"}
			})
			if err != nil {
				t.Fatal(err)
			}
			metric := result.OperationEmission
			if metric == nil || !metric.Available || metric.Count != test.emitted || metric.OperationOmissions != test.omitted || metric.HelperOmissions != test.helpers {
				t.Fatalf("emission metric = %#v", metric)
			}
			if result.OperationRetention.Retained != test.retained || result.OperationRetention.Omitted != test.omitted {
				t.Fatalf("legacy retention changed: %#v", result.OperationRetention)
			}
			summary := summarizeDocuments("fixture", []documentResult{result})
			if summary.OperationEmission == nil || summary.OperationEmission.Count != test.emitted || summary.OperationEmission.HelperOmissions != test.helpers {
				t.Fatalf("summary = %#v", summary)
			}
		})
	}
}

func TestHistoricalReportKeepsEmissionUnavailable(t *testing.T) {
	var report benchmarkReport
	if err := json.Unmarshal([]byte(`{"schemaVersion":1,"documents":[{"operationRetention":{"available":true,"total":4,"retained":3,"omitted":1}}]}`), &report); err != nil {
		t.Fatal(err)
	}
	if report.Documents[0].OperationEmission != nil || summarizeDocuments("", report.Documents).OperationEmission != nil {
		t.Fatal("historical retention interpreted as emitted coverage")
	}
	if report.SchemaVersion != 1 || report.Documents[0].OperationRetention.Retained != 3 {
		t.Fatal("historical values changed")
	}
}
