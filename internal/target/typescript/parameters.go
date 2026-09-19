package typescript

import (
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/compiler/naming"
)

type operationParameter struct {
	Name                  string
	Property              string
	Binding               string
	Description           string
	Location              string
	Style                 string
	Explode               bool
	Required              bool
	Deprecated            bool
	AllowReserved         bool
	EnvironmentControlled bool
	ContentType           string
	Schema                any
	Raw                   map[string]any
	Pointer               string
	Sort                  *ir.SortParameterPlan
}

type preparedOperation struct {
	parameters                 []operationParameter
	clientParameters           []operationParameter
	parametersByLocation       map[string][]operationParameter
	clientParametersByLocation map[string][]operationParameter
	pathBindings               map[string]string
	requiredByLocation         map[string]bool
	bodyRequired               bool
	inputRequired              bool
	resourceInputRequired      bool
}

func prepareOperation(document *ir.Document, operation ir.Operation) (preparedOperation, error) {
	parameters, err := operationParameters(document, operation)
	if err != nil {
		return preparedOperation{}, err
	}
	prepared := preparedOperation{
		parameters:                 parameters,
		clientParameters:           make([]operationParameter, len(parameters)),
		parametersByLocation:       make(map[string][]operationParameter),
		clientParametersByLocation: make(map[string][]operationParameter),
		pathBindings:               make(map[string]string),
		requiredByLocation:         make(map[string]bool),
	}
	for index, parameter := range parameters {
		prepared.parametersByLocation[parameter.Location] = append(prepared.parametersByLocation[parameter.Location], parameter)
		if parameter.Location == "path" {
			prepared.pathBindings[parameter.Name] = parameter.Binding
		}
		clientParameter := projectClientParameter(parameter)
		prepared.clientParameters[index] = clientParameter
		prepared.clientParametersByLocation[clientParameter.Location] = append(prepared.clientParametersByLocation[clientParameter.Location], clientParameter)
		prepared.requiredByLocation[clientParameter.Location] = prepared.requiredByLocation[clientParameter.Location] || clientParameter.Required
	}
	body, err := operationRequestBody(document, operation)
	if err != nil {
		return preparedOperation{}, err
	}
	if body != nil {
		prepared.bodyRequired = body.Required
	}
	return prepared, nil
}

func (prepared preparedOperation) clientInputRequired(document *ir.Document, operation ir.Operation, inputTypes []string, omitPath bool) (bool, error) {
	operationName := operationTypeName(operationRouteKey(operation))
	for _, inputType := range inputTypes {
		field := strings.TrimSuffix(strings.TrimPrefix(inputType, operationName), "Input")
		if omitPath && field == "Path" {
			continue
		}
		if field == "Body" {
			if prepared.bodyRequired {
				return true, nil
			}
			continue
		}
		if prepared.requiredByLocation[strings.ToLower(field)] {
			return true, nil
		}
	}
	return false, nil
}

func (prepared preparedOperation) inputFieldRequired(field string) bool {
	if field == "Body" {
		return prepared.bodyRequired
	}
	return prepared.requiredByLocation[strings.ToLower(field)]
}

type requestHeaderPolicy uint8

const (
	requestHeaderCallerManaged requestHeaderPolicy = iota
	requestHeaderEnvironmentControlled
)

var fetchEnvironmentControlledRequestHeaders = map[string]struct{}{
	"accept-charset":                 {},
	"accept-encoding":                {},
	"access-control-request-headers": {},
	"access-control-request-method":  {},
	"connection":                     {},
	"content-length":                 {},
	"cookie":                         {},
	"cookie2":                        {},
	"date":                           {},
	"dnt":                            {},
	"expect":                         {},
	"host":                           {},
	"keep-alive":                     {},
	"origin":                         {},
	"referer":                        {},
	"set-cookie":                     {},
	"te":                             {},
	"trailer":                        {},
	"transfer-encoding":              {},
	"upgrade":                        {},
	"via":                            {},
}

func classifyFetchRequestHeader(name string) requestHeaderPolicy {
	name = strings.ToLower(name)
	if _, exists := fetchEnvironmentControlledRequestHeaders[name]; exists ||
		strings.HasPrefix(name, "proxy-") ||
		strings.HasPrefix(name, "sec-") {
		return requestHeaderEnvironmentControlled
	}
	return requestHeaderCallerManaged
}

func operationParameters(document *ir.Document, operation ir.Operation) ([]operationParameter, error) {
	if operation.Parameters == nil {
		return syntheticOperationParameters(document, operation)
	}
	result := make([]operationParameter, 0, len(operation.Parameters))
	for _, parameter := range operation.Parameters {
		key := parameter.Location + "\x00" + parameter.Name
		var sortPlan *ir.SortParameterPlan
		if value, exists := operation.SortParameters[key]; exists {
			copied := value
			sortPlan = &copied
		} else if value, exists := document.ParameterSortPlans[parameter.Pointer]; exists {
			copied := value
			sortPlan = &copied
		}
		headerPolicy := requestHeaderCallerManaged
		if parameter.Location == "header" {
			headerPolicy = classifyFetchRequestHeader(parameter.Name)
		}
		result = append(result, operationParameter{
			Name:                  parameter.Name,
			Property:              parameter.Name,
			Binding:               stablePrivateIdentifier("operation-parameter", operationRouteKey(operation)+"\x00"+parameter.Location+"\x00"+parameter.Name),
			Description:           parameter.Description,
			Location:              parameter.Location,
			Style:                 parameter.Style,
			Explode:               parameter.Explode,
			Required:              parameter.Required,
			Deprecated:            parameter.Deprecated,
			AllowReserved:         parameter.AllowReserved,
			EnvironmentControlled: headerPolicy == requestHeaderEnvironmentControlled,
			ContentType:           parameter.ContentType,
			Schema:                parameter.Schema,
			Raw:                   parameter.Raw,
			Pointer:               parameter.Pointer,
			Sort:                  sortPlan,
		})
	}
	usedPathBindings := make(map[string]bool, len(operation.PathParameterOrder))
	for index := range result {
		if result[index].Location == "path" {
			result[index].Binding = readablePathParameterBinding(result[index].Name, usedPathBindings)
		}
	}
	return result, nil
}

func operationRequestBody(document *ir.Document, operation ir.Operation) (*ir.RequestBody, error) {
	if operation.RequestBody != nil || operation.Parameters != nil {
		return operation.RequestBody, nil
	}
	return syntheticOperationRequestBody(document, operation)
}

// syntheticOperationRequestBody preserves the raw-map path used by focused
// target tests that construct ir.Operation directly instead of compiling an
// OpenAPI document. Compiler-built path operations always carry a non-nil
// Parameters slice and use the normalized RequestBody above.
func syntheticOperationRequestBody(document *ir.Document, operation ir.Operation) (*ir.RequestBody, error) {
	raw, ok := operation.Raw["requestBody"].(map[string]any)
	if !ok {
		return nil, nil
	}
	pointer := operation.Pointer + "/requestBody"
	if reference, _ := raw["$ref"].(string); reference != "" {
		if resolvedPointer := localReferencePointer(reference); resolvedPointer != "" {
			pointer = resolvedPointer
		}
	}
	resolved, err := resolveComponentObject(document, raw, "requestBodies")
	if err != nil {
		return nil, err
	}
	content, _ := resolved["content"].(map[string]any)
	mediaTypes := make([]string, 0, len(content))
	for mediaType := range content {
		mediaTypes = append(mediaTypes, mediaType)
	}
	sort.Strings(mediaTypes)
	media := make([]ir.MediaType, 0, len(mediaTypes))
	for _, contentType := range mediaTypes {
		value, _ := content[contentType].(map[string]any)
		value, err = resolveMediaTypeObject(document, value)
		if err != nil {
			return nil, err
		}
		media = append(media, ir.MediaType{
			ContentType: contentType,
			Schema:      value["schema"],
			ItemSchema:  value["itemSchema"],
			Stream:      ir.StreamPlanForMediaType(contentType, mediaTypeHasSequentialShape(value)),
			Raw:         value,
		})
	}
	description, _ := resolved["description"].(string)
	return &ir.RequestBody{
		Description: description,
		Required:    boolValue(resolved, "required"),
		Content:     media,
		Raw:         resolved,
		Pointer:     pointer,
	}, nil
}

// syntheticOperationParameters preserves the raw-map path used by inbound
// webhook/callback operations and focused target tests that do not pass through
// compiler IR construction. Compiled path operations always provide the typed
// Parameters slice above.
func syntheticOperationParameters(document *ir.Document, operation ir.Operation) ([]operationParameter, error) {
	merged := make(map[string]operationParameter)
	order := make([]string, 0)
	for _, source := range []struct {
		value   any
		pointer string
	}{
		{value: operation.PathItemRaw["parameters"], pointer: operationPathItemPointer(operation) + "/parameters"},
		{value: operation.Raw["parameters"], pointer: operation.Pointer + "/parameters"},
	} {
		values, _ := source.value.([]any)
		for index, value := range values {
			raw, _ := value.(map[string]any)
			pointer := source.pointer + "/" + strconv.Itoa(index)
			if reference, _ := raw["$ref"].(string); reference != "" {
				if resolvedPointer := localReferencePointer(reference); resolvedPointer != "" {
					pointer = resolvedPointer
				}
			}
			var err error
			raw, err = resolveComponentObject(document, raw, "parameters")
			if err != nil {
				return nil, err
			}
			name, _ := raw["name"].(string)
			location, _ := raw["in"].(string)
			if name == "" || location == "" {
				continue
			}
			style, _ := raw["style"].(string)
			if style == "" {
				style = defaultParameterStyle(location)
			}
			explode, hasExplode := raw["explode"].(bool)
			if !hasExplode {
				explode = style == "form"
			}
			schema := raw["schema"]
			contentType := ""
			if content, ok := raw["content"].(map[string]any); ok {
				mediaTypes := make([]string, 0, len(content))
				for mediaType := range content {
					mediaTypes = append(mediaTypes, mediaType)
				}
				sort.Strings(mediaTypes)
				if len(mediaTypes) > 0 {
					contentType = mediaTypes[0]
					media, _ := content[contentType].(map[string]any)
					media, err = resolveMediaTypeObject(document, media)
					if err != nil {
						return nil, err
					}
					schema = media["schema"]
				}
			}
			key := location + "\x00" + name
			if _, exists := merged[key]; !exists {
				order = append(order, key)
			}
			var sortPlan *ir.SortParameterPlan
			if value, exists := operation.SortParameters[key]; exists {
				copied := value
				sortPlan = &copied
			} else if value, exists := document.ParameterSortPlans[pointer]; exists {
				copied := value
				sortPlan = &copied
			}
			description, _ := raw["description"].(string)
			headerPolicy := requestHeaderCallerManaged
			if location == "header" {
				headerPolicy = classifyFetchRequestHeader(name)
			}
			merged[key] = operationParameter{
				Name:                  name,
				Property:              name,
				Binding:               stablePrivateIdentifier("operation-parameter", operationRouteKey(operation)+"\x00"+location+"\x00"+name),
				Description:           description,
				Location:              location,
				Style:                 style,
				Explode:               explode,
				Required:              boolValue(raw, "required"),
				Deprecated:            boolValue(raw, "deprecated"),
				AllowReserved:         boolValue(raw, "allowReserved"),
				EnvironmentControlled: headerPolicy == requestHeaderEnvironmentControlled,
				ContentType:           contentType,
				Schema:                schema,
				Raw:                   raw,
				Pointer:               pointer,
				Sort:                  sortPlan,
			}
		}
	}
	result := make([]operationParameter, 0, len(merged))
	for _, key := range order {
		result = append(result, merged[key])
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Location != "path" || result[j].Location != "path" {
			return false
		}
		return pathParameterIndex(operation.PathParameterOrder, result[i].Name) < pathParameterIndex(operation.PathParameterOrder, result[j].Name)
	})
	usedPathBindings := make(map[string]bool, len(operation.PathParameterOrder))
	for index := range result {
		if result[index].Location == "path" {
			result[index].Binding = readablePathParameterBinding(result[index].Name, usedPathBindings)
		}
	}
	return result, nil
}

func readablePathParameterBinding(name string, used map[string]bool) string {
	base, err := naming.Property(name)
	if err != nil || !isTypeScriptBindingIdentifier(base) {
		public, publicErr := naming.Public(name)
		if publicErr == nil && public != "" {
			runes := []rune(public)
			runes[0] = unicode.ToLower(runes[0])
			base = string(runes)
		}
	}
	if !isTypeScriptBindingIdentifier(base) {
		base = "pathParameter"
	}
	binding := base
	for suffix := 2; used[binding]; suffix++ {
		binding = base + strconv.Itoa(suffix)
	}
	used[binding] = true
	return binding
}

func isTypeScriptBindingIdentifier(value string) bool {
	runes := []rune(value)
	if len(runes) == 0 || !(runes[0] == '_' || runes[0] == '$' || unicode.IsLetter(runes[0])) {
		return false
	}
	for _, value := range runes[1:] {
		if value != '_' && value != '$' && !unicode.IsLetter(value) && !unicode.IsDigit(value) {
			return false
		}
	}
	return true
}

func operationPathItemPointer(operation ir.Operation) string {
	if before, _, found := strings.Cut(operation.Pointer, "/additionalOperations/"); found {
		return before
	}
	if index := strings.LastIndex(operation.Pointer, "/"); index >= 0 {
		return operation.Pointer[:index]
	}
	return operation.Pointer
}

func localReferencePointer(reference string) string {
	if !strings.HasPrefix(reference, "#") {
		return ""
	}
	decoded, err := url.PathUnescape(strings.TrimPrefix(reference, "#"))
	if err != nil || !strings.HasPrefix(decoded, "/") {
		return ""
	}
	return "#" + decoded
}

func defaultParameterStyle(location string) string {
	if location == "query" || location == "cookie" {
		return "form"
	}
	return "simple"
}

func pathParameterIndex(order []string, name string) int {
	for index, value := range order {
		if value == name {
			return index
		}
	}
	return len(order)
}

func parametersIn(document *ir.Document, operation ir.Operation, location string) ([]operationParameter, error) {
	parameters, err := operationParameters(document, operation)
	if err != nil {
		return nil, err
	}
	result := make([]operationParameter, 0, len(parameters))
	for _, parameter := range parameters {
		if parameter.Location == location {
			result = append(result, parameter)
		}
	}
	return result, nil
}

func clientParametersIn(document *ir.Document, operation ir.Operation, location string) ([]operationParameter, error) {
	parameters, err := clientOperationParameters(document, operation)
	if err != nil {
		return nil, err
	}
	result := make([]operationParameter, 0, len(parameters))
	for _, parameter := range parameters {
		if parameter.Location == location {
			result = append(result, parameter)
		}
	}
	return result, nil
}

func clientOperationParameters(document *ir.Document, operation ir.Operation) ([]operationParameter, error) {
	parameters, err := operationParameters(document, operation)
	if err != nil {
		return nil, err
	}
	for index := range parameters {
		parameters[index] = projectClientParameter(parameters[index])
	}
	return parameters, nil
}

func projectClientParameter(parameter operationParameter) operationParameter {
	if parameter.EnvironmentControlled {
		parameter.Required = false
	}
	return parameter
}
