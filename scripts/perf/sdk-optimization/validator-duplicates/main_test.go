package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDuplicateValidatorIdentityAndPlacement(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"p1/a.go": `package p1
type validationVisit struct{ Program string }
func checkValidateA() any { return validationVisit{Program:"A"} }
func checkValidateB() any { return validationVisit{Program:"B"} }
func checkValidateSame() any { return validationVisit{Program:"A"} }
func ignoredCodec() int {return 1}
`,
		"p2/a.go": `package p2
type validationVisit struct{ Program string }
func checkValidateA() any { return validationVisit{Program:"A"} }
`,
		"p1/_ignored.go": "not valid Go",
	}
	for name, data := range files {
		file := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	report, err := inspect(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.Files != 2 || report.Validators != 4 || report.DuplicateGroups != 1 || report.ProgramIdentityGuards != 4 || report.RepeatedBodyBytes == 0 || len(report.SourceManifestSHA) != 64 {
		t.Fatalf("identity or placement was normalized away: %+v", report)
	}
	if report.Duplicates[0].Count != 2 || report.Duplicates[0].Package != "p1:p1" {
		t.Fatal(report.Duplicates)
	}
	if report.BoundProgramGroups != 1 || report.BoundProgramRepeatedBytes <= report.RepeatedBodyBytes || report.BoundProgramDuplicates[0].Count != 3 {
		t.Fatal("parameterized opportunity must retain distinct bindings separately from exact identity", report)
	}
}
