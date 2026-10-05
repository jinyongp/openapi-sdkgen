package typescript

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	schemaemit "openapi-sdkgen/internal/target/typescript/schema/emit"
	schemaplan "openapi-sdkgen/internal/target/typescript/schema/plan"
	"sort"
	"strings"
)

type schemaRuntimeModule struct {
	path         string
	alias        string
	node         *schemaplan.Node
	source       []byte
	dependencies []string
}
type schemaProgramBinding struct {
	*schemaRuntimeModule
	programs    map[*schemaplan.Node]*schemaRuntimeModule
	descriptors map[wirePropertiesMode]string
	properties  map[wirePropertiesMode]bool
	imports     map[wirePropertiesMode]map[string]schemaProgramImport
}
type schemaRuntimePlan struct {
	views             bool
	kindComponents    map[string]map[projection]map[string]bool
	annotations       bool
	sealed            bool
	nodes             map[string]*schemaplan.Node
	contracts         map[string]*schemaplan.Node
	modules           map[string]*schemaRuntimeModule
	owners            map[*schemaplan.Node]*schemaProgramBinding
	descriptorKeys    map[*schemaplan.Node]string
	descriptorUses    map[string]int
	descriptorModules map[string]*schemaRuntimeModule
}
type schemaProgramImport struct {
	alias string
	path  string
	name  string
}

func newSchemaRuntimePlan() *schemaRuntimePlan {
	return &schemaRuntimePlan{nodes: make(map[string]*schemaplan.Node), contracts: make(map[string]*schemaplan.Node), modules: make(map[string]*schemaRuntimeModule), owners: make(map[*schemaplan.Node]*schemaProgramBinding), descriptorKeys: make(map[*schemaplan.Node]string), descriptorUses: make(map[string]int), descriptorModules: make(map[string]*schemaRuntimeModule)}
}

func schemaSemanticKey(value any, direction projection, format, legacy, ignore bool) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append([]byte(fmt.Sprintf("%s:%t:%t:%t:", direction, format, legacy, ignore)), data...))
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func (runtime *schemaRuntimePlan) lower(value any, direction projection, format, legacy, ignore bool, observer schemaplan.Observer) (*schemaplan.Node, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	flags := 0
	if format {
		flags |= 1
	}
	if legacy {
		flags |= 2
	}
	if ignore {
		flags |= 4
	}
	key := string(direction) + ":" + "01234567"[flags:flags+1] + ":" + string(data)
	if node, exists := runtime.nodes[key]; exists {
		return node, nil
	}
	if runtime.sealed {
		return nil, fmt.Errorf("unprepared schema program for %s projection", direction)
	}
	node, err := schemaplan.Lower(value, schemaplan.Options{Projection: schemaplan.Projection(direction), FormatAssertion: format, LegacyNullable: legacy, ReferenceName: componentSchemaReferenceName, Observer: observer})
	if err != nil {
		return nil, err
	}
	if ignore {
		node.Fields = append(node.Fields, schemaplan.Field{Name: "ignoreContentMediaType", Value: schemaplan.Literal{Data: true}})
	}
	policy := schemaemit.ProgramPolicy(node, schemaemit.ProgramOptions{Annotations: runtime.annotations, Views: runtime.views})
	semanticKey, err := schemaSemanticKey(node, projection("shared"), policy.Annotations, policy.Views, ignore)
	if err != nil {
		return nil, err
	}
	if canonical, exists := runtime.contracts[semanticKey]; exists {
		runtime.nodes[key] = canonical
		runtime.descriptorUses[runtime.descriptorKeys[canonical]]++
		return canonical, nil
	}
	runtime.nodes[key] = node
	runtime.contracts[semanticKey] = node
	nodes, slots := schemaemit.ProgramSlots(node)
	modules := make([]*schemaRuntimeModule, len(nodes))
	for index, child := range nodes {
		module, err := runtime.prepareProgram(child, policy)
		if err != nil {
			return nil, err
		}
		modules[index] = module
	}
	programs := make(map[*schemaplan.Node]*schemaRuntimeModule, len(slots))
	for child, slot := range slots {
		programs[child] = modules[slot]
		identity, err := schemaSemanticKey(child, projection("descriptor"), runtime.annotations, runtime.views, false)
		if err != nil {
			return nil, err
		}
		runtime.descriptorKeys[child] = identity
		runtime.descriptorUses[identity]++
	}
	runtime.owners[node] = &schemaProgramBinding{schemaRuntimeModule: programs[node], programs: programs, descriptors: make(map[wirePropertiesMode]string), properties: make(map[wirePropertiesMode]bool), imports: make(map[wirePropertiesMode]map[string]schemaProgramImport)}
	return node, nil
}

// Programs read each contract's data from its descriptor. Share identical emitted
// algorithms across all roots, retaining projection and execution policy in code.
func (runtime *schemaRuntimePlan) prepareProgram(node *schemaplan.Node, policy schemaemit.ProgramOptions) (*schemaRuntimeModule, error) {
	policy.Nodes = []*schemaplan.Node{node}
	policy = schemaemit.ProgramPolicy(node, policy)
	const modulePath = "internal/schema-programs/shared/schema.ts"
	from := func(target string) string {
		value, err := relativeModuleSpecifier(modulePath, target)
		if err != nil {
			panic(err)
		}
		return value
	}
	policy.PropertiesPath = from(runtimeTemplatePath("program-properties.ts"))
	source, err := schemaemit.Programs(node, schemaemit.ProgramOptions{Nodes: policy.Nodes, Views: policy.Views, Annotations: policy.Annotations, Literal: schemaDescriptorLiteral, TypesPath: from(runtimeTemplatePath("wire-types.ts")), StatePath: from(runtimeTemplatePath("wire-state.ts")), MappingPath: from(runtimeTemplatePath("wire-object-mapping.ts")), SupportPath: from(runtimeTemplatePath("runtime-support.ts")), PropertiesPath: policy.PropertiesPath, DerivedPath: from(runtimeTemplatePath("program-derived.ts"))})
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(source)
	algorithmKey := base64.RawURLEncoding.EncodeToString(sum[:])
	if module, exists := runtime.modules[algorithmKey]; exists {
		return module, nil
	}
	module := &schemaRuntimeModule{path: "internal/schema-programs/shared/schema_" + algorithmKey + ".ts", alias: "__sdkgen_P" + strings.ReplaceAll(algorithmKey, "-", "$"), node: node, source: source}
	runtime.modules[algorithmKey] = module
	module.dependencies = schemaemit.ProgramDependencies(node, policy)
	return module, nil
}

func (runtime *schemaRuntimePlan) moduleFor(node *schemaplan.Node) (*schemaProgramBinding, error) {
	if module, exists := runtime.owners[node]; exists {
		return module, nil
	}
	return nil, fmt.Errorf("unprepared schema program identity")
}

func (wire *wireRenderContext) programExpression(node *schemaplan.Node) (func(*schemaplan.Node) (string, error), error) {
	if wire.schemaPrograms == nil {
		return nil, nil
	}
	module, err := wire.schemaPrograms.moduleFor(node)
	if err != nil {
		return nil, err
	}
	if wire.programImports == nil {
		wire.programImports = make(map[string]schemaProgramImport)
	}
	for _, program := range module.programs {
		wire.programImports[program.path] = schemaProgramImport{alias: program.alias, path: program.path}
	}
	return func(child *schemaplan.Node) (string, error) {
		program, exists := module.programs[child]
		if !exists {
			return "", fmt.Errorf("unprepared schema program slot")
		}
		return program.alias + "_program0", nil
	}, nil
}

func (wire *wireRenderContext) programImportSource(artifact string) (string, error) {
	var result string
	paths := make([]string, 0, len(wire.programImports))
	for key := range wire.programImports {
		paths = append(paths, key)
	}
	sort.Strings(paths)
	for _, target := range paths {
		dependency := wire.programImports[target]
		specifier, err := relativeModuleSpecifier(artifact, target)
		if err != nil {
			return "", err
		}
		name := dependency.name
		if name == "" {
			name = "program0"
		}
		result += fmt.Sprintf("import { %s as %s_%s } from %s\n", name, dependency.alias, name, quoteTS(specifier))
	}
	return result, nil
}

// Preparation visits only emission-owned contracts. Emission cannot add a program.
func prepareSchemaRuntimePlan(source *sourcePlan) error {
	runtime := source.schemaPrograms
	if runtime == nil {
		runtime = newSchemaRuntimePlan()
		for _, execution := range source.executions {
			for _, feature := range execution.features {
				if strings.HasSuffix(string(feature), ".media.xml") || strings.HasSuffix(string(feature), ".media.open") || strings.HasSuffix(string(feature), ".media.multipart") || strings.Contains(string(feature), ".open-part-media") {
					runtime.views = true
				}
				switch string(feature) {
				case "media.xml", "media.open", "media.multipart", "schema.contentXML":
					runtime.views = true
				}
				switch string(feature) {
				case "schema.allOf", "schema.oneOf", "schema.anyOf", "schema.not", "schema.if", "schema.contains", "schema.dependentSchemas", "schema.unevaluatedProperties", "schema.unevaluatedItems", "schema.patternProperties":
					runtime.annotations = true
				}
			}
		}
	}
	source.schemaPrograms = runtime
	owners := []*sourcePlan{source}
	if source.root != nil {
		owners = append(owners, source.root)
	}
	for _, client := range source.clients {
		owners = append(owners, client.view)
	}
	for _, owner := range owners {
		if owner == nil || owner.modules == nil {
			continue
		}
		owner.schemaPrograms = runtime
		owner.modules.schemaPrograms = runtime
		for _, schema := range owner.modules.schemas {
			value := componentSchemaValue(source.document, schema.name)
			wire := newWireRenderContext(wirePropertiesLiteral)
			wire.semanticOnly = true
			wire.schemaPrograms = runtime
			if schema.inputWire {
				if _, err := wire.wireSchemaDescriptorForDocument(source.document, value, projectionInput); err != nil {
					return err
				}
			}
			if schema.outputWire {
				if _, err := wire.wireSchemaDescriptorForDocument(source.document, value, projectionOutput); err != nil {
					return err
				}
			}
		}
		for _, item := range owner.manifest.Operations {
			wire := newWireRenderContext(wirePropertiesConstructed)
			wire.semanticOnly = true
			wire.schemaPrograms = runtime
			if _, err := wire.operationDefinition(source.document, item.compiled, item); err != nil {
				return err
			}
		}
	}
	return runtime.freeze()
}

func (runtime *schemaRuntimePlan) freeze() error {
	for node, binding := range runtime.owners {
		for _, mode := range []wirePropertiesMode{wirePropertiesLiteral, wirePropertiesConstructed} {
			if mode == wirePropertiesConstructed && !binding.properties[wirePropertiesLiteral] {
				binding.descriptors[mode] = binding.descriptors[wirePropertiesLiteral]
				binding.imports[mode] = binding.imports[wirePropertiesLiteral]
				continue
			}
			wire := newWireRenderContext(mode)
			wire.schemaPrograms = runtime
			if _, err := wire.emitSchemaDescriptor(node); err != nil {
				return err
			}
		}
	}
	runtime.sealed = true
	return nil
}
