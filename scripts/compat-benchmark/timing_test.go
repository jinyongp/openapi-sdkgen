package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerationTimingExcludesTypecheckingAndTracksServerProfile(t *testing.T) {
	for _, inbound := range []bool{false, true} {
		name := "client"
		extra := ""
		if inbound {
			name = "server"
			extra = `,"webhooks":{"event":{"post":{"responses":{"204":{"description":"OK"}}}}}`
		}
		t.Run(name, func(t *testing.T) {
			input := filepath.Join(t.TempDir(), "openapi.json")
			document := `{"openapi":"3.2.0","info":{"title":"Timing","version":"1"},"paths":{"/items":{"get":{"responses":{"204":{"description":"OK"}}}}}` + extra + `}`
			if err := os.WriteFile(input, []byte(document), 0o600); err != nil {
				t.Fatal(err)
			}
			const typecheckDelay = 100 * time.Millisecond
			started := time.Now()
			result, err := benchmarkDocument(corpusSpec{ID: name, Cohort: "fixture", Input: "openapi.json"}, input, func(string) verificationResult {
				time.Sleep(typecheckDelay)
				return verificationResult{Status: "pass"}
			})
			elapsed := time.Since(started)
			if err != nil {
				t.Fatal(err)
			}
			generation := result.Generation
			if inbound {
				if generation.DurationMillis != nil || generation.Status != "blocked" {
					t.Fatalf("blocked generation has timing: %#v", generation)
				}
				generation = result.SupportProfiles[0].Generation
			}
			if generation.Status != "pass" || generation.DurationMillis == nil || *generation.DurationMillis <= 0 {
				t.Fatalf("successful generation lacks duration: %#v", generation)
			}
			measured := time.Duration(*generation.DurationMillis * float64(time.Millisecond))
			if elapsed-measured < typecheckDelay {
				t.Fatalf("generation duration includes typechecking: generation=%s total=%s", measured, elapsed)
			}
		})
	}
}

func TestHistoricalGenerationTimingIsUnavailable(t *testing.T) {
	var result verificationResult
	if err := json.Unmarshal([]byte(`{"status":"pass","artifactCount":1}`), &result); err != nil {
		t.Fatal(err)
	}
	if result.DurationMillis != nil {
		t.Fatal("historical generation reported zero duration")
	}
}
