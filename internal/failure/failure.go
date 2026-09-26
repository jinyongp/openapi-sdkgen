// Package failure defines shared generation failure ownership and effect contracts.
package failure

// Scope identifies the smallest semantic owner of one generation finding.
type Scope string

const (
	ScopeNone       Scope = ""
	ScopeDocument   Scope = "document"
	ScopeOperation  Scope = "operation"
	ScopeCapability Scope = "capability"
)

// Effect identifies how a finding changes generation.
type Effect string

const (
	EffectNone           Effect = ""
	EffectBlock          Effect = "block"
	EffectOmitOperation  Effect = "omit-operation"
	EffectOmitCapability Effect = "omit-capability"
)

// Valid reports whether scope is a declared contract value.
func (scope Scope) Valid() bool {
	switch scope {
	case ScopeNone, ScopeDocument, ScopeOperation, ScopeCapability:
		return true
	default:
		return false
	}
}

// Valid reports whether effect is a declared contract value.
func (effect Effect) Valid() bool {
	switch effect {
	case EffectNone, EffectBlock, EffectOmitOperation, EffectOmitCapability:
		return true
	default:
		return false
	}
}

// ValidPair reports whether scope/effect form one supported generation contract.
func ValidPair(scope Scope, effect Effect) bool {
	if !scope.Valid() || !effect.Valid() {
		return false
	}
	switch effect {
	case EffectNone:
		return scope == ScopeNone
	case EffectBlock:
		return scope != ScopeNone
	case EffectOmitOperation:
		return scope == ScopeOperation
	case EffectOmitCapability:
		return scope == ScopeCapability
	default:
		return false
	}
}
