// Command audit-go inventories final generated packages without rewriting them.
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
	"strconv"
	"strings"
)

type row struct {
	Path      string            `json:"path"`
	Package   string            `json:"package"`
	Bytes     int               `json:"bytes"`
	SHA       string            `json:"sha256"`
	Reasons   []string          `json:"reasons"`
	ImportKey string            `json:"importKey"`
	Types     []string          `json:"definedTypes"`
	Public    map[string]string `json:"publicSignatures"`
}

func inspect(root, path string, assembly bool) (row, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return row{}, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return row{}, err
	}
	sum := sha256.Sum256(b)
	r := row{Path: filepath.ToSlash(rel), Bytes: len(b), SHA: hex.EncodeToString(sum[:]), Reasons: []string{}, Public: map[string]string{}}
	name := filepath.Base(path)
	// Go ignores these names even when they contain syntactically valid source.
	if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
		r.Reasons = append(r.Reasons, "go_ignored_filename")
		return r, nil
	}
	if len(b) > 8<<20 {
		r.Reasons = append(r.Reasons, "parse_cap_8MiB")
		return r, nil
	}
	set := token.NewFileSet()
	f, err := parser.ParseFile(set, path, b, parser.ParseComments)
	if err != nil {
		return r, err
	}
	r.Package = f.Name.Name
	if assembly {
		r.Reasons = append(r.Reasons, "assembly_package")
	}
	for _, suffix := range strings.Fields("test aix android darwin dragonfly freebsd hurd illumos ios js linux nacl netbsd openbsd plan9 solaris wasip1 windows zos 386 amd64 amd64p32 arm armbe arm64 arm64be loong64 mips mipsle mips64 mipsle64 mips64le mips64p32 mips64p32le ppc ppc64 ppc64le riscv riscv64 s390 s390x sparc sparc64 wasm") {
		stem := strings.TrimSuffix(name, ".go")
		if strings.HasSuffix(stem, "_"+suffix) || strings.Contains(stem, "_"+suffix+"_") {
			r.Reasons = append(r.Reasons, "test_or_platform_file")
			break
		}
	}
	for _, group := range f.Comments {
		for _, c := range group.List {
			if strings.HasPrefix(c.Text, "//go:") || strings.HasPrefix(c.Text, "//line ") || strings.HasPrefix(c.Text, "/*line ") || strings.HasPrefix(c.Text, "//export ") || strings.Contains(c.Text, "#cgo") || strings.Contains(c.Text, "+build") {
				r.Reasons = append(r.Reasons, "directive")
			}
		}
	}
	imports := []string{}
	for _, im := range f.Imports {
		alias := ""
		if im.Name != nil {
			alias = im.Name.Name
		}
		value, err := strconv.Unquote(im.Path.Value)
		if err != nil {
			return r, err
		}
		if value == "C" || alias == "_" || alias == "." {
			r.Reasons = append(r.Reasons, "cgo_blank_dot_import")
		}
		imports = append(imports, alias+" "+im.Path.Value)
	}
	sort.Strings(imports)
	r.ImportKey = strings.Join(imports, "\n")
	signature := func(name string, node ast.Node) error {
		var text bytes.Buffer
		if err := format.Node(&text, set, node); err != nil {
			return err
		}
		r.Public[name] = text.String()
		return nil
	}
	for _, declaration := range f.Decls {
		switch d := declaration.(type) {
		case *ast.GenDecl:
			if d.Tok == token.VAR {
				r.Reasons = append(r.Reasons, "package_variable")
			}
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if !s.Assign.IsValid() {
						r.Types = append(r.Types, s.Name.Name)
					}
					if s.Name.IsExported() {
						if err := signature(s.Name.Name, s); err != nil {
							return r, err
						}
					}
				case *ast.ValueSpec:
					for _, n := range s.Names {
						if n.IsExported() {
							if err := signature(n.Name, s); err != nil {
								return r, err
							}
						}
					}
				}
			}
		case *ast.FuncDecl:
			if d.Name.Name == "init" && d.Recv == nil {
				r.Reasons = append(r.Reasons, "init_function")
			}
			if d.Name.IsExported() {
				// Signature retains receiver/method ownership and excludes bodies.
				copy := *d
				copy.Body = nil
				copy.Doc = nil
				key := d.Name.Name
				if d.Recv != nil {
					var recv bytes.Buffer
					if err := format.Node(&recv, set, d.Recv.List[0].Type); err != nil {
						return r, err
					}
					key = recv.String() + "." + key
				}
				if err := signature(key, &copy); err != nil {
					return r, err
				}
			}
		}
	}
	return r, nil
}

func main() {
	root := flag.String("root", "", "generated SDK root")
	output := flag.String("output", "", "fresh JSONL report")
	flag.Parse()
	if *root == "" || *output == "" {
		panic("root and output required")
	}
	file, err := os.OpenFile(*output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	assembly := map[string]bool{}
	paths := []string{}
	err = filepath.WalkDir(*root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != *root && (strings.HasPrefix(d.Name(), ".") || strings.HasPrefix(d.Name(), "_") || d.Name() == "testdata") {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			if strings.HasSuffix(path, ".s") || strings.HasSuffix(path, ".S") {
				assembly[filepath.Dir(path)] = true
			}
			if strings.HasSuffix(path, ".go") {
				paths = append(paths, path)
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	if len(paths) == 0 {
		panic("no Go sources in audit root")
	}
	groups := map[string]int{}
	reasons := map[string]int{}
	packages := map[string][2]int{}
	eligible, total, eligibleBytes := 0, 0, 0
	for _, path := range paths {
		r, err := inspect(*root, path, assembly[filepath.Dir(path)])
		if err != nil {
			panic(err)
		}
		total += r.Bytes
		key := filepath.Dir(r.Path) + "|" + r.Package
		p := packages[key]
		p[0]++
		p[1] += r.Bytes
		packages[key] = p
		if len(r.Reasons) == 0 {
			eligible++
			eligibleBytes += r.Bytes
			groups[key+"|"+r.ImportKey]++
		}
		for _, reason := range r.Reasons {
			reasons[reason]++
		}
		if err := encoder.Encode(r); err != nil {
			panic(err)
		}
	}
	mergeable := 0
	for _, count := range groups {
		if count > 1 {
			mergeable += count
		}
	}
	summary := map[string]any{"summary": true, "goFiles": len(paths), "bytes": total, "eligibleFiles": eligible, "eligibleBytes": eligibleBytes, "sameImportGroupFiles": mergeable, "packageCount": len(packages), "packages": packages, "excludedReasons": reasons, "scope": "audit only; exact import set, same final package; no packing or compile claim"}
	if err := encoder.Encode(summary); err != nil {
		panic(err)
	}
	delete(summary, "packages")
	fmt.Println(summary)
}
