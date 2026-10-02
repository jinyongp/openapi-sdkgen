// clients-benchmark measures the shared compiler/target publication API using
// explicit root/client selections. It is a repository verification driver.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
	"openapi-sdkgen/internal/output"
	"openapi-sdkgen/internal/target/typescript"
)

type selectors struct {
	Operations []string `toml:"operations"`
	Routes     []string `toml:"routes"`
}

type config struct {
	Source    string     `toml:"source"`
	Target    string     `toml:"target"`
	Output    string     `toml:"output"`
	Selection *selectors `toml:"selection"`
	Clients   map[string]struct {
		Selection *selectors `toml:"selection"`
	} `toml:"clients"`
}

type report struct {
	CompileCalls         int     `json:"compileCalls"`
	PrepareCalls         int     `json:"prepareCalls"`
	EmitCalls            int     `json:"emitCalls"`
	ModuleAnalysisPasses int     `json:"moduleAnalysisPasses"`
	CompileMS            float64 `json:"compileMS"`
	PrepareMS            float64 `json:"prepareMS"`
	EmitPublishMS        float64 `json:"emitPublishMS"`
	GeneratedFiles       int     `json:"generatedFiles"`
	GeneratedBytes       int64   `json:"generatedBytes"`
	OperationModules     int     `json:"operationModules"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	path := flag.String("config", "", "measurement config")
	flag.Parse()
	data, err := os.ReadFile(*path)
	if err != nil {
		return err
	}
	var config config
	if err := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields().Decode(&config); err != nil {
		return err
	}
	if config.Target != "typescript" || config.Source == "" || config.Output == "" {
		return fmt.Errorf("source, typescript target and output are required")
	}
	resolve := func(value string) string {
		if filepath.IsAbs(value) {
			return value
		}
		return filepath.Join(filepath.Dir(*path), value)
	}
	selection := func(value *selectors) *generator.Selection {
		if value == nil {
			return nil
		}
		return &generator.Selection{Operations: value.Operations, Routes: value.Routes}
	}
	options := generator.Options{Selection: selection(config.Selection)}
	if config.Clients != nil {
		options.Clients = make(map[string]generator.Client, len(config.Clients))
		for name, client := range config.Clients {
			options.Clients[name] = generator.Client{Selection: selection(client.Selection)}
		}
	}
	target := typescript.Generator{}
	if err := generator.ValidateTargetOptions(target, options); err != nil {
		return err
	}
	var measured report
	started := time.Now()
	measured.CompileCalls++
	compiled, err := compiler.CompileInputResultWithOptions(resolve(config.Source), compiler.CompileOptions{})
	if err != nil {
		return err
	}
	measured.CompileMS = float64(time.Since(started).Microseconds()) / 1000
	started = time.Now()
	measured.PrepareCalls++
	prepared, err := generator.PrepareCompilation(target, compiled, options)
	if err != nil {
		return err
	}
	if diagnostic.HasErrors(prepared.Diagnostics) {
		return fmt.Errorf("%s", diagnostic.RenderHuman(prepared.Diagnostics, nil))
	}
	measured.PrepareMS = float64(time.Since(started).Microseconds()) / 1000
	for _, coverage := range prepared.Coverage {
		if coverage.Analyzer == "target.modules" && coverage.Status == diagnostic.CoverageComplete {
			measured.ModuleAnalysisPasses++
		}
	}
	started = time.Now()
	err = output.StreamArtifacts(resolve(config.Output), false, nil, func(sink generator.ArtifactSink) error {
		measured.EmitCalls++
		return target.EmitTo(prepared.Plan, generator.ArtifactSinkFunc(func(artifact generator.Artifact) error {
			measured.GeneratedFiles++
			measured.GeneratedBytes += int64(len(artifact.Data))
			if strings.HasPrefix(artifact.Path, "internal/operations/") {
				measured.OperationModules++
			}
			return sink.WriteArtifact(artifact)
		}))
	})
	if err != nil {
		return err
	}
	measured.EmitPublishMS = float64(time.Since(started).Microseconds()) / 1000
	return json.NewEncoder(os.Stdout).Encode(measured)
}
