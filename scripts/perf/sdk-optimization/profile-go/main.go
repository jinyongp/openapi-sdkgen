// Command profile-go separates preparation, final emission and publication.
// It uses the product publisher and never changes package placement or sources.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
	"openapi-sdkgen/internal/output"
	"openapi-sdkgen/internal/target/golang"
)

type packageCount struct {
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
}

type checkpoint struct {
	Phase          string                  `json:"phase"`
	ElapsedSeconds float64                 `json:"elapsedSeconds"`
	LiveHeapBytes  uint64                  `json:"liveHeapBytes"`
	GC             uint32                  `json:"gc"`
	Files          int                     `json:"files"`
	Bytes          int64                   `json:"bytes"`
	Packages       map[string]packageCount `json:"packages,omitempty"`
}

func main() {
	input := flag.String("input", "", "pinned local OpenAPI input")
	sdk := flag.String("output", "", "fresh generated SDK directory")
	report := flag.String("report", "", "fresh JSONL phase report")
	profile := flag.String("profile", "", "optional intrusive CPU profile")
	flag.Parse()
	if *input == "" || *sdk == "" || *report == "" {
		panic("input, output and report required")
	}
	if _, err := os.Stat(*sdk); !os.IsNotExist(err) {
		panic("fresh output required")
	}
	log, err := os.OpenFile(*report, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	defer log.Close()
	if *profile != "" {
		cpu, err := os.OpenFile(*profile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			panic(err)
		}
		defer cpu.Close()
		if err := pprof.StartCPUProfile(cpu); err != nil {
			panic(err)
		}
		defer pprof.StopCPUProfile()
	}
	encoder := json.NewEncoder(log)
	started := time.Now()
	state := checkpoint{Packages: map[string]packageCount{}}
	mark := func(phase string) {
		var memory runtime.MemStats
		runtime.ReadMemStats(&memory)
		state.Phase = phase
		state.ElapsedSeconds = time.Since(started).Seconds()
		state.LiveHeapBytes = memory.HeapAlloc
		state.GC = memory.NumGC
		if err := encoder.Encode(state); err != nil {
			panic(err)
		}
		fmt.Fprintf(os.Stderr, "phase=%s seconds=%.3f files=%d bytes=%d heap=%d GC=%d\n", phase, state.ElapsedSeconds, state.Files, state.Bytes, state.LiveHeapBytes, state.GC)
	}
	mark("compile-start")
	compiled, err := compiler.CompileFileResultWithOptions(*input, compiler.CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		panic(err)
	}
	mark("compile-finished")
	registry, err := generator.NewAddonRegistry(generator.AddonServer, generator.AddonMetadata)
	if err != nil {
		panic(err)
	}
	options, err := registry.Resolve([]string{"server"})
	if err != nil {
		panic(err)
	}
	options.DiagnosticMode = diagnostic.ModeCollect
	options.Golang = &generator.GoOptions{ImportPath: "example.test/graph/api"}
	mark("prepare-start")
	prepared, err := generator.PrepareCompilation(golang.Generator{}, compiled, options)
	if err != nil {
		panic(err)
	}
	if diagnostic.HasErrors(prepared.Diagnostics) {
		panic("generation blocked")
	}
	mark("prepare-finished")
	// The plan owns everything needed by emission; release input/compiler state.
	compiled = compiler.Result{}
	runtime.GC()
	mark("input-released")
	metadata, err := os.OpenFile(*report+".files.jsonl", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	defer metadata.Close()
	inventory := json.NewEncoder(metadata)
	mark("publisher-start")
	err = output.StreamArtifacts(*sdk, false, nil, func(sink generator.ArtifactSink) error {
		mark("emit-start")
		err := generator.EmitTo(golang.Generator{}, prepared.Plan, generator.ArtifactSinkFunc(func(a generator.Artifact) error {
			state.Files++
			state.Bytes += int64(len(a.Data))
			key := filepath.ToSlash(filepath.Dir(a.Path))
			count := state.Packages[key]
			count.Files++
			count.Bytes += int64(len(a.Data))
			state.Packages[key] = count
			sum := sha256.Sum256(a.Data)
			if err := inventory.Encode(struct {
				Path  string `json:"path"`
				Bytes int    `json:"bytes"`
				SHA   string `json:"sha256"`
			}{a.Path, len(a.Data), hex.EncodeToString(sum[:])}); err != nil {
				return err
			}
			if state.Files%25000 == 0 {
				mark("emit-progress")
			}
			return sink.WriteArtifact(a)
		}))
		mark("emit-finished")
		return err
	})
	if err != nil {
		panic(err)
	}
	mark("publish-finished")
}
