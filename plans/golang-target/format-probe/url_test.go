package probe

import (
	"encoding/json"
	"os/exec"
	"testing"

	whatwg "github.com/nlnwa/whatwg-url/url"
)

func TestNodeURLParity(t *testing.T) {
	inputs := []string{"", "relative/path", "http://example.test/%zz", "http://[::1]", "http://[::gg]", "http://0177.0.0.1", "http://0x7f.1", "https://example.test:65536", "http:\\\\example.test\\path", "http:relative", "https://例え.テスト/path", "https://☃.net", "https://💩.test", "https://example.test./", "https://-invalid.test", "https://example..test", "https://example.test/%00", "https://example.test:\\x", "ftp://user:pass@example.test:21/path", "file:///C:/path", "file://localhost/C|/path", "mailto:user@example.test", "data:text/plain,hi", "non-special:/path", "non-special://opaque%20host/", "https://example.test\n/path", "https://\u00ad.test", "https://\U0001e6c0.test"}
	data, _ := json.Marshal(inputs)
	script := `const inputs=JSON.parse(process.argv[1]);process.stdout.write(JSON.stringify(inputs.map(value=>{try{const u=new URL(value,"https://format.invalid/");return{ok:true,host:u.hostname,href:u.href}}catch{return{ok:false}}})))`
	output, err := exec.Command("node", "-e", script, string(data)).CombinedOutput()
	if err != nil {
		t.Fatalf("Node: %v %s", err, output)
	}
	var baseline []struct {
		OK   bool   `json:"ok"`
		Host string `json:"host"`
		Href string `json:"href"`
	}
	if err := json.Unmarshal(output, &baseline); err != nil {
		t.Fatal(err)
	}
	for index, input := range inputs {
		parsed, err := whatwg.ParseRef("https://format.invalid/", input)
		want := baseline[index]
		if (err == nil) != want.OK {
			t.Errorf("%q: Go %v, Node %v", input, err, want.OK)
			continue
		}
		if err == nil && (parsed.Hostname() != want.Host || parsed.Href(false) != want.Href) {
			t.Errorf("%q: Go %s %s; Node %s %s", input, parsed.Hostname(), parsed.Href(false), want.Host, want.Href)
		}
	}
	t.Logf("%d Node URL parse/hostname/serialization cases", len(inputs))
}
