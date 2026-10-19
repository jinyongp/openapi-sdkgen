package golang

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	schemaplan "openapi-sdkgen/internal/schema/plan"
)

// schemaProgram contains an already lowered, target-bound validation algorithm.
// No schema document is interpreted in the generated request path.
type schemaProgram struct {
	MediaType, ViewTarget string
	RejectNull            bool
	Owner                 string
	Name                  string
	Body                  string
	Literals              []schemaLiteral
	Patterns              []schemaPattern
	Public                bool
}
type schemaLiteral struct{ Name, JSON string }
type schemaPattern struct{ Name, Pattern, Program string }

type schemaPlanner struct {
	xmlRequests       map[*schemaplan.Node]string
	annotations       map[*schemaplan.Node]any
	views             map[string]string
	viewBindings      map[string]schemaProgram
	programMedia      map[string]string
	programProjection map[string]schemaplan.Projection
	programs          []schemaProgram
	reference         func(string, schemaplan.Projection) (string, error)
	needsRegex        bool
	compiledPatterns  map[string]schemaPattern
	nodes             map[string]*schemaplan.Node
	nodeNames         map[*schemaplan.Node]string
	sources           map[*schemaplan.Node]string
	formatRoots       map[string]bool
	helperRoots       map[string]bool
	memberMedia       map[*schemaplan.Node]string
	nativeMedia       map[string]bool
}

type schemaOccurrence struct {
	Source, MediaType string
	Members           map[string]schemaOccurrence
}

func (planner *schemaPlanner) prepare(name string, value any, projection schemaplan.Projection, legacy bool, occurrences ...schemaOccurrence) (string, error) {
	occurrence := schemaOccurrence{}
	if len(occurrences) != 0 {
		occurrence = occurrences[0]
	}
	if planner.annotations == nil {
		planner.annotations = make(map[*schemaplan.Node]any)
	}
	node, err := schemaplan.Lower(copySchemaValue(value), schemaplan.Options{Source: func(node *schemaplan.Node, value any) { planner.annotations[node] = value }, Projection: projection, LegacyNullable: legacy, FormatAssertion: schemaplan.RequiresFormatAssertion(value), ReferenceName: func(reference string) (string, error) { return planner.reference(reference, projection) }})
	if err != nil {
		return "", err
	}
	if planner.nodes == nil {
		planner.nodes = make(map[string]*schemaplan.Node)
	}
	planner.nodes[name] = node
	if planner.sources == nil {
		planner.sources = make(map[*schemaplan.Node]string)
	}
	source := name
	if occurrence.Source != "" {
		source = occurrence.Source
	}
	planner.recordSources(node, source)
	if properties, ok := node.Get("properties").(schemaplan.Properties); ok {
		for _, property := range properties.Entries {
			if member, present := occurrence.Members[property.Name]; present {
				if member.Source != "" {
					planner.recordSources(property.Schema, member.Source)
				}
				if member.MediaType != "" {
					if planner.memberMedia == nil {
						planner.memberMedia = make(map[*schemaplan.Node]string)
					}
					planner.memberMedia[property.Schema] = member.MediaType
				}
			}
		}
	}
	start := len(planner.programs)
	if _, err := planner.node(name, node); err != nil {
		return "", err
	}
	planner.programs[len(planner.programs)-1].Public = true
	planner.programs[len(planner.programs)-1].MediaType = occurrence.MediaType
	if planner.programMedia == nil {
		planner.programMedia = make(map[string]string)
	}
	planner.programMedia[name] = occurrence.MediaType
	if planner.programProjection == nil {
		planner.programProjection = make(map[string]schemaplan.Projection)
	}
	planner.programProjection[name] = projection
	for index := start; index < len(planner.programs); index++ {
		planner.programs[index].Owner = name
	}
	return name, nil
}

func pointerToken(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}

func (planner *schemaPlanner) recordSources(node *schemaplan.Node, path string) {
	planner.sources[node] = path
	for _, field := range node.Fields {
		base := path + "/" + field.Name
		switch value := field.Value.(type) {
		case schemaplan.Child:
			planner.recordSources(value.Node, base)
		case schemaplan.Children:
			for index, child := range value.Nodes {
				planner.recordSources(child, base+"/"+strconv.Itoa(index))
			}
		case schemaplan.Properties:
			for _, property := range value.Entries {
				planner.recordSources(property.Schema, base+"/"+pointerToken(property.Name))
			}
		case schemaplan.SchemaMap:
			for _, property := range value.Entries {
				planner.recordSources(property.Schema, base+"/"+pointerToken(property.Name))
			}
		case schemaplan.DynamicReference:
			planner.recordSources(value.Fallback, path+"/$dynamicRef")
		case schemaplan.Discriminator:
			for _, property := range value.Mapping {
				planner.recordSources(property.Schema, base+"/mapping/"+pointerToken(property.Name))
			}
			if value.Default != nil {
				planner.recordSources(value.Default, base+"/defaultMapping")
			}
		}
	}
}

func copySchemaValue(value any) any {
	switch typed := value.(type) {
	case schemaplan.BinaryInput:
		cloned, _ := copySchemaValue(typed.Schema).(map[string]any)
		typed.Schema = cloned
		return typed
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			result[key] = copySchemaValue(child)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = copySchemaValue(child)
		}
		return result
	case json.RawMessage:
		return append(json.RawMessage(nil), typed...)
	default:
		return value
	}
}

func (planner *schemaPlanner) node(name string, node *schemaplan.Node) (string, error) {
	if planner.nodeNames == nil {
		planner.nodeNames = make(map[*schemaplan.Node]string)
	}
	planner.nodeNames[node] = name
	var body bytes.Buffer
	if media := planner.memberMedia[node]; media != "" {
		fmt.Fprintf(&body, "mediaState:=*state;mediaState.MediaType=%q;mediaState.RootPath=path;state=&mediaState\n", media)
	}
	var literals []schemaLiteral
	var patterns []schemaPattern
	pattern := func(text string) (string, error) {
		prepared, err := planner.preparePattern(text)
		if err != nil {
			return "", err
		}
		patterns = append(patterns, prepared)
		planner.needsRegex = true
		return prepared.Name, nil
	}

	if node.Get("patternProperties") != nil {
		fmt.Fprintln(&body, "patternKeys:=make(map[string]bool)")
	}
	fail := func(keyword, condition string) {
		fmt.Fprintf(&body, "if %s { return result, Failure(%s,path) }\n", condition, strconv.Quote(keyword))
	}
	fields := append([]schemaplan.Field(nil), node.Fields...)
	order := func(field schemaplan.Field) int {
		if field.Name == "dynamicAnchor" {
			return -1
		}
		if field.Name == "unevaluatedProperties" || field.Name == "unevaluatedItems" {
			return 1
		}
		return 0
	}
	sort.SliceStable(fields, func(i, j int) bool { return order(fields[i]) < order(fields[j]) })
	for _, field := range fields {
		switch field.Name {
		case "reference":
			binding := field.Value.(schemaplan.Literal).Data.(string)
			fmt.Fprintf(&body, "{evaluated,err:=%s(ctx,value,path,state);if err!=nil{return result,err};Merge(&result,evaluated)}\n", binding)
		case "dynamicAnchor":
			anchor := field.Value.(schemaplan.Literal).Data.(string)
			fmt.Fprintf(&body, "state.scope=append(state.scope,DynamicBinding{Anchor:%s,Validate:check%s});defer func(){state.scope=state.scope[:len(state.scope)-1]}()\n", strconv.Quote(anchor), name)
		case "dynamicReference":
			dynamic := field.Value.(schemaplan.DynamicReference)
			fallback, err := planner.node(name+"Fallback", dynamic.Fallback)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&body, "{target:=Validator(%s);for _,binding:=range state.scope {if binding.Anchor==%s {target=binding.Validate;break}};evaluated,err:=target(ctx,value,path,state);if err!=nil{return result,err};Merge(&result,evaluated)}\n", fallback, strconv.Quote(dynamic.Anchor))
		case "pattern":
			binding, err := pattern(field.Value.(schemaplan.Literal).Data.(string))
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&body, "if text,ok:=value.String();ok {matched,err:=%s.Match(ctx,text);if err!=nil{return result,err};if !matched{return result,Failure(\"pattern\",path)}}\n", binding)
		case "patternProperties":
			for index, property := range field.Value.(schemaplan.SchemaMap).Entries {
				binding, err := pattern(property.Name)
				if err != nil {
					return "", err
				}
				child, err := planner.node(fmt.Sprintf("%sPatternProperty%d", name, index), property.Schema)
				if err != nil {
					return "", err
				}
				fmt.Fprintf(&body, "if value.Kind()==wire.Object {for _,key:=range value.Keys(){matched,err:=%s.Match(ctx,key);if err!=nil{return result,err};if matched {item,_:=value.Get(key);if _,err:=%s(ctx,item,ChildPath(path,key),state);err!=nil{return result,err};result.MarkKey(key);patternKeys[key]=true}}}\n", binding, child)
			}
		case "propertyNames":
			child, err := planner.node(name+"PropertyName", field.Value.(schemaplan.Child).Node)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&body, "if value.Kind()==wire.Object {nameState:=*state;nameState.Binaries=nil;nameState.OpaqueAncestors=nil;for _,key:=range value.Keys(){if _,err:=%s(ctx,wire.Text(key),ChildPath(path,key),&nameState);err!=nil{return result,err}}}\n", child)
		case "constValue", "enumValues":
			planner.requireHelper("wire-literal")
			values := []any{field.Value.(schemaplan.Literal).Data}
			if field.Name == "enumValues" {
				values = field.Value.(schemaplan.Literal).Data.([]any)
			}
			conditions := make([]string, 0, len(values))
			for _, value := range values {
				encoded, err := json.Marshal(value)
				if err != nil {
					return "", err
				}
				literalName := fmt.Sprintf("%sLiteral%d", name, len(literals))
				literals = append(literals, schemaLiteral{Name: literalName, JSON: string(encoded)})
				conditions = append(conditions, "value.Equal("+literalName+".Load("+strconv.Quote(string(encoded))+"))")
			}
			fail(field.Name, "state.OpaqueAncestors[path] || !("+strings.Join(conditions, " || ")+")")
		case "binaryInput":
			nullable := node.Get("binaryNullable") != nil
			condition := "!present && !text"
			if nullable {
				condition += " && value.Kind()!=wire.Null"
			}
			fmt.Fprintf(&body, "{_,present:=state.Binaries[path];_,text:=value.String();if %s{return result,Failure(\"binary input\",path)}}\n", condition)
		case "binaryNullable", "binaryContentType":
		case "boolean":
			if accepted, _ := field.Value.(schemaplan.Literal).Data.(bool); !accepted {
				fail("false", "true")
			}
		case "types":
			types := field.Value.(schemaplan.Literal).Data.([]string)
			condition := "false"
			for _, kind := range types {
				switch kind {
				case "null":
					condition += " || value.Kind()==wire.Null"
				case "boolean":
					condition += " || value.Kind()==wire.Boolean"
				case "string":
					condition += " || value.Kind()==wire.String"
				case "number":
					condition += " || value.Kind()==wire.Number"
				case "integer":
					planner.requireHelper("schema-integer")
					condition += " || IsInteger(value)"
				case "array":
					condition += " || value.Kind()==wire.Array"
				case "object":
					condition += " || value.Kind()==wire.Object"
				}
			}
			fail("type", "!("+condition+")")
		case "required":
			for _, key := range field.Value.(schemaplan.Literal).Data.([]string) {
				fmt.Fprintf(&body, "if value.Kind()==wire.Object { if _,present:=value.Get(%s);!present { return result,Failure(\"required\",ChildPath(path,%s)) } }\n", strconv.Quote(key), strconv.Quote(key))
			}
		case "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum":
			encoded, err := json.Marshal(field.Value.(schemaplan.Literal).Data)
			if err != nil {
				return "", err
			}
			operator := map[string]string{"minimum": "<", "maximum": ">", "exclusiveMinimum": "<=", "exclusiveMaximum": ">="}[field.Name]
			fmt.Fprintf(&body, "if number,ok:=value.Number();ok&&number %s %s { return result,Failure(%s,path) }\n", operator, encoded, strconv.Quote(field.Name))
		case "multipleOf":
			planner.requireHelper("schema-multiple")
			encoded, err := json.Marshal(field.Value.(schemaplan.Literal).Data)
			if err != nil {
				return "", err
			}
			fail(field.Name, fmt.Sprintf("!MultipleOf(value,%s)", encoded))
		case "minLength", "maxLength", "minItems", "maxItems", "minProperties", "maxProperties":
			encoded, err := json.Marshal(field.Value.(schemaplan.Literal).Data)
			if err != nil {
				return "", err
			}
			kind := "wire.String"
			if field.Name == "minItems" || field.Name == "maxItems" {
				kind = "wire.Array"
			}
			if field.Name == "minProperties" || field.Name == "maxProperties" {
				kind = "wire.Object"
			}
			operator := "<"
			if field.Name[:3] == "max" {
				operator = ">"
			}
			fail(field.Name, fmt.Sprintf("value.Kind()==%s && float64(value.Len()) %s %s", kind, operator, encoded))
		case "uniqueItems":
			planner.requireHelper("schema-unique")
			fmt.Fprintln(&body, "{unique,err:=UniqueMediaItems(ctx,value,path,state);if err!=nil{return result,err};if !unique{return result,Failure(\"uniqueItems\",path)}}")
		case "allOf", "oneOf", "anyOf":
			fmt.Fprintln(&body, "{")
			if field.Name != "allOf" {
				fmt.Fprintln(&body, "match:=func(ctx context.Context)(int,Evaluation,error){matches:=0;combined:=NewEvaluation()")
			}
			for index, nested := range field.Value.(schemaplan.Children).Nodes {
				child, err := planner.node(fmt.Sprintf("%s%s%d", name, exportedName(field.Name), index), nested)
				if err != nil {
					return "", err
				}
				if field.Name == "allOf" {
					fmt.Fprintf(&body, "{evaluated,err:=%s(ctx,value,path,state);if err!=nil{return result,err};Merge(&result,evaluated)}\n", child)
				} else {
					fmt.Fprintf(&body, "if evaluated,err:=%s(ctx,value,path,state);err==nil {matches++;Merge(&combined,evaluated)} else if !IsValidationError(err) {return 0,combined,err}\n", child)
				}
			}
			if field.Name != "allOf" {
				fmt.Fprintln(&body, "return matches,combined,nil};matches:=0;combined:=NewEvaluation();var matchError error;if IsResponseContext(ctx){matches,combined,matchError=match(strictContext(ctx));if matchError!=nil{return result,matchError}};if !IsResponseContext(ctx)||matches==0{matches,combined,matchError=match(ctx);if matchError!=nil{return result,matchError}}")
			}
			if field.Name == "oneOf" {
				fail("oneOf", "matches!=1")
			}
			if field.Name == "anyOf" {
				fail("anyOf", "matches==0")
			}
			if field.Name != "allOf" {
				fmt.Fprintln(&body, "Merge(&result,combined)")
			}
			fmt.Fprintln(&body, "}")
		case "not":
			child, err := planner.node(name+"Not", field.Value.(schemaplan.Child).Node)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&body, "if _,err:=ControlFlow(ctx,value,path,state,%s);err==nil {return result,Failure(\"not\",path)} else if !IsValidationError(err) {return result,err}\n", child)
		case "if":
			child, err := planner.node(name+"If", field.Value.(schemaplan.Child).Node)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&body, "{evaluated,conditionErr:=ControlFlow(ctx,value,path,state,%s);if conditionErr!=nil&&!IsValidationError(conditionErr) {return result,conditionErr};if conditionErr==nil {Merge(&result,evaluated)\n", child)
			if then, ok := node.Get("then").(schemaplan.Child); ok {
				child, err := planner.node(name+"Then", then.Node)
				if err != nil {
					return "", err
				}
				fmt.Fprintf(&body, "branch,err:=%s(ctx,value,path,state);if err!=nil{return result,err};Merge(&result,branch)\n", child)
			}
			fmt.Fprintln(&body, "} else {")
			if otherwise, ok := node.Get("else").(schemaplan.Child); ok {
				child, err := planner.node(name+"Else", otherwise.Node)
				if err != nil {
					return "", err
				}
				fmt.Fprintf(&body, "branch,err:=%s(ctx,value,path,state);if err!=nil{return result,err};Merge(&result,branch)\n", child)
			}
			fmt.Fprintln(&body, "}}")
		case "then", "else":
			// Only the conditional owns execution of its selected branch.
		case "additionalProperties", "unevaluatedProperties", "unevaluatedItems":
			condition := "!result.Keys[key]"
			if field.Name == "additionalProperties" {
				conditions := []string{"true"}
				if properties, ok := node.Get("properties").(schemaplan.Properties); ok {
					for _, property := range properties.Entries {
						conditions = append(conditions, "key!="+strconv.Quote(property.Name))
					}
				}
				condition = strings.Join(conditions, " && ")
				if node.Get("patternProperties") != nil {
					condition += " && !patternKeys[key]"
				}
			}
			if field.Name == "unevaluatedItems" {
				fmt.Fprintln(&body, "if value.Kind()==wire.Array { for index:=0;index<value.Len();index++ { if !result.Items[index] {item,_:=value.At(index)")
			} else {
				fmt.Fprintf(&body, "if value.Kind()==wire.Object { for _,key:=range value.Keys() { if %s {item,_:=value.Get(key)\n", condition)
			}
			if literal, ok := field.Value.(schemaplan.Literal); ok && literal.Data == false {
				if field.Name == "unevaluatedItems" {
					fmt.Fprintf(&body, "_=item;return result,Failure(%s,path)\n", strconv.Quote(field.Name))
				} else {
					fmt.Fprintf(&body, "_=item;if !IsResponseContext(ctx){return result,Failure(%s,path)}\n", strconv.Quote(field.Name))
				}
			} else {
				child, err := planner.node(name+exportedName(field.Name), field.Value.(schemaplan.Child).Node)
				if err != nil {
					return "", err
				}
				pathExpression := "ChildPath(path,key)"
				if field.Name == "unevaluatedItems" {
					pathExpression = "IndexPath(path,index)"
				}
				fmt.Fprintf(&body, "if _,err:=%s(ctx,item,%s,state);err!=nil{return result,err}\n", child, pathExpression)
				if field.Name == "unevaluatedItems" {
					fmt.Fprintln(&body, "result.MarkItem(index)")
				} else {
					fmt.Fprintln(&body, "result.MarkKey(key)")
				}
			}
			fmt.Fprintln(&body, "}}}")
		case "dependentRequired":
			dependencies := field.Value.(schemaplan.Literal).Data.(map[string][]string)
			for _, key := range sortedKeys(dependencies) {
				fmt.Fprintf(&body, "if value.Kind()==wire.Object {if _,present:=value.Get(%s);present {\n", strconv.Quote(key))
				for _, dependency := range dependencies[key] {
					fmt.Fprintf(&body, "if _,present:=value.Get(%s);!present{return result,Failure(\"dependentRequired\",ChildPath(path,%s))}\n", strconv.Quote(dependency), strconv.Quote(dependency))
				}
				fmt.Fprintln(&body, "}}")
			}
		case "dependentSchemas":
			for index, property := range field.Value.(schemaplan.SchemaMap).Entries {
				child, err := planner.node(fmt.Sprintf("%sDependent%d", name, index), property.Schema)
				if err != nil {
					return "", err
				}
				fmt.Fprintf(&body, "if value.Kind()==wire.Object {if _,present:=value.Get(%s);present {evaluated,err:=%s(ctx,value,path,state);if err!=nil{return result,err};Merge(&result,evaluated)}}\n", strconv.Quote(property.Name), child)
			}
		case "contains":
			child, err := planner.node(name+"Contains", field.Value.(schemaplan.Child).Node)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&body, "if value.Kind()==wire.Array {matches:=0;for index:=0;index<value.Len();index++ {item,_:=value.At(index);if _,err:=%s(ctx,item,IndexPath(path,index),state);err==nil {matches++;result.MarkItem(index)} else if !IsValidationError(err){return result,err}}\n", child)
			minimum := "1"
			if bound, ok := node.Get("minContains").(schemaplan.Literal); ok {
				encoded, _ := json.Marshal(bound.Data)
				minimum = string(encoded)
			}
			fail("contains", "float64(matches)<"+minimum)
			if bound, ok := node.Get("maxContains").(schemaplan.Literal); ok {
				encoded, _ := json.Marshal(bound.Data)
				fail("maxContains", "float64(matches)>"+string(encoded))
			}
			fmt.Fprintln(&body, "}")
		case "minContains", "maxContains":
		case "properties":
			for index, property := range field.Value.(schemaplan.Properties).Entries {
				child, err := planner.node(fmt.Sprintf("%sProperty%d", name, index), property.Schema)
				if err != nil {
					return "", err
				}
				fmt.Fprintf(&body, "if value.Kind()==wire.Object { if item,present:=value.Get(%s);present { if _,err:=%s(ctx,item,ChildPath(path,%s),state);err!=nil { return result,err };result.MarkKey(%s) } }\n", strconv.Quote(property.Name), child, strconv.Quote(property.Name), strconv.Quote(property.Name))
			}
		case "items":
			child, err := planner.node(name+"Item", field.Value.(schemaplan.Child).Node)
			if err != nil {
				return "", err
			}
			start := 0
			if prefix, ok := node.Get("prefixItems").(schemaplan.Children); ok {
				start = len(prefix.Nodes)
			}
			fmt.Fprintf(&body, "if value.Kind()==wire.Array { for index:=%d;index<value.Len();index++ {item,_:=value.At(index);if _,err:=%s(ctx,item,IndexPath(path,index),state);err!=nil { return result,err };result.MarkItem(index) } }\n", start, child)
		case "prefixItems":
			for index, nested := range field.Value.(schemaplan.Children).Nodes {
				child, err := planner.node(fmt.Sprintf("%sPrefix%d", name, index), nested)
				if err != nil {
					return "", err
				}
				fmt.Fprintf(&body, "if value.Kind()==wire.Array && value.Len()>%d {item,_:=value.At(%d);if _,err:=%s(ctx,item,IndexPath(path,%d),state);err!=nil { return result,err };result.MarkItem(%d) }\n", index, index, child, index, index)
			}
		case "formatAssertion":
		case "contentSchema":
			planner.requireHelper("schema-content")
			child, err := planner.node(name+"Content", field.Value.(schemaplan.Child).Node)
			if err != nil {
				return "", err
			}
			encoding, mediaType := "", ""
			if literal, ok := node.Get("contentEncoding").(schemaplan.Literal); ok {
				encoding, _ = literal.Data.(string)
			}
			if literal, ok := node.Get("contentMediaType").(schemaplan.Literal); ok {
				mediaType, _ = literal.Data.(string)
			}
			bare := bareMediaType(mediaType)
			if bare == "application/xml" || bare == "text/xml" || strings.HasSuffix(bare, "+xml") {
				planner.requireHelper("wire-xml")
				if planner.xmlRequests == nil {
					planner.xmlRequests = make(map[*schemaplan.Node]string)
				}
				contentNode := field.Value.(schemaplan.Child).Node
				binding := "decodeXML" + planner.nodeNames[contentNode]
				planner.xmlRequests[contentNode] = binding
				fmt.Fprintf(&body, "if text,ok:=value.String();ok&&!(path==state.RootPath&&ContentMediaConflict(state.MediaType,%s)){decoded,err:=DecodeContent(text,%s,\"\",path);if err!=nil{return result,err};text,_=decoded.String();node,err:=wire.ParseXML(ctx,text);if err!=nil{if ctx.Err()!=nil{return result,ctx.Err()};return result,Failure(\"contentMediaType\",path)};content,err:=%s(ctx,node,ChildPath(path,\"$content\"),&xmlState{validation:state});if err!=nil{return result,err};if _,err:=%s(ctx,content,ChildPath(path,\"$content\"),state);err!=nil{return result,err}}\n", strconv.Quote(mediaType), strconv.Quote(encoding), binding, child)
			} else {
				fmt.Fprintf(&body, "if text,ok:=value.String();ok&&!(path==state.RootPath&&ContentMediaConflict(state.MediaType,%s)){content,err:=DecodeContent(text,%s,%s,path);if err!=nil{return result,err};if _,err:=%s(ctx,content,ChildPath(path,\"$content\"),state);err!=nil{return result,err}}\n", strconv.Quote(mediaType), strconv.Quote(encoding), strconv.Quote(mediaType), child)
			}
		case "discriminator":
			// Dispatch metadata cannot bypass oneOf/anyOf validation. Prepare
			// mapped contracts for the model's explicit typed projections.
			data := field.Value.(schemaplan.Discriminator)
			for index, mapping := range data.Mapping {
				if _, err := planner.node(fmt.Sprintf("%sMapping%d", name, index), mapping.Schema); err != nil {
					return "", err
				}
			}
			if data.Default != nil {
				if _, err := planner.node(name+"DefaultMapping", data.Default); err != nil {
					return "", err
				}
			}
		case "format":
			code, err := planner.format(node, field.Value.(schemaplan.Literal).Data.(string), pattern)
			if err != nil {
				return "", err
			}
			body.WriteString(code)
		case "xml", "contentEncoding", "contentMediaType":
			// Annotations are retained by the common plan. Assertion and media
			// bindings are prepared separately when their capabilities are needed.
		default:
			return "", fmt.Errorf("Go schema keyword %q is not implemented yet", field.Name)
		}
	}
	planner.programs = append(planner.programs, schemaProgram{Name: name, Body: "kind:=value.Kind();if state.Binaries[path]{kind=wire.Kind(255)};_=kind;\n" + strings.ReplaceAll(body.String(), "value.Kind()", "kind"), Literals: literals, Patterns: patterns})
	return "check" + name, nil
}

func (planner *schemaPlanner) requireHelper(name string) {
	if planner.helperRoots == nil {
		planner.helperRoots = make(map[string]bool)
	}
	planner.helperRoots[name] = true
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
