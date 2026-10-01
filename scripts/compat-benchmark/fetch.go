package main

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const corpusReceiptSchemaVersion = 1

type corpusReceipt struct {
	SchemaVersion  int    `json:"schemaVersion"`
	Repository     string `json:"repository"`
	Commit         string `json:"commit"`
	ManifestSHA256 string `json:"manifestSha256"`
}

func fetchCorpus(manifestPath, destination string, offline bool) error {
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	var manifest benchmarkManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("decode manifest: %w", err)
	}
	if err := validateManifest(manifest); err != nil {
		return err
	}
	manifestSum := sha256.Sum256(manifestData)
	manifestSHA := hex.EncodeToString(manifestSum[:])
	if offline {
		return verifyMaterializedCorpus(destination, manifest, manifestSHA)
	}
	if manifest.Source == nil && !manifest.Pinned {
		return errors.New("corpus fetch requires manifest source metadata; pinned local corpora use --offline")
	}
	if manifest.Source != nil && manifest.Source.RawBaseURL == "" {
		return errors.New("corpus source rawBaseUrl is required for fetch")
	}
	if _, err := os.Stat(destination); err == nil {
		return fmt.Errorf("corpus destination already exists; use --offline to verify it: %s", destination)
	} else if !os.IsNotExist(err) {
		return err
	}

	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(parent, ".openapi-sdkgen-compat-corpus-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	client := &http.Client{Timeout: 2 * time.Minute}
	for _, corpus := range manifest.Corpora {
		base, input := corpus.SourceURL, ""
		if manifest.Source != nil {
			base, input = manifest.Source.RawBaseURL, corpus.Input
		}
		data, err := fetchCorpusFile(client, base, input)
		if err != nil {
			return fmt.Errorf("%s: %w", corpus.ID, err)
		}
		if err := verifyCorpusBytes(corpus, data); err != nil {
			return fmt.Errorf("%s: %w", corpus.ID, err)
		}
		path, err := safeCorpusPath(staging, corpus.Input)
		if err != nil {
			return fmt.Errorf("%s: %w", corpus.ID, err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
	}
	for _, file := range manifest.Files {
		var base, input string
		if manifest.Source != nil {
			base, input = manifest.Source.RawBaseURL, file.Input
		} else {
			base, err = pinnedAuxiliaryURL(manifest.Corpora, file.Input)
			if err != nil {
				return err
			}
		}
		data, err := fetchCorpusFile(client, base, input)
		if err != nil {
			return fmt.Errorf("%s: %w", file.Input, err)
		}
		if err := verifyCorpusBytes(corpusSpec{SHA256: file.SHA256}, data); err != nil {
			return fmt.Errorf("%s: %w", file.Input, err)
		}
		path, err := safeCorpusPath(staging, file.Input)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
	}
	receipt := corpusReceipt{
		SchemaVersion:  corpusReceiptSchemaVersion,
		ManifestSHA256: manifestSHA,
	}
	if manifest.Source != nil {
		receipt.Repository, receipt.Commit = manifest.Source.Repository, manifest.Source.Commit
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(staging, ".openapi-sdkgen-compat-corpus.json"), append(encoded, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.Rename(staging, destination); err != nil {
		return fmt.Errorf("publish corpus: %w", err)
	}
	return nil
}

func pinnedAuxiliaryURL(corpora []corpusSpec, input string) (string, error) {
	var owner *corpusSpec
	for i := range corpora {
		directory := filepath.ToSlash(filepath.Dir(corpora[i].Input))
		if (directory == "." || strings.HasPrefix(input, directory+"/")) &&
			(owner == nil || len(directory) > len(filepath.Dir(owner.Input))) {
			owner = &corpora[i]
		}
	}
	if owner == nil {
		return "", fmt.Errorf("auxiliary file %q has no pinned document directory", input)
	}
	base, err := url.Parse(owner.SourceURL)
	if err != nil {
		return "", err
	}
	relative := strings.TrimPrefix(input, filepath.ToSlash(filepath.Dir(owner.Input))+"/")
	return base.ResolveReference(&url.URL{Path: relative}).String(), nil
}

func fetchCorpusFile(client *http.Client, rawBaseURL, input string) ([]byte, error) {
	base, err := url.Parse(rawBaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse raw base URL: %w", err)
	}
	if base.Scheme != "https" && base.Scheme != "http" {
		return nil, fmt.Errorf("unsupported raw base URL scheme %q", base.Scheme)
	}
	relative, err := url.Parse(strings.TrimPrefix(filepath.ToSlash(input), "/"))
	if err != nil {
		return nil, err
	}
	target := base.ResolveReference(relative)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", target.Redacted(), err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: HTTP %d", target.Redacted(), response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 128<<20))
	if err != nil {
		return nil, err
	}
	if len(data) == 128<<20 {
		return nil, errors.New("corpus file exceeds 128 MiB limit")
	}
	return data, nil
}

func verifyMaterializedCorpus(root string, manifest benchmarkManifest, manifestSHA string) error {
	if manifest.Source == nil {
		if !manifest.Pinned {
			return errors.New("manifest requires source metadata or a pinned local corpus")
		}
		return verifyPinnedCorpusFiles(root, manifest)
	}
	receiptData, err := os.ReadFile(filepath.Join(root, ".openapi-sdkgen-compat-corpus.json"))
	if err != nil {
		return fmt.Errorf("read corpus receipt: %w", err)
	}
	var receipt corpusReceipt
	if err := json.Unmarshal(receiptData, &receipt); err != nil {
		return fmt.Errorf("decode corpus receipt: %w", err)
	}
	if manifest.Source == nil {
		return errors.New("manifest source metadata is required")
	}
	if receipt.SchemaVersion != corpusReceiptSchemaVersion ||
		receipt.Repository != manifest.Source.Repository ||
		receipt.Commit != manifest.Source.Commit ||
		receipt.ManifestSHA256 != manifestSHA {
		return fmt.Errorf("corpus receipt does not match manifest/source identity")
	}
	for _, corpus := range manifest.Corpora {
		path, err := safeCorpusPath(root, corpus.Input)
		if err != nil {
			return fmt.Errorf("%s: %w", corpus.ID, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("%s: read materialized input: %w", corpus.ID, err)
		}
		if err := verifyCorpusBytes(corpus, data); err != nil {
			return fmt.Errorf("%s: %w", corpus.ID, err)
		}
	}
	return verifyPinnedAuxiliaryFiles(root, manifest.Files)
}

func verifyPinnedCorpusFiles(root string, manifest benchmarkManifest) error {
	for _, corpus := range manifest.Corpora {
		path, err := safeCorpusPath(root, corpus.Input)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := verifyCorpusBytes(corpus, data); err != nil {
			return fmt.Errorf("%s: %w", corpus.ID, err)
		}
	}
	return verifyPinnedAuxiliaryFiles(root, manifest.Files)
}

func verifyPinnedAuxiliaryFiles(root string, files []pinnedCorpusFile) error {
	for _, file := range files {
		path, err := safeCorpusPath(root, file.Input)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := verifyCorpusBytes(corpusSpec{SHA256: file.SHA256}, data); err != nil {
			return fmt.Errorf("%s: %w", file.Input, err)
		}
	}
	return nil
}

func validSHA256(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func verifyCorpusBytes(corpus corpusSpec, data []byte) error {
	if corpus.SHA256 != "" {
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != corpus.SHA256 {
			return errors.New("sha256 mismatch")
		}
	}
	if corpus.OpenAPIVersion != "" {
		value, err := decodeFeatureInput(data)
		if err != nil {
			return err
		}
		root, _ := value.(map[string]any)
		if root["openapi"] != corpus.OpenAPIVersion {
			return fmt.Errorf("OpenAPI version mismatch: got %v want %s", root["openapi"], corpus.OpenAPIVersion)
		}
	}
	if corpus.Bytes != 0 && int64(len(data)) != corpus.Bytes {
		return fmt.Errorf("byte size mismatch: got %d want %d", len(data), corpus.Bytes)
	}
	if corpus.GitBlob != "" {
		got := gitBlobSHA(data)
		if got != corpus.GitBlob {
			return fmt.Errorf("git blob mismatch: got %s want %s", got, corpus.GitBlob)
		}
	}
	return nil
}

func gitBlobSHA(data []byte) string {
	hash := sha1.New() // Git's v1 object identity is SHA-1 by definition.
	fmt.Fprintf(hash, "blob %d\x00", len(data))
	_, _ = hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}
