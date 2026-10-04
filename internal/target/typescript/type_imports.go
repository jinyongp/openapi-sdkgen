package typescript

import "strings"

// These emitters own single-line imports and do not emit interpolated template
// literals. Plan their type-only dependencies from the final localized body,
// excluding documentation and OpenAPI string literals from identifier usage.
func generatedTypeImports(source string) string {
	lines := strings.SplitAfter(source, "\n")
	var body strings.Builder
	body.Grow(len(source))
	for _, line := range lines {
		if !strings.HasPrefix(line, "import ") {
			body.WriteString(line)
		}
	}
	used := generatedIdentifiers(body.String())
	var result strings.Builder
	result.Grow(len(source))
	for _, line := range lines {
		if !strings.HasPrefix(line, "import ") {
			result.WriteString(line)
			continue
		}
		if strings.HasPrefix(line, "import type * as ") {
			name := strings.Fields(line)[4]
			if used[name] > 0 {
				result.WriteString(line)
			}
			continue
		}
		open, close := strings.IndexByte(line, '{'), strings.IndexByte(line, '}')
		if open < 0 || close < open {
			result.WriteString(line)
			continue
		}
		typeOnly := strings.HasPrefix(line, "import type ")
		var entries []string
		for _, entry := range strings.Split(line[open+1:close], ",") {
			entry = strings.TrimSpace(entry)
			fields := strings.Fields(entry)
			if len(fields) == 0 {
				continue
			}
			isType := typeOnly || fields[0] == "type"
			name := fields[len(fields)-1]
			if !isType || used[name] > 0 {
				entries = append(entries, entry)
			}
		}
		if len(entries) > 0 {
			result.WriteString(line[:open+1] + " " + strings.Join(entries, ", ") + " " + line[close:])
		}
	}
	return result.String()
}

func generatedIdentifiers(source string) map[string]int {
	result := make(map[string]int)
	for i := 0; i < len(source); {
		if strings.HasPrefix(source[i:], "//") {
			end := strings.IndexByte(source[i:], '\n')
			if end < 0 {
				break
			}
			i += end + 1
			continue
		}
		if strings.HasPrefix(source[i:], "/*") {
			end := strings.Index(source[i+2:], "*/")
			if end < 0 {
				break
			}
			i += end + 4
			continue
		}
		if source[i] == '"' || source[i] == '\'' || source[i] == '`' {
			quote := source[i]
			i++
			for i < len(source) {
				if source[i] == '\\' {
					i += 2
					continue
				}
				if source[i] == quote {
					i++
					break
				}
				i++
			}
			continue
		}
		if !operationHelperIdentifierStart(source[i]) {
			i++
			continue
		}
		start := i
		i++
		for i < len(source) && operationHelperIdentifierPart(source[i]) {
			i++
		}
		result[source[start:i]]++
	}
	return result
}
