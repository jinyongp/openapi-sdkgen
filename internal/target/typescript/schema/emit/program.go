package emit

import (
	"fmt"
	"openapi-sdkgen/internal/target/typescript/schema/plan"
	"strings"
)

type ProgramOptions struct {
	Nodes          []*plan.Node
	Views          bool
	Annotations    bool
	Literal        func(any) (string, error)
	TypesPath      string
	StatePath      string
	MappingPath    string
	SupportPath    string
	PropertiesPath string
	DerivedPath    string
}

// ProgramPolicy removes execution policies that cannot affect this contract's
// emitted algorithm, so client/server and primitive projections can share it.
func ProgramPolicy(root *plan.Node, options ProgramOptions) ProgramOptions {
	annotations, views := false, false
	nodes := options.Nodes
	if nodes == nil {
		walkProgramNodes(root, func(node *plan.Node) { nodes = append(nodes, node) })
	}
	for _, node := range nodes {
		views = views || len(programViews(node)) > 0
		if referenceOnly(node) {
			continue
		}
		annotations = annotations || has(node, "reference", "dynamicReference", "allOf", "oneOf", "anyOf", "if", "dependentSchemas") || typeAllows(node, "object") && (has(node, "properties", "patternProperties", "unevaluatedProperties") || schemaChild(node, "additionalProperties")) || typeAllows(node, "array") && has(node, "items", "prefixItems", "contains", "unevaluatedItems")
	}
	options.Annotations = options.Annotations && annotations
	options.Views = options.Views && views
	return options
}

// ProgramNodes gives every nested contract a stable slot in the root program module.
func ProgramNodes(root *plan.Node) []*plan.Node {
	nodes, _ := ProgramSlots(root)
	return nodes
}

func programNodes(root *plan.Node, options ProgramOptions) []*plan.Node {
	if options.Nodes != nil {
		return options.Nodes
	}
	return ProgramNodes(root)
}

// VisitProgramNodes visits descriptor nodes without rebuilding recursive identities.
func VisitProgramNodes(root *plan.Node, consume func(*plan.Node)) {
	walkProgramNodes(root, consume)
}

func ProgramSlots(root *plan.Node) ([]*plan.Node, map[*plan.Node]int) {
	var nodes []*plan.Node
	slots := make(map[*plan.Node]int)
	identities := make(map[string]int)
	var visit func(*plan.Node)
	visit = func(node *plan.Node) {
		identity := ProgramIdentity(node)
		index, exists := identities[identity]
		if !exists {
			index = len(nodes)
			nodes = append(nodes, node)
			identities[identity] = index
		}
		slots[node] = index
	}
	walkProgramNodes(root, visit)
	return nodes, slots
}

func walkProgramNodes(root *plan.Node, consume func(*plan.Node)) {
	var visit func(*plan.Node)
	visit = func(node *plan.Node) {
		consume(node)
		for _, field := range node.Fields {
			switch value := field.Value.(type) {
			case plan.Child:
				visit(value.Node)
			case plan.Children:
				for _, child := range value.Nodes {
					visit(child)
				}
			case plan.Properties:
				for _, entry := range value.Entries {
					visit(entry.Schema)
				}
			case plan.SchemaMap:
				for _, entry := range value.Entries {
					visit(entry.Schema)
				}
			case plan.DynamicReference:
				visit(value.Fallback)
			case plan.Discriminator:
				for _, entry := range value.Mapping {
					visit(entry.Schema)
				}
				if value.Default != nil {
					visit(value.Default)
				}
			}
		}
	}
	visit(root)
}

type programWriter struct {
	strings.Builder
	options ProgramOptions
	err     error
}

func (writer *programWriter) line(format string, args ...any) {
	fmt.Fprintf(&writer.Builder, format+"\n", args...)
}
func (writer *programWriter) literal(value any) string {
	result, err := writer.options.Literal(value)
	if err != nil {
		writer.err = err
		return "undefined"
	}
	return result
}
func (writer *programWriter) child(expression string, value string) string {
	return "_context.execution.validate(" + value + ", " + expression + ", _components, _direction, _options, scope, _context)"
}
func (writer *programWriter) transform(expression string, value string) string {
	return "_context.execution.transform(" + value + ", " + expression + ", _components, _direction, _options, scope, _context)"
}
func has(node *plan.Node, names ...string) bool {
	for _, name := range names {
		if node.Get(name) != nil {
			return true
		}
	}
	return false
}

const programArguments = "_value: unknown, _schema: WireSchema, _components: WireSchemas, _direction: 'encode' | 'decode', _options: WireTransformOptions, _scope: DynamicScope, _context: ValidationContext, _ignore: boolean = _schema.ignoreContentMediaType === true"

// Programs emits direct node checks. Recursive calls use only the prepared executor port.
func Programs(root *plan.Node, options ProgramOptions) ([]byte, error) {
	writer := &programWriter{options: options}
	nodes := programNodes(root, options)
	flagNodes := append([]*plan.Node(nil), nodes...)
	needsMerge, needsMapping, needsRecord, needsEvaluation := false, false, false, false
	for _, node := range flagNodes {
		needsMerge = needsMerge || !referenceOnly(node) && has(node, "reference", "dynamicReference", "allOf", "oneOf", "anyOf", "if", "patternProperties")
		needsMapping = needsMapping || typeAllows(node, "object") && has(node, "patternProperties")
		boolean, ok := node.Get("boolean").(plan.Literal)
		if !referenceOnly(node) && (!ok || boolean.Data != false) {
			needsRecord = needsRecord || typeAllows(node, "object")
			needsEvaluation = true
		}
	}
	writer.line("import type { SchemaProgram, WireSchema, WireSchemas, WireTransformOptions, DynamicScope, ValidationContext, Evaluation } from %s", writer.literal(options.TypesPath))
	var stateImports []string
	if options.Annotations && referenceMerge(flagNodes) {
		stateImports = append(stateImports, "mergeEvaluation")
	}
	if needsEvaluation {
		stateImports = append(stateImports, "emptyEvaluation")
	}
	if len(stateImports) > 0 {
		writer.line("import { %s } from %s", strings.Join(stateImports, ", "), writer.literal(options.StatePath))
	}
	if needsMapping {
		writer.line("import { classifyWireProperties, mergeWireRepresentations } from %s", writer.literal(options.MappingPath))
	} else if needsMerge {
		writer.line("import { mergeWireRepresentations } from %s", writer.literal(options.MappingPath))
	}
	if hasObjectMapping(flagNodes) {
		if needsOwnProperties(flagNodes) || options.PropertiesPath == "" {
			writer.line("import { isRecord, defineOwnDataProperty } from %s", writer.literal(options.SupportPath))
		} else {
			writer.line("import { isRecord } from %s", writer.literal(options.SupportPath))
		}
	} else if needsRecord {
		writer.line("import { isRecord } from %s", writer.literal(options.SupportPath))
	}
	if options.PropertiesPath != "" {
		var imports []string
		if objectHas(flagNodes, "properties") {
			imports = append(imports, "validateProgramProperties")
			for _, node := range flagNodes {
				if typeAllows(node, "object") && has(node, "properties") && !has(node, "patternProperties") {
					imports = append(imports, "transformProgramProperties")
					break
				}
			}
		}
		if objectHas(flagNodes, "required") {
			imports = append(imports, "validateProgramRequired")
		}
		if len(imports) > 0 {
			writer.line("import { %s } from %s", strings.Join(imports, ", "), writer.literal(options.PropertiesPath))
		}
	}
	if options.Views {
		for _, node := range nodes {
			if referenceOnly(node) {
				writer.line("import { unconstrainedSchema } from %s", writer.literal(options.DerivedPath))
				break
			}
		}
	}
	for index, node := range nodes {
		var views []string
		if options.Views {
			variants := programViews(node)
			for _, kind := range []string{"local", "inherited", "alternatives"} {
				if variant := variants[kind]; variant != nil {
					expression := fmt.Sprintf("program%d", index)
					if referenceOnly(node) {
						expression = "unconstrainedSchema.program!"
					}
					views = append(views, kind+": "+expression)
				}
			}
		}
		writer.line("export const program%d: SchemaProgram = {", index)
		if len(views) > 0 {
			writer.line("get views(): NonNullable<SchemaProgram['views']> { return { %s } },", strings.Join(views, ", "))
		}
		writer.line("validate(%s): Evaluation {", programArguments)
		writer.validation(node)
		writer.line("}")
		if needsTransformation(node) {
			writer.line(", transform(%s): unknown {", programArguments)
			writer.transformation(node)
			writer.line("}")
		}
		writer.line("}")
	}
	if writer.err != nil {
		return nil, writer.err
	}
	return []byte(writer.String()), nil
}

func (writer *programWriter) validation(node *plan.Node) {
	if referenceOnly(node) {
		writer.line("const referenced: WireSchema | undefined = _schema.reference === undefined ? undefined : _components[_schema.reference]")
		writer.line("if (referenced === undefined) throw new TypeError(`missing generated schema reference ${_schema.reference}`)")
		writer.line("return _context.execution.validate(_value, referenced, _components, _direction, _options, _scope, _context, _ignore)")
		return
	}
	if value, ok := node.Get("boolean").(plan.Literal); ok && value.Data == false {
		writer.line("throw new TypeError('schema is false')")
		return
	}
	if writer.options.Annotations && (typeAllows(node, "array") || typeAllows(node, "object")) {
		writer.line("const evaluation: Evaluation = typeof _value !== 'object' || _value === null ? emptyEvaluation : { properties: new Set<string>(), indexes: new Set<number>() }")
	} else {
		writer.line("const evaluation: Evaluation = emptyEvaluation")
	}
	writer.line("if (_value === undefined) return evaluation")
	if has(node, "binaryInput") {
		nullable := ""
		if has(node, "binaryNullable") {
			nullable = " || _value === null"
		}
		writer.line("if (!(typeof _value === 'string' || _value instanceof Blob || _value instanceof ArrayBuffer || ArrayBuffer.isView(_value)%s)) throw new TypeError('expected binary body or text string')", nullable)
	}
	if has(node, "reference", "dynamicReference", "allOf", "anyOf", "oneOf", "not", "if", "contentSchema") || typeAllows(node, "array") && has(node, "contains", "uniqueItems", "items", "prefixItems", "unevaluatedItems") || typeAllows(node, "object") && (has(node, "properties", "patternProperties", "dependentRequired", "dependentSchemas", "propertyNames", "unevaluatedProperties") || schemaChild(node, "additionalProperties")) {
		writer.scope(node)
	}
	if has(node, "dynamicReference") {
		writer.line("const target: WireSchema | undefined = _context.handlers.dynamic!.resolve(_schema, scope)")
		if writer.options.Annotations {
			writer.line("if (target !== undefined) mergeEvaluation(evaluation, _context.execution.validate(_value, target, _components, _direction, _options, scope, _context, _ignore))")
		} else {
			writer.line("if (target !== undefined) _context.execution.validate(_value, target, _components, _direction, _options, scope, _context, _ignore)")
		}
	}
	if has(node, "reference") {
		writer.line("const referenced: WireSchema | undefined = _schema.reference === undefined ? undefined : _components[_schema.reference]")
		if writer.options.Annotations {
			writer.line("if (referenced !== undefined) mergeEvaluation(evaluation, _context.execution.validate(_value, referenced, _components, _direction, _options, scope, _context, _ignore))")
		} else {
			writer.line("if (referenced !== undefined) _context.execution.validate(_value, referenced, _components, _direction, _options, scope, _context, _ignore)")
		}
	}
	if value, ok := node.Get("types").(plan.Literal); ok {
		var tests []string
		for _, kind := range value.Data.([]string) {
			test := "true"
			switch kind {
			case "null":
				test = "_value === null"
			case "boolean", "string":
				test = "typeof _value === " + writer.literal(kind)
			case "number":
				test = "typeof _value === 'number' && Number.isFinite(_value)"
			case "integer":
				test = "typeof _value === 'number' && Number.isInteger(_value)"
			case "array":
				test = "Array.isArray(_value)"
			case "object":
				test = "isRecord(_value)"
			}
			tests = append(tests, "("+test+")")
		}
		writer.line("if (!(%s)) throw new TypeError(%s)", strings.Join(tests, " || "), writer.literal("expected "+strings.Join(value.Data.([]string), " | ")))
	}
	if has(node, "constValue", "enumValues") {
		writer.line("_context.handlers.literal!(_value, _schema)")
	}
	if has(node, "multipleOf", "maximum", "exclusiveMaximum", "minimum", "exclusiveMinimum") {
		writer.line("if (typeof _value === 'number') {")
		if has(node, "multipleOf") {
			writer.line("_context.handlers.multipleOf!(_value, _schema)")
		}
		for _, bound := range []struct{ name, operator, message string }{{"maximum", ">", "<="}, {"exclusiveMaximum", ">=", "<"}, {"minimum", "<", ">="}, {"exclusiveMinimum", "<=", ">"}} {
			if has(node, bound.name) {
				writer.line("if (_value %s _schema.%s!) throw new TypeError(`must be %s ${_schema.%s}`)", bound.operator, bound.name, bound.message, bound.name)
			}
		}
		writer.line("}")
	}
	if has(node, "minLength", "maxLength", "pattern", "formatAssertion") {
		writer.line("if (typeof _value === 'string') {")
		for _, bound := range []struct{ name, operator, message string }{{"minLength", "<", ">="}, {"maxLength", ">", "<="}} {
			if has(node, bound.name) {
				writer.line("if ([..._value].length %s _schema.%s!) throw new TypeError(`must have length %s ${_schema.%s}`)", bound.operator, bound.name, bound.message, bound.name)
			}
		}
		if has(node, "pattern") {
			writer.line("_context.handlers.stringPattern!(_value, _schema)")
		}
		if has(node, "formatAssertion") {
			writer.line("if (_context.handlers.format?.(_value, _schema.format!) === false) throw new TypeError(`must match format ${_schema.format}`)")
		}
		writer.line("}")
	}
	if has(node, "allOf", "oneOf", "anyOf", "not", "if") {
		writer.line("_context.handlers.composition!(_value, _schema, _components, _direction, _options, scope, _context, evaluation)")
	}
	if has(node, "contentSchema") {
		writer.line("if (!_ignore && typeof _value === 'string') %s", writer.child("_schema.contentSchema!", "_context.handlers.decodeContent!(_value, _schema, _components, _ignore)"))
	}
	if typeAllows(node, "array") {
		writer.line("if (Array.isArray(_value)) {")
		writer.line("for (let index: number = 0; index < _value.length; index++) { if (!Object.hasOwn(_value, index)) throw new TypeError('must not contain sparse items') }")
		for _, bound := range []struct{ name, operator, message string }{{"minItems", "<", ">="}, {"maxItems", ">", "<="}} {
			if has(node, bound.name) {
				writer.line("if (_value.length %s _schema.%s!) throw new TypeError(`must have items %s ${_schema.%s}`)", bound.operator, bound.name, bound.message, bound.name)
			}
		}
		if has(node, "uniqueItems") {
			writer.line("_context.handlers.arrayUnique!(_value, _schema, _components, _direction, _options, scope, _context, evaluation)")
		}
		if has(node, "contains") {
			writer.line("_context.handlers.arrayContains!(_value, _schema, _components, _direction, _options, scope, _context, evaluation)")
		}
		if has(node, "items", "prefixItems") {
			if writer.options.Annotations || has(node, "prefixItems") {
				writer.line("for (const [index, item] of _value.entries()) {")
			} else {
				writer.line("for (const item of _value) {")
			}
			writer.itemValidation(node)
			writer.line("}")
		}
		if has(node, "unevaluatedItems") {
			writer.line("_context.handlers.arrayAfter!(_value, _schema, _components, _direction, _options, scope, _context, evaluation)")
		}
		writer.line("return evaluation }")
	}
	if !typeAllows(node, "object") {
		writer.line("return evaluation")
		return
	}
	writer.line("if (!isRecord(_value)) return evaluation")
	for _, bound := range []struct{ name, operator, message string }{{"minProperties", "<", ">="}, {"maxProperties", ">", "<="}} {
		if has(node, bound.name) {
			writer.line("if (Object.keys(_value).length %s _schema.%s!) throw new TypeError(`must have properties %s ${_schema.%s}`)", bound.operator, bound.name, bound.message, bound.name)
		}
	}
	if has(node, "properties") {
		if writer.options.PropertiesPath != "" {
			annotation := ""
			if writer.options.Annotations {
				annotation = ", evaluation"
			}
			writer.line("validateProgramProperties(_value, _schema, _components, _direction, _options, scope, _context%s)", annotation)
		} else {
			writer.line("for (const [name, property] of Object.entries(_schema.properties!)) {")
			writer.line("const source: string = _direction === 'encode' ? property.property : name")
			annotation := ""
			if writer.options.Annotations {
				annotation = "; evaluation.properties.add(source)"
			}
			writer.line("if (Object.hasOwn(_value, source)) { try { %s%s } catch (cause: unknown) { throw new TypeError(`property ${name}: ${cause instanceof Error ? cause.message : 'invalid value'}`, { cause }) } }", writer.child("property.schema", "_value[source]"), annotation)
			writer.line("}")
		}
	}
	if has(node, "required") {
		if writer.options.PropertiesPath != "" {
			writer.line("validateProgramRequired(_value, _schema, _direction)")
		} else {
			writer.line("for (const name of _schema.required!) {")
			writer.line("const required: string = _direction === 'encode' ? (_schema.properties?.[name]?.property ?? name) : name")
			writer.line("if (!Object.hasOwn(_value, required) || _value[required] === undefined) throw new TypeError(`missing required property ${name}`)")
			writer.line("}")
		}
	}
	if has(node, "dependentRequired", "dependentSchemas") {
		writer.line("_context.handlers.dependencies!(_value, _schema, _components, _direction, _options, scope, _context, evaluation)")
	}
	if has(node, "patternProperties") {
		writer.line("for (const property of classifyWireProperties(_value, _schema, _direction, _context)) {")
		writer.line("for (const child of property.schemas) { %s }; if (property.schemas.length !== 0) evaluation.properties.add(property.sourceName)", writer.child("child", "_value[property.sourceName]"))
		if additional, ok := node.Get("additionalProperties").(plan.Literal); ok && additional.Data == false {
			writer.line("if (property.additional && _options.unknownProperties === 'reject') throw new TypeError(`unexpected property ${property.sourceName}`)")
		}
		writer.line("}")
	} else if has(node, "additionalProperties") {
		writer.line("for (const key of Object.keys(_value)) {")
		condition := "true"
		if has(node, "properties") {
			condition = "!Object.hasOwn(_schema.properties!, key)"
		}
		writer.line("if (%s) {", condition)
		if _, ok := node.Get("additionalProperties").(plan.Child); ok {
			annotation := ""
			if writer.options.Annotations {
				annotation = "; evaluation.properties.add(key)"
			}
			writer.line("%s%s", writer.child("_schema.additionalProperties as WireSchema", "_value[key]"), annotation)
		} else {
			writer.line("if (_options.unknownProperties === 'reject') throw new TypeError(`unexpected property ${key}`)")
		}
		writer.line("} }")
	}
	if has(node, "propertyNames") {
		writer.line("_context.handlers.propertyNames!(_value, _schema, _components, _direction, _options, scope, _context, evaluation)")
	}
	if has(node, "unevaluatedProperties") {
		writer.line("_context.handlers.objectAfter!(_value, _schema, _components, _direction, _options, scope, _context, evaluation)")
	}
	writer.line("return evaluation")
}

func (writer *programWriter) scope(node *plan.Node) {
	if has(node, "dynamicAnchor", "dynamicReference") {
		writer.line("const scope: DynamicScope = _context.handlers.dynamic!.extend(_scope, _schema)")
	} else {
		writer.line("const scope: DynamicScope = _scope")
	}
}
func (writer *programWriter) itemValidation(node *plan.Node) {
	annotation := ""
	if writer.options.Annotations {
		annotation = "; evaluation.indexes.add(index)"
	}
	if has(node, "prefixItems") {
		writer.line("const prefix: WireSchema | undefined = _schema.prefixItems![index]")
		writer.line("if (prefix !== undefined) { %s%s }", writer.child("prefix", "item"), annotation)
		if has(node, "items") {
			writer.line("else { %s%s }", writer.child("_schema.items!", "item"), annotation)
		}
	} else {
		writer.line("%s%s", writer.child("_schema.items!", "item"), annotation)
	}
}
func (writer *programWriter) transformation(node *plan.Node) {
	if referenceOnly(node) {
		writer.line("const referenced: WireSchema | undefined = _schema.reference === undefined ? undefined : _components[_schema.reference]")
		writer.line("if (referenced === undefined) throw new TypeError(`missing generated schema reference ${_schema.reference}`)")
		writer.line("return _context.execution.transform(_value, referenced, _components, _direction, _options, _scope, _context, _ignore)")
		return
	}
	if has(node, "reference", "dynamicReference", "allOf", "anyOf", "oneOf", "if") || typeAllows(node, "array") && has(node, "items", "prefixItems") || typeAllows(node, "object") && (has(node, "properties", "patternProperties") || schemaChild(node, "additionalProperties")) {
		writer.scope(node)
	}
	representations := has(node, "reference", "dynamicReference", "allOf", "anyOf", "oneOf", "if")
	if representations {
		writer.line("const representations: unknown[] = []")
	}
	if has(node, "dynamicReference") {
		writer.line("const target: WireSchema | undefined = _context.handlers.dynamic!.resolve(_schema, scope)")
		writer.line("if (target !== undefined) representations.push(_context.execution.transform(_value, target, _components, _direction, _options, scope, _context, _ignore))")
	}
	if has(node, "reference") {
		writer.line("const referenced: WireSchema | undefined = _schema.reference === undefined ? undefined : _components[_schema.reference]")
		writer.line("if (referenced !== undefined) representations.push(_context.execution.transform(_value, referenced, _components, _direction, _options, scope, _context, _ignore))")
	}
	writer.line("let transformed: unknown = _value")
	if typeAllows(node, "array") && has(node, "items", "prefixItems") {
		writer.line("if (Array.isArray(_value)) transformed = _value.map((item: unknown, _index: number): unknown => {")
		if has(node, "prefixItems") {
			writer.line("const prefix: WireSchema | undefined = _schema.prefixItems![_index]")
			writer.line("if (prefix !== undefined) return %s", writer.transform("prefix", "item"))
		}
		if has(node, "items") {
			writer.line("return %s", writer.transform("_schema.items!", "item"))
		} else {
			writer.line("return item")
		}
		writer.line("})")
	} else if typeAllows(node, "array") {
		writer.line("if (Array.isArray(_value)) transformed = _value.slice()")
	}
	if typeAllows(node, "object") && has(node, "properties", "patternProperties", "additionalProperties") {
		if !has(node, "patternProperties") {
			writer.staticObjectTransformation(node)
		} else {
			writer.line("if (isRecord(transformed)) {")
			writer.line("const source: Record<string, unknown> = transformed; const result: Record<string, unknown> = {}; let targets: Set<string> | undefined")
			writer.line("for (const [key, item] of Object.entries(source)) defineOwnDataProperty(result, key, item)")
			// Mapping preserves correlated pattern intersections and explicit destination ownership.
			writer.line("const properties: ReturnType<typeof classifyWireProperties> = classifyWireProperties(source, _schema, _direction, _context)")
			writer.line("for (const { sourceName, targetName } of properties) { if (sourceName !== targetName) { delete result[sourceName]; targets ??= new Set<string>(); targets.add(targetName) } }")
			writer.line("for (const property of properties) { const { sourceName, targetName }: (typeof properties)[number] = property; const item: unknown = mergeWireRepresentations(source[sourceName], property.schemas.map((child: WireSchema): unknown => %s), _context); if (sourceName !== targetName || !targets?.has(targetName)) defineOwnDataProperty(result, targetName, item) }", writer.transform("child", "source[sourceName]"))
			writer.line("if (targets !== undefined) { _context.mappedProperties ??= new WeakMap<object, ReadonlySet<string>>(); _context.mappedProperties.set(result, targets) }; transformed = result }")
		}
	}
	if representations {
		writer.line("representations.push(transformed)")
		if has(node, "allOf", "oneOf", "anyOf", "if") {
			writer.line("_context.handlers.transformComposition!(_value, _schema, _components, _direction, _options, scope, _context, representations)")
		}
		writer.line("return mergeWireRepresentations(_value, representations, _context)")
	} else {
		writer.line("return transformed")
	}
}

func hasNodes(nodes []*plan.Node, names ...string) bool {
	for _, node := range nodes {
		if has(node, names...) {
			return true
		}
	}
	return false
}

func schemaChild(node *plan.Node, name string) bool { _, ok := node.Get(name).(plan.Child); return ok }

func referenceOnly(node *plan.Node) bool { return len(node.Fields) == 1 && has(node, "reference") }
func hasObjectMapping(nodes []*plan.Node) bool {
	for _, node := range nodes {
		if typeAllows(node, "object") && has(node, "properties", "patternProperties", "additionalProperties") {
			return true
		}
	}
	return false
}

func objectHas(nodes []*plan.Node, name string) bool {
	for _, node := range nodes {
		if typeAllows(node, "object") && has(node, name) {
			return true
		}
	}
	return false
}

func needsOwnProperties(nodes []*plan.Node) bool {
	for _, node := range nodes {
		if typeAllows(node, "object") && (has(node, "patternProperties") || schemaChild(node, "additionalProperties") || has(node, "additionalProperties") && !has(node, "properties")) {
			return true
		}
	}
	return false
}
func referenceMerge(nodes []*plan.Node) bool {
	for _, node := range nodes {
		if !referenceOnly(node) && has(node, "reference", "dynamicReference") {
			return true
		}
	}
	return false
}

func ProgramDependencies(root *plan.Node, options ProgramOptions) []string {
	dependencies := []string{"wire-types.ts", "wire-state.ts"}
	nodes := programNodes(root, options)
	if options.Views {
		for _, node := range nodes {
			if referenceOnly(node) {
				dependencies = append(dependencies, "program-derived.ts")
				break
			}
		}
	}
	if options.PropertiesPath != "" && (objectHas(nodes, "properties") || objectHas(nodes, "required")) {
		dependencies = append(dependencies, "program-properties.ts")
	}
	if options.Annotations && referenceMerge(nodes) {
		dependencies = append(dependencies, "wire-state.ts")
	}
	for _, node := range nodes {
		if !referenceOnly(node) && has(node, "reference", "dynamicReference", "allOf", "oneOf", "anyOf", "if", "patternProperties") {
			dependencies = append(dependencies, "wire-object-mapping.ts")
			break
		}
	}
	for _, node := range nodes {
		if !referenceOnly(node) && typeAllows(node, "object") {
			dependencies = append(dependencies, "runtime-support.ts")
			break
		}
	}
	return dependencies
}

func typeAllows(node *plan.Node, wanted string) bool {
	if has(node, "binaryInput") {
		return wanted == "string"
	}
	value, exists := node.Get("types").(plan.Literal)
	if !exists {
		return true
	}
	for _, kind := range value.Data.([]string) {
		if kind == wanted {
			return true
		}
		switch kind {
		case "null", "boolean", "string", "number", "integer", "array", "object":
		default:
			return true
		}
	}
	return false
}

func needsTransformation(node *plan.Node) bool {
	return typeAllows(node, "array") || has(node, "reference", "dynamicReference", "allOf", "oneOf", "anyOf", "if") || typeAllows(node, "object") && has(node, "properties", "patternProperties", "additionalProperties")
}

func programViews(node *plan.Node) map[string]*plan.Node {
	result := make(map[string]*plan.Node)
	for _, kind := range []string{"local", "inherited", "alternatives"} {
		view := &plan.Node{}
		for _, field := range node.Fields {
			omit := field.Name == "reference" || field.Name == "allOf"
			if kind != "alternatives" {
				omit = omit || field.Name == "dynamicReference"
			}
			if kind != "inherited" {
				omit = omit || field.Name == "oneOf" || field.Name == "anyOf"
			}
			if kind == "local" {
				omit = omit || field.Name == "if" || field.Name == "then" || field.Name == "else"
			}
			if !omit {
				view.Fields = append(view.Fields, field)
			}
		}
		if len(view.Fields) != len(node.Fields) {
			result[kind] = view
		}
	}
	return result
}

// ProgramViews exposes the prepared schema projections used by media codecs.
func ProgramViews(node *plan.Node) map[string]*plan.Node { return programViews(node) }

func (writer *programWriter) staticObjectTransformation(node *plan.Node) {
	writer.line("if (isRecord(transformed)) {")
	if has(node, "properties") && writer.options.PropertiesPath != "" {
		writer.line("const source: Record<string, unknown> = transformed; const result: Record<string, unknown> = transformProgramProperties(source, _schema, _components, _direction, _options, scope, _context)")
	} else {
		writer.line("const source: Record<string, unknown> = transformed; const result: Record<string, unknown> = {}")
		writer.line("for (const [key, item] of Object.entries(source)) defineOwnDataProperty(result, key, item)")
		// Lowering preserves exact JSON property names in both projections. Generated
		// programs therefore need no runtime rename table or destination collision set.
		if has(node, "properties") {
			writer.line("for (const [name, property] of Object.entries(_schema.properties!)) {")
			writer.line("if (Object.hasOwn(source, name)) defineOwnDataProperty(result, name, %s)", writer.transform("property.schema", "source[name]"))
			writer.line("}")
		}
	}
	if schemaChild(node, "additionalProperties") {
		condition := "true"
		if has(node, "properties") {
			condition = "!Object.hasOwn(_schema.properties!, key)"
		}
		writer.line("for (const key of Object.keys(source)) { if (%s) defineOwnDataProperty(result, key, %s) }", condition, writer.transform("_schema.additionalProperties as WireSchema", "source[key]"))
	}
	writer.line("transformed = result }")
}
