package golang

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"openapi-sdkgen/internal/generator"
	"openapi-sdkgen/internal/output"
)

// Optional parameter diagnosis over frozen final artifacts. Sorted-manifest
// replay differs from native emitter order and is not a generation benchmark.
func BenchmarkPackFinalOutput(b *testing.B) {
	input, destination := os.Getenv("SDKGEN_PACK_INPUT"), os.Getenv("SDKGEN_PACK_OUTPUT")
	if input == "" || destination == "" {
		b.Skip("optional frozen-output packing trial")
	}
	if !filepath.IsAbs(input) || !filepath.IsAbs(destination) {
		b.Fatal("use absolute paths; go test executes in the package directory")
	}
	if b.N != 1 {
		b.Fatal("use -benchtime=1x; output must be fresh")
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		b.Fatal("output must not exist")
	}
	inputAbs, err := filepath.Abs(input)
	if err != nil {
		b.Fatal(err)
	}
	outputAbs, err := filepath.Abs(destination)
	if err != nil {
		b.Fatal(err)
	}
	relative, err := filepath.Rel(inputAbs, outputAbs)
	if err != nil || filepath.IsLocal(relative) || relative == "." {
		b.Fatal("output must be outside frozen input")
	}
	target := packTargetBytes
	if text := os.Getenv("SDKGEN_PACK_TARGET"); text != "" {
		value, err := strconv.Atoi(text)
		if err != nil || value <= 0 {
			b.Fatal("invalid target", text)
		}
		target = value
	}
	data, err := os.ReadFile(filepath.Join(input, output.ManifestName))
	if err != nil {
		b.Fatal(err)
	}
	var manifest output.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		b.Fatal(err)
	}
	files := make([]string, 0, len(manifest.Files))
	for file := range manifest.Files {
		if !filepath.IsLocal(file) || filepath.ToSlash(filepath.Clean(file)) != file {
			b.Fatal("invalid frozen artifact path", file)
		}
		files = append(files, file)
	}
	sort.Strings(files)
	var bytes, count, peak int
	b.ResetTimer()
	err = output.StreamArtifacts(destination, false, nil, func(sink generator.ArtifactSink) error {
		packer := newArtifactPacker(generator.ArtifactSinkFunc(func(artifact generator.Artifact) error {
			bytes += len(artifact.Data)
			count++
			return sink.WriteArtifact(artifact)
		}), target, packBufferBytes)
		for _, file := range files {
			data, err := os.ReadFile(filepath.Join(input, filepath.FromSlash(file)))
			if err != nil {
				return err
			}
			hash := sha256.Sum256(data)
			if hex.EncodeToString(hash[:]) != manifest.Files[file] {
				b.Fatalf("source hash changed: %s", file)
			}
			if err := packer.WriteArtifact(generator.Artifact{Path: file, Data: data}); err != nil {
				return err
			}
		}
		if err := packer.finish(); err != nil {
			return err
		}
		peak = packer.peakBytes
		return nil
	})
	b.StopTimer()
	if err != nil {
		b.Fatal(err)
	}
	b.ReportMetric(float64(len(files)), "input-files")
	b.ReportMetric(float64(count), "output-files")
	b.ReportMetric(float64(bytes), "output-bytes")
	b.ReportMetric(float64(peak), "peak-buffer-bytes")
}
