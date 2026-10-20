// Command compat-input emits the explicitly corrected, pinnable provider input.
package main

import (
	"flag"
	"fmt"
	"os"

	"openapi-sdkgen/internal/compatinput"
)

func main() {
	input := flag.String("input", "", "original provider entry document")
	kind := flag.String("kind", "", "versioned preparation kind")
	output := flag.String("output", "", "prepared JSON output")
	flag.Parse()
	if *input == "" || *kind == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "input, kind and output are required")
		os.Exit(2)
	}
	data, operations, err := compatinput.BundleWithKind(*input, *kind)
	if err == nil {
		err = os.WriteFile(*output, data, 0600)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("{\"sha256\":%q,\"operations\":%d}\n", compatinput.Digest(data), len(operations))
}
