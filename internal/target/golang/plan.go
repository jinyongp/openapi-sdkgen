package golang

import (
	"fmt"
	"go/token"
	"sort"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
	"openapi-sdkgen/internal/http/extension"
	schemaplan "openapi-sdkgen/internal/schema/plan"
)

type sourcePlan struct {
	ImportPath string
	Files      []filePlan
	Inspection map[string]generator.OperationInspection
	Inbound    []generator.InboundOperationInspection
}

type filePlan struct {
	OpenAPIVersion, OpenAPIVersionLine string
	Path                               string
	Template                           string
	Source                             string
	Package                            string
	Imports                            []importPlan
	Operations                         []operationPlan
	ClientOperations                   []operationPlan
	ExportsOnly                        bool
	Programs                           []schemaProgram
	Models                             []modelPlan
	XMLPrograms                        []xmlProgram
	XMLEncodePrograms                  []xmlEncodeProgram
	Prepared                           string
	Resources                          []resourceNodePlan
	LinkSources                        []linkSourcePlan
}

type importPlan struct{ Alias, Path string }
type operationPlan struct {
	Documentation                                          string
	DependencyOnly                                         bool
	LinkSource                                             bool
	ResponseValidation                                     bool
	LinkTarget                                             *linkTargetPlan
	Pagination                                             *paginationPlan
	ResourceBound, ResourceHasInput, ResourceRequiredInput bool
	ResourceInputName                                      string
	ResourceInputFields                                    []inputFieldPlan
	ResourcePathField                                      *inputFieldPlan
	ResourceBindings                                       []resourceBindingPlan
	HasMultipart                                           bool
	MultipartHeaders                                       []headerFieldPlan
	MultipartResponses                                     []string
	MultipartParts                                         []multipartPartPlan
	BodyEncodings                                          string
	Servers                                                string
	Security                                               []securityRequirementPlan
	SecurityMetadata                                       bool
	Name, Method, Path                                     string
	RouteName                                              string
	OperationID, Route                                     string
	Responses                                              []responsePlan
	InputFields                                            []inputFieldPlan
	Parameters                                             []parameterPlan
	HasInput, RequiredInput                                bool
	OutputType                                             string
	OutputUnion                                            bool
	Headers                                                []responseHeaderPlan
	HasHeaders                                             bool
	NeedsCookieHeaders                                     bool
	RequestBodies                                          []bodyVariantPlan
	RequestStreams                                         []*requestStreamPlan
	BodyUnion                                              bool
	HasSequential, CanDo                                   bool
	HasOpenMedia                                           bool
	HasTypedStream, StreamUnion                            bool
	HasMultipartStream                                     bool
	StreamType, StreamAccept                               string
	Envelope, Visibility                                   string
}
type responseHeaderPlan struct {
	Status, Name, Model, Validation string
	Index                           int
	Fields                          []headerFieldPlan
}
type multipartPartPlan struct {
	Validation string
}
type headerFieldPlan struct {
	XMLDecoder                             string
	Name, Validation, ContentType, Decoder string
	Explode, Required, Text                bool
}
type responsePlan struct {
	DataModel, DataValidation, DataType                    string
	EnvelopeFallback                                       bool
	Framing, ItemModel, ItemValidation, ItemXMLDecoder     string
	ItemParts, CompleteParts                               string
	Buffered, TypedStream, StreamSuccess                   bool
	Decoder                                                string
	Status, Validation, MediaType, Kind, Name, Model, Type string
	Success                                                bool
	Sequential                                             bool
	Open                                                   bool
	BinaryFallback                                         bool
	Index                                                  int
}
type inputFieldPlan struct {
	Stream                            *requestStreamPlan
	Incremental                       bool
	Multipart                         bool
	Encoder                           string
	Name, Type, Validation, MediaType string
	Required, Binary                  bool
	Open, BinaryFallback              bool
	Union                             bool
}
type bodyVariantPlan struct {
	Stream                             *requestStreamPlan
	Incremental                        bool
	Multipart                          bool
	Encoder                            string
	Name, Model, MediaType, Validation string
	Binary, Wildcard, Open             bool
	BinaryFallback                     bool
	Index                              int
}
type requestStreamPlan struct {
	Name, MediaType, Framing, ItemModel, ItemValidation string
	Parts                                               string
	Multipart                                           bool
	Complete                                            bool
}
type parameterPlan struct {
	Encoder                                     string
	Name, Location, Style, ContentType, Section string
	Explode, AllowReserved, AllowEmptyValue     bool
}

func prepare(document *ir.Document, options generator.Options) (generator.Plan, []diagnostic.Diagnostic, error) {
	if document == nil {
		return generator.Plan{}, nil, fmt.Errorf("cannot prepare a nil Go document")
	}
	if err := generator.ValidateTargetOptions(Generator{}, options); err != nil {
		return generator.Plan{}, nil, err
	}
	config, err := generator.CanonicalGoOptions(options.Golang)
	if err != nil {
		return generator.Plan{}, nil, err
	}
	sourceDocument := document
	preparedDocument, extensionDiagnostics, err := (&extension.Planner{Target: "golang", ItemTypeSignature: func(document *ir.Document, schema map[string]any) string {
		return goTypeSignature(document, schema, false)
	}}).Prepare(document)
	if err != nil {
		return generator.Plan{}, nil, err
	}
	selections, selectionDiagnostics, err := prepareClientSelections(preparedDocument, options)
	if err != nil {
		return generator.Plan{}, nil, err
	}
	selection := selections.Union
	if diagnostic.HasErrors(selectionDiagnostics) {
		return generator.Plan{}, selectionDiagnostics, nil
	}
	extensionDiagnostics = selection.diagnostics(preparedDocument, extensionDiagnostics)
	for _, finding := range extensionDiagnostics {
		if finding.Severity == diagnostic.SeverityError {
			return generator.Plan{}, extensionDiagnostics, nil
		}
	}
	document = preparedDocument
	names, err := planNames(document)
	if err != nil {
		return generator.Plan{}, nil, err
	}
	omitted, restrictionDiagnostics := operationOmissions(document)
	if len(restrictionDiagnostics) != 0 {
		return generator.Plan{}, restrictionDiagnostics, nil
	}
	serverScopes := inboundScopes(document, selection, omitted)
	if findings := inboundAddonDiagnostics(document, serverScopes, options); len(findings) != 0 {
		return generator.Plan{}, append(extensionDiagnostics, findings...), nil
	}
	document = selection.view(document)
	operations := make([]operationPlan, 0, len(document.Operations))
	diagnostics := append([]diagnostic.Diagnostic(nil), extensionDiagnostics...)
	components := make(map[string]any, len(document.Schemas))
	for name, value := range document.ComponentSchemas {
		components[name] = value
	}
	for name, value := range document.Schemas {
		components[name] = value.Value
	}
	bindings := make(map[string]string, len(components))
	inputBindings := make(map[string]string, len(components))
	for index, name := range sortedKeys(components) {
		bindings[name] = fmt.Sprintf("ValidateComponent__%dOutput__", index)
		inputBindings[name] = fmt.Sprintf("ValidateComponent__%dInput__", index)
	}
	needed := make(map[string]bool)
	inputNeeded := make(map[string]bool)
	if len(document.Operations) == 0 {
		for name := range components {
			needed[name] = true
		}
	}
	schemas := schemaPlanner{reference: func(reference string, projection schemaplan.Projection) (string, error) {
		name, err := schemaplan.ComponentReferenceName(reference)
		if err != nil {
			return "", err
		}
		selected, roots := bindings, needed
		if projection == schemaplan.Input {
			selected, roots = inputBindings, inputNeeded
		}
		binding, ok := selected[name]
		if !ok {
			return "", fmt.Errorf("missing Go schema reference %q", reference)
		}
		roots[name] = true
		return "check" + binding, nil
	}}
	sourceOperations := append([]ir.Operation(nil), document.Operations...)
	sort.Slice(sourceOperations, func(i, j int) bool { return sourceOperations[i].RouteKey < sourceOperations[j].RouteKey })
	for _, operation := range sourceOperations {
		if omitted[operation.RouteKey] || operation.Visibility == "hidden" {
			continue
		}
		planned := operationPlan{Name: names.operations[operation.RouteKey], RouteName: names.routes[operation.RouteKey], Method: operation.Method, Path: operation.Path, OperationID: operation.OperationID, Route: operation.RouteKey, Envelope: operation.Envelope, Visibility: operation.Visibility, Documentation: operationDocumentation(operation)}
		planned.DependencyOnly = selection != nil && !selection.Direct[operation.RouteKey]

		planned.Servers = operationServers(operation.Servers)
		bodyEncodings, err := prepareBodyEncodings(&planned, operation, document, &schemas)
		if err != nil {
			return generator.Plan{}, nil, err
		}
		planned.BodyEncodings = bodyEncodings
		security, err := prepareOperationSecurity(document, operation)
		if err != nil {
			return generator.Plan{}, nil, err
		}
		planned.Security = security
		for index, requirement := range security {
			planned.SecurityMetadata = planned.SecurityMetadata || strings.Contains(requirement.Definition, "securityMetadata(")
			planned.Security[index].Definition = strings.ReplaceAll(requirement.Definition, "securityMetadata(", "securityMetadataFor"+planned.Name+"(")
		}
		if err := prepareOperationInput(&planned, operation, &schemas, document.OpenAPIVersionLine == "3.0", document); err != nil {
			return generator.Plan{}, nil, err
		}
		if err := prepareOperationResponses(&planned, operation, &schemas, document.OpenAPIVersionLine == "3.0", document); err != nil {
			return generator.Plan{}, nil, err
		}
		if err := prepareOperationStreams(&planned, operation, document, &schemas, document.OpenAPIVersionLine == "3.0"); err != nil {
			return generator.Plan{}, nil, err
		}
		if err := prepareResponseHeaders(&planned, operation, document, &schemas); err != nil {
			return generator.Plan{}, nil, err
		}
		if err := preparePagination(&planned, operation, document, &schemas); err != nil {
			return generator.Plan{}, nil, err
		}
		operations = append(operations, planned)
	}
	if selections.Root != nil && len(clientOperations(operations, selections.Root)) == 0 {
		diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.SeverityError, Phase: diagnostic.PhaseTarget, Target: "golang", Code: "SDKGEN-E512", Message: "The Go target has no meaningful requested entry surface after scoped omissions."})
	}
	for _, name := range sortedKeys(selections.Named) {
		if len(clientOperations(operations, selections.Named[name])) == 0 {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.SeverityError, Phase: diagnostic.PhaseTarget, Target: "golang", Code: "SDKGEN-E512", Message: fmt.Sprintf("Client %q has no meaningful requested entry surface after scoped omissions.", name)})
		}
	}
	if diagnostic.HasErrors(diagnostics) {
		return generator.Plan{}, diagnostics, nil
	}
	resourcePlans, resourceFindings, err := prepareClientResources(document, operations, names, &schemas, selections, options.FailOnResourceOmission)
	if err != nil {
		return generator.Plan{}, nil, err
	}
	resources := resourcePlans.Root
	diagnostics = append(diagnostics, resourceFindings...)
	if diagnostic.HasErrors(diagnostics) {
		return generator.Plan{}, diagnostics, nil
	}
	links, linkDiagnostics, err := prepareLinks(document, operations, names, &schemas)
	if err != nil {
		return generator.Plan{}, nil, err
	}
	diagnostics = append(diagnostics, linkDiagnostics...)
	errorContracts, err := prepareErrorContracts(document, names, &schemas)
	if err != nil {
		return generator.Plan{}, nil, err
	}
	var inbound []inboundOperationPlan
	if options.HasAddon(generator.AddonServer) {
		var findings []diagnostic.Diagnostic
		inbound, findings = prepareInboundOperations(preparedDocument, serverScopes, names, &schemas)
		diagnostics = append(diagnostics, findings...)
		if diagnostic.HasErrors(diagnostics) {
			return generator.Plan{}, diagnostics, nil
		}
	}
	modelOperations := append([]operationPlan(nil), operations...)
	for _, entry := range inbound {
		modelOperations = append(modelOperations, entry.Operation)
	}
	compileComponents := func(roots map[string]bool, bindings map[string]string, projection schemaplan.Projection) map[string]bool {
		compiled := make(map[string]bool)
		for {
			progress := false
			for _, name := range sortedKeys(roots) {
				if compiled[name] {
					continue
				}
				compiled[name] = true
				progress = true
				origin := document.Schemas[name].Pointer
				if origin == "" {
					origin = "#/components/schemas/" + pointerToken(name)
				}
				if _, err := schemas.prepare(bindings[name], components[name], projection, document.OpenAPIVersionLine == "3.0", schemaOccurrence{Source: origin}); err != nil {
					diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.SeverityError, Phase: diagnostic.PhaseTarget, Target: "golang", Code: "GO_SCHEMA_PENDING", Message: err.Error()})
				}
			}
			if !progress {
				break
			}
		}
		return compiled
	}
	compiled := compileComponents(needed, bindings, schemaplan.Output)
	for name := range compiled {
		inputNeeded[name] = true
	}
	inputCompiled := compileComponents(inputNeeded, inputBindings, schemaplan.Input)
	for _, finding := range diagnostics {
		if finding.Severity == diagnostic.SeverityError {
			return generator.Plan{}, diagnostics, nil
		}
	}
	models := modelPlanner{nativeMedia: schemas.nativeMedia, version: document.OpenAPIVersionLine, annotations: schemas.annotations, componentSources: components, references: make(map[string]modelType), validators: schemas.nodeNames, sources: schemas.sources, used: make(map[string]bool), referenceGraph: make(map[string][]string), referenceNodes: make(map[string]*schemaplan.Node), validationView: schemas.view, nonNullValidation: schemas.nonNullView, programMedia: schemas.programMedia, programProjection: schemas.programProjection}
	prepareModelSorts(&models, operations, sourceOperations, &schemas)
	for binding, node := range schemas.nodes {
		models.referenceNodes["check"+binding] = node
		schemaplan.Walk(node, func(child *schemaplan.Node) {
			if reference, ok := child.Get("reference").(schemaplan.Literal); ok {
				models.referenceGraph["check"+binding] = append(models.referenceGraph["check"+binding], reference.Data.(string))
			}
		})
	}
	for name := range names.reserved {
		models.used[name] = true
	}
	for name, binding := range bindings {
		models.references["check"+binding] = modelType{Type: names.schemas[name] + "Output", Codec: names.schemas[name] + "Output"}
	}
	for name, binding := range inputBindings {
		models.references["check"+binding] = modelType{Type: names.schemas[name] + "Input", Codec: names.schemas[name] + "Input"}
	}
	for _, resource := range resourcePlans.All {
		if resource.Selector != nil && resource.Selector.Model != "" {
			selector := resource.Selector
			if err := models.root(selector.Model, schemas.nodes[selector.Validation], selector.Validation); err != nil {
				return generator.Plan{}, nil, err
			}
			models.plans[len(models.plans)-1].Binding = true
		}
	}
	for _, contract := range errorContracts {
		if err := models.root(contract.Name+"Details", schemas.nodes[contract.Validation], contract.Validation); err != nil {
			return generator.Plan{}, nil, err
		}
	}
	for _, operation := range modelOperations {
		if operation.LinkTarget != nil {
			for _, field := range operation.LinkTarget.Fields {
				if field.Native {
					continue
				}
				if err := models.root(field.Type, schemas.nodes[field.Validation], field.Validation); err != nil {
					return generator.Plan{}, nil, err
				}
				models.plans[len(models.plans)-1].Binding = true
			}
		}
		if operation.Pagination != nil {
			page := operation.Pagination
			if err := models.root(page.ItemModel, schemas.nodes[page.Validation], page.Validation); err != nil {
				return generator.Plan{}, nil, err
			}
			models.plans[len(models.plans)-1].ValidatedView = true
		}
		for _, request := range operation.RequestStreams {
			if request.ItemValidation != "" {
				if err := models.root(request.ItemModel, schemas.nodes[request.ItemValidation], request.ItemValidation); err != nil {
					return generator.Plan{}, nil, err
				}
				if operation.LinkSource {
					models.plans[len(models.plans)-1].Binding = true
				}
			}
		}
		for _, field := range operation.InputFields {
			if field.Binary || field.Union || field.Incremental {
				continue
			}
			if err := models.root(field.Type, schemas.nodes[field.Validation], field.Validation); err != nil {
				return generator.Plan{}, nil, err
			}
			if operation.LinkSource {
				models.plans[len(models.plans)-1].Binding = true
			}
		}
		for _, body := range operation.RequestBodies {
			if !body.Binary && !body.Incremental {
				if err := models.root(body.Model, schemas.nodes[body.Validation], body.Validation); err != nil {
					return generator.Plan{}, nil, err
				}
				if operation.LinkSource {
					models.plans[len(models.plans)-1].Binding = true
				}
			}
		}
		for _, response := range operation.Responses {
			if response.DataValidation != "" {
				if err := models.root(response.DataModel, schemas.nodes[response.DataValidation], response.DataValidation); err != nil {
					return generator.Plan{}, nil, err
				}
			}
			if response.ItemValidation != "" {
				if err := models.root(response.ItemModel, schemas.nodes[response.ItemValidation], response.ItemValidation); err != nil {
					return generator.Plan{}, nil, err
				}
			}
			if response.Validation != "" {
				if err := models.root(response.Model, schemas.nodes[response.Validation], response.Validation); err != nil {
					return generator.Plan{}, nil, err
				}
			}
		}
		for _, header := range operation.Headers {
			if header.Validation != "" {
				if err := models.root(header.Model, schemas.nodes[header.Validation], header.Validation); err != nil {
					return generator.Plan{}, nil, err
				}
			}
		}
	}
	for _, name := range sortedKeys(compiled) {
		if err := models.root(names.schemas[name]+"Output", schemas.nodes[bindings[name]], bindings[name]); err != nil {
			return generator.Plan{}, nil, err
		}
	}
	for _, name := range sortedKeys(inputCompiled) {
		if err := models.root(names.schemas[name]+"Input", schemas.nodes[inputBindings[name]], inputBindings[name]); err != nil {
			return generator.Plan{}, nil, err
		}
	}
	var genericFormatFile filePlan
	if options.HasAddon(generator.AddonServer) {
		genericFormatFile, err = prepareInboundGenericFormats(config, &schemas)
		if err != nil {
			return generator.Plan{}, nil, err
		}
	}
	xmlOperations := append(append([]operationPlan(nil), modelOperations...), prepareInboundXMLViews(inbound, &schemas)...)
	xmlEncodePrograms, xmlEncodePrepared, err := prepareXMLBodyEncoders(xmlOperations, &schemas)
	if err != nil {
		return generator.Plan{}, nil, err
	}
	headerPrepared, err := prepareHeaderDecoders(modelOperations, &schemas)
	if err != nil {
		return generator.Plan{}, nil, err
	}
	for index := range schemas.programs {
		schemas.programs[index].Public = true
	}
	xmlPrepared := prepareXMLResponses(xmlOperations, &schemas)
	xmlPrograms, xmlDiagnostic := schemas.prepareXML()
	if xmlDiagnostic != nil {
		return generator.Plan{}, []diagnostic.Diagnostic{*xmlDiagnostic}, nil
	}
	sort.Slice(operations, func(i, j int) bool { return operations[i].Name < operations[j].Name })
	runtimeRoots := append(sortedKeys(schemas.formatRoots), sortedKeys(schemas.helperRoots)...)
	if options.HasAddon(generator.AddonServer) {
		runtimeRoots = append(runtimeRoots, "server-handler", "server-parameters", "server-media", "server-stream", "server-headers", "server-schema", "server-multipart", "server-multipart-output", "server-protocols")
		for _, entry := range inbound {
			if entry.Operation.HasOpenMedia {
				runtimeRoots = append(runtimeRoots, "types-media")
			}
			for _, body := range entry.Operation.RequestBodies {
				if body.Open {
					runtimeRoots = append(runtimeRoots, "types-media")
				}
			}
			for _, request := range entry.Operation.RequestStreams {
				switch request.Framing {
				case "line-delimited-json", "json-sequence":
					runtimeRoots = append(runtimeRoots, "stream-json")
				case "sse":
					runtimeRoots = append(runtimeRoots, "stream-sse")
				}
			}
			for _, response := range entry.Operation.Responses {
				switch response.Framing {
				case "line-delimited-json", "json-sequence":
					runtimeRoots = append(runtimeRoots, "stream-json")
				case "sse":
					runtimeRoots = append(runtimeRoots, "stream-sse")
				}
			}
		}
	}
	if len(operations) != 0 {
		runtimeRoots = append(runtimeRoots, "types-stream", "types-multipart", "core-links")
		for _, operation := range operations {
			if len(operation.RequestStreams) != 0 {
				runtimeRoots = append(runtimeRoots, "core-link-sources")
				break
			}
		}
	}
	if len(links) != 0 {
		runtimeRoots = append(runtimeRoots, "core-links")
		for _, operation := range operations {
			if len(operation.RequestStreams) != 0 {
				runtimeRoots = append(runtimeRoots, "core-link-sources")
				break
			}
		}
	}
	if headerPrepared != "" {
		runtimeRoots = append(runtimeRoots, "schema-header")
	}
	if models.needsLiteral {
		runtimeRoots = append(runtimeRoots, "wire-literal")
	}
	files, err := planFiles(config, operations, schemas.needsRegex, len(models.plans) != 0, runtimeRoots...)
	if err != nil {
		return generator.Plan{}, nil, err
	}
	files = appendResourceFiles(config, files, resources)
	files = appendLinkFiles(config, files, links, operations)
	files = appendClientErrorFiles(config, files, preparedDocument, selections, errorContracts, names)
	files, err = appendClientEntries(config, files, operations, resourcePlans, links, selections)
	if err != nil {
		return generator.Plan{}, nil, err
	}
	files = appendStaticSelectionFiles(config, files, operations)
	files = appendInboundFiles(config, files, inbound, &schemas)
	if options.HasAddon(generator.AddonServer) {
		files = append(files, genericFormatFile)
	}
	files = appendDynamicFiles(config, files, operations, links)
	if len(operations) != 0 {
		for _, file := range appendErrorFiles(config, nil, errorContracts, names) {
			file.Path = "operations/errors.go"
			file.Package = "operations"
			files = append(files, file)
		}
	}
	files, err = completeFiles(config, files, schemas.programs, models.plans)
	if err != nil {
		return generator.Plan{}, nil, err
	}
	if headerPrepared != "" {
		imports := []importPlan{{Path: "context"}, {Path: config.ImportPath + "/internal/wire"}}
		if strings.Contains(headerPrepared, "strings.") {
			imports = append(imports, importPlan{Path: "strings"})
		}
		files = append(files, filePlan{Path: "internal/schema/headers.go", Package: "schema", Template: "templates/prepared.go.tmpl", Imports: imports, Prepared: headerPrepared})
	}
	if len(xmlPrograms) != 0 {
		files = append(files, filePlan{Path: "internal/schema/xml.go", Package: "schema", Template: "templates/schema-xml.go.tmpl", Imports: []importPlan{{Path: "context"}, {Path: config.ImportPath + "/internal/wire"}}, XMLPrograms: xmlPrograms, Prepared: xmlPrepared})
	}
	if len(xmlEncodePrograms) != 0 {
		files = append(files, filePlan{Path: "internal/schema/xml_encode.go", Package: "schema", Template: "templates/schema-xml-encode.go.tmpl", Imports: []importPlan{{Path: "context"}, {Path: config.ImportPath + "/internal/wire"}}, XMLEncodePrograms: xmlEncodePrograms, Prepared: xmlEncodePrepared})
	}
	if options.HasAddon(generator.AddonMetadata) {
		files, err = appendMetadataFile(files, sourceDocument)
		if err != nil {
			return generator.Plan{}, nil, err
		}
	}
	for index := range files {
		if files[index].Template == "templates/client.go.tmpl" {
			files[index].OpenAPIVersion = sourceDocument.OpenAPIVersion
			files[index].OpenAPIVersionLine = sourceDocument.OpenAPIVersionLine
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	if err := validateFileImports(config.ImportPath, files); err != nil {
		return generator.Plan{}, nil, err
	}
	inspectionOperations := append([]operationPlan(nil), operations...)
	if selections.Root != nil {
		for index := range inspectionOperations {
			inspectionOperations[index].DependencyOnly = !selections.Root.Direct[inspectionOperations[index].Route]
		}
	}
	inboundInspection := make([]generator.InboundOperationInspection, 0, len(inbound))
	for _, entry := range inbound {
		definition := entry.Definition
		inboundInspection = append(inboundInspection, generator.InboundOperationInspection{Kind: definition.Kind, Identity: definition.Identity, Name: definition.Name, Method: definition.Operation.Method, Expression: definition.Expression, SourceRoute: definition.SourceRoute, SourceOperationID: definition.SourceOperationID, ComponentName: definition.ComponentName})
	}
	return generator.NewPlan("golang", sourcePlan{ImportPath: config.ImportPath, Files: files, Inbound: inboundInspection, Inspection: prepareInspection(preparedDocument, inspectionOperations, clientResources(resources, selections.Root), omitted)}), diagnostics, nil
}

func validPackage(name string) bool { return token.IsIdentifier(name) && name != "_" }
