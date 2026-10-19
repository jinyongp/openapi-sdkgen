package golang

import (
	"encoding/json"
	"fmt"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

func BenchmarkGoGeneration(b *testing.B) {
	paths := make(map[string]any)
	for index := 0; index < 100; index++ {
		paths[fmt.Sprintf("/items/%d", index)] = map[string]any{"get": map[string]any{"operationId": fmt.Sprintf("readItem%d", index), "responses": map[string]any{"200": map[string]any{"description": "Item", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "integer"}, "title": map[string]any{"type": "string"}}, "required": []string{"id", "title"}}}}}}}}
	}
	data, err := json.Marshal(map[string]any{"openapi": "3.2.0", "info": map[string]any{"title": "Generation workload", "version": "1"}, "paths": paths})
	if err != nil {
		b.Fatal(err)
	}
	document, err := compiler.Compile(data)
	if err != nil {
		b.Fatal(err)
	}
	options := generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}}
	target := Generator{}
	plan, findings, err := target.Prepare(document, options)
	if err != nil || diagnostic.HasErrors(findings) {
		b.Fatal(err, findings)
	}
	b.Run("prepare-100-operations", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, findings, err := target.Prepare(document, options); err != nil || diagnostic.HasErrors(findings) {
				b.Fatal(err, findings)
			}
		}
	})
	b.Run("emit-100-operations", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := target.Emit(plan); err != nil {
				b.Fatal(err)
			}
		}
	})
}
