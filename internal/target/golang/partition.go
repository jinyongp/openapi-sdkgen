package golang

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"openapi-sdkgen/internal/generator"
)

// Large generated packages exhaust the compiler's heap even with serial builds.
// Keep recursive dependencies together, then batch the dependency DAG into
// implementation packages. Public aliases and forwarding functions retain the
// consumer's import paths, distinct model identities and method sets.
const partitionThreshold = 8 << 20
const partitionBatch = 4 << 20

type partitionFile struct {
	artifact generator.Artifact
	shared   bool
	group    int
	edges    []int
}

type partitionSymbol struct {
	file  int
	name  string
	users []int
}

type partitionReference struct {
	path, name string
}

type packagePartition struct {
	root, directory, packageName string
	files                        []partitionFile
	symbols                      map[string]partitionSymbol
	fields, methods              map[string]string
	sharedTypes                  map[string]bool
	private                      bool
}

func emitPartitioned(files []partitionFile, root, directory string, sink generator.ArtifactSink, bindings map[string]partitionReference) error {
	size := 0
	for _, file := range files {
		size += len(file.artifact.Data)
	}
	if root == "" || size < partitionThreshold {
		for _, file := range files {
			if err := sink.WriteArtifact(file.artifact); err != nil {
				return err
			}
		}
		return nil
	}
	return partitionPackageBound(files, root, directory, sink, partitionBatch, bindings)
}

func partitionPackage(files []partitionFile, root, directory string, sink generator.ArtifactSink, batch int) error {
	return partitionPackageBound(files, root, directory, sink, batch, nil)
}

func partitionPackageBound(files []partitionFile, root, directory string, sink generator.ArtifactSink, batch int, bindings map[string]partitionReference) error {
	p := packagePartition{root: root, directory: directory, packageName: path.Base(directory), files: files, private: bindings != nil && directory == "internal/schema",
		symbols: make(map[string]partitionSymbol), fields: make(map[string]string), methods: make(map[string]string), sharedTypes: make(map[string]bool)}
	for index, file := range files {
		node, err := parser.ParseFile(token.NewFileSet(), file.artifact.Path, file.artifact.Data, 0)
		if err != nil {
			return err
		}
		for name := range node.Scope.Objects {
			p.symbols[name] = partitionSymbol{file: index, name: name}
		}
		if file.shared || p.packageName == "schema" {
			ast.Inspect(node, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.TypeSpec:
					if file.shared {
						p.sharedTypes[n.Name.Name] = true
					}
				case *ast.StructType:
					for _, field := range n.Fields.List {
						for _, name := range field.Names {
							if !name.IsExported() {
								p.fields[name.Name] = "SdkgenPrivate__" + name.Name
							}
						}
					}
				case *ast.FuncDecl:
					if file.shared && n.Recv != nil && !n.Name.IsExported() {
						p.methods[n.Name.Name] = "SdkgenPrivate__" + n.Name.Name
					}
				}
				return true
			})
		}
	}
	for index, file := range files {
		node, err := parser.ParseFile(token.NewFileSet(), file.artifact.Path, file.artifact.Data, 0)
		if err != nil {
			return err
		}
		ignored := partitionIgnored(node)
		edges := make(map[int]bool)
		uses := make(map[string]bool)
		ast.Inspect(node, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && !ignored[id] && partitionGlobal(node, id) {
				if symbol, exists := p.symbols[id.Name]; exists && symbol.file != index {
					edges[symbol.file] = true
					uses[id.Name] = true
				}
			}
			return true
		})
		for name := range uses {
			symbol := p.symbols[name]
			symbol.users = append(symbol.users, index)
			p.symbols[name] = symbol
		}
		for dependency := range edges {
			if file.shared && !files[dependency].shared {
				return fmt.Errorf("shared generated runtime %s depends on a model program", file.artifact.Path)
			}
			p.files[index].edges = append(p.files[index].edges, dependency)
		}
		sort.Ints(p.files[index].edges)
	}
	p.assignGroups(batch)
	used := make(map[string]bool, len(p.symbols))
	for name := range p.symbols {
		used[name] = true
	}
	for _, name := range sortedKeys(p.symbols) {
		symbol := p.symbols[name]
		if ast.IsExported(name) {
			continue
		}
		// Local codecs must stay private: exporting every helper inflates the
		// import data loaded by the public facade and every dependent package.
		for _, user := range symbol.users {
			if p.files[user].group != p.files[symbol.file].group {
				symbol.name = "SdkgenPrivate__" + name
				for used[symbol.name] {
					symbol.name += "_"
				}
				used[symbol.name] = true
				p.symbols[name] = symbol
				break
			}
		}
	}
	for index := range p.files {
		if err := p.emitFile(index, sink); err != nil {
			return fmt.Errorf("partition %s: %w", p.files[index].artifact.Path, err)
		}
		p.files[index].artifact.Data = nil
	}
	if bindings != nil {
		for name, symbol := range p.symbols {
			if ast.IsExported(name) {
				bindings[name] = partitionReference{path: p.importPath(p.files[symbol.file].group), name: symbol.name}
			}
		}
	}
	return nil
}

// Generated callers use implementation bindings directly instead of reloading
// duplicate facade export data. Internal schema facades are unnecessary once
// callers have direct bindings; public model facades retain their API.
func bindPartitionImports(artifact generator.Artifact, source string, bindings map[string]partitionReference) (generator.Artifact, error) {
	if !strings.HasSuffix(artifact.Path, ".go") || len(bindings) == 0 || !bytes.Contains(artifact.Data, []byte(strconv.Quote(source))) {
		return artifact, nil
	}
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, artifact.Path, artifact.Data, parser.ParseComments)
	if err != nil {
		return artifact, err
	}
	alias := ""
	for _, imported := range file.Imports {
		location, _ := strconv.Unquote(imported.Path.Value)
		if location == source {
			alias = path.Base(source)
			if imported.Name != nil {
				alias = imported.Name.Name
			}
		}
	}
	if alias == "" {
		return artifact, nil
	}
	used := make(map[string]bool)
	ast.Inspect(file, func(node ast.Node) bool {
		if id, ok := node.(*ast.Ident); ok {
			used[id.Name] = true
		}
		return true
	})
	imports := make(map[string]string)
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		qualifier, ok := selector.X.(*ast.Ident)
		if !ok || qualifier.Name != alias || qualifier.Obj != nil {
			return true
		}
		reference, exists := bindings[selector.Sel.Name]
		if !exists {
			return true
		}
		name := imports[reference.path]
		if name == "" {
			name = fmt.Sprintf("sdkgenSchema%d", len(imports))
			for used[name] {
				name += "_"
			}
			used[name] = true
			imports[reference.path] = name
		}
		qualifier.Name = name
		selector.Sel.Name = reference.name
		return true
	})
	for _, location := range sortedKeys(imports) {
		addPartitionImport(file, imports[location], location)
	}
	artifact.Data, err = formatPartitionFile(set, file)
	return artifact, err
}

func partitionGlobal(file *ast.File, id *ast.Ident) bool {
	return id.Obj == nil || file.Scope.Objects[id.Name] == id.Obj
}

// Selectors, declarations, struct fields and composite-literal keys do not
// refer to package-level symbols. Local bindings are handled by parser scopes.
func partitionIgnored(file *ast.File) map[*ast.Ident]bool {
	ignored := map[*ast.Ident]bool{file.Name: true}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			ignored[n.Sel] = true
		case *ast.Field:
			for _, id := range n.Names {
				ignored[id] = true
			}
		case *ast.KeyValueExpr:
			// Map keys may be package-level constants; only unresolved identifiers
			// are field labels in the generated composite literals.
			if id, ok := n.Key.(*ast.Ident); ok && id.Obj == nil {
				ignored[id] = true
			}
		case *ast.ImportSpec:
			if n.Name != nil {
				ignored[n.Name] = true
			}
		case *ast.FuncDecl:
			ignored[n.Name] = true
		case *ast.TypeSpec:
			ignored[n.Name] = true
		case *ast.ValueSpec:
			for _, id := range n.Names {
				ignored[id] = true
			}
		}
		return true
	})
	return ignored
}

// Tarjan visits dependencies first. Consecutive components can therefore share
// a package without introducing a cycle between the resulting packages.
func (p *packagePartition) assignGroups(batch int) {
	next, group, size := 1, 1, 0
	indices, low, active := make([]int, len(p.files)), make([]int, len(p.files)), make([]bool, len(p.files))
	var stack []int
	var visit func(int)
	visit = func(index int) {
		indices[index], low[index] = next, next
		next++
		stack, active[index] = append(stack, index), true
		for _, dependency := range p.files[index].edges {
			if p.files[dependency].shared {
				continue
			}
			if indices[dependency] == 0 {
				visit(dependency)
				low[index] = min(low[index], low[dependency])
			} else if active[dependency] {
				low[index] = min(low[index], indices[dependency])
			}
		}
		if low[index] != indices[index] {
			return
		}
		var component []int
		weight := 0
		for {
			member := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			active[member] = false
			component = append(component, member)
			weight += len(p.files[member].artifact.Data)
			if member == index {
				break
			}
		}
		if size != 0 && size+weight > batch {
			group++
			size = 0
		}
		for _, member := range component {
			p.files[member].group = group
		}
		size += weight
	}
	for index, file := range p.files {
		if !file.shared && indices[index] == 0 {
			visit(index)
		}
	}
}

func (p *packagePartition) importPath(group int) string {
	return fmt.Sprintf("%s/internal/%simpl/p%d", p.root, p.packageName, group)
}

func (p *packagePartition) emitFile(index int, sink generator.ArtifactSink) error {
	file := p.files[index]
	set := token.NewFileSet()
	node, err := parser.ParseFile(set, file.artifact.Path, file.artifact.Data, parser.ParseComments)
	if err != nil {
		return err
	}
	var facade []byte
	if !p.private {
		facade, err = p.facade(node, file.group)
		if err != nil {
			return err
		}
	}
	ignored := partitionIgnored(node)
	imports := make(map[int]string)
	qualify := func(symbol partitionSymbol) ast.Expr {
		group := p.files[symbol.file].group
		if group == file.group {
			return ast.NewIdent(symbol.name)
		}
		alias := fmt.Sprintf("sdkgenimpl%d", group)
		imports[group] = alias
		return &ast.SelectorExpr{X: ast.NewIdent(alias), Sel: ast.NewIdent(symbol.name)}
	}
	// Schema helpers are internal, including XML's own state and its nested
	// validation state. Export their fields consistently across implementations.
	// Public model backing fields and methods remain private to their owner.
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			state, isState := n.X.(*ast.Ident)
			if file.shared || p.packageName == "schema" || isState && state.Name == "state" {
				if name := p.fields[n.Sel.Name]; name != "" {
					n.Sel.Name = name
				} else if name := p.methods[n.Sel.Name]; name != "" {
					n.Sel.Name = name
				}
			}
		case *ast.CompositeLit:
			if name, ok := n.Type.(*ast.Ident); ok && (p.packageName == "schema" || p.sharedTypes[name.Name]) {
				for _, element := range n.Elts {
					if pair, ok := element.(*ast.KeyValueExpr); ok {
						if key, ok := pair.Key.(*ast.Ident); ok && p.fields[key.Name] != "" {
							key.Name = p.fields[key.Name]
						}
					}
				}
			}
		case *ast.StructType:
			if file.shared || p.packageName == "schema" {
				for _, field := range n.Fields.List {
					for _, id := range field.Names {
						if name := p.fields[id.Name]; name != "" {
							id.Name = name
						}
					}
				}
			}
		}
		return true
	})
	if err := transformPartitionAST(reflect.ValueOf(node), func(n ast.Node) ast.Node {
		if id, ok := n.(*ast.Ident); ok && !ignored[id] && partitionGlobal(node, id) {
			if symbol, exists := p.symbols[id.Name]; exists {
				return qualify(symbol)
			}
		}
		return n
	}); err != nil {
		return err
	}
	for _, declaration := range node.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			if declaration.Recv == nil {
				declaration.Name.Name = p.symbols[declaration.Name.Name].name
			} else if file.shared && p.methods[declaration.Name.Name] != "" {
				declaration.Name.Name = p.methods[declaration.Name.Name]
			}
		case *ast.GenDecl:
			for _, spec := range declaration.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					spec.Name.Name = p.symbols[spec.Name.Name].name
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						name.Name = p.symbols[name.Name].name
					}
				}
			}
		}
	}
	node.Name.Name = fmt.Sprintf("p%d", file.group)
	groups := make([]int, 0, len(imports))
	for group := range imports {
		groups = append(groups, group)
	}
	sort.Ints(groups)
	for _, group := range groups {
		addPartitionImport(node, imports[group], p.importPath(group))
	}
	data, err := formatPartitionFile(set, node)
	if err != nil {
		return err
	}
	implementation := fmt.Sprintf("internal/%simpl/p%d/%s", p.packageName, file.group, path.Base(file.artifact.Path))
	if err := sink.WriteArtifact(generator.Artifact{Path: implementation, Data: data}); err != nil {
		return err
	}
	if p.private {
		return nil
	}
	return sink.WriteArtifact(generator.Artifact{Path: file.artifact.Path, Data: facade})
}

func addPartitionImport(file *ast.File, alias, location string) {
	spec := &ast.ImportSpec{Name: ast.NewIdent(alias), Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(location)}}
	file.Decls = append([]ast.Decl{&ast.GenDecl{Tok: token.IMPORT, Specs: []ast.Spec{spec}}}, file.Decls...)
	file.Imports = append(file.Imports, spec)
}

func formatPartitionFile(set *token.FileSet, file *ast.File) ([]byte, error) {
	used := make(map[string]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		if selector, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := selector.X.(*ast.Ident); ok && id.Obj == nil {
				used[id.Name] = true
			}
		}
		return true
	})
	var declarations []ast.Decl
	for _, declaration := range file.Decls {
		if imports, ok := declaration.(*ast.GenDecl); ok && imports.Tok == token.IMPORT {
			var specs []ast.Spec
			for _, spec := range imports.Specs {
				imported := spec.(*ast.ImportSpec)
				location, _ := strconv.Unquote(imported.Path.Value)
				alias := path.Base(location)
				if imported.Name != nil {
					alias = imported.Name.Name
				}
				if used[alias] || alias == "_" || alias == "." {
					specs = append(specs, spec)
				}
			}
			imports.Specs = specs
			if len(specs) == 0 {
				continue
			}
		}
		declarations = append(declarations, declaration)
	}
	file.Decls = declarations
	var buffer bytes.Buffer
	if err := format.Node(&buffer, set, file); err != nil {
		return nil, err
	}
	return format.Source(buffer.Bytes())
}

// Reflection visits AST syntax fields only, never parser object/scope links.
// It allows replacing an identifier expression with a qualified selector while
// retaining all declaration, literal, comment and local-binding identities.
func transformPartitionAST(value reflect.Value, transform func(ast.Node) ast.Node) error {
	if !value.IsValid() || value.Kind() == reflect.Pointer && value.IsNil() || value.Kind() == reflect.Interface && value.IsNil() {
		return nil
	}
	if value.CanInterface() {
		if node, ok := value.Interface().(ast.Node); ok {
			replacement := transform(node)
			if replacement != node {
				candidate := reflect.ValueOf(replacement)
				if !value.CanSet() || !candidate.Type().AssignableTo(value.Type()) {
					return fmt.Errorf("cannot qualify generated identifier at %T", node)
				}
				value.Set(candidate)
				return nil
			}
		}
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		return transformPartitionAST(value.Elem(), transform)
	case reflect.Slice:
		for index := 0; index < value.Len(); index++ {
			if err := transformPartitionAST(value.Index(index), transform); err != nil {
				return err
			}
		}
	case reflect.Struct:
		for index := 0; index < value.NumField(); index++ {
			field := value.Type().Field(index).Name
			if field == "Obj" || field == "Scope" || field == "Unresolved" {
				continue
			}
			if err := transformPartitionAST(value.Field(index), transform); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *packagePartition) facade(original *ast.File, group int) ([]byte, error) {
	// Parse again: signatures retained by the facade must not be rewritten when
	// the implementation is qualified and its private runtime state is exported.
	var buffer bytes.Buffer
	if err := format.Node(&buffer, token.NewFileSet(), original); err != nil {
		return nil, err
	}
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "facade.go", buffer.Bytes(), parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var declarations []ast.Decl
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			if declaration.Recv != nil || !declaration.Name.IsExported() {
				continue
			}
			var arguments []ast.Expr
			parameter := 0
			for _, field := range declaration.Type.Params.List {
				if len(field.Names) == 0 {
					field.Names = []*ast.Ident{ast.NewIdent(fmt.Sprintf("argument%d", parameter))}
				}
				for _, name := range field.Names {
					if name.Name == "_" {
						name.Name = fmt.Sprintf("argument%d", parameter)
					}
					arguments = append(arguments, ast.NewIdent(name.Name))
					parameter++
				}
			}
			call := &ast.CallExpr{Fun: &ast.SelectorExpr{X: ast.NewIdent("sdkgenimpl"), Sel: ast.NewIdent(declaration.Name.Name)}, Args: arguments}
			if len(declaration.Type.Params.List) != 0 {
				if _, variadic := declaration.Type.Params.List[len(declaration.Type.Params.List)-1].Type.(*ast.Ellipsis); variadic {
					call.Ellipsis = 1
				}
			}
			var statement ast.Stmt = &ast.ExprStmt{X: call}
			if declaration.Type.Results != nil && len(declaration.Type.Results.List) != 0 {
				statement = &ast.ReturnStmt{Results: []ast.Expr{call}}
			}
			declaration.Body = &ast.BlockStmt{List: []ast.Stmt{statement}}
			declarations = append(declarations, declaration)
		case *ast.GenDecl:
			if declaration.Tok == token.IMPORT {
				declarations = append(declarations, declaration)
				continue
			}
			for _, spec := range declaration.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					if !spec.Name.IsExported() {
						continue
					}
					name := p.symbols[spec.Name.Name].name
					spec.Assign = 1
					spec.Type = &ast.SelectorExpr{X: ast.NewIdent("sdkgenimpl"), Sel: ast.NewIdent(name)}
					declarations = append(declarations, &ast.GenDecl{Doc: declaration.Doc, Tok: token.TYPE, Specs: []ast.Spec{spec}})
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						if !name.IsExported() {
							continue
						}
						if declaration.Tok != token.CONST {
							if p.packageName == "schema" {
								// Validator literal caches are implementation data.
								continue
							}
							return nil, fmt.Errorf("cannot alias mutable public variable %s", name.Name)
						}
						declarations = append(declarations, &ast.GenDecl{Doc: declaration.Doc, Tok: token.CONST, Specs: []ast.Spec{&ast.ValueSpec{Names: []*ast.Ident{name}, Values: []ast.Expr{&ast.SelectorExpr{X: ast.NewIdent("sdkgenimpl"), Sel: ast.NewIdent(name.Name)}}}}})
					}
				}
			}
		}
	}
	file.Decls = declarations
	// Detached body comments must not remain interleaved with forwarding code.
	file.Comments = nil
	addPartitionImport(file, "sdkgenimpl", p.importPath(group))
	return formatPartitionFile(set, file)
}
