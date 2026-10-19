package typescript

import (
	"strconv"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
	httpplan "openapi-sdkgen/internal/http/plan"
)

type responseStatusBranch = httpplan.ResponseBranch

func reachableSuccessResponses(responses []ir.Response) []ir.Response {
	return httpplan.SuccessResponses(responses)
}
func responseStatusBranches(responses []ir.Response, success bool) []responseStatusBranch {
	return httpplan.Branches(responses, success)
}
func responseStatusRange(pattern string) (int, int, int) { return httpplan.StatusRange(pattern) }
func responseStatusScore(pattern string, status int) int {
	return httpplan.StatusScore(pattern, status)
}
func responseMediaRangeCovers(covering, candidate string) bool {
	return httpplan.MediaRangeCovers(covering, candidate)
}

func responseStatusUnion(statuses []int) string {
	// A range excluding more than half its members cannot be shorter than
	// these literals. Most operations declare only one or two exact statuses.
	if len(statuses) < 50 {
		values := make([]string, len(statuses))
		for index, status := range statuses {
			values[index] = strconv.Itoa(status)
		}
		if len(values) == 0 {
			return "never"
		}
		return strings.Join(values, " | ")
	}
	set := make(map[int]bool, len(statuses))
	for _, status := range statuses {
		set[status] = true
	}
	var values, complete []string
	for hundred := 1; hundred <= 5; hundred++ {
		var present, missing []string
		for status := hundred * 100; status < (hundred+1)*100; status++ {
			if set[status] {
				present = append(present, strconv.Itoa(status))
			} else {
				missing = append(missing, strconv.Itoa(status))
			}
		}
		if len(present) == 0 {
			continue
		}
		if len(missing) == 0 {
			complete = append(complete, strconv.Itoa(hundred))
			continue
		}
		literal := strings.Join(present, " | ")
		rangeType := "Exclude<HTTPStatusRange<" + strconv.Itoa(hundred) + ">, " + strings.Join(missing, " | ") + ">"
		if len(rangeType) < len(literal) {
			values = append(values, rangeType)
		} else {
			values = append(values, literal)
		}
	}
	if len(complete) > 0 {
		values = append(values, "HTTPStatusRange<"+strings.Join(complete, " | ")+">")
	}
	for _, status := range statuses {
		if status < 100 || status >= 600 {
			values = append(values, strconv.Itoa(status))
		}
	}
	if len(values) == 0 {
		return "never"
	}
	return strings.Join(values, " | ")
}

func successfulHTTPStatuses() []int {
	statuses := make([]int, 100)
	for index := range statuses {
		statuses[index] = 200 + index
	}
	return statuses
}

// Raw responses expose the concrete normalized header, unlike HTTP errors,
// which carry the selected declaration separately. Media ranges cannot be
// represented by their wildcard spelling as a runtime literal.
func rawResponseContentType(media string) string {
	normalized := strings.ToLower(strings.TrimSpace(strings.SplitN(media, ";", 2)[0]))
	if !strings.Contains(normalized, "*") {
		return quoteTS(normalized)
	}
	parts := strings.SplitN(normalized, "/", 2)
	if len(parts) != 2 || normalized == "*/*" {
		return "string"
	}
	pattern := strings.ReplaceAll(normalized, "*", "${string}")
	return "`" + pattern + "`"
}
