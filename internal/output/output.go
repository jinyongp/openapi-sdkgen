// Package output owns rollback-safe publication of generated SDK artifacts.
package output

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"openapi-sdkgen/internal/generator"
)

// PublicationError marks an author/environment-correctable output failure.
type PublicationError struct {
	cause error
}

func (value *PublicationError) Error() string { return value.cause.Error() }
func (value *PublicationError) Unwrap() error { return value.cause }

func publicationFailure(cause error) error {
	var existing *PublicationError
	if errors.As(cause, &existing) {
		return cause
	}
	return &PublicationError{cause: cause}
}

// IsPublicationError reports whether an error belongs to output state rather
// than generator implementation.
func IsPublicationError(err error) bool {
	var failure *PublicationError
	return errors.As(err, &failure)
}

// Stage identifies whether streaming failed during emission or publication.
type Stage string

const (
	StageEmit    Stage = "emit"
	StagePublish Stage = "publish"
)

// StageError preserves the streaming stage that failed.
type StageError struct {
	Stage Stage
	Err   error
}

func (value *StageError) Error() string { return value.Err.Error() }
func (value *StageError) Unwrap() error { return value.Err }

// Generation fingerprints the generator inputs that produced a managed output.
type Generation struct {
	Generator   string   `json:"generator"`
	Target      string   `json:"target"`
	Addons      []string `json:"addons,omitempty"`
	InputSHA256 string   `json:"inputSha256"`
}

// Manifest records generator-owned output files and their content hashes.
type Manifest struct {
	Version    int               `json:"version"`
	Files      map[string]string `json:"files"`
	Generation *Generation       `json:"generation,omitempty"`
}

// ManifestName is the reserved managed-output manifest path.
const ManifestName = ".openapi-sdkgen-manifest.json"

// Publisher stages artifacts and atomically commits them to one output tree.
type Publisher struct {
	output             string
	staging            string
	seen               map[string]bool
	directories        map[string]bool
	hashes             map[string]string
	previous           map[string]string
	generation         *Generation
	previousGeneration *Generation
	incremental        bool
	lock               *outputLock
	committed          bool
	failure            error
}

// Checker compares emitted artifacts with one managed output without writing.
type Checker struct {
	output             string
	seen               map[string]bool
	hashes             map[string]string
	previous           map[string]string
	previousGeneration *Generation
	failure            error
}

// StagingPath returns the private staging directory for verification.
func (publisher *Publisher) StagingPath() string {
	if publisher == nil {
		return ""
	}
	return publisher.staging
}

// CheckManaged compares a generated artifact stream with an existing managed
// output. It never creates an output, staging directory, backup, manifest, or
// lock file.
func CheckManaged(path string, generation *Generation, emit func(generator.ArtifactSink) error) error {
	if emit == nil {
		return errors.New("artifact emitter is required")
	}
	checker, err := NewChecker(path)
	if err != nil {
		return err
	}
	if err := emit(checker); err != nil {
		if checker.failure != nil {
			return checker.failure
		}
		return err
	}
	return checker.Complete(generation)
}

// NewChecker opens one existing managed output for read-only comparison.
func NewChecker(path string) (*Checker, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, publicationFailure(fmt.Errorf("inspect managed output %s: %w", path, err))
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, publicationFailure(fmt.Errorf("output path %s must not be a symlink", path))
	}
	if !info.IsDir() {
		return nil, publicationFailure(fmt.Errorf("managed output path %s must be a directory", path))
	}
	if err := checkExistingOutputLock(path, "managed output"); err != nil {
		return nil, err
	}
	manifest, err := ReadManifest(path)
	if err != nil {
		return nil, publicationFailure(err)
	}
	if err := ValidateOwnedFiles(path, manifest.Files); err != nil {
		return nil, publicationFailure(err)
	}
	return &Checker{
		output:             path,
		seen:               make(map[string]bool),
		hashes:             make(map[string]string),
		previous:           manifest.Files,
		previousGeneration: manifest.Generation,
	}, nil
}

// WriteArtifact records one emitted artifact for read-only comparison.
func (checker *Checker) WriteArtifact(artifact generator.Artifact) error {
	cleanPath, err := SafeArtifactPath(artifact.Path)
	if err != nil {
		checker.failure = err
		return err
	}
	if checker.seen[cleanPath] {
		err := fmt.Errorf("duplicate generated artifact %q", cleanPath)
		checker.failure = err
		return err
	}
	if cleanPath == ManifestName {
		err := fmt.Errorf("generated artifact path %q is reserved", cleanPath)
		checker.failure = err
		return err
	}
	checker.seen[cleanPath] = true
	checker.hashes[cleanPath] = artifactContentHash(artifact.Data)
	return nil
}

// Complete verifies generation identity and exact generated artifact parity.
func (checker *Checker) Complete(generation *Generation) error {
	if !GenerationEqual(checker.previousGeneration, generation) {
		return publicationFailure(fmt.Errorf("managed output %s has a different generation fingerprint", checker.output))
	}
	generated := make([]string, 0, len(checker.hashes))
	for path := range checker.hashes {
		generated = append(generated, path)
	}
	sort.Strings(generated)
	for _, path := range generated {
		expected := checker.hashes[path]
		previous, owned := checker.previous[path]
		if owned {
			if previous != expected {
				return publicationFailure(fmt.Errorf("managed output artifact %s differs from the current generated content", filepath.Join(checker.output, path)))
			}
			continue
		}
		fullPath := filepath.Join(checker.output, path)
		if err := validateExistingSafeParents(checker.output, filepath.Dir(fullPath)); err != nil {
			return publicationFailure(err)
		}
		if _, err := os.Lstat(fullPath); err == nil {
			return publicationFailure(fmt.Errorf("generated artifact %s conflicts with an unowned existing path", fullPath))
		} else if !errors.Is(err, os.ErrNotExist) {
			return publicationFailure(fmt.Errorf("inspect generated artifact path %s: %w", fullPath, err))
		}
		return publicationFailure(fmt.Errorf("managed output %s is missing generated artifact %s", checker.output, path))
	}
	stale := make([]string, 0)
	for path := range checker.previous {
		if !checker.seen[path] {
			stale = append(stale, path)
		}
	}
	sort.Strings(stale)
	if len(stale) != 0 {
		return publicationFailure(fmt.Errorf("managed output %s contains stale generated artifact %s", checker.output, stale[0]))
	}
	return nil
}

// PublishArtifacts validates and publishes a collected artifact set.
func PublishArtifacts(path string, artifacts []generator.Artifact, incremental bool, generation *Generation) error {
	artifacts = append([]generator.Artifact(nil), artifacts...)
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	seen := make(map[string]bool, len(artifacts))
	for _, artifact := range artifacts {
		cleanPath, err := SafeArtifactPath(artifact.Path)
		if err != nil {
			return err
		}
		if seen[cleanPath] {
			return fmt.Errorf("duplicate generated artifact %q", cleanPath)
		}
		seen[cleanPath] = true
	}
	publisher, err := NewPublisher(path, incremental)
	if err != nil {
		return err
	}
	defer publisher.Rollback()
	publisher.generation = generation
	for _, artifact := range artifacts {
		if err := publisher.WriteArtifact(artifact); err != nil {
			return err
		}
	}
	return publisher.Commit()
}

// StreamArtifacts publishes artifacts produced through a generator sink.
func StreamArtifacts(path string, incremental bool, generation *Generation, emit func(generator.ArtifactSink) error) error {
	if emit == nil {
		return &StageError{Stage: StageEmit, Err: errors.New("artifact emitter is required")}
	}
	publisher, err := NewPublisher(path, incremental)
	if err != nil {
		return &StageError{Stage: StagePublish, Err: err}
	}
	defer publisher.Rollback()
	publisher.generation = generation
	if err := emit(publisher); err != nil {
		stage := StageEmit
		if publisher.failure != nil {
			stage = StagePublish
			err = publisher.failure
		}
		return &StageError{Stage: stage, Err: err}
	}
	if err := publisher.Commit(); err != nil {
		return &StageError{Stage: StagePublish, Err: err}
	}
	return nil
}

// GenerationMatches validates one managed output and compares its generation
// fingerprint. It intentionally acquires the incremental publication lock so a
// matching fast path cannot race another writer.
func GenerationMatches(path string, expected *Generation) (bool, error) {
	publisher, err := newIncrementalPublisher(path)
	if err != nil {
		return false, err
	}
	defer publisher.Rollback()
	return GenerationEqual(publisher.previousGeneration, expected), nil
}

// GenerationEqual compares two generation fingerprints.
func GenerationEqual(left, right *Generation) bool {
	if left == nil || right == nil {
		return left == right
	}
	if left.Generator != right.Generator || left.Target != right.Target || left.InputSHA256 != right.InputSHA256 || len(left.Addons) != len(right.Addons) {
		return false
	}
	for index := range left.Addons {
		if left.Addons[index] != right.Addons[index] {
			return false
		}
	}
	return true
}

func validateGeneration(generation Generation) error {
	if generation.Generator == "" || generation.Target == "" || len(generation.InputSHA256) != sha256.Size*2 {
		return errors.New("required fields are missing")
	}
	if _, err := hex.DecodeString(generation.InputSHA256); err != nil {
		return errors.New("input digest is not SHA-256")
	}
	for index, addon := range generation.Addons {
		if addon == "" || (index > 0 && generation.Addons[index-1] >= addon) {
			return errors.New("add-ons are not unique stable names")
		}
	}
	return nil
}

// Preflight checks whether the requested fresh or incremental output can be
// used before compilation begins.
func Preflight(path string, incremental bool) error {
	info, err := os.Lstat(path)
	if !incremental {
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return publicationFailure(fmt.Errorf("output path %s must not be a symlink", path))
			}
			return publicationFailure(fmt.Errorf("output path %s already exists; choose a fresh directory or use --incremental for managed output", path))
		}
		if !errors.Is(err, os.ErrNotExist) {
			return publicationFailure(fmt.Errorf("inspect output path %s: %w", path, err))
		}
		return nil
	}

	if err := checkExistingOutputLock(path, "incremental output"); err != nil {
		return err
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return publicationFailure(fmt.Errorf("inspect output path %s: %w", path, err))
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return publicationFailure(fmt.Errorf("output path %s must not be a symlink", path))
	}
	if !info.IsDir() {
		return publicationFailure(fmt.Errorf("incremental output path %s must be a directory", path))
	}
	if _, err := ReadManifestFiles(path); err != nil {
		return publicationFailure(err)
	}
	return nil
}

// NewPublisher creates a rollback-safe fresh or incremental publisher.
func NewPublisher(path string, incremental bool) (*Publisher, error) {
	if incremental {
		return newIncrementalPublisher(path)
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, publicationFailure(fmt.Errorf("output path %s must not be a symlink", path))
		}
		return nil, publicationFailure(fmt.Errorf("output path %s already exists; choose a fresh directory or use --incremental for managed output", path))
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, publicationFailure(fmt.Errorf("inspect output path %s: %w", path, err))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, publicationFailure(fmt.Errorf("create output parent directory: %w", err))
	}
	staging, err := os.MkdirTemp(filepath.Dir(path), ".openapi-sdkgen-output-*")
	if err != nil {
		return nil, publicationFailure(fmt.Errorf("create output staging directory: %w", err))
	}
	return &Publisher{
		output: path, staging: staging, seen: make(map[string]bool), directories: make(map[string]bool), hashes: make(map[string]string),
	}, nil
}

// WriteArtifact stages one validated artifact.
func (publisher *Publisher) WriteArtifact(artifact generator.Artifact) error {
	cleanPath, err := SafeArtifactPath(artifact.Path)
	if err != nil {
		publisher.failure = err
		return err
	}
	if publisher.seen[cleanPath] {
		err := fmt.Errorf("duplicate generated artifact %q", cleanPath)
		publisher.failure = err
		return err
	}
	if cleanPath == ManifestName {
		err := fmt.Errorf("generated artifact path %q is reserved", cleanPath)
		publisher.failure = err
		return err
	}
	publisher.seen[cleanPath] = true
	hash := artifactContentHash(artifact.Data)
	publisher.hashes[cleanPath] = hash
	if publisher.incremental && publisher.previous[cleanPath] == hash {
		return nil
	}
	path := filepath.Join(publisher.staging, cleanPath)
	directory := filepath.Dir(path)
	if !publisher.directories[directory] {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			err = publicationFailure(fmt.Errorf("create artifact directory %s: %w", directory, err))
			publisher.failure = err
			return err
		}
		publisher.directories[directory] = true
	}
	if err := writeStagedFile(path, artifact.Data); err != nil {
		err = publicationFailure(err)
		publisher.failure = err
		return err
	}
	return nil
}

// Commit atomically publishes all staged artifacts.
func (publisher *Publisher) Commit() error {
	if publisher.incremental {
		return publisher.commitIncremental()
	}
	if err := writeStagedManifest(publisher.staging, publisher.hashes, publisher.generation); err != nil {
		return err
	}
	if err := os.Rename(publisher.staging, publisher.output); err != nil {
		return publicationFailure(fmt.Errorf("publish generated output %s: %w", publisher.output, err))
	}
	publisher.committed = true
	return nil
}

// Rollback removes uncommitted staging state and releases the incremental lock.
func (publisher *Publisher) Rollback() {
	if publisher != nil && !publisher.committed {
		_ = os.RemoveAll(publisher.staging)
		publisher.releaseLock()
	}
}

func newIncrementalPublisher(path string) (*Publisher, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, publicationFailure(fmt.Errorf("create output parent directory: %w", err))
	}
	lock, err := acquireOutputLock(path)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*Publisher, error) {
		lock.release()
		return nil, err
	}

	previous := map[string]string{}
	var previousGeneration *Generation
	if info, statErr := os.Lstat(path); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fail(publicationFailure(fmt.Errorf("output path %s must not be a symlink", path)))
		}
		if !info.IsDir() {
			return fail(publicationFailure(fmt.Errorf("incremental output path %s must be a directory", path)))
		}
		manifest, manifestErr := ReadManifest(path)
		err = manifestErr
		if err != nil {
			return fail(publicationFailure(err))
		}
		previous = manifest.Files
		previousGeneration = manifest.Generation
		if err := ValidateOwnedFiles(path, previous); err != nil {
			return fail(publicationFailure(err))
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fail(publicationFailure(fmt.Errorf("inspect output path %s: %w", path, statErr)))
	}
	staging, err := os.MkdirTemp(filepath.Dir(path), ".openapi-sdkgen-output-*")
	if err != nil {
		return fail(publicationFailure(fmt.Errorf("create output staging directory: %w", err)))
	}
	return &Publisher{
		output: path, staging: staging, seen: make(map[string]bool), directories: make(map[string]bool), hashes: make(map[string]string),
		previous: previous, previousGeneration: previousGeneration, incremental: true, lock: lock,
	}, nil
}

func artifactContentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func manifestData(hashes map[string]string, generation *Generation) ([]byte, error) {
	manifest := Manifest{Version: 1, Files: hashes}
	if generation != nil {
		if err := validateGeneration(*generation); err != nil {
			return nil, fmt.Errorf("encode generated artifact manifest fingerprint: %w", err)
		}
		manifest.Version = 2
		manifest.Generation = generation
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode generated artifact manifest: %w", err)
	}
	return append(data, '\n'), nil
}

func writeStagedManifest(directory string, hashes map[string]string, generation *Generation) error {
	data, err := manifestData(hashes, generation)
	if err != nil {
		return err
	}
	if err := writeStagedFile(filepath.Join(directory, ManifestName), data); err != nil {
		return publicationFailure(fmt.Errorf("write generated artifact manifest: %w", err))
	}
	return nil
}

func writeManifest(directory string, hashes map[string]string, generation *Generation) error {
	data, err := manifestData(hashes, generation)
	if err != nil {
		return err
	}
	if err := writeAtomicFile(filepath.Join(directory, ManifestName), data); err != nil {
		return publicationFailure(fmt.Errorf("write generated artifact manifest: %w", err))
	}
	return nil
}

// ReadManifestFiles returns the managed artifact hashes.
func ReadManifestFiles(path string) (map[string]string, error) {
	manifest, err := ReadManifest(path)
	return manifest.Files, err
}

// ReadManifest decodes and validates a managed-output manifest.
func ReadManifest(path string) (Manifest, error) {
	manifestPath := filepath.Join(path, ManifestName)
	info, err := os.Lstat(manifestPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("incremental output %s requires a valid %s: %w", path, ManifestName, err)
	}
	if !info.Mode().IsRegular() {
		return Manifest{}, fmt.Errorf("incremental output manifest %s must be a regular file", manifestPath)
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("read incremental output manifest %s: %w", manifestPath, err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode incremental output manifest %s: %w", manifestPath, err)
	}
	if (manifest.Version != 1 && manifest.Version != 2) || manifest.Files == nil || (manifest.Version == 2) != (manifest.Generation != nil) {
		return Manifest{}, fmt.Errorf("incremental output manifest %s has unsupported or incomplete content", manifestPath)
	}
	if manifest.Generation != nil {
		if err := validateGeneration(*manifest.Generation); err != nil {
			return Manifest{}, fmt.Errorf("incremental output manifest %s has invalid generation fingerprint: %w", manifestPath, err)
		}
	}
	artifactPaths := make([]string, 0, len(manifest.Files))
	for artifactPath := range manifest.Files {
		artifactPaths = append(artifactPaths, artifactPath)
	}
	sort.Strings(artifactPaths)
	for _, artifactPath := range artifactPaths {
		hash := manifest.Files[artifactPath]
		clean, err := SafeArtifactPath(artifactPath)
		if err != nil || clean != artifactPath || artifactPath == ManifestName || len(hash) != sha256.Size*2 {
			return Manifest{}, fmt.Errorf("incremental output manifest contains invalid artifact %q", artifactPath)
		}
		if _, err := hex.DecodeString(hash); err != nil {
			return Manifest{}, fmt.Errorf("incremental output manifest contains invalid hash for %q", artifactPath)
		}
	}
	return manifest, nil
}

// ValidateOwnedFiles verifies that every manifest-owned artifact is unchanged.
func ValidateOwnedFiles(path string, files map[string]string) error {
	buffer := make([]byte, 32*1024)
	artifactPaths := make([]string, 0, len(files))
	for artifactPath := range files {
		artifactPaths = append(artifactPaths, artifactPath)
	}
	sort.Strings(artifactPaths)
	for _, artifactPath := range artifactPaths {
		expected := files[artifactPath]
		fullPath := filepath.Join(path, artifactPath)
		if err := validateSafeParents(path, filepath.Dir(fullPath)); err != nil {
			return err
		}
		info, err := os.Lstat(fullPath)
		if err != nil {
			return fmt.Errorf("manifest-owned generated artifact %s is missing or unreadable: %w", fullPath, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("manifest-owned generated artifact %s must be a regular file", fullPath)
		}
		actual, err := fileHash(fullPath, buffer)
		if err != nil {
			return fmt.Errorf("read manifest-owned generated artifact %s: %w", fullPath, err)
		}
		if actual != expected {
			return fmt.Errorf("manifest-owned generated artifact %s was edited; refusing incremental overwrite", fullPath)
		}
	}
	return nil
}

func fileHash(path string, buffer []byte) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	if _, err := io.CopyBuffer(hash, struct{ io.Reader }{file}, buffer); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (publisher *Publisher) commitIncremental() error {
	if err := ValidateOwnedFiles(publisher.output, publisher.previous); err != nil && len(publisher.previous) != 0 {
		return publicationFailure(err)
	}
	if _, err := os.Stat(publisher.output); errors.Is(err, os.ErrNotExist) {
		if err := writeStagedManifest(publisher.staging, publisher.hashes, publisher.generation); err != nil {
			return err
		}
		if err := os.Rename(publisher.staging, publisher.output); err != nil {
			return publicationFailure(fmt.Errorf("publish generated output %s: %w", publisher.output, err))
		}
		publisher.committed = true
		publisher.releaseLock()
		return nil
	}

	changed := make([]string, 0)
	stale := make([]string, 0)
	for path, hash := range publisher.hashes {
		if publisher.previous[path] != hash {
			changed = append(changed, path)
		}
	}
	for path := range publisher.previous {
		if _, exists := publisher.hashes[path]; !exists {
			stale = append(stale, path)
		}
	}
	sort.Strings(changed)
	sort.Strings(stale)
	if len(changed) == 0 && len(stale) == 0 && GenerationEqual(publisher.previousGeneration, publisher.generation) {
		publisher.committed = true
		_ = os.RemoveAll(publisher.staging)
		publisher.releaseLock()
		return nil
	}
	for _, path := range changed {
		if _, owned := publisher.previous[path]; owned {
			continue
		}
		fullPath := filepath.Join(publisher.output, path)
		if _, err := os.Lstat(fullPath); err == nil {
			return publicationFailure(fmt.Errorf("generated artifact %s conflicts with an unowned existing path", fullPath))
		} else if !errors.Is(err, os.ErrNotExist) {
			return publicationFailure(fmt.Errorf("inspect generated artifact path %s: %w", fullPath, err))
		}
	}

	backup, err := os.MkdirTemp(filepath.Dir(publisher.output), ".openapi-sdkgen-backup-*")
	if err != nil {
		return publicationFailure(fmt.Errorf("create incremental backup: %w", err))
	}
	defer os.RemoveAll(backup)
	backed := make([]string, 0, len(changed)+len(stale)+1)
	installed := make([]string, 0, len(changed)+1)
	rollback := func(cause error) error {
		for index := len(installed) - 1; index >= 0; index-- {
			_ = os.Remove(filepath.Join(publisher.output, installed[index]))
		}
		for index := len(backed) - 1; index >= 0; index-- {
			path := backed[index]
			_ = os.MkdirAll(filepath.Dir(filepath.Join(publisher.output, path)), 0o755)
			_ = os.Rename(filepath.Join(backup, path), filepath.Join(publisher.output, path))
		}
		return cause
	}
	backupPath := func(path string) error {
		source := filepath.Join(publisher.output, path)
		target := filepath.Join(backup, path)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.Rename(source, target); err != nil {
			return err
		}
		backed = append(backed, path)
		return nil
	}
	if err := backupPath(ManifestName); err != nil {
		return rollback(publicationFailure(fmt.Errorf("backup incremental manifest: %w", err)))
	}
	for _, path := range append(append([]string(nil), changed...), stale...) {
		if _, owned := publisher.previous[path]; !owned {
			continue
		}
		if err := backupPath(path); err != nil {
			return rollback(publicationFailure(fmt.Errorf("backup generated artifact %s: %w", path, err)))
		}
	}
	for _, path := range changed {
		target := filepath.Join(publisher.output, path)
		if err := ensureSafeParents(publisher.output, filepath.Dir(target)); err != nil {
			return rollback(publicationFailure(err))
		}
		if err := os.Rename(filepath.Join(publisher.staging, path), target); err != nil {
			return rollback(publicationFailure(fmt.Errorf("replace generated artifact %s: %w", target, err)))
		}
		installed = append(installed, path)
	}
	if err := writeManifest(publisher.output, publisher.hashes, publisher.generation); err != nil {
		return rollback(err)
	}
	installed = append(installed, ManifestName)
	for _, path := range stale {
		removeEmptyParents(publisher.output, filepath.Dir(filepath.Join(publisher.output, path)))
	}
	publisher.committed = true
	_ = os.RemoveAll(publisher.staging)
	publisher.releaseLock()
	return nil
}

func ensureSafeParents(output, directory string) error {
	return safeParents(output, directory, true)
}

func validateSafeParents(output, directory string) error {
	return safeParents(output, directory, false)
}

func validateExistingSafeParents(output, directory string) error {
	relative, err := filepath.Rel(output, directory)
	if err != nil {
		return fmt.Errorf("resolve generated artifact directory: %w", err)
	}
	current := output
	for _, segment := range strings.Split(relative, string(filepath.Separator)) {
		if segment == "." || segment == "" {
			continue
		}
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("artifact directory %s is not a safe directory", current)
		}
	}
	return nil
}

func safeParents(output, directory string, create bool) error {
	relative, err := filepath.Rel(output, directory)
	if err != nil {
		return fmt.Errorf("resolve generated artifact directory: %w", err)
	}
	current := output
	for _, segment := range strings.Split(relative, string(filepath.Separator)) {
		if segment == "." || segment == "" {
			continue
		}
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			if !create {
				return fmt.Errorf("artifact directory %s is missing", current)
			}
			if err := os.Mkdir(current, 0o755); err != nil {
				return fmt.Errorf("create artifact directory %s: %w", current, err)
			}
			continue
		}
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("artifact directory %s is not a safe directory", current)
		}
	}
	return nil
}

func removeEmptyParents(output, directory string) {
	for directory != output && strings.HasPrefix(directory, output+string(filepath.Separator)) {
		if err := os.Remove(directory); err != nil {
			return
		}
		directory = filepath.Dir(directory)
	}
}

func (publisher *Publisher) releaseLock() {
	if publisher.lock != nil {
		publisher.lock.release()
		publisher.lock = nil
	}
}

// SafeArtifactPath validates and normalizes one generated relative path.
func SafeArtifactPath(value string) (string, error) {
	cleanPath := filepath.Clean(filepath.FromSlash(value))
	if cleanPath == "." || filepath.IsAbs(cleanPath) || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid generated artifact path %q", value)
	}
	return cleanPath, nil
}

func writeStagedFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create generated artifact %s: %w", path, err)
	}
	failed := true
	defer func() {
		if failed {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write generated artifact %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close generated artifact %s: %w", path, err)
	}
	failed = false
	return nil
}

func writeAtomicFile(path string, data []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".openapi-sdkgen-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary artifact %s: %w", path, err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write generated artifact %s: %w", path, err)
	}
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return fmt.Errorf("set generated artifact mode %s: %w", path, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close generated artifact %s: %w", path, err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace generated artifact %s: %w", path, err)
	}
	return nil
}
