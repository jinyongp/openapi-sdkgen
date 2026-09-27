package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	var mode string
	var manifestPath string
	var corpusRoot string
	var outputPath string
	var typescriptRoot string
	var typecheckTimeout time.Duration
	var offline bool

	flag.StringVar(&mode, "mode", "run", "compatibility benchmark mode: run or fetch")
	flag.StringVar(&manifestPath, "manifest", "", "benchmark corpus manifest JSON")
	flag.StringVar(&corpusRoot, "corpus-root", "", "root directory containing corpus inputs")
	flag.StringVar(&outputPath, "output", "", "benchmark report JSON destination")
	flag.StringVar(&typescriptRoot, "typescript-root", "test/typescript", "TypeScript verification workspace")
	flag.DurationVar(&typecheckTimeout, "typecheck-timeout", 3*time.Minute, "strict TypeScript timeout per document")
	flag.BoolVar(&offline, "offline", false, "verify an existing fetched corpus without network access")
	flag.Parse()

	if manifestPath == "" || corpusRoot == "" {
		fatal(fmt.Errorf("--manifest and --corpus-root are required"))
	}

	switch mode {
	case "fetch":
		if outputPath != "" {
			fatal(fmt.Errorf("--output is not valid in fetch mode"))
		}
		if err := fetchCorpus(manifestPath, corpusRoot, offline); err != nil {
			fatal(err)
		}
	case "run":
		if offline {
			fatal(fmt.Errorf("--offline is only valid in fetch mode"))
		}
		if outputPath == "" {
			fatal(fmt.Errorf("--output is required in run mode"))
		}
		if typecheckTimeout <= 0 {
			fatal(fmt.Errorf("--typecheck-timeout must be positive"))
		}
		if err := runBenchmark(manifestPath, corpusRoot, outputPath, typescriptRoot, typecheckTimeout); err != nil {
			fatal(err)
		}
	default:
		fatal(fmt.Errorf("unsupported --mode %q (available: run, fetch)", mode))
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "compat-benchmark:", err)
	os.Exit(1)
}
