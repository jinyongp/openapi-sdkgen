package generator

import "testing"

func TestCanonicalGoOptions(t *testing.T) {
	input := &GoOptions{ImportPath: "example.test/team/sdk"}
	value, err := CanonicalGoOptions(input)
	if err != nil || value.Package != "api" || input.Package != "" {
		t.Fatalf("canonical: %#v, %v", value, err)
	}
	for _, options := range []*GoOptions{nil, {}, {Package: "main", ImportPath: "example.test/sdk"}, {Package: "type", ImportPath: "example.test/sdk"}, {Package: "_", ImportPath: "example.test/sdk"}, {ImportPath: "../sdk"}, {ImportPath: "example.test/sdk/"}, {ImportPath: "example.test/sdk\n"}, {ImportPath: "example.test/vendor/sdk"}, {ImportPath: "https://example.test/sdk"}} {
		if _, err := CanonicalGoOptions(options); err == nil {
			t.Fatalf("accepted %#v", options)
		}
	}
	for _, packageName := range []string{"api", "client_v2", "고객"} {
		if _, err := CanonicalGoOptions(&GoOptions{Package: packageName, ImportPath: "example.test/sdk/v2"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, packageName := range []string{"API", "Client", "1client", "a-b"} {
		if _, err := CanonicalGoOptions(&GoOptions{Package: packageName, ImportPath: "example.test/sdk"}); err == nil {
			t.Errorf("accepted package %q", packageName)
		}
	}
	for _, path := range []string{"-example.test/sdk", "example.test/CON", "example.test/nul.txt", "example.test/COM1", "example.test/LPT9.txt", "example.test/x~1", "example.test/VENDOR/a", "example.test/foo.", "example.test/a//b"} {
		if _, err := CanonicalGoOptions(&GoOptions{ImportPath: path}); err == nil {
			t.Errorf("accepted nonportable import path %q", path)
		}
	}
	for _, path := range []string{"example.test/.internal", "example.test/~name", "example.test/COM0", "example.test/COM10", "example.test/a+b"} {
		if _, err := CanonicalGoOptions(&GoOptions{ImportPath: path}); err != nil {
			t.Errorf("rejected valid path %q: %v", path, err)
		}
	}
}
