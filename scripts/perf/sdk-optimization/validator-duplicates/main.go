// Inspect exact lowered-validator bodies in their final packages. Program IDs,
// binding names and media/direction literals are deliberately not normalized.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type group struct {
	Package, SignatureSHA, BodySHA string
	Count, BodyBytes               int
	Examples                       []string
}

type report struct {
	Scope, SourceManifestSHA                      string
	Files, Validators, BodyBytes                  int
	DuplicateGroups, RepeatedBodyBytes            int
	ProgramIdentityGuards                         int
	Duplicates                                    []group
	BoundProgramGroups, BoundProgramRepeatedBytes int
	BoundProgramScope                             string
	BoundProgramDuplicates                        []group
}

func inspect(root string) (report, error) {
	result := report{Scope: "exact body + signature + imports + final package; function name alone is omitted; no adoption/performance claim"}
	groups := make(map[string]*group)
	boundGroups := make(map[string]*group)
	manifest := sha256.New()
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") || strings.HasPrefix(entry.Name(), "_") || strings.HasPrefix(entry.Name(), ".") {
			return nil
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		sourceSHA := sha256.Sum256(data)
		fmt.Fprintf(manifest, "%s\x00%x\n", filepath.ToSlash(rel), sourceSHA)
		result.Files++
		set := token.NewFileSet()
		node, err := parser.ParseFile(set, file, data, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		imports := make([]string, 0, len(node.Imports))
		for _, imported := range node.Imports {
			alias := ""
			if imported.Name != nil {
				alias = imported.Name.Name
			}
			imports = append(imports, alias+" "+imported.Path.Value)
		}
		sort.Strings(imports)
		pkg := filepath.ToSlash(filepath.Dir(rel)) + ":" + node.Name.Name
		for _, declaration := range node.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || function.Body == nil || !strings.HasPrefix(strings.ToLower(function.Name.Name), "checkvalidate") {
				continue
			}
			var signature, body bytes.Buffer
			if err := format.Node(&signature, set, function.Type); err != nil {
				return err
			}
			if err := format.Node(&body, set, function.Body); err != nil {
				return err
			}
			signatureSHA := sha256.Sum256(signature.Bytes())
			bodySHA := sha256.Sum256(body.Bytes())
			key := pkg + "\x00" + strings.Join(imports, "\n") + "\x00" + string(signatureSHA[:]) + string(bodySHA[:])
			value := groups[key]
			if value == nil {
				value = &group{Package: pkg, SignatureSHA: hex.EncodeToString(signatureSHA[:]), BodySHA: hex.EncodeToString(bodySHA[:]), BodyBytes: body.Len()}
				groups[key] = value
			}
			value.Count++
			if len(value.Examples) < 4 {
				value.Examples = append(value.Examples, filepath.ToSlash(rel)+":"+function.Name.Name)
			}
			result.Validators++
			result.BodyBytes += body.Len()
			if bytes.Contains(body.Bytes(), []byte("validationVisit{")) {
				result.ProgramIdentityGuards++
			}
			// Parameterize only the retained visit.Program binding for a separate
			// opportunity count. This does not change source or adopt sharing.
			bindings := 0
			ast.Inspect(function.Body, func(n ast.Node) bool {
				literal, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				kind := ""
				switch value := literal.Type.(type) {
				case *ast.Ident:
					kind = value.Name
				case *ast.SelectorExpr:
					kind = value.Sel.Name
				}
				if !strings.HasSuffix(kind, "validationVisit") {
					return true
				}
				for _, element := range literal.Elts {
					field, ok := element.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					name, ok := field.Key.(*ast.Ident)
					if !ok || name.Name != "Program" {
						continue
					}
					value, ok := field.Value.(*ast.BasicLit)
					if !ok || value.Kind != token.STRING {
						continue
					}
					field.Value = ast.NewIdent("boundProgramID")
					bindings++
				}
				return true
			})
			if bindings == 1 {
				var normalized bytes.Buffer
				if err := format.Node(&normalized, set, function.Body); err != nil {
					return err
				}
				hash := sha256.Sum256(normalized.Bytes())
				key := pkg + "\x00" + strings.Join(imports, "\n") + "\x00" + string(signatureSHA[:]) + string(hash[:])
				value := boundGroups[key]
				if value == nil {
					value = &group{Package: pkg, SignatureSHA: hex.EncodeToString(signatureSHA[:]), BodySHA: hex.EncodeToString(hash[:]), BodyBytes: normalized.Len()}
					boundGroups[key] = value
				}
				value.Count++
				if len(value.Examples) < 4 {
					value.Examples = append(value.Examples, filepath.ToSlash(rel)+":"+function.Name.Name)
				}
			}
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	for _, value := range groups {
		if value.Count > 1 {
			result.DuplicateGroups++
			result.RepeatedBodyBytes += (value.Count - 1) * value.BodyBytes
			result.Duplicates = append(result.Duplicates, *value)
		}
	}
	sort.Slice(result.Duplicates, func(i, j int) bool {
		a, b := result.Duplicates[i], result.Duplicates[j]
		if a.Package != b.Package {
			return a.Package < b.Package
		}
		if a.SignatureSHA != b.SignatureSHA {
			return a.SignatureSHA < b.SignatureSHA
		}
		return a.BodySHA < b.BodySHA
	})
	result.SourceManifestSHA = hex.EncodeToString(manifest.Sum(nil))
	for _, value := range boundGroups {
		if value.Count > 1 {
			result.BoundProgramGroups++
			result.BoundProgramRepeatedBytes += (value.Count - 1) * value.BodyBytes
			result.BoundProgramDuplicates = append(result.BoundProgramDuplicates, *value)
		}
	}
	sort.Slice(result.BoundProgramDuplicates, func(i, j int) bool {
		a, b := result.BoundProgramDuplicates[i], result.BoundProgramDuplicates[j]
		if a.Package != b.Package {
			return a.Package < b.Package
		}
		if a.SignatureSHA != b.SignatureSHA {
			return a.SignatureSHA < b.SignatureSHA
		}
		return a.BodySHA < b.BodySHA
	})
	result.BoundProgramScope = "opportunity upper bound only; distinct visit.Program remains a runtime binding, requiring wrappers/parameters and original partition placement; not bytes saved"
	return result, nil
}

func main() {
	root := flag.String("root", "", "final schema/schemaimpl package root")
	output := flag.String("output", "", "fresh report file")
	flag.Parse()
	if *root == "" || *output == "" {
		panic("root/output required")
	}
	result, err := inspect(*root)
	if err != nil {
		panic(err)
	}
	file, err := os.OpenFile(*output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(result); err != nil {
		panic(err)
	}
	result.Duplicates = nil
	result.BoundProgramDuplicates = nil
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		panic(err)
	}
}
