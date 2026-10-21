package golang

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path"
	"reflect"
	"strings"
	"testing"

	"openapi-sdkgen/internal/generator"
)

func packTestArtifact(t *testing.T, name, body string) generator.Artifact {
	t.Helper()
	data, err := format.Source([]byte(generatedGoMarker + "\n\npackage models\n" + body))
	if err != nil {
		t.Fatal(err)
	}
	return generator.Artifact{Path: name, Data: data}
}

func TestPackPreservesBindingsTypesAndInitialization(t *testing.T) {
	files := []generator.Artifact{
		packTestArtifact(t, "models/a.go", `import f "fmt"
// Node retains its documentation.
type Node struct { Value string `+"`json:\"value\"`"+` }
func (n Node) Render() string { return f.Sprint(n.Value) }
const ( First = iota; Second )
`),
		packTestArtifact(t, "models/b.go", `import f "fmt"
func Shadow(f interface{ Render() string }) string { return f.Render() }
func Other() string { return f.Sprint("other") }
const ( Third = iota; Fourth )
`),
		packTestArtifact(t, "models/alias.go", "type Alias = Node\n"),
		packTestArtifact(t, "models/alias2.go", "func New() Alias { return Alias{Value: \"hello\"} }\n"),
		packTestArtifact(t, "models/state_a.go", "var Order = mark(\"a\")\nfunc mark(s string) string { return s }\n"),
		packTestArtifact(t, "models/state_z.go", "var Last = Order + mark(\"z\")\nfunc init() { Last += \"!\" }\n"),
		{Path: "models/_ignored.go", Data: []byte(generatedGoMarker + "\ninvalid ignored source\n")},
	}
	var previous []generator.Artifact
	for repeat := 0; repeat < 2; repeat++ {
		collector := &artifactCollector{}
		packer := newArtifactPacker(collector, 4<<10, 8<<10)
		for _, file := range files {
			if err := packer.WriteArtifact(file); err != nil {
				t.Fatal(err)
			}
		}
		if err := packer.finish(); err != nil {
			t.Fatal(err)
		}
		if len(collector.artifacts) != 5 {
			t.Fatalf("safe groups were not combined: %d", len(collector.artifacts))
		}
		if repeat == 1 {
			if !reflect.DeepEqual(previous, collector.artifacts) {
				t.Fatal("packing is nondeterministic")
			}
			continue
		}
		previous = collector.artifacts
		var plans []filePlan
		for _, file := range collector.artifacts {
			if strings.Contains(file.Path, "packed_") {
				formatted, err := format.Source(file.Data)
				if err != nil || !bytes.Equal(formatted, file.Data) {
					t.Fatalf("packed source is not formatted: %s %v", file.Path, err)
				}
			}
			plans = append(plans, filePlan{Path: file.Path, Source: "pack-test", Prepared: string(file.Data)})
		}
		testGeneratedModule(t, generator.NewPlan("golang", sourcePlan{Files: plans}), map[string]string{"pack_test.go": `package consumer_test
import("reflect";"testing";"example.test/consumer/api/models")
func TestPackedContracts(t *testing.T) {
 n:=models.New(); if models.Shadow(n)!="hello" || models.Other()!="other" { t.Fatal("import/local binding changed") }
 if models.First!=0 || models.Second!=1 || models.Third!=0 || models.Fourth!=1 { t.Fatal("iota changed") }
 typ:=reflect.TypeOf(n); if typ.PkgPath()!="example.test/consumer/api/models" || typ.Name()!="Node" || typ.NumMethod()!=1 || typ.Field(0).Tag.Get("json")!="value" { t.Fatal(typ) }
 if models.Order!="a" || models.Last!="az!" { t.Fatal("initialization changed") }
}
`})
	}
}

func TestPackExclusionsAndExactImportGroups(t *testing.T) {
	for _, test := range []struct{ name, body string }{
		{"x_test.go", "type X int"}, {"x_linux.go", "type X int"}, {"_ignored.go", "type X int"},
		{"x.go", "var X = 1"}, {"x.go", "func init() {}"}, {"x.go", "//go:noinline\nfunc X() {}"},
		{"x.go", "//line input.go:4\nfunc X() {}"}, {"x.go", "import _ \"fmt\"\ntype X int"},
		{"x.go", "import . \"fmt\"\nfunc X() string {return Sprint(1)}"},
		{"x.go", "import \"C\"\ntype X int"},
	} {
		t.Run(test.name+test.body, func(t *testing.T) {
			artifact := packTestArtifact(t, "models/"+test.name, test.body)
			if _, _, _, ok := packableArtifact(artifact, 4<<10, false); ok {
				t.Fatal("unsafe file became eligible")
			}
		})
	}
	a := packTestArtifact(t, "models/a.go", "import f \"fmt\"\nfunc A() string{return f.Sprint(1)}")
	b := packTestArtifact(t, "models/b.go", "import fmt \"fmt\"\nfunc B() string{return fmt.Sprint(1)}")
	ka, _, _, oka := packableArtifact(a, 4<<10, false)
	kb, _, _, okb := packableArtifact(b, 4<<10, false)
	if !oka || !okb || ka == kb {
		t.Fatal("different import aliases shared a group")
	}
	b.Path = "other/a.go"
	b.Data = a.Data
	kb, _, _, _ = packableArtifact(b, 4<<10, false)
	if ka == kb {
		t.Fatal("different final packages shared a group")
	}
}

func TestPackBoundsFailureAndDuplicatePaths(t *testing.T) {
	collector := &artifactCollector{}
	packer := newArtifactPacker(collector, 512, 1024)
	for i := 0; i < 500; i++ {
		artifact := packTestArtifact(t, fmt.Sprintf("p%d/a%d.go", i%150, i), fmt.Sprintf("type X%d struct{ Field string }", i))
		if err := packer.WriteArtifact(artifact); err != nil {
			t.Fatal(err)
		}
	}
	large := packTestArtifact(t, "large.go", "type Large struct { "+strings.Repeat("/* padding */", 100)+" Value int }")
	if err := packer.WriteArtifact(large); err != nil {
		t.Fatal(err)
	}
	if err := packer.WriteArtifact(generator.Artifact{Path: "README.txt", Data: []byte("non-Go")}); err != nil {
		t.Fatal(err)
	}
	if err := packer.finish(); err != nil || packer.bytes != 0 || packer.peakBytes > 1024 || packer.peakGroups > packBufferGroups {
		t.Fatalf("unbounded buffers or incomplete flush: %+v, %v", packer, err)
	}
	for _, file := range collector.artifacts {
		if strings.Contains(file.Path, "packed_") && len(file.Data) > 512 {
			t.Fatal("combined artifact exceeds target", file.Path)
		}
	}
	failure := errors.New("sink failure")
	writes := 0
	packer = newArtifactPacker(generator.ArtifactSinkFunc(func(generator.Artifact) error { writes++; return failure }), 512, 1024)
	for _, name := range []string{"a.go", "b.go"} {
		if err := packer.WriteArtifact(packTestArtifact(t, name, "type "+strings.ToUpper(name[:1])+" int")); err != nil {
			t.Fatal(err)
		}
	}
	if !errors.Is(packer.finish(), failure) || !errors.Is(packer.WriteArtifact(large), failure) || writes != 1 {
		t.Fatal("sink failure did not stop emission")
	}
	packer = newArtifactPacker(&artifactCollector{}, 512, 1024)
	artifact := packTestArtifact(t, "duplicate.go", "type X int")
	if err := packer.WriteArtifact(artifact); err != nil {
		t.Fatal(err)
	}
	if err := packer.WriteArtifact(artifact); err == nil {
		t.Fatal("packing hid a duplicate producer path")
	}
}

func TestPackCollisionAndSensitivePackage(t *testing.T) {
	a := packTestArtifact(t, "models/a.go", "type A int")
	b := packTestArtifact(t, "models/b.go", "type B int")
	collector := &artifactCollector{}
	packer := newArtifactPacker(collector, 512, 1024)
	for _, file := range []generator.Artifact{a, b} {
		if err := packer.WriteArtifact(file); err != nil {
			t.Fatal(err)
		}
	}
	if err := packer.finish(); err != nil {
		t.Fatal(err)
	}
	packed := collector.artifacts[0].Path
	if err := packer.WriteArtifact(packTestArtifact(t, packed, "type C int")); err == nil {
		t.Fatal("new original path collided with an earlier combined path")
	}
	packer = newArtifactPacker(&artifactCollector{}, 512, 1024)
	for _, file := range []generator.Artifact{{Path: packed, Data: []byte("package models\ntype C int\n")}, a, b} {
		if err := packer.WriteArtifact(file); err != nil {
			t.Fatal(err)
		}
	}
	if err := packer.finish(); err == nil {
		t.Fatal("combined path collided with an earlier original path")
	}
	collector = &artifactCollector{}
	packer = newArtifactPacker(collector, 512, 1024)
	packer.unsafeDirectories = map[string]bool{"models": true}
	for _, file := range []generator.Artifact{a, b} {
		if err := packer.WriteArtifact(file); err != nil {
			t.Fatal(err)
		}
	}
	if err := packer.finish(); err != nil || len(collector.artifacts) != 2 || collector.artifacts[0].Path != a.Path || collector.artifacts[1].Path != b.Path {
		t.Fatal("assembly/embedding package was combined", err)
	}
}

func TestPackGroupLimitAndEmptyFlush(t *testing.T) {
	collector := &artifactCollector{}
	packer := newArtifactPacker(collector, 512, 1<<20)
	if err := packer.finish(); err != nil || len(collector.artifacts) != 0 {
		t.Fatal("empty pack emitted an artifact", err)
	}
	for i := 0; i < packBufferGroups+10; i++ {
		if err := packer.WriteArtifact(packTestArtifact(t, fmt.Sprintf("p%d/a.go", i), "type A int")); err != nil {
			t.Fatal(err)
		}
	}
	if packer.peakGroups != packBufferGroups || len(collector.artifacts) != 10 {
		t.Fatal("group cap did not flush oldest groups")
	}
	if err := packer.finish(); err != nil || len(collector.artifacts) != packBufferGroups+10 {
		t.Fatal("final singleton flush lost files", err)
	}
}

func TestPackFinalMarkerlessFacadesPreservesDefinedPackage(t *testing.T) {
	files := []partitionFile{
		{artifact: packTestArtifact(t, "models/a.go", "type A struct { Value int `json:\"value\"` }\nfunc(a A) Number() int { return a.Value }\n")},
		{artifact: packTestArtifact(t, "models/b.go", "type B int\n")},
		{artifact: packTestArtifact(t, "models/c.go", "type C string\n")},
	}
	baseline := &artifactCollector{}
	if err := partitionPackage(files, "example.test/consumer/api", "models", baseline, 4096); err != nil {
		t.Fatal(err)
	}
	wantPackage := ""
	var prepared []filePlan
	for _, file := range baseline.artifacts {
		node, err := parser.ParseFile(token.NewFileSet(), file.Path, file.Data, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range node.Decls {
			if declaration, ok := declaration.(*ast.GenDecl); ok {
				for _, spec := range declaration.Specs {
					if typ, ok := spec.(*ast.TypeSpec); ok && typ.Name.Name == "A" && !typ.Assign.IsValid() {
						wantPackage = "example.test/consumer/api/" + path.Dir(file.Path)
					}
				}
			}
		}
		prepared = append(prepared, filePlan{Path: file.Path, Source: "native-partition-output", Prepared: string(file.Data)})
	}
	if wantPackage == "" {
		t.Fatal("baseline definition missing")
	}
	plan := generator.NewPlan("golang", sourcePlan{ImportPath: "example.test/consumer/api", Files: prepared})
	packed, err := (Generator{}).Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	modelFiles := 0
	for _, file := range packed {
		if path.Dir(file.Path) == "models" {
			modelFiles++
		}
	}
	if modelFiles != 1 {
		t.Fatalf("native markerless facades bypassed packing: %d", modelFiles)
	}
	testGeneratedModule(t, plan, map[string]string{"facade_test.go": fmt.Sprintf(`package consumer_test
import("reflect";"testing";"example.test/consumer/api/models")
func TestFacadeIdentity(t *testing.T){
 value:=models.A{Value:3};typ:=reflect.TypeOf(value)
 if typ.PkgPath()!=%q || typ.Name()!="A" || typ.NumMethod()!=1 || typ.Field(0).Tag.Get("json")!="value" || value.Number()!=3 {t.Fatal(typ)}
 var _ models.B=2;var _ models.C="value"
}
`, wantPackage)})
	artifact := generator.Artifact{Path: "copyright.go", Data: []byte("// Copyright retained\npackage models\ntype Licensed int\n")}
	if _, _, _, ok := packableArtifact(artifact, 4096, true); ok {
		t.Fatal("native provenance discarded a header comment")
	}
}
