package typescript

import (
	"strconv"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
)

type responseStatusBranch struct {
	response ir.Response
	media    *ir.MediaType
	statuses []int
}

// reachableSuccessResponses is the common view for success-only consumers.
// Subtraction is computed against every representation before normal/stream
// partitioning, so an exact streaming response can shadow a buffered default.
func reachableSuccessResponses(responses []ir.Response) []ir.Response {
	// Exact response declarations cannot shadow one another. The common sorted
	// exact-only case can borrow its success slice without allocating status
	// lists or rebuilding media contracts on every semantic consumer.
	allExact := true
	first, last := -1, -1
	interleaved := false
	for index, response := range responses {
		low, _, rank := responseStatusRange(response.Status)
		if rank == 1 || rank == 2 {
			allExact = false
			break
		}
		if rank == 3 && low >= 200 && low < 300 {
			if first < 0 {
				first = index
			} else if last != index-1 {
				interleaved = true
			}
			last = index
		}
	}
	if allExact {
		if first < 0 {
			return nil
		}
		if !interleaved {
			return responses[first : last+1]
		}
		var result []ir.Response
		for _, response := range responses {
			low, _, rank := responseStatusRange(response.Status)
			if rank == 3 && low >= 200 && low < 300 {
				result = append(result, response)
			}
		}
		return result
	}
	allowed := make(map[string]map[string]bool)
	for _, branch := range responseStatusBranches(responses, true) {
		media := ""
		if branch.media != nil {
			media = branch.media.ContentType
		}
		if allowed[branch.response.Status] == nil {
			allowed[branch.response.Status] = make(map[string]bool)
		}
		allowed[branch.response.Status][media] = true
	}
	var result []ir.Response
	for _, response := range responses {
		media, found := allowed[response.Status]
		if !found {
			continue
		}
		if len(response.Content) > 0 {
			content := make([]ir.MediaType, 0, len(response.Content))
			for _, representation := range response.Content {
				if media[representation.ContentType] {
					content = append(content, representation)
				}
			}
			response.Content = content
		}
		result = append(result, response)
	}
	return result
}

// responseStatusBranches proves status subtraction only when a more specific
// response's media range covers the entire candidate range.
func responseStatusBranches(responses []ir.Response, success bool) []responseStatusBranch {
	var result []responseStatusBranch
	for _, response := range responses {
		mediaValues := response.Content
		if len(mediaValues) == 0 {
			mediaValues = []ir.MediaType{{}}
		}
		for mediaIndex := range mediaValues {
			media := &mediaValues[mediaIndex]
			first, last, rank := responseStatusRange(response.Status)
			if rank == 0 {
				continue
			}
			if success {
				first = max(first, 200)
				last = min(last, 299)
			}
			if first > last {
				continue
			}
			var shadows [][2]int
			for _, covering := range responses {
				low, high, priority := responseStatusRange(covering.Status)
				if priority <= rank || high < first || low > last {
					continue
				}
				coversMedia := len(covering.Content) == 0 && media.ContentType == ""
				for _, representation := range covering.Content {
					if responseMediaRangeCovers(representation.ContentType, media.ContentType) {
						coversMedia = true
						break
					}
				}
				if coversMedia {
					shadows = append(shadows, [2]int{low, high})
				}
			}
			statuses := make([]int, 0, last-first+1)
			for status := first; status <= last; status++ {
				if !success && status >= 200 && status < 300 {
					continue
				}
				shadowed := false
				for _, span := range shadows {
					if status >= span[0] && status <= span[1] {
						shadowed = true
						break
					}
				}
				if !shadowed {
					statuses = append(statuses, status)
				}
			}
			if len(statuses) == 0 {
				continue
			}
			branch := responseStatusBranch{response: response, statuses: statuses}
			if len(response.Content) > 0 {
				branch.media = media
			}
			result = append(result, branch)
		}
	}
	return result
}

func responseStatusRange(pattern string) (int, int, int) {
	if pattern == "default" {
		return 100, 599, 1
	}
	if len(pattern) != 3 || pattern[0] < '1' || pattern[0] > '5' {
		return 0, 0, 0
	}
	first := int(pattern[0]-'0') * 100
	if strings.EqualFold(pattern[1:], "XX") {
		return first, first + 99, 2
	}
	if pattern[1] < '0' || pattern[1] > '9' || pattern[2] < '0' || pattern[2] > '9' {
		return 0, 0, 0
	}
	status := first + int(pattern[1]-'0')*10 + int(pattern[2]-'0')
	return status, status, 3
}

func responseStatusScore(pattern string, status int) int {
	first, last, rank := responseStatusRange(pattern)
	if status >= first && status <= last {
		return rank
	}
	return 0
}

func responseMediaRangeCovers(covering, candidate string) bool {
	outer := strings.ToLower(strings.TrimSpace(strings.SplitN(covering, ";", 2)[0]))
	inner := strings.ToLower(strings.TrimSpace(strings.SplitN(candidate, ";", 2)[0]))
	if outer == inner {
		return true
	}
	if outer == "" || inner == "" {
		return false
	}
	if outer == "*/*" {
		return true
	}
	left, right := strings.SplitN(outer, "/", 2), strings.SplitN(inner, "/", 2)
	if len(left) != 2 || len(right) != 2 || (left[0] != "*" && left[0] != right[0]) {
		return false
	}
	if left[1] == "*" {
		return true
	}
	if strings.HasPrefix(left[1], "*+") && right[1] != "*" {
		return strings.HasSuffix(right[1], left[1][1:])
	}
	return left[1] == right[1]
}

func responseStatusUnion(statuses []int) string {
	values := make([]string, len(statuses))
	for index, status := range statuses {
		values[index] = strconv.Itoa(status)
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
