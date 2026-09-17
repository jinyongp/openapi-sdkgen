package ir

type Document struct {
	Title              string
	ContractVersion    string
	OpenAPIVersion     string
	OpenAPIVersionLine string
	Servers            []Server
	Security           []SecurityRequirement
	SecuritySchemes    map[string]SecurityScheme
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
	Variables   []ServerVariable
	Pointer     string
	Raw         map[string]any
}

type ServerVariable struct {
	Name        string
	Default     string
	Enum        []string
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
	Parameters         []Parameter
	RequestBody        *RequestBody
	Responses          []Response
	Servers            []Server
	Security           []SecurityRequirement
	SecurityDeclared   bool
	PathItemRaw        map[string]any
	Raw                map[string]any
}

// Parameter is the normalized request parameter contract after path-level and
// operation-level inheritance, local reusable-object resolution, style/default
// application, and content-media resolution. Raw is retained for extensions
// and lossless metadata that are intentionally outside the common HTTP IR.
type Parameter struct {
	Name          string
	Description   string
	Location      string
	Style         string
	Explode       bool
	Required      bool
	Deprecated    bool
	AllowReserved bool
	ContentType   string
	Content       []MediaType
	Schema        any
	Raw           map[string]any
	Pointer       string
}

// MediaType is one normalized request/response media representation. Common
// schema fields are explicit while Raw retains version-specific media and
// Encoding Object details that lowerers may still need losslessly.
type MediaType struct {
	ContentType string
	Schema      any
	ItemSchema  any
	Raw         map[string]any
}

// RequestBody is the normalized outbound request-body contract for an
// operation after local reusable-object resolution.
type RequestBody struct {
	Description string
	Required    bool
	Content     []MediaType
	Raw         map[string]any
	Pointer     string
}

// Response is one normalized operation response after local reusable-object
// resolution. Content is normalized independently while Raw retains headers,
// links, extensions, and lossless metadata that are not yet separate IR nodes.
type Response struct {
	Status      string
	Description string
	Summary     string
	Content     []MediaType
	Raw         map[string]any
	SourceRaw   map[string]any
	Pointer     string
}

type SecurityRequirement struct {
	Schemes []SecurityRequirementScheme
	Raw     map[string]any
}

type SecurityRequirementScheme struct {
	Name   string
	Scopes []string
}

type SecurityScheme struct {
	Name              string
	Type              string
	Location          string
	ParameterName     string
	Scheme            string
	BearerFormat      string
	Flows             any
	OpenIDConnectURL  string
	OAuth2MetadataURL string
	Deprecated        bool
	Raw               map[string]any
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
