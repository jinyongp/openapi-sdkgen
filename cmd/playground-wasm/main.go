//go:build js && wasm

// Command playground-wasm exposes the in-memory generator to the docs site.
package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	"openapi-sdkgen/internal/playground"
)

type browserResult struct {
	playground.Result
	Error string `json:"error,omitempty"`
}

var generateFunction js.Func

func main() {
	generateFunction = js.FuncOf(generate)
	js.Global().Set("openapiSDKGenGenerate", generateFunction)
	select {}
}

func generate(_ js.Value, arguments []js.Value) any {
	if len(arguments) < 2 || len(arguments) > 3 {
		return encodeResult(browserResult{Error: "OpenAPI source and target are required"})
	}

	if arguments[0].Type() != js.TypeString || arguments[1].Type() != js.TypeString {
		return encodeResult(browserResult{Error: "OpenAPI source and target must be strings"})
	}
	options := playground.Options{}
	if len(arguments) == 3 {
		if arguments[2].Type() != js.TypeString {
			return encodeResult(browserResult{Error: "Generation options must be JSON text"})
		}
		if err := json.Unmarshal([]byte(arguments[2].String()), &options); err != nil {
			return encodeResult(browserResult{Error: "Invalid generation options: " + err.Error()})
		}
	}
	result, err := playground.Generate([]byte(arguments[0].String()), arguments[1].String(), options)
	if err != nil {
		return encodeResult(browserResult{Error: err.Error()})
	}
	return encodeResult(browserResult{Result: result})
}

func encodeResult(result browserResult) string {
	encoded, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, "encode playground result: "+err.Error())
	}
	return string(encoded)
}
