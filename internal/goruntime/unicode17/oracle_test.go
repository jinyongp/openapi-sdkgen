package unicode17

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"testing"
)

// Node is an optional development oracle; generated consumers need only Go.
// Official fixture tests remain mandatory when this supplementary check skips.
func TestNodeUnicodePropertyOracle(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("supplementary Node oracle unavailable")
	}
	version, err := exec.Command(node, "-p", "process.versions.unicode").Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(bytes.TrimSpace(version)) != "17.0" {
		t.Skipf("supplementary oracle Unicode %s differs from 17.0", bytes.TrimSpace(version))
	}
	type sample struct {
		Name    string `json:"name"`
		Points  []rune `json:"points"`
		Matches []bool `json:"matches"`
	}
	var cases []sample
	for _, alias := range propertyAliases {
		property := Property{index: alias.index + 1}
		ranges := property.AppendRanges(nil)
		s := sample{Name: alias.name}
		previous := rune(-1)
		for _, r := range ranges {
			if previous+1 < r.Lo {
				s.Points = append(s.Points, previous+1)
				s.Matches = append(s.Matches, false)
			}
			s.Points = append(s.Points, r.Lo, r.Hi)
			s.Matches = append(s.Matches, true, true)
			previous = r.Hi
		}
		if previous < 0x10ffff {
			s.Points = append(s.Points, previous+1)
			s.Matches = append(s.Matches, false)
		}
		cases = append(cases, s)
	}
	data, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(node, "-e", `const fs=require('node:fs');const cases=JSON.parse(fs.readFileSync(0,'utf8'));let n=0;const failures=[];for(const s of cases){let p;try{p=new RegExp('\\p{'+s.name+'}','u')}catch(e){failures.push({name:s.name,error:String(e)});continue}for(let i=0;i<s.points.length;i++){n++;if(p.test(String.fromCodePoint(s.points[i]))!==s.matches[i]&&failures.length<8)failures.push({name:s.name,point:s.points[i],expected:s.matches[i]})}}console.log(JSON.stringify({properties:cases.length,checks:n,failures}));if(failures.length)process.exitCode=1;`)
	command.Stdin = bytes.NewReader(data)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Unicode property oracle: %v %s", err, output)
	}
	t.Log(string(output))
}
