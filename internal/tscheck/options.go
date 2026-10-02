// Package tscheck provides the shared consumer compiler contract for verification.
package tscheck

import (
	_ "embed"
	"encoding/json"
)

//go:embed strict-options.json
var profile []byte

// CompilerOptions returns a fresh map, so one verification cannot weaken another.
func CompilerOptions() map[string]any {
	var config struct {
		CompilerOptions map[string]any `json:"compilerOptions"`
	}
	if err := json.Unmarshal(profile, &config); err != nil {
		panic(err)
	}
	return config.CompilerOptions
}
