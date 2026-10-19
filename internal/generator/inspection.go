package generator

// OperationInspection reports actual prepared client surfaces. Target-specific
// JSON keys remain the responsibility of the inspection command.
type OperationInspection struct {
	Status           string  `json:"status"`
	ResourceCall     *string `json:"resourceCall"`
	Routes           bool    `json:"routes"`
	Operations       bool    `json:"operations"`
	ResourceOmission string  `json:"resourceOmission,omitempty"`
}

type PlanInspector interface {
	InspectPlan(Plan) (map[string]OperationInspection, error)
}

// InboundOperationInspection identifies one prepared host-bound receive contract.
// It contains immutable source identities, never IR nodes or runtime expressions.
type InboundOperationInspection struct {
	Kind              string `json:"kind"`
	Identity          string `json:"identity"`
	Name              string `json:"name"`
	Method            string `json:"method"`
	Expression        string `json:"expression,omitempty"`
	SourceRoute       string `json:"sourceRoute,omitempty"`
	SourceOperationID string `json:"sourceOperationId,omitempty"`
	ComponentName     string `json:"componentName,omitempty"`
}
