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
	var documentID string

	flag.StringVar(&mode, "mode", "run", "compatibility benchmark mode: run, fetch, or merge")
	flag.StringVar(&manifestPath, "manifest", "", "benchmark corpus manifest JSON")
	flag.StringVar(&corpusRoot, "corpus-root", "", "root directory containing corpus inputs")
	flag.StringVar(&outputPath, "output", "", "benchmark report JSON destination")
	flag.StringVar(&typescriptRoot, "typescript-root", "test/typescript", "TypeScript verification workspace")
	flag.DurationVar(&typecheckTimeout, "typecheck-timeout", 3*time.Minute, "strict TypeScript timeout per document")
	flag.BoolVar(&offline, "offline", false, "verify an existing fetched corpus without network access")
	flag.StringVar(&documentID, "document", "", "benchmark only this document from the original manifest")
	flag.Parse()

	if manifestPath == "" {
		fatal(fmt.Errorf("--manifest is required"))
	}

	switch mode {
	case "fetch":
		if corpusRoot == "" || documentID != "" || len(flag.Args()) != 0 {
			fatal(fmt.Errorf("fetch requires --corpus-root and does not accept --document or report paths"))
		}
		if outputPath != "" {
			fatal(fmt.Errorf("--output is not valid in fetch mode"))
		}
		if err := fetchCorpus(manifestPath, corpusRoot, offline); err != nil {
			fatal(err)
		}
	case "run":
		if corpusRoot == "" || len(flag.Args()) != 0 {
			fatal(fmt.Errorf("run requires --corpus-root and does not accept report paths"))
		}
		if offline {
			fatal(fmt.Errorf("--offline is only valid in fetch mode"))
		}
		if outputPath == "" {
			fatal(fmt.Errorf("--output is required in run mode"))
		}
		if typecheckTimeout <= 0 {
			fatal(fmt.Errorf("--typecheck-timeout must be positive"))
		}
		if err := runBenchmarkSelected(manifestPath, corpusRoot, outputPath, typescriptRoot, typecheckTimeout, documentID); err != nil {
			fatal(err)
		}
	case "merge":
		if outputPath == "" || len(flag.Args()) == 0 || offline || documentID != "" || corpusRoot != "" {
			fatal(fmt.Errorf("merge requires --output and report paths; --corpus-root, --offline, and --document are not valid"))
		}
		if err := mergeBenchmarkReports(manifestPath, flag.Args(), outputPath); err != nil {
			fatal(err)
		}
	default:
		fatal(fmt.Errorf("unsupported --mode %q (available: run, fetch, merge)", mode))
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "compat-benchmark:", err)
	os.Exit(1)
}
