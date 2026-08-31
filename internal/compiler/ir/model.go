package ir

type Document struct {
	Title              string
	ContractVersion    string
	OpenAPIVersion     string
	OpenAPIVersionLine string
	Servers            []Server
	Operations         []Operation
	ComponentSchemas   map[string]map[string]any
	// Schemas is the target-neutral schema registry. Unlike ComponentSchemas it
	// retains boolean schemas and records the dialect/resource identity needed
	// for JSON Schema reference resolution.
	Schemas map[string]Schema
	Raw     map[string]any
	// Provenance contains explicit caller overrides. Production compilation
	// keeps this map empty and resolves source locations through ProvenanceIndex.
	Provenance map[string]Provenance
	// ProvenanceIndex resolves normalized pointers without materializing one map
	// entry per document node.
	ProvenanceIndex ProvenanceResolver
	// ErrorCategories is populated only by a target preparation plan after
	// validating recognized error-envelope schemas.
	ErrorCategories    map[string]string
	ParameterSortPlans map[string]SortParameterPlan
}

type SourceLocation struct {
	Source  string
	Pointer string
}

type Provenance struct {
	Primary SourceLocation
	Related []SourceLocation
}

type ProvenanceResolver interface {
	LookupProvenance(pointer string) (Provenance, bool)
}

func (document *Document) LookupProvenance(pointer string) (Provenance, bool) {
	if document == nil {
		return Provenance{}, false
	}
	if value, exists := document.Provenance[pointer]; exists {
		return value, true
	}
	if document.ProvenanceIndex == nil {
		return Provenance{}, false
	}
	return document.ProvenanceIndex.LookupProvenance(pointer)
}

// Schema is a normalized schema resource. Value remains lossless so target
// lowerers can preserve every version-specific JSON Schema keyword while the
// compiler owns resource identity and dialect selection in one place.
type Schema struct {
	Name        string
	Pointer     string
	ResourceURI string
	Dialect     string
	Value       any
}

type Server struct {
	URL         string
	Description string
}

type Operation struct {
	RouteKey           string
	Pointer            string
	OperationID        string
	Method             string
	Path               string
	Summary            string
	Description        string
	Tags               []string
	Visibility         string
	Envelope           string
	Pagination         string
	Extensions         OperationExtensions
	PaginationPlan     *PaginationPlan
	SortParameters     map[string]SortParameterPlan
	PathParameterOrder []string
	PathItemRaw        map[string]any
	Raw                map[string]any
}

// StringExtension preserves declaration presence independently from its value
// and JSON type. Targets validate the declaration before assigning behavior.
type StringExtension struct {
	Present bool
	Valid   bool
	Value   string
	Raw     any
	Pointer string
}

type OperationExtensions struct {
	Envelope   StringExtension
	Pagination ValueExtension
	Visibility StringExtension
}

// ValueExtension preserves declaration presence and the untyped decoded value
// until a target validates its complete shape.
type ValueExtension struct {
	Present bool
	Raw     any
	Pointer string
}

// SortParameterPlan is a validated projection from structured SDK input to
// exact OpenAPI enum wire values.
type SortParameterPlan struct {
	Values []SortValue
}

type SortValue struct {
	Wire      string
	Field     string
	Direction string
}

// PaginationPlan is the validated correlation between exact query parameters
// and decoded response-body JSON Pointers.
type PaginationPlan struct {
	Mode       string
	Request    PaginationRequestPlan
	Response   PaginationResponsePlan
	ItemSchema map[string]any
}

type PaginationRequestPlan struct {
	Cursor string
	Offset string
	Limit  string
}

type PaginationResponsePlan struct {
	Items      []string
	NextCursor []string
	Offset     []string
	Limit      []string
	Total      []string
}
