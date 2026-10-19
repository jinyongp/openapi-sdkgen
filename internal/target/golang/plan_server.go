package golang

import (
	"fmt"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
	"openapi-sdkgen/internal/generator"
	"openapi-sdkgen/internal/http/extension"
	httpplan "openapi-sdkgen/internal/http/plan"
	"strings"
)

type inboundOperationPlan struct {
	Definition                 httpplan.InboundDefinition
	Operation                  operationPlan
	PublicName                 string
	BodyDeclared, BodyRequired bool
	XMLViews                   operationPlan
	ResponseEncodings          map[string]string
}

// Item input is an already-open source. A complete schema alongside itemSchema
// describes a different consumption mode; the existing inbound contract selects
// itemSchema for that media. The original document remains unchanged.
func inboundInputOperation(operation ir.Operation) ir.Operation {
	if operation.RequestBody == nil {
		return operation
	}
	body := *operation.RequestBody
	body.Content = append([]ir.MediaType(nil), body.Content...)
	for index := range body.Content {
		media := &body.Content[index]
		if _, present := media.Raw["itemSchema"]; !present {
			continue
		}
		cloned := make(map[string]any, len(media.Raw))
		for key, value := range media.Raw {
			if key != "schema" {
				cloned[key] = value
			}
		}
		media.Raw = cloned
		media.Schema = nil
	}
	operation.RequestBody = &body
	return operation
}

func inboundNames(definition httpplan.InboundDefinition) (string, string) {
	raw := definition.Name + " " + strings.ToLower(definition.Operation.Method)
	if definition.Kind == "callback" {
		owner := definition.SourceOperationID
		if owner == "" {
			owner = definition.SourceRoute
		}
		if definition.ComponentName != "" {
			owner = "component"
		}
		raw = owner + " " + raw
	}
	return definition.Kind + " " + raw, raw
}

func inboundNameFamily(base string, operation ir.Operation, legacy bool) ([]string, error) {
	roots, factories, err := operationContractNames(operation, legacy)
	if err != nil {
		return nil, err
	}
	result := append(modelRootNames(base), base+"Response", base+"Handler", base+"Info", "New"+base+"Handler")
	for _, root := range roots {
		result = append(result, modelNames(base+root)...)
		result = append(result, "New"+base+root+"Response", "New"+base+root+"CompleteResponse")
	}
	for _, factory := range factories {
		result = append(result, "New"+base+factory)
	}
	used := make(map[string]bool)
	for _, response := range operation.Responses {
		content := response.Content
		if len(content) == 0 {
			content = []ir.MediaType{{}}
		}
		for _, media := range content {
			kind := bodyMediaKind(media.ContentType, media.Schema, legacy)
			if media.Stream.IsStreaming() {
				kind = "stream"
			}
			variant, err := responseVariantName(operation, response, media, kind, used)
			if err != nil {
				return nil, err
			}
			result = append(result, "New"+base+variant+"Response", "New"+base+variant+"CompleteResponse", "New"+base+variant+"BinaryResponse")
		}
	}
	return result, nil
}

func prepareInboundOperations(document *ir.Document, scopes []httpplan.InboundScope, names namePlan, schemas *schemaPlanner) ([]inboundOperationPlan, []diagnostic.Diagnostic) {
	var diagnostics []diagnostic.Diagnostic
	finding := func(pointer string, err error) {
		location, related := (&extension.Planner{}).DiagnosticLocation(document, pointer)
		diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.SeverityError, Phase: diagnostic.PhaseTarget, Target: "golang", Code: "SDKGEN-E506", Scope: failure.ScopeDocument, Effect: failure.EffectBlock, Capability: "inbound contracts", Location: location, Related: related, Message: "Cannot prepare inbound contracts: " + err.Error()})
	}
	// Invalid contracts outside selection cannot block this entry. Successfully
	// read declarations still reserve the whole document's names before scoping.
	all, _ := httpplan.CollectInbound(document, httpplan.InboundScopes(document, nil, nil))
	internalNames, publicNames := make(map[string]string), make(map[string]string)
	publicUsed := map[string]map[string]bool{"webhook": {"Options": true, "Handlers": true, "Routes": true, "NewRouter": true}, "callback": {"Options": true, "Handlers": true, "Endpoints": true, "NewEndpoints": true}}
	legacy := document.OpenAPIVersionLine == "3.0"
	for _, definition := range all {
		operation := inboundInputOperation(definition.Operation)
		internalRaw, publicRaw := inboundNames(definition)
		family, err := inboundNameFamily("", operation, legacy)
		if err != nil {
			continue
		}
		derive := func(base string) []string {
			result := make([]string, len(family))
			for index, member := range family {
				if strings.HasPrefix(member, "New") {
					result[index] = "New" + base + strings.TrimPrefix(member, "New")
				} else {
					result[index] = base + member
				}
			}
			return result
		}
		name, err := reserveDerivedName(names.reserved, internalRaw, definition.Identity, derive)
		if err != nil {
			finding(definition.Identity, err)
			continue
		}
		public, err := reserveDerivedName(publicUsed[definition.Kind], publicRaw, definition.Identity, derive)
		if err != nil {
			finding(definition.Identity, err)
			continue
		}
		internalNames[definition.Identity], publicNames[definition.Identity] = name, public
	}
	var result []inboundOperationPlan
	for _, scope := range scopes {
		definitions, failures := httpplan.CollectInbound(document, []httpplan.InboundScope{scope})
		for _, err := range failures {
			finding(scope.Pointer, err)
		}
		for _, definition := range definitions {
			operation := inboundInputOperation(definition.Operation)
			planned := operationPlan{Name: internalNames[definition.Identity], OperationID: operation.OperationID, Method: operation.Method, Path: operation.Path, Route: operation.RouteKey, Documentation: operationDocumentation(operation)}
			var responseEncodings map[string]string
			if planned.Name == "" {
				finding(definition.Identity, fmt.Errorf("cannot reserve inbound operation identity"))
				continue
			}
			prepare := func() error {
				var err error
				planned.BodyEncodings, err = prepareBodyEncodings(&planned, operation, document, schemas)
				if err != nil {
					return err
				}
				planned.Security, err = prepareOperationSecurity(document, operation)
				if err != nil {
					return err
				}
				for index, requirement := range planned.Security {
					planned.SecurityMetadata = planned.SecurityMetadata || strings.Contains(requirement.Definition, "securityMetadata(")
					planned.Security[index].Definition = strings.ReplaceAll(requirement.Definition, "securityMetadata(", "securityMetadataFor"+planned.Name+"(")
				}
				if err := prepareOperationInput(&planned, operation, schemas, legacy, document, true); err != nil {
					return err
				}
				if err := prepareOperationResponses(&planned, operation, schemas, legacy, document, true); err != nil {
					return err
				}
				if err := prepareOperationStreams(&planned, operation, document, schemas, legacy, true); err != nil {
					return err
				}
				if err := prepareInboundRequestParts(&planned, operation, document, schemas); err != nil {
					return err
				}
				if err := prepareResponseHeaders(&planned, operation, document, schemas); err != nil {
					return err
				}
				responseEncodings, err = prepareInboundResponseEncodings(&planned, operation, document, schemas)
				return err
			}
			if err := prepare(); err != nil {
				finding(definition.Identity, err)
				continue
			}
			entry := inboundOperationPlan{Definition: definition, Operation: planned, PublicName: publicNames[definition.Identity], BodyDeclared: operation.RequestBody != nil, ResponseEncodings: responseEncodings}
			if operation.RequestBody != nil {
				entry.BodyRequired = operation.RequestBody.Required
			}
			result = append(result, entry)
		}
	}
	return result, diagnostic.Sort(diagnostics)
}

func inboundScopes(document *ir.Document, selection *generationSelection, omitted map[string]bool) []httpplan.InboundScope {
	var direct map[string]bool
	if selection != nil {
		direct = selection.Direct
	}
	return httpplan.InboundScopes(document, omitted, direct)
}

func inboundAddonDiagnostics(document *ir.Document, scopes []httpplan.InboundScope, options generator.Options) []diagnostic.Diagnostic {
	if options.HasAddon(generator.AddonServer) {
		return nil
	}
	var result []diagnostic.Diagnostic
	for _, scope := range scopes {
		location, related := (&extension.Planner{}).DiagnosticLocation(document, scope.Pointer)
		result = append(result, diagnostic.Diagnostic{Severity: diagnostic.SeverityError, Phase: diagnostic.PhaseTarget, Target: "golang", Code: "SDKGEN-E505", Scope: failure.ScopeDocument, Effect: failure.EffectBlock, Capability: "inbound " + scope.Kind + " contracts", Location: location, Related: related, Message: fmt.Sprintf("Generating %s contracts requires the server addon.", scope.Kind), Hint: "Enable the server addon with --with server."})
	}
	return diagnostic.Sort(result)
}
