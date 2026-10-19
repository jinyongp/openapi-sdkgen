package golang

import (
	"bytes"
	"fmt"
	"openapi-sdkgen/internal/compiler/ir"
	"strconv"
	"strings"

	schemaplan "openapi-sdkgen/internal/schema/plan"
)

type modelPlan struct {
	ResponseView                           bool
	ValidatedView                          bool
	Binding                                bool
	Media                                  bool
	Documentation                          string
	NullableRoot, NullableLiteral          bool
	NullableInner                          string
	Owner                                  string
	UsesWire, UsesFmt                      bool
	Name, Type, Encode, Decode, Validation string
	Named, Wrapper, ScalarLiteral          bool
	Literals                               []modelLiteral
	Branches                               []modelBranch
}
type modelLiteral struct{ Name, Expression, JSON, Documentation string }
type modelBranch struct {
	Name, Type, Codec, Validation, Constructor string
	Predicate, Tag, TagValue                   string
	Default                                    bool
	Known                                      []string
}
type modelType struct{ Type, Codec string }
type modelPlanner struct {
	sorts                             map[*schemaplan.Node]ir.SortParameterPlan
	media                             bool
	nativeMedia                       map[string]bool
	version                           string
	componentSources                  map[string]any
	annotations                       map[*schemaplan.Node]any
	memberDocs                        map[*schemaplan.Node]map[string]schemaplan.Documentation
	validationView                    func(string, string) string
	nonNullValidation                 func(string, string) string
	programMedia                      map[string]string
	programProjection                 map[string]schemaplan.Projection
	plans                             []modelPlan
	references                        map[string]modelType
	validators                        map[*schemaplan.Node]string
	sources                           map[*schemaplan.Node]string
	used                              map[string]bool
	needsWire, needsFmt, needsLiteral bool
	referenceGraph                    map[string][]string
	referenceNodes                    map[string]*schemaplan.Node
	currentRoot                       string
	activeObjects                     map[*schemaplan.Node]string
	activeObjectValidators            map[string]string
}

func (planner *modelPlanner) root(name string, node *schemaplan.Node, validation string) error {
	planner.currentRoot = "check" + validation
	start := len(planner.plans)
	previousWire, previousFmt, previousLiteral := planner.needsWire, planner.needsFmt, planner.needsLiteral
	planner.needsWire, planner.needsFmt, planner.needsLiteral = false, false, false
	planner.media = planner.nativeMedia[validation] || planner.programProjection[validation] == schemaplan.Input && strings.HasPrefix(strings.ToLower(planner.programMedia[validation]), "multipart/")
	if planner.media {
		planner.needsWire = true
	}
	_, err := planner.shape(name, node, true, planner.programMedia[validation])
	if err != nil {
		return err
	}
	planner.plans[len(planner.plans)-1].Validation = validation
	for index := start; index < len(planner.plans); index++ {
		planner.plans[index].ResponseView = planner.programProjection[validation] == schemaplan.Output
		planner.plans[index].Media = planner.media
		planner.plans[index].Owner = name
		planner.plans[index].UsesWire = planner.needsWire
		planner.plans[index].UsesFmt = planner.needsFmt
	}
	planner.needsWire = planner.needsWire || previousWire
	planner.needsFmt = planner.needsFmt || previousFmt
	planner.needsLiteral = planner.needsLiteral || previousLiteral
	return nil
}

func (planner *modelPlanner) shape(name string, node *schemaplan.Node, root bool, modes ...string) (result modelType, err error) {
	if planner.memberDocs == nil {
		planner.memberDocs = make(map[*schemaplan.Node]map[string]schemaplan.Documentation)
	}
	if node.Get("binaryInput") == nil {
		node, err = planner.promoteAnnotatedEnum(node)
		if err != nil {
			return modelType{}, err
		}
	}
	mediaType := ""
	if len(modes) != 0 {
		mediaType = modes[0]
	}
	if !root {
		identity := planner.sources[node]
		if identity == "" {
			identity = name
		}
		reserved, reserveErr := reserveDerivedName(planner.used, name, identity, modelNames)
		if reserveErr != nil {
			return modelType{}, reserveErr
		}
		name = reserved
	}
	defer func() {
		if err == nil && len(planner.plans) != 0 {
			planner.plans[len(planner.plans)-1].Validation = planner.validation(node, mediaType)
			planner.plans[len(planner.plans)-1].Documentation = planner.documentation(node)
		}
	}()
	if target, active := planner.activeObjects[node]; active {
		return planner.recursivePointer(name, target), nil
	}
	// Flattening duplicate allOf properties copies nodes while preserving their
	// validation identity. That identity also closes the same DTO recursion.
	if validation := planner.validators[node]; validation != "" {
		if target, active := planner.activeObjectValidators[validation]; active {
			return planner.recursivePointer(name, target), nil
		}
	}
	if sort, exists := planner.sorts[node]; exists {
		return planner.sort(name, node, sort)
	}
	if len(node.Fields) == 0 || len(node.Fields) == 1 && node.Fields[0].Name == "boolean" {
		if planner.media {
			planner.plans = append(planner.plans, modelPlan{Name: name, Type: "types.MultipartValue", Encode: "prepared,err:=types.MultipartValue(value).WireValue();if err!=nil{return types.Value{},err};return state.absorbMedia(prepared)", Decode: "return " + name + "(types.MultipartFromWireValue(wire.MediaValue[types.Binary]{Value:value,Slots:state.relativeBinaries()})),nil", Named: root})
			return modelType{Type: chooseModelName(root, name, "types.MultipartValue"), Codec: name}, nil
		}
		planner.plans = append(planner.plans, modelPlan{Name: name, Type: "types.Value", Encode: "return value,nil", Decode: "return value,nil"})
		return modelType{Type: chooseModelName(root, name, "types.Value"), Codec: name}, nil
	}
	if node.Get("binaryInput") != nil {
		return planner.binary(name, node, root)
	}
	typeField, _ := node.Get("types").(schemaplan.Literal)
	types, _ := typeField.Data.([]string)
	nonNull := make([]string, 0, len(types))
	nullable := false
	for _, kind := range types {
		if kind == "null" {
			nullable = true
		} else {
			nonNull = append(nonNull, kind)
		}
	}
	if nullable && len(nonNull) == 1 {
		planner.needsWire = true
		planner.needsFmt = true
		copy := &schemaplan.Node{Fields: append([]schemaplan.Field(nil), node.Fields...)}
		for index, field := range copy.Fields {
			if field.Name == "types" {
				copy.Fields[index].Value = schemaplan.Literal{Data: nonNull}
			}
			if field.Name == "enumValues" {
				values := field.Value.(schemaplan.Literal).Data.([]any)
				filtered := make([]any, 0, len(values))
				for _, value := range values {
					if value != nil {
						filtered = append(filtered, value)
					}
				}
				copy.Fields[index].Value = schemaplan.Literal{Data: filtered}
			}
		}
		planner.validators[copy] = planner.nonNullValidation(planner.validators[node], mediaType)
		planner.sources[copy] = planner.sources[node]
		planner.annotations[copy] = planner.annotations[node]
		planner.memberDocs[copy] = planner.memberDocs[node]
		inner, err := planner.shape(name+"NonNull", copy, false, mediaType)
		if err != nil {
			return modelType{}, err
		}
		typeName := "types.Nullable[" + inner.Type + "]"
		encode := fmt.Sprintf("if item,present:=value.Get();present{encoded,err:=encode%s(item,state);if err!=nil{return types.Value{},err};if encoded.Kind()==wire.Null{return types.Value{},fmt.Errorf(\"non-null value encoded as null\")};return encoded,nil};return types.Value{},nil", inner.Codec)
		decode := fmt.Sprintf("if value.Kind()==wire.Null{return %s(types.Null[%s]()),nil};item,err:=decode%s(value,state);if err!=nil{return zero,err};return %s(types.NonNull(item)),nil", name, inner.Type, inner.Codec, name)
		innerPlan := planner.plans[len(planner.plans)-1]
		literals, err := planner.nullableLiterals(name, node, inner, innerPlan)
		if err != nil {
			return modelType{}, err
		}
		planner.plans = append(planner.plans, modelPlan{Name: name, Type: typeName, Encode: encode, Decode: decode, Named: root, NullableRoot: root, NullableInner: inner.Type, NullableLiteral: len(literals) != 0, Literals: literals})
		return modelType{Type: chooseModelName(root, name, typeName), Codec: name}, nil
	}
	if literal, ok, err := planner.literal(name, node); ok || err != nil {
		return literal, err
	}
	if len(nonNull) == 1 && nonNull[0] == "array" && node.Get("prefixItems") != nil {
		return planner.tuple(name, node)
	}
	if node.Get("allOf") != nil || node.Get("reference") != nil && node.Get("properties") != nil {
		if merged, ok := planner.flattenObject(node); ok {
			return planner.objectShape(name, merged, node)
		}
	}
	if reference, ok := node.Get("reference").(schemaplan.Literal); ok {
		target, exists := planner.references[reference.Data.(string)]
		if !exists {
			return modelType{}, fmt.Errorf("missing Go model reference %q", reference.Data)
		}
		if targetNode := planner.referenceNodes[reference.Data.(string)]; targetNode != nil && !planner.aliasCycle(reference.Data.(string)) && (planner.contentPolicyMatters(targetNode, mediaType) || root && len(node.Fields) > 1) {
			representation := &schemaplan.Node{Fields: append([]schemaplan.Field(nil), targetNode.Fields...)}
			planner.validators[representation] = planner.validators[node]
			planner.sources[representation] = planner.sources[targetNode]
			planner.annotations[representation] = planner.annotations[targetNode]
			return planner.shape(name, representation, true, mediaType)
		}
		if root {
			if planner.aliasCycle("check" + planner.validators[node]) {
				return planner.wrapper(name), nil
			}
			if len(node.Fields) == 1 {
				planner.plans = append(planner.plans, modelPlan{Name: name, Type: target.Type, Encode: fmt.Sprintf("return encode%s(value,state)", target.Codec), Decode: fmt.Sprintf("return decode%s(value,state)", target.Codec)})
				return modelType{Type: name, Codec: name}, nil
			}
			planner.plans = append(planner.plans, modelPlan{Name: name, Type: target.Type, Encode: fmt.Sprintf("return encode%s(%s(value),state)", target.Codec, target.Type), Decode: fmt.Sprintf("item,err:=decode%s(value,state);if err!=nil{return zero,err};return %s(item),nil", target.Codec, name), Named: true})
			return modelType{Type: name, Codec: name}, nil
		}
		typeName := "*" + target.Type
		if !planner.reaches(reference.Data.(string), planner.currentRoot) {
			planner.plans = append(planner.plans, modelPlan{Name: name, Type: target.Type, Encode: fmt.Sprintf("return encode%s(value,state)", target.Codec), Decode: fmt.Sprintf("return decode%s(value,state)", target.Codec)})
			return modelType{Type: target.Type, Codec: name}, nil
		}
		planner.needsFmt = true
		encode := fmt.Sprintf("if value==nil{return types.Value{},fmt.Errorf(\"nil non-null model pointer\")};if state.active[value]{return types.Value{},fmt.Errorf(\"cyclic model value\")};state.active[value]=true;defer delete(state.active,value);return encode%s(*value,state)", target.Codec)
		decode := fmt.Sprintf("item,err:=decode%s(value,state);if err!=nil{return zero,err};return &item,nil", target.Codec)
		planner.plans = append(planner.plans, modelPlan{Name: name, Type: typeName, Encode: encode, Decode: decode})
		return modelType{Type: chooseModelName(root, name, typeName), Codec: name}, nil
	}
	for _, keyword := range []string{"allOf", "oneOf", "anyOf", "not", "if", "prefixItems", "dynamicReference"} {
		if node.Get(keyword) != nil {
			return planner.composite(name, node, mediaType)
		}
	}
	if len(nonNull) != 1 {
		return planner.wrapper(name), nil
	}
	typeName := ""
	encode, decode := "", ""
	switch nonNull[0] {
	case "string":
		typeName = "string"
		encode = "return wire.CheckedText(string(value))"
		decode = fmt.Sprintf("item,ok:=value.String();if !ok{return zero,fmt.Errorf(\"expected string\")};return %s(item),nil", name)
	case "boolean":
		typeName = "bool"
		encode = "return wire.Bool(bool(value)),nil"
		decode = fmt.Sprintf("item,ok:=value.Bool();if !ok{return zero,fmt.Errorf(\"expected boolean\")};return %s(item),nil", name)
	case "number", "integer":
		typeName = "float64"
		encode = "return wire.Numeric(float64(value))"
		decode = fmt.Sprintf("item,ok:=value.Number();if !ok{return zero,fmt.Errorf(\"expected number\")};return %s(item),nil", name)
	case "array":
		itemNode := &schemaplan.Node{}
		if item, ok := node.Get("items").(schemaplan.Child); ok {
			itemNode = item.Node
		}
		item, err := planner.shape(name+"Item", itemNode, false)
		if err != nil {
			return modelType{}, err
		}
		typeName = "[]" + item.Type
		encode = fmt.Sprintf("items:=make([]types.Value,len(value));for index,item:=range value{encoded,err:=encode%s(item,state.Index(index));if err!=nil{return types.Value{},err};items[index]=encoded};return wire.List(items),nil", item.Codec)
		decode = fmt.Sprintf("if value.Kind()!=wire.Array{return zero,fmt.Errorf(\"expected array\")};items:=make(%s,value.Len());for index:=range items{item,_:=value.At(index);decoded,err:=decode%s(item,state.Index(index));if err!=nil{return zero,err};items[index]=decoded};return %s(items),nil", typeName, item.Codec, name)
	case "object":
		return planner.objectShape(name, node, node)
	default:
		return planner.wrapper(name), nil
	}
	planner.needsWire = true
	planner.needsFmt = true
	planner.plans = append(planner.plans, modelPlan{Name: name, Type: typeName, Encode: encode, Decode: decode, Named: root})
	return modelType{Type: chooseModelName(root, name, typeName), Codec: name}, nil
}

func chooseModelName(root bool, name, typeName string) string {
	if root {
		return name
	}
	return typeName
}

func (planner *modelPlanner) wrapper(name string) modelType {

	if planner.media {
		planner.needsWire = true
		planner.plans = append(planner.plans, modelPlan{Name: name, Type: "struct{value types.Value;binaries map[string]types.Binary;response bool}", Encode: "return state.absorbMedia(wire.MediaValue[types.Binary]{Value:value.value,Slots:value.binaries})", Decode: "return " + name + "{value:value,binaries:state.relativeBinaries(),response:schema.IsResponseContext(state.ctx)},nil", Wrapper: true})
	} else {
		planner.plans = append(planner.plans, modelPlan{Name: name, Type: "struct{ value types.Value;response bool }", Encode: "return value.value,nil", Decode: "return " + name + "{value:value,response:schema.IsResponseContext(state.ctx)},nil", Wrapper: true})
	}
	return modelType{Type: name, Codec: name}
}

func (planner *modelPlanner) composite(name string, node *schemaplan.Node, mediaType string) (modelType, error) {
	var branches []modelBranch
	used := make(map[string]bool)
	for _, keyword := range []string{"allOf", "oneOf", "anyOf"} {
		children, ok := node.Get(keyword).(schemaplan.Children)
		if !ok {
			continue
		}
		for index, child := range children.Nodes {
			branchName := fmt.Sprintf("%s%d", exportedName(keyword), index+1)
			if reference, ok := child.Get("reference").(schemaplan.Literal); ok {
				if target, exists := planner.references[reference.Data.(string)]; exists {
					branchName = target.Type
				}
			}
			var err error
			branchName, err = reservePublicName(used, branchName, planner.sources[child], "")
			if err != nil {
				return modelType{}, err
			}
			typed, err := planner.shape(name+branchName, child, false, mediaType)
			if err != nil {
				return modelType{}, err
			}
			constructor, err := reservePublicName(planner.used, name+"From"+branchName, planner.sources[child]+"/constructor", "")
			if err != nil {
				return modelType{}, err
			}
			branches = append(branches, modelBranch{Name: branchName, Type: typed.Type, Codec: typed.Codec, Validation: planner.validation(child, mediaType), Constructor: constructor})
		}
	}
	if discriminator, ok := node.Get("discriminator").(schemaplan.Discriminator); ok {
		known := make([]string, 0, len(discriminator.Mapping))
		for _, mapping := range discriminator.Mapping {
			known = append(known, mapping.Name)
		}
		appendMapping := func(raw, tag string, child *schemaplan.Node, isDefault bool) error {
			branchName, err := reservePublicName(used, raw, planner.sources[child], "")
			if err != nil {
				return err
			}
			typed, err := planner.shape(name+branchName, child, false, mediaType)
			if err != nil {
				return err
			}
			constructor, err := reservePublicName(planner.used, name+"From"+branchName, planner.sources[child]+"/constructor", "")
			if err != nil {
				return err
			}
			branches = append(branches, modelBranch{Name: branchName, Type: typed.Type, Codec: typed.Codec, Validation: planner.validation(child, mediaType), Constructor: constructor, Predicate: "matches" + name + branchName, Tag: discriminator.Property, TagValue: tag, Default: isDefault, Known: known})
			return nil
		}
		for _, mapping := range discriminator.Mapping {
			if err := appendMapping("Mapped"+exportedName(mapping.Name), mapping.Name, mapping.Schema, false); err != nil {
				return modelType{}, err
			}
		}
		if discriminator.Default != nil {
			if err := appendMapping("DefaultMapping", "", discriminator.Default, true); err != nil {
				return modelType{}, err
			}
		}
	}
	result := planner.wrapper(name)
	planner.plans[len(planner.plans)-1].Branches = branches
	return result, nil
}

// Inline reference siblings can lead back to the same object representation.
// Reuse that named DTO through a pointer instead of expanding it indefinitely.
// The original node's validator still enforces all reference and sibling rules.
func (planner *modelPlanner) objectShape(name string, representation, original *schemaplan.Node) (modelType, error) {
	if planner.activeObjects == nil {
		planner.activeObjects = make(map[*schemaplan.Node]string)
	}
	planner.activeObjects[original] = name
	defer delete(planner.activeObjects, original)
	if validation := planner.validators[original]; validation != "" {
		if planner.activeObjectValidators == nil {
			planner.activeObjectValidators = make(map[string]string)
		}
		planner.activeObjectValidators[validation] = name
		defer delete(planner.activeObjectValidators, validation)
	}
	return planner.object(name, representation)
}

func (planner *modelPlanner) recursivePointer(name, target string) modelType {
	planner.needsFmt = true
	encode := fmt.Sprintf("if value==nil{return types.Value{},fmt.Errorf(\"nil non-null model pointer\")};if state.active[value]{return types.Value{},fmt.Errorf(\"cyclic model value\")};state.active[value]=true;defer delete(state.active,value);return encode%s(*value,state)", target)
	decode := fmt.Sprintf("item,err:=decode%s(value,state);if err!=nil{return zero,err};return &item,nil", target)
	planner.plans = append(planner.plans, modelPlan{Name: name, Type: "*" + target, Encode: encode, Decode: decode})
	return modelType{Type: "*" + target, Codec: name}
}

func (planner *modelPlanner) object(name string, node *schemaplan.Node) (modelType, error) {
	planner.needsWire = true
	planner.needsFmt = true
	var declaration, encode, decode bytes.Buffer
	fmt.Fprintln(&declaration, "struct {")
	fmt.Fprintln(&encode, "fields:=make(map[string]types.Value)")
	fmt.Fprintf(&decode, "if value.Kind()!=wire.Object{return zero,fmt.Errorf(\"expected object\")};var result %s\n", name)
	required := make(map[string]bool)
	if field, ok := node.Get("required").(schemaplan.Literal); ok {
		for _, key := range field.Data.([]string) {
			required[key] = true
		}
	}
	used := map[string]bool{"AdditionalProperties": true, "JSONValue": true, "MarshalJSON": true, "UnmarshalJSON": true}
	properties, _ := node.Get("properties").(schemaplan.Properties)
	known := make([]string, 0, len(properties.Entries))
	for _, property := range properties.Entries {
		fieldName, err := reservePublicName(used, property.Name, planner.sources[property.Schema], "")
		if err != nil {
			return modelType{}, err
		}
		child, err := planner.shape(name+fieldName, property.Schema, false)
		if err != nil {
			return modelType{}, err
		}
		typeName := child.Type
		if !required[property.Name] {
			typeName = "types.Optional[" + typeName + "]"
		}
		declaration.WriteString(planner.documentation(property.Schema))
		fmt.Fprintf(&declaration, "%s %s\n", fieldName, typeName)
		known = append(known, strconv.Quote(property.Name))
		if required[property.Name] {
			fmt.Fprintf(&encode, "{item:=value.%s;", fieldName)
		} else {
			fmt.Fprintf(&encode, "if item,present:=value.%s.Get();present{", fieldName)
		}
		fmt.Fprintf(&encode, "encoded,err:=encode%s(item,state.Child(%s));if err!=nil{return types.Value{},err};fields[%s]=encoded}\n", child.Codec, strconv.Quote(property.Name), strconv.Quote(property.Name))
		fmt.Fprintf(&decode, "if item,present:=value.Get(%s);present{decoded,err:=decode%s(item,state.Child(%s));if err!=nil{return zero,err};", strconv.Quote(property.Name), child.Codec, strconv.Quote(property.Name))
		if required[property.Name] {
			fmt.Fprintf(&decode, "result.%s=decoded}", fieldName)
		} else {
			fmt.Fprintf(&decode, "result.%s=types.Some(decoded)}", fieldName)
		}
		if required[property.Name] {
			fmt.Fprintf(&decode, "else{return zero,fmt.Errorf(\"missing required field %%s\",%s)}", strconv.Quote(property.Name))
		}
		fmt.Fprintln(&decode)
	}
	additional := true
	if literal, ok := node.Get("additionalProperties").(schemaplan.Literal); ok && literal.Data == false && node.Get("patternProperties") == nil {
		additional = false
	}
	if additional {
		additionalType := modelType{Type: "types.Value"}
		if planner.media {
			planned, err := planner.shape(name+"AdditionalProperty", &schemaplan.Node{}, false)
			if err != nil {
				return modelType{}, err
			}
			additionalType = planned
		}
		if child, ok := node.Get("additionalProperties").(schemaplan.Child); ok && node.Get("patternProperties") == nil {
			planned, err := planner.shape(name+"AdditionalProperty", child.Node, false)
			if err != nil {
				return modelType{}, err
			}
			additionalType = planned
		}
		fmt.Fprintf(&declaration, "AdditionalProperties map[string]%s\n", additionalType.Type)
		fmt.Fprintln(&encode, "for key,item:=range value.AdditionalProperties {")
		fmt.Fprintln(&encode, "normalized,err:=wire.CanonicalText(key);if err!=nil{return types.Value{},err};key=normalized;if _,exists:=fields[key];exists{return types.Value{},fmt.Errorf(\"additional property collides with another field\")}")
		if len(known) != 0 {
			fmt.Fprintf(&encode, "switch key {case %s:return types.Value{},fmt.Errorf(\"additional property collides with a declared field: %%s\",key)}\n", joinComma(known))
		}
		if additionalType.Codec != "" {
			fmt.Fprintf(&encode, "encoded,err:=encode%s(item,state.Child(key));if err!=nil{return types.Value{},err};fields[key]=encoded}\n", additionalType.Codec)
		} else {
			fmt.Fprintln(&encode, "fields[key]=item}")
		}
		fmt.Fprintf(&decode, "result.AdditionalProperties=make(map[string]%s);for _,key:=range value.Keys(){\n", additionalType.Type)
		if len(known) != 0 {
			fmt.Fprintf(&decode, "switch key{case %s:continue}\n", joinComma(known))
		}
		if additionalType.Codec != "" {
			fmt.Fprintf(&decode, "item,_:=value.Get(key);decoded,err:=decode%s(item,state.Child(key));if err!=nil{return zero,err};result.AdditionalProperties[key]=decoded}\n", additionalType.Codec)
		} else {
			fmt.Fprintln(&decode, "item,_:=value.Get(key);result.AdditionalProperties[key]=item}")
		}
	}
	fmt.Fprintln(&declaration, "}")
	fmt.Fprintln(&encode, "return wire.CheckedRecord(fields)")
	fmt.Fprintln(&decode, "return result,nil")
	planner.plans = append(planner.plans, modelPlan{Name: name, Type: declaration.String(), Encode: encode.String(), Decode: decode.String(), Named: true})
	return modelType{Type: name, Codec: name}, nil
}

func joinComma(values []string) string {
	var result bytes.Buffer
	for index, value := range values {
		if index != 0 {
			result.WriteByte(',')
		}
		result.WriteString(value)
	}
	return result.String()
}

func (planner *modelPlanner) reaches(source, target string) bool {
	seen := make(map[string]bool)
	var visit func(string) bool
	visit = func(current string) bool {
		if current == target {
			return true
		}
		if seen[current] {
			return false
		}
		seen[current] = true
		for _, next := range planner.referenceGraph[current] {
			if visit(next) {
				return true
			}
		}
		return false
	}
	return visit(source)
}

func (planner *modelPlanner) aliasCycle(source string) bool {
	seen := make(map[string]bool)
	for {
		if seen[source] {
			return true
		}
		seen[source] = true
		node := planner.referenceNodes[source]
		if node == nil {
			return false
		}
		reference, ok := node.Get("reference").(schemaplan.Literal)
		if !ok {
			return false
		}
		source = reference.Data.(string)
	}
}
