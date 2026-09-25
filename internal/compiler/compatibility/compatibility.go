// Package compatibility defines target-neutral OpenAPI compatibility decisions.
//
// Policy application is deliberately separate from OpenAPI parsing, reference
// transport, IR lowering, and target capability checks. ConsumerPolicy carries
// reviewed source semantics; NoopPolicy remains available for identity proofs
// and focused pipeline tests.
package compatibility

import (
	openapidoc "openapi-sdkgen/internal/compiler/openapi"
	"openapi-sdkgen/internal/openapiwalk"
)

// Conformance classifies whether source syntax is permitted by the declared
// OpenAPI line. Undefined and implementation-defined behavior belong to the
// separate NormativeDisposition axis.
type Conformance string

const (
	ConformanceConforming    Conformance = "conforming"
	ConformanceNonconforming Conformance = "nonconforming"
)

// NormativeDisposition records what the applicable OpenAPI line requires a
// consumer to do independently of the generator's compatibility action.
type NormativeDisposition string

const (
	DispositionDefined               NormativeDisposition = "defined"
	DispositionIgnored               NormativeDisposition = "ignored"
	DispositionUndefined             NormativeDisposition = "undefined"
	DispositionImplementationDefined NormativeDisposition = "implementation-defined"
	DispositionNotDefined            NormativeDisposition = "not-defined"
	DispositionInvalid               NormativeDisposition = "invalid"
)

// Action is the generator's selected compatibility treatment.
type Action string

const (
	ActionPreserve          Action = "preserve"
	ActionIgnore            Action = "ignore"
	ActionNormalize         Action = "normalize"
	ActionPreserveExtension Action = "preserve-extension"
	ActionReject            Action = "reject"
)

// SemanticImpact describes what can change when a compatibility decision is
// applied.
type SemanticImpact string

const (
	ImpactAnnotation   SemanticImpact = "annotation"
	ImpactValidation   SemanticImpact = "validation"
	ImpactWire         SemanticImpact = "wire"
	ImpactRouting      SemanticImpact = "routing"
	ImpactSecurity     SemanticImpact = "security"
	ImpactReference    SemanticImpact = "reference"
	ImpactDialect      SemanticImpact = "dialect"
	ImpactMetadataOnly SemanticImpact = "metadata-only"
)

// Context identifies one source occurrence without coupling compatibility
// policy to compiler transport or target state.
type Context struct {
	Version openapidoc.VersionLine
	Object  openapiwalk.ObjectContext
	Source  string
	Pointer string
}

// Rule is the stable machine-readable identity and classification for one
// compatibility behavior. Executable policies use these identities without
// coupling rule classification to compiler transport or target state.
type Rule struct {
	ID          string
	Versions    []openapidoc.VersionLine
	Object      openapiwalk.ObjectContext
	Conformance Conformance
	Disposition NormativeDisposition
	Action      Action
	Impact      SemanticImpact
}

// Finding is target-neutral evidence produced while applying a rule.
type Finding struct {
	RuleID      string
	Conformance Conformance
	Disposition NormativeDisposition
	Action      Action
	Impact      SemanticImpact
	Source      string
	Pointer     string
	Message     string
}

// LedgerEntry records an applied compatibility action independently of how a
// diagnostic is eventually rendered.
type LedgerEntry struct {
	RuleID  string
	Action  Action
	Impact  SemanticImpact
	Source  string
	Pointer string
	Version openapidoc.VersionLine
}

// Result is one compatibility pass result.
type Result struct {
	Value    any
	Findings []Finding
	Ledger   []LedgerEntry
	// Omit removes this occurrence from the effective semantic view. It does not
	// remove it from source metadata or provenance.
	Omit bool
	// Changed reports that Value differs semantically from the source occurrence.
	// It lets the compiler retain exact source bytes for no-op paths.
	Changed bool
}

// Policy transforms one decoded value into its effective semantic view.
type Policy interface {
	Apply(Context, any) Result
}

// NoopPolicy preserves the exact value and produces no findings or ledger
// entries. It is the required pipeline baseline before semantic rules land.
type NoopPolicy struct{}

func (NoopPolicy) Apply(_ Context, value any) Result {
	return Result{Value: value}
}
