package golang

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	schemaplan "openapi-sdkgen/internal/schema/plan"
)

func bareMediaType(value string) string {
	bare, _, _ := strings.Cut(value, ";")
	return strings.ToLower(strings.TrimSpace(bare))
}

// A view binds one media-root policy to a shared algorithm. Private reference
// calls retain the invocation's policy; nested instances are distinguished by path.
func (planner *schemaPlanner) validationBinding(name string) schemaProgram {
	if binding, ok := planner.viewBindings[name]; ok {
		return binding
	}
	return schemaProgram{ViewTarget: "check" + name, MediaType: planner.programMedia[name]}
}

func (planner *schemaPlanner) bindView(name, mediaType string, rejectNull bool) string {
	binding := planner.validationBinding(name)
	mediaType = bareMediaType(mediaType)
	if mediaType == "" {
		mediaType = bareMediaType(binding.MediaType)
	}
	rejectNull = rejectNull || binding.RejectNull
	if mediaType == bareMediaType(binding.MediaType) && rejectNull == binding.RejectNull {
		return name
	}
	if planner.views == nil {
		planner.views = make(map[string]string)
	}
	key := binding.ViewTarget + "\x00" + mediaType
	prefix := "ValidateView_"
	if rejectNull {
		key += "\x00nonnull"
		prefix = "ValidateNonNull_"
	}
	if existing, ok := planner.views[key]; ok {
		return existing
	}
	digest := sha256.Sum256([]byte(key))
	view := prefix + hex.EncodeToString(digest[:])
	program := schemaProgram{Name: view, Owner: view, Public: true, ViewTarget: binding.ViewTarget, MediaType: mediaType, RejectNull: rejectNull}
	planner.programs = append(planner.programs, program)
	if planner.viewBindings == nil {
		planner.viewBindings = make(map[string]schemaProgram)
	}
	planner.viewBindings[view] = program
	planner.views[key] = view
	if planner.programMedia == nil {
		planner.programMedia = make(map[string]string)
	}
	planner.programMedia[view] = mediaType
	return view
}

func (planner *schemaPlanner) view(name, mediaType string) string {
	return planner.bindView(name, mediaType, false)
}
func (planner *schemaPlanner) nonNullView(name, mediaType string) string {
	return planner.bindView(name, mediaType, true)
}

func (planner *modelPlanner) validation(node *schemaplan.Node, mediaType string) string {
	name := planner.validators[node]
	if name == "" || !planner.contentPolicyMatters(node, mediaType) {
		return name
	}
	return planner.validationView(name, mediaType)
}

func (planner *modelPlanner) contentPolicyMatters(root *schemaplan.Node, mediaType string) bool {
	if mediaType == "" {
		return false
	}
	seen := make(map[*schemaplan.Node]bool)
	var visit func(*schemaplan.Node) bool
	visit = func(node *schemaplan.Node) bool {
		if node == nil || seen[node] {
			return false
		}
		seen[node] = true
		if node.Get("contentSchema") != nil {
			if literal, ok := node.Get("contentMediaType").(schemaplan.Literal); ok {
				declared, _ := literal.Data.(string)
				if declared != "" && bareMediaType(declared) != bareMediaType(mediaType) {
					return true
				}
			}
		}
		if node.Get("dynamicReference") != nil {
			return true
		}
		for _, field := range node.Fields {
			switch field.Name {
			case "reference":
				if visit(planner.referenceNodes[field.Value.(schemaplan.Literal).Data.(string)]) {
					return true
				}
			case "allOf", "oneOf", "anyOf":
				for _, child := range field.Value.(schemaplan.Children).Nodes {
					if visit(child) {
						return true
					}
				}
			case "not", "if", "then", "else":
				if visit(field.Value.(schemaplan.Child).Node) {
					return true
				}
			case "dependentSchemas":
				for _, child := range field.Value.(schemaplan.SchemaMap).Entries {
					if visit(child.Schema) {
						return true
					}
				}
			case "discriminator":
				data := field.Value.(schemaplan.Discriminator)
				for _, mapping := range data.Mapping {
					if visit(mapping.Schema) {
						return true
					}
				}
				if visit(data.Default) {
					return true
				}
			}
		}
		return false
	}
	return visit(root)
}
