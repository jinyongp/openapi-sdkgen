package failure

import "testing"

func TestScopeEffectContracts(t *testing.T) {
	for _, test := range []struct {
		scope  Scope
		effect Effect
		valid  bool
	}{
		{ScopeNone, EffectNone, true},
		{ScopeDocument, EffectBlock, true},
		{ScopeOperation, EffectBlock, true},
		{ScopeCapability, EffectBlock, true},
		{ScopeOperation, EffectOmitOperation, true},
		{ScopeCapability, EffectOmitCapability, true},
		{ScopeDocument, EffectOmitOperation, false},
		{ScopeOperation, EffectOmitCapability, false},
		{ScopeCapability, EffectOmitOperation, false},
		{ScopeNone, EffectBlock, false},
	} {
		if got := ValidPair(test.scope, test.effect); got != test.valid {
			t.Fatalf("ValidPair(%q, %q) = %v, want %v", test.scope, test.effect, got, test.valid)
		}
	}
	if Scope("unknown").Valid() || Effect("unknown").Valid() {
		t.Fatal("unknown scope/effect unexpectedly valid")
	}
}
