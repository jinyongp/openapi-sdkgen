package output

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"openapi-sdkgen/internal/generator"
)

func TestStreamingWriteFailureWinsOverEmissionAndCleansStaging(t *testing.T) {
	for _, failEmitter := range []bool{false, true} {
		name := "commit"
		if failEmitter {
			name = "emitter also fails"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "generated")
			var staging string
			emissionFailure := errors.New("emission interrupted")
			err := StreamArtifacts(path, false, nil, func(sink generator.ArtifactSink) error {
				publisher := sink.(*Publisher)
				staging = publisher.StagingPath()
				// A real filesystem conflict makes the asynchronous write fail.
				if err := os.Mkdir(filepath.Join(staging, "blocked.ts"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := sink.WriteArtifact(generator.Artifact{Path: "blocked.ts", Data: []byte("export {}\n")}); err != nil {
					return err
				}
				if failEmitter {
					return emissionFailure
				}
				return nil
			})
			var stage *StageError
			if !errors.As(err, &stage) || stage.Stage != StagePublish || !IsPublicationError(err) {
				t.Fatalf("write failure = %v, want publication stage", err)
			}
			if !errors.Is(err, os.ErrExist) || errors.Is(err, emissionFailure) {
				t.Fatalf("filesystem cause was lost: %v", err)
			}
			for _, candidate := range []string{path, staging} {
				if _, err := os.Stat(candidate); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("partial publication remains at %s: %v", candidate, err)
				}
			}
			if err := PublishArtifacts(path, []generator.Artifact{{Path: "valid.ts", Data: []byte("export {}\n")}}, false, nil); err != nil {
				t.Fatalf("retry after failed publication: %v", err)
			}
		})
	}
}

func TestIncrementalFailureRestoresReplacedAndStaleFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generated")
	initial := []generator.Artifact{
		{Path: "a.ts", Data: []byte("before\n")},
		{Path: "stale.ts", Data: []byte("stale\n")},
	}
	if err := PublishArtifacts(path, initial, false, nil); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(path, ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := NewPublisher(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer publisher.Rollback()
	for _, artifact := range []generator.Artifact{
		{Path: "a.ts", Data: []byte("after\n")},
		{Path: "z.ts", Data: []byte("new\n")},
	} {
		if err := publisher.WriteArtifact(artifact); err != nil {
			t.Fatal(err)
		}
	}
	// Lose the later staged file so commit must undo an earlier replacement.
	staging := publisher.StagingPath()
	if err := os.Remove(filepath.Join(staging, "z.ts")); err != nil {
		t.Fatal(err)
	}
	if err := publisher.Commit(); err == nil || !IsPublicationError(err) || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("commit failure = %v, want missing staged file", err)
	}
	publisher.Rollback()
	for _, artifact := range initial {
		got, err := os.ReadFile(filepath.Join(path, artifact.Path))
		if err != nil || !bytes.Equal(got, artifact.Data) {
			t.Fatalf("rollback failed for %s: %q, %v", artifact.Path, got, err)
		}
	}
	after, err := os.ReadFile(filepath.Join(path, ManifestName))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("rollback changed manifest: %q, %v", after, err)
	}
	if _, err := os.Stat(filepath.Join(path, "z.ts")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed artifact was installed: %v", err)
	}
	if _, err := os.Stat(staging); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rollback left staging state: %v", err)
	}
	if err := PublishArtifacts(path, initial, true, nil); err != nil {
		t.Fatalf("retry after rollback must reacquire lock: %v", err)
	}
}

func TestIncrementalCommitRechecksOwnershipAfterStaging(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generated")
	if err := PublishArtifacts(path, []generator.Artifact{{Path: "index.ts", Data: []byte("before\n")}}, false, nil); err != nil {
		t.Fatal(err)
	}
	publisher, err := NewPublisher(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer publisher.Rollback()
	if err := publisher.WriteArtifact(generator.Artifact{Path: "index.ts", Data: []byte("generated\n")}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "index.ts"), []byte("user edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := publisher.Commit(); err == nil || !IsPublicationError(err) {
		t.Fatalf("commit ignored concurrent edit: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(path, "index.ts"))
	if err != nil || string(got) != "user edit\n" {
		t.Fatalf("concurrent user edit was overwritten: %q, %v", got, err)
	}
}
