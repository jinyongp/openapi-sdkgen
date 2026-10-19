package probes

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticVersusDynamicReachability(t *testing.T) {
	directory := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.test/linkprobe\n\ngo 1.27.0\n",
		"sdk/client.go": `package sdk
import ("encoding/xml"; "bytes")
type Client struct{}
func New() *Client { return &Client{} }
//go:noinline
func (*Client) JSON() string { return "json" }
//go:noinline
func xmlMarker() string { var b bytes.Buffer; _ = xml.EscapeText(&b, []byte("<xml>")); return b.String() }
//go:noinline
func (*Client) XML() string { return xmlMarker() }
func Dynamic() map[string]func() string { c:=New(); return map[string]func() string{"json":c.JSON,"xml":c.XML} }
`,
		"static/main.go":  "package main\nimport(\"example.test/linkprobe/sdk\"; \"os\")\nfunc main(){os.Stdout.WriteString(sdk.New().JSON())}\n",
		"dynamic/main.go": "package main\nimport(\"example.test/linkprobe/sdk\"; \"os\")\nfunc main(){key:=\"json\";if len(os.Args)>1{key=os.Args[1]};os.Stdout.WriteString(sdk.Dynamic()[key]())}\n",
	}
	for name, data := range files {
		path := filepath.Join(directory, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, mode := range []string{"static", "dynamic"} {
		binary := filepath.Join(directory, mode+"-client")
		command := exec.Command("go", "build", "-buildvcs=false", "-o", binary, "./"+mode)
		command.Dir = directory
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v: %s", mode, err, output)
		}
		output, err := exec.Command("go", "tool", "nm", binary).CombinedOutput()
		if err != nil {
			t.Fatalf("nm: %v: %s", err, output)
		}
		retained := strings.Contains(string(output), "example.test/linkprobe/sdk.xmlMarker")
		if retained != (mode == "dynamic") {
			t.Fatalf("%s: XML retention=%v", mode, retained)
		}
		info, err := os.Stat(binary)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: XML retained=%v binary bytes=%d", mode, retained, info.Size())
	}
}
