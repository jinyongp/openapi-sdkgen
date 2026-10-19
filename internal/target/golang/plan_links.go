package golang

import (
	"encoding/json"
	"fmt"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
	httpplan "openapi-sdkgen/internal/http/plan"
	schemaplan "openapi-sdkgen/internal/schema/plan"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type linkOverrideFieldPlan struct {
	inputFieldPlan
	Native   bool
	Original inputFieldPlan
	Key      string
}
type linkTargetPlan struct {
	Name            string
	Fields          []linkOverrideFieldPlan
	OptionsRequired bool
	Security        bool
}
type linkSourcePlan struct {
	Operation operationPlan
	Groups    []linkGroupPlan
}
type linkGroupPlan struct {
	OriginalName                      string
	Name, Member                      string
	Source                            operationPlan
	Links                             []linkLeafPlan
	DifferentTargets, OptionsRequired bool
}
type linkLeafPlan struct {
	Name, Status, StatusName, Definition, ServerURL string
	Target                                          operationPlan
	SourceSections                                  map[string]bool
	SourceBody                                      bool
}

func linkStatusName(status string) string {
	if status == "default" {
		return "Default"
	}
	return strings.ToUpper(status)
}
func prepareLinkNames(names *namePlan, document *ir.Document, operations []ir.Operation, used map[string]bool) error {
	names.linkGroups = make(map[string]string)
	names.linkMembers = make(map[string]string)
	for _, operation := range operations {
		statuses := make(map[string][]string)
		for _, response := range operation.Responses {
			links, _ := response.Raw["links"].(map[string]any)
			for name := range links {
				statuses[name] = append(statuses[name], response.Status)
			}
		}
		members := make(map[string]bool)
		for _, name := range sortedKeys(statuses) {
			identity := operation.RouteKey + "\x00" + name
			member, err := reservePublicName(members, name, identity, "")
			if err != nil {
				return err
			}
			names.linkMembers[identity] = member
			group, err := reserveDerivedName(used, names.operations[operation.RouteKey]+exportedName(name)+"Link", identity, func(base string) []string {
				result := []string{base, base + "Request", base + "Output", base + "RawResponse", base + "StreamItem", base + "Input", base + "Options", base + "Invocation"}
				for _, status := range statuses[name] {
					suffix := linkStatusName(status)
					result = append(result, base+"ByStatus"+suffix, base+"InvocationFor"+suffix, base+"InputFor"+suffix, base+"OptionsFor"+suffix)
				}
				return result
			})
			if err != nil {
				return err
			}
			names.linkGroups[identity] = group
		}
	}
	return nil
}

func prepareLinks(document *ir.Document, operations []operationPlan, names namePlan, schemas *schemaPlanner) ([]linkSourcePlan, []diagnostic.Diagnostic, error) {
	visible := make(map[string]bool)
	byRoute := make(map[string]int)
	for index, operation := range operations {
		visible[operation.Route] = true
		byRoute[operation.Route] = index
	}
	prepared, failures := (&httpplan.LinkPlanner{Visible: visible}).Prepare(document)
	var diagnostics []diagnostic.Diagnostic
	for _, finding := range failures {
		severity, code, scope, effect := diagnostic.SeverityWarning, "SDKGEN-W509", failure.ScopeCapability, failure.EffectOmitCapability
		if finding.Blocking || strings.Contains(finding.Error(), "targets hidden operation") {
			severity, code, scope, effect = diagnostic.SeverityError, "SDKGEN-E509", failure.ScopeDocument, failure.EffectBlock
			if strings.Contains(finding.Error(), "targets hidden operation") {
				code = "SDKGEN-E621"
			}
		}
		diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: severity, Phase: diagnostic.PhaseTarget, Target: "golang", Code: code, Scope: scope, Effect: effect, Capability: "response-link", Route: finding.SourceOperation.RouteKey, Operation: finding.SourceOperation.OperationID, Location: diagnostic.Location{Pointer: finding.Pointer}, Message: finding.Error()})
	}
	sources := make(map[string]*linkSourcePlan)
	sourceOperations := make(map[string]ir.Operation)
	for _, operation := range document.Operations {
		sourceOperations[operation.RouteKey] = operation
	}
	for _, link := range prepared {
		sourceIndex, targetIndex := byRoute[link.SourceOperation.RouteKey], byRoute[link.TargetOperation.RouteKey]
		source, target := &operations[sourceIndex], &operations[targetIndex]
		if target.LinkTarget == nil {
			planned, err := prepareLinkTarget(*target, document, schemas)
			if err != nil {
				return nil, nil, err
			}
			target.LinkTarget = planned
		}
		leaf, err := prepareLinkLeaf(link, *source, *target, sourceOperations[link.SourceOperation.RouteKey])
		if err != nil {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.SeverityWarning, Phase: diagnostic.PhaseTarget, Target: "golang", Code: "SDKGEN-W509", Scope: failure.ScopeCapability, Effect: failure.EffectOmitCapability, Capability: "response-link", Route: source.Route, Operation: source.OperationID, Location: diagnostic.Location{Pointer: link.Pointers.Parameters}, Message: err.Error()})
			continue
		}
		source.LinkSource = true
		owner := sources[source.Route]
		if owner == nil {
			owner = &linkSourcePlan{}
			sources[source.Route] = owner
		}
		identity := source.Route + "\x00" + link.Name
		groupIndex := -1
		for index, group := range owner.Groups {
			if group.Name == names.linkGroups[identity] {
				groupIndex = index
				break
			}
		}
		if groupIndex < 0 {
			owner.Groups = append(owner.Groups, linkGroupPlan{Name: names.linkGroups[identity], Member: names.linkMembers[identity], OriginalName: link.Name})
			groupIndex = len(owner.Groups) - 1
		}
		group := &owner.Groups[groupIndex]
		leaf.Name = group.Name + "ByStatus" + leaf.StatusName
		group.Links = append(group.Links, leaf)
	}
	var result []linkSourcePlan
	for _, route := range sortedKeys(sources) {
		owner := sources[route]
		owner.Operation = operations[byRoute[route]]
		for index := range owner.Groups {
			group := &owner.Groups[index]
			group.Source = owner.Operation
			targets := make(map[string]bool)
			for leafIndex := range group.Links {
				leaf := &group.Links[leafIndex]
				leaf.Target = operations[byRoute[leaf.Target.Route]]
				targets[leaf.Target.Route] = true
				group.OptionsRequired = group.OptionsRequired || leaf.Target.LinkTarget.OptionsRequired
			}
			group.DifferentTargets = len(targets) > 1
			sort.Slice(group.Links, func(i, j int) bool { return group.Links[i].Status < group.Links[j].Status })
		}
		result = append(result, *owner)
	}
	return result, diagnostics, nil
}

func prepareLinkTarget(operation operationPlan, document *ir.Document, schemas *schemaPlanner) (*linkTargetPlan, error) {
	target := &linkTargetPlan{Name: operation.Name + "LinkOverride", OptionsRequired: len(operation.Security) > 1, Security: len(operation.Security) > 0}
	for _, field := range operation.InputFields {
		planned := linkOverrideFieldPlan{Original: field, Key: strings.ToLower(field.Name), Native: field.Binary || field.Union || field.Incremental}
		planned.inputFieldPlan = inputFieldPlan{Name: field.Name, Type: field.Type, Multipart: field.Multipart, MediaType: field.MediaType}
		if !planned.Native {
			planned.Type = operation.Name + "LinkOverride" + field.Name
			schema := partialLinkSchema(document, schemas.annotations[schemas.nodes[field.Validation]], make(map[string]bool))
			validation, err := schemas.prepare(httpValidationName(planned.Type), schema, schemaplan.Input, document.OpenAPIVersionLine == "3.0", schemaOccurrence{Source: operation.Route + "/link-override/" + planned.Key, MediaType: field.MediaType})
			if err != nil {
				return nil, err
			}
			planned.Validation = validation
		}
		target.Fields = append(target.Fields, planned)
	}
	return target, nil
}
func partialLinkSchema(document *ir.Document, value any, seen map[string]bool) any {
	source, ok := value.(map[string]any)
	if !ok {
		return value
	}
	result := make(map[string]any, len(source))
	for key, item := range source {
		result[key] = item
	}
	if reference, _ := source["$ref"].(string); reference != "" && !seen[reference] {
		name, err := schemaplan.ComponentReferenceName(reference)
		if err == nil {
			seen[reference] = true
			if target, ok := partialLinkSchema(document, document.ComponentSchemas[name], seen).(map[string]any); ok {
				delete(result, "$ref")
				for key, item := range target {
					if _, exists := result[key]; !exists {
						result[key] = item
					}
				}
			}
			delete(seen, reference)
		}
	}
	delete(result, "required")
	for _, key := range []string{"allOf", "oneOf", "anyOf"} {
		if children, ok := source[key].([]any); ok {
			items := make([]any, len(children))
			for index, child := range children {
				items[index] = partialLinkSchema(document, child, seen)
			}
			result[key] = items
		}
	}
	return result
}

var goLinkRequestParameter = regexp.MustCompile(`^\$request\.(path|query|header|cookie)\.([^#]+)(#.*)?$`)

func prepareLinkLeaf(link httpplan.ResponseLink, source, target operationPlan, sourceOperation ir.Operation) (linkLeafPlan, error) {
	leaf := linkLeafPlan{Status: link.Status, StatusName: linkStatusName(link.Status), ServerURL: link.ServerURL, Target: target, SourceSections: make(map[string]bool)}
	value := func(raw any) (string, error) {
		if text, ok := raw.(string); ok && strings.HasPrefix(text, "$") {
			if matches := goLinkRequestParameter.FindStringSubmatch(text); matches != nil {
				found := false
				for _, parameter := range sourceOperation.Parameters {
					if parameter.Location == matches[1] && parameter.Name == matches[2] {
						found = true
						break
					}
				}
				if !found {
					return "", fmt.Errorf("unknown source parameter in Link expression %q", text)
				}
				leaf.SourceSections[matches[1]] = true
				return fmt.Sprintf("core.LinkValue{Parameter:true,Section:%q,Name:%q,Pointer:%q}", matches[1], matches[2], strings.TrimPrefix(matches[3], "#")), nil
			}
			if text == "$request.body" || strings.HasPrefix(text, "$request.body#") {
				leaf.SourceBody = true
				leaf.SourceSections["body"] = true
			}
			return "core.LinkValue{Expression:" + strconv.Quote(text) + "}", nil
		}
		encoded, err := json.Marshal(raw)
		if err != nil {
			return "", err
		}
		return "core.LinkValue{Literal:linkLiteral(" + strconv.Quote(string(encoded)) + ")}", nil
	}
	var assignments []string
	parameters, _ := link.Object["parameters"].(map[string]any)
	for _, name := range sortedKeys(parameters) {
		var matches []parameterPlan
		for _, parameter := range target.Parameters {
			if parameter.Name == name {
				matches = append(matches, parameter)
			}
		}
		if len(matches) != 1 {
			return leaf, fmt.Errorf("Link parameter %q matches %d target parameters", name, len(matches))
		}
		literal, err := value(parameters[name])
		if err != nil {
			return leaf, err
		}
		assignments = append(assignments, fmt.Sprintf("{Section:%q,Name:%q,Value:%s}", matches[0].Location, name, literal))
	}
	leaf.Definition = "core.LinkDefinition{Parameters:[]core.LinkAssignment{" + strings.Join(assignments, ",") + "}"
	if body, present := link.Object["requestBody"]; present {
		literal, err := value(body)
		if err != nil {
			return leaf, err
		}
		leaf.Definition += ",Body:&" + literal
	}
	leaf.Definition += "}"
	return leaf, nil
}
