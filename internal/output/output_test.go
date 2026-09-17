package output

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"openapi-sdkgen/internal/generator"
)

func TestSafeArtifactPathRejectsTraversal(t *testing.T) {
	for _, value := range []string{"", ".", "..", "../outside.ts", "/outside.ts"} {
		t.Run(value, func(t *testing.T) {
			if _, err := SafeArtifactPath(value); err == nil {
				t.Fatalf("invalid path %q was accepted", value)
			}
		})
	}
}

func TestPublishArtifactsRollsBackPathConflict(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "output")
	err := PublishArtifacts(path, []generator.Artifact{
		{Path: "nested", Data: []byte("not a directory\n")},
		{Path: "nested/client.ts", Data: []byte("export {}\n")},
	}, false, nil)
	if err == nil || !strings.Contains(err.Error(), "create artifact directory") {
		t.Fatalf("publish error = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial output stat error = %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(root, ".openapi-sdkgen-output-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("staging directories = %v, %v", matches, err)
	}
}

func TestIncrementalPublicationPreservesOwnershipBoundaries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generated")
	initial := []generator.Artifact{
		{Path: "stable.ts", Data: []byte("stable\n")},
		{Path: "changed.ts", Data: []byte("before\n")},
		{Path: "stale.ts", Data: []byte("stale\n")},
	}
	if err := PublishArtifacts(path, initial, false, nil); err != nil {
		t.Fatal(err)
	}
	stableBefore, err := os.Stat(filepath.Join(path, "stable.ts"))
	if err != nil {
		t.Fatal(err)
	}
	userFile := filepath.Join(path, "notes.txt")
	if err := os.WriteFile(userFile, []byte("user-owned\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PublishArtifacts(path, []generator.Artifact{
		{Path: "stable.ts", Data: []byte("stable\n")},
		{Path: "changed.ts", Data: []byte("after\n")},
		{Path: "new.ts", Data: []byte("new\n")},
	}, true, nil); err != nil {
		t.Fatal(err)
	}
	stableAfter, err := os.Stat(filepath.Join(path, "stable.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(stableBefore, stableAfter) || !stableBefore.ModTime().Equal(stableAfter.ModTime()) {
		t.Fatal("unchanged generated artifact did not preserve identity and mtime")
	}
	if value, err := os.ReadFile(filepath.Join(path, "changed.ts")); err != nil || string(value) != "after\n" {
		t.Fatalf("changed artifact = %q, %v", value, err)
	}
	if _, err := os.Stat(filepath.Join(path, "stale.ts")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale artifact still exists: %v", err)
	}
	if value, err := os.ReadFile(userFile); err != nil || string(value) != "user-owned\n" {
		t.Fatalf("user-owned file = %q, %v", value, err)
	}
	manifest, err := ReadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) != 3 || manifest.Files["stale.ts"] != "" || manifest.Files["new.ts"] == "" {
		t.Fatalf("manifest = %#v", manifest)
	}
}

func TestGenerationManifestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generated")
	generation := &Generation{
		Generator:   "release:7.3.0",
		Target:      "typescript",
		Addons:      []string{"server"},
		InputSHA256: strings.Repeat("a", 64),
	}
	if err := PublishArtifacts(path, []generator.Artifact{{Path: "index.ts", Data: []byte("export {}\n")}}, false, generation); err != nil {
		t.Fatal(err)
	}
	manifest, err := ReadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version != 2 || !GenerationEqual(manifest.Generation, generation) {
		t.Fatalf("manifest = %#v", manifest)
	}
	if !IsPublicationError(Preflight(path, false)) {
		t.Fatal("fresh-output conflict was not classified as a publication error")
	}
}

func TestIncrementalPublisherRejectsEditedOwnedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generated")
	if err := PublishArtifacts(path, []generator.Artifact{{Path: "client.ts", Data: []byte("before\n")}}, false, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "client.ts"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := PublishArtifacts(path, []generator.Artifact{{Path: "client.ts", Data: []byte("after\n")}}, true, nil)
	if err == nil || !IsPublicationError(err) || !strings.Contains(err.Error(), "was edited") {
		t.Fatalf("edited artifact error = %v", err)
	}
	if value, readErr := os.ReadFile(filepath.Join(path, "client.ts")); readErr != nil || string(value) != "edited\n" {
		t.Fatalf("edited artifact was overwritten: %q, %v", value, readErr)
	}
}

func TestCheckManagedIsReadOnlyForExactOutput(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "generated")
	artifacts := []generator.Artifact{
		{Path: "index.ts", Data: []byte("export {}\n")},
		{Path: "nested/client.ts", Data: []byte("export const client = true\n")},
	}
	generation := &Generation{Generator: "release:7.3.0", Target: "typescript", InputSHA256: strings.Repeat("a", 64)}
	if err := PublishArtifacts(path, artifacts, false, generation); err != nil {
		t.Fatal(err)
	}
	manifestBefore, err := os.Stat(filepath.Join(path, ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	artifactBefore, err := os.Stat(filepath.Join(path, "index.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckManaged(path, generation, emitArtifacts(artifacts)); err != nil {
		t.Fatal(err)
	}
	manifestAfter, err := os.Stat(filepath.Join(path, ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	artifactAfter, err := os.Stat(filepath.Join(path, "index.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(manifestBefore, manifestAfter) || !manifestBefore.ModTime().Equal(manifestAfter.ModTime()) {
		t.Fatal("check replaced the manifest")
	}
	if !os.SameFile(artifactBefore, artifactAfter) || !artifactBefore.ModTime().Equal(artifactAfter.ModTime()) {
		t.Fatal("check replaced an artifact")
	}
	if _, err := os.Stat(path + ".openapi-sdkgen.lock"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("check created an output lock: %v", err)
	}
	for _, pattern := range []string{".openapi-sdkgen-output-*", ".openapi-sdkgen-backup-*"} {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil || len(matches) != 0 {
			t.Fatalf("check created temporary paths for %s: %v, %v", pattern, matches, err)
		}
	}
}

func TestCheckManagedDetectsArtifactAndGenerationDrift(t *testing.T) {
	generation := &Generation{Generator: "release:7.3.0", Target: "typescript", InputSHA256: strings.Repeat("a", 64)}
	otherGeneration := &Generation{Generator: "release:7.3.1", Target: "typescript", InputSHA256: strings.Repeat("a", 64)}
	for _, test := range []struct {
		name       string
		prepare    func(*testing.T, string)
		generation *Generation
		artifacts  []generator.Artifact
		want       string
	}{
		{
			name:       "content",
			generation: generation,
			artifacts:  []generator.Artifact{{Path: "index.ts", Data: []byte("changed\n")}},
			want:       "differs from the current generated content",
		},
		{
			name:       "missing generated path",
			generation: generation,
			artifacts: []generator.Artifact{
				{Path: "index.ts", Data: []byte("stable\n")},
				{Path: "new.ts", Data: []byte("new\n")},
			},
			want: "is missing generated artifact new.ts",
		},
		{
			name:       "stale generated path",
			generation: generation,
			artifacts:  nil,
			want:       "contains stale generated artifact index.ts",
		},
		{
			name:       "generation fingerprint",
			generation: otherGeneration,
			artifacts:  []generator.Artifact{{Path: "index.ts", Data: []byte("stable\n")}},
			want:       "different generation fingerprint",
		},
		{
			name: "unowned conflict",
			prepare: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(path, "new.ts"), []byte("user\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			generation: generation,
			artifacts: []generator.Artifact{
				{Path: "index.ts", Data: []byte("stable\n")},
				{Path: "new.ts", Data: []byte("new\n")},
			},
			want: "conflicts with an unowned existing path",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "generated")
			if err := PublishArtifacts(path, []generator.Artifact{{Path: "index.ts", Data: []byte("stable\n")}}, false, generation); err != nil {
				t.Fatal(err)
			}
			if test.prepare != nil {
				test.prepare(t, path)
			}
			err := CheckManaged(path, test.generation, emitArtifacts(test.artifacts))
			if err == nil || !IsPublicationError(err) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("check error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestCheckManagedRejectsEditedMissingAndMalformedManagedOutput(t *testing.T) {
	for _, test := range []struct {
		name    string
		prepare func(*testing.T, string)
		want    string
	}{
		{
			name: "edited",
			prepare: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(path, "index.ts"), []byte("edited\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "was edited",
		},
		{
			name: "missing",
			prepare: func(t *testing.T, path string) {
				t.Helper()
				if err := os.Remove(filepath.Join(path, "index.ts")); err != nil {
					t.Fatal(err)
				}
			},
			want: "is missing or unreadable",
		},
		{
			name: "malformed manifest",
			prepare: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(path, ManifestName), []byte("not json\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "decode incremental output manifest",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "generated")
			if err := PublishArtifacts(path, []generator.Artifact{{Path: "index.ts", Data: []byte("stable\n")}}, false, nil); err != nil {
				t.Fatal(err)
			}
			test.prepare(t, path)
			err := CheckManaged(path, nil, emitArtifacts([]generator.Artifact{{Path: "index.ts", Data: []byte("stable\n")}}))
			if err == nil || !IsPublicationError(err) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("check error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestAdvisoryLockBlocksLiveOwnerAndRecoversAfterProcessExit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generated")
	artifacts := []generator.Artifact{{Path: "index.ts", Data: []byte("stable\n")}}
	if err := PublishArtifacts(path, artifacts, false, nil); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(executable, "-test.run=^TestOutputLockHelperProcess$", "--", path)
	command.Env = append(os.Environ(), "OPENAPI_SDKGEN_OUTPUT_LOCK_HELPER=1")
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "locked" {
		_ = stdin.Close()
		_ = command.Wait()
		t.Fatalf("helper did not acquire lock: stdout=%q stderr=%q", scanner.Text(), stderr.String())
	}
	if publisher, err := NewPublisher(path, true); err == nil {
		publisher.Rollback()
		_ = stdin.Close()
		_ = command.Wait()
		t.Fatal("live advisory lock allowed a concurrent incremental publisher")
	} else if !strings.Contains(err.Error(), "locked by another generation") {
		_ = stdin.Close()
		_ = command.Wait()
		t.Fatalf("live lock error = %v", err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("helper exit: %v: %s", err, stderr.String())
	}
	lockPath := path + ".openapi-sdkgen.lock"
	if info, err := os.Stat(lockPath); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("persistent lock identity = %#v, %v", info, err)
	}
	publisher, err := NewPublisher(path, true)
	if err != nil {
		t.Fatalf("abandoned advisory lock was not recoverable: %v", err)
	}
	publisher.Rollback()
	if err := CheckManaged(path, nil, emitArtifacts(artifacts)); err != nil {
		t.Fatalf("read-only check rejected released persistent lock: %v", err)
	}
}

func TestOutputLockHelperProcess(t *testing.T) {
	if os.Getenv("OPENAPI_SDKGEN_OUTPUT_LOCK_HELPER") != "1" {
		return
	}
	var output string
	for index, argument := range os.Args {
		if argument == "--" && index+1 < len(os.Args) {
			output = os.Args[index+1]
			break
		}
	}
	if output == "" {
		fmt.Fprintln(os.Stderr, "missing output path")
		os.Exit(2)
	}
	lock, err := acquireOutputLock(output)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	_ = lock
	fmt.Println("locked")
	_, _ = io.Copy(io.Discard, os.Stdin)
	os.Exit(0)
}

func emitArtifacts(artifacts []generator.Artifact) func(generator.ArtifactSink) error {
	return func(sink generator.ArtifactSink) error {
		for _, artifact := range artifacts {
			if err := sink.WriteArtifact(artifact); err != nil {
				return err
			}
		}
		return nil
	}
}
