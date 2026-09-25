package sdkgen

import (
	"testing"

	"openapi-sdkgen/internal/compiler/compatibility"
	openapidoc "openapi-sdkgen/internal/compiler/openapi"
	"openapi-sdkgen/internal/openapiwalk"
)

type contextRecordingPolicy struct {
	contexts []compatibility.Context
}

func (policy *contextRecordingPolicy) Apply(context compatibility.Context, value any) compatibility.Result {
	policy.contexts = append(policy.contexts, context)
	return compatibility.Result{Value: value}
}

func TestCompatibilitySessionUsesEntryVersionAndReferencedRootContext(t *testing.T) {
	policy := &contextRecordingPolicy{}
	session := newCompatibilitySession(map[string]any{"openapi": "3.0.4"}, policy)
	source := decodedSource{
		data:  []byte("type: string\n"),
		value: map[string]any{"type": "string"},
	}
	if _, err := session.effectiveSource("schema.yaml", source, openapiwalk.ObjectSchema); err != nil {
		t.Fatal(err)
	}
	if len(policy.contexts) == 0 {
		t.Fatal("policy was not invoked")
	}
	first := policy.contexts[0]
	if first.Version != openapidoc.Version30 || first.Object != openapiwalk.ObjectSchema || first.Source != "schema.yaml" || first.Pointer != "#" {
		t.Fatalf("root context = %#v", first)
	}
}

func TestCompatibilitySessionTracksAmbiguousReferencedContexts(t *testing.T) {
	session := newCompatibilitySession(map[string]any{"openapi": "3.1.2"}, nil)
	session.registerSourceContext("shared.yaml", openapiwalk.ObjectSchema)
	session.registerSourceContext("shared.yaml", openapiwalk.ObjectPathItem)
	if got := session.sourceContext("shared.yaml"); got != openapiwalk.ObjectUnknown {
		t.Fatalf("ambiguous source context = %q", got)
	}
	contexts := session.sourceContexts("shared.yaml")
	if len(contexts) != 2 || contexts[0] != openapiwalk.ObjectPathItem || contexts[1] != openapiwalk.ObjectSchema {
		t.Fatalf("source contexts = %#v", contexts)
	}
}
