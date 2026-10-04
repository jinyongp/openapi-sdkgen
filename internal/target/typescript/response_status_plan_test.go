package typescript

import (
	"reflect"
	"testing"

	"openapi-sdkgen/internal/compiler/ir"
)

func TestResponseStatusReachabilityPlan(t *testing.T) {
	response := func(status string, media ...string) ir.Response {
		value := ir.Response{Status: status}
		for _, name := range media {
			value.Content = append(value.Content, ir.MediaType{ContentType: name})
		}
		return value
	}
	for _, test := range []struct {
		name      string
		responses []ir.Response
		status    int
		media     string
		expected  []string
	}{
		{"exact excludes same media default", []ir.Response{response("200", "application/json"), response("default", "application/json")}, 200, "application/json", []string{"200"}},
		{"different media retains default", []ir.Response{response("200", "application/json"), response("default", "application/xml")}, 200, "application/xml", []string{"default"}},
		{"range excludes default", []ir.Response{response("2XX", "application/json"), response("default", "application/json")}, 202, "application/json", []string{"2XX"}},
		{"exact excludes range", []ir.Response{response("400", "application/json"), response("4XX", "application/json"), response("default", "application/json")}, 400, "application/json", []string{"400"}},
		{"bodyless distinct", []ir.Response{response("200"), response("default", "application/json")}, 200, "application/json", []string{"default"}},
		{"wide media covers narrow", []ir.Response{response("200", "application/*"), response("default", "application/json")}, 200, "application/json", nil},
		{"narrow media cannot remove wildcard", []ir.Response{response("200", "application/json"), response("default", "application/*")}, 200, "application/*", []string{"default"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var actual []string
			for _, branch := range responseStatusBranches(test.responses, test.status >= 200 && test.status < 300) {
				media := ""
				if branch.media != nil {
					media = branch.media.ContentType
				}
				if media != test.media {
					continue
				}
				for _, status := range branch.statuses {
					if status == test.status {
						actual = append(actual, branch.response.Status)
					}
				}
			}
			if !reflect.DeepEqual(actual, test.expected) {
				t.Fatalf("reachable statuses %v, expected %v", actual, test.expected)
			}
		})
	}
	for _, branch := range responseStatusBranches([]ir.Response{response("default", "application/json")}, true) {
		if len(branch.statuses) != 100 || branch.statuses[0] != 200 || branch.statuses[99] != 299 {
			t.Fatalf("success domain: %v", branch.statuses)
		}
	}
}
