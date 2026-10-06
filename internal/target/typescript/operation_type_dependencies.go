package typescript

import (
	"fmt"
	"strings"
)

type operationHelperType struct {
	slot string
	kind string
}

var operationHelperTypes = map[string]operationHelperType{
	"RouteInput":            {slot: "Input", kind: "public"},
	"RouteResourceInput":    {slot: "ResourceInput", kind: "public"},
	"RouteOptions":          {slot: "Options", kind: "contract"},
	"RouteOutput":           {slot: "Output", kind: "public"},
	"RouteRawResponse":      {slot: "RawResponse", kind: "public"},
	"OperationRawCall":      {slot: "RawCall", kind: "identity"},
	"ResourceRawCapability": {slot: "ResourceRawCall", kind: "resource"},
	"PaginateCall":          {slot: "Pagination", kind: "identity"},
	"LinkCalls":             {slot: "Links", kind: "identity"},
	"StreamCall":            {slot: "Stream", kind: "identity"},
}

// Generated leaves already own their exact contracts. Do not route those type
// references back through the full document registry. Only compiler-emitted
// unqualified helper applications are rewritten; literals and comments remain
// untouched, and cross-operation helpers import the resolved target directly.
func localizeOperationHelperTypes(source string, module operationModulePlan, plan *semanticModulePlan) (string, error) {
	var output strings.Builder
	cursor := 0
	for index := 0; index < len(source); {
		if source[index] == '/' && index+1 < len(source) && source[index+1] == '/' {
			end := strings.IndexByte(source[index:], '\n')
			if end < 0 {
				break
			}
			index += end + 1
			continue
		}
		if strings.HasPrefix(source[index:], "/*") {
			end := strings.Index(source[index+2:], "*/")
			if end < 0 {
				break
			}
			index += end + 4
			continue
		}
		if source[index] == '"' || source[index] == '\'' || source[index] == '`' {
			quote := source[index]
			index++
			for index < len(source) {
				if source[index] == '\\' {
					index += 2
					continue
				}
				if source[index] == quote {
					index++
					break
				}
				index++
			}
			continue
		}
		if !operationHelperIdentifierStart(source[index]) {
			index++
			continue
		}
		start := index
		index++
		for index < len(source) && operationHelperIdentifierPart(source[index]) {
			index++
		}
		helper, exists := operationHelperTypes[source[start:index]]
		previous := start
		for previous > 0 && strings.ContainsRune(" \t\r\n", rune(source[previous-1])) {
			previous--
		}
		if !exists || (previous > 0 && source[previous-1] == '.') {
			continue
		}
		open := operationHelperSkipSpace(source, index)
		if open >= len(source) || source[open] != '<' {
			continue
		}
		argument := operationHelperSkipSpace(source, open+1)
		end := argument
		route := ""
		if strings.HasPrefix(source[argument:], "RouteKey") {
			end += len("RouteKey")
			route = module.routeKey
		} else {
			var ok bool
			end, ok = quotedTokenEnd(source, argument)
			if !ok {
				continue
			}
			route = plan.operationByQuotedRoute[source[argument:end]]
		}
		close := operationHelperSkipSpace(source, end)
		if route == "" || close >= len(source) || source[close] != '>' {
			continue
		}
		contract := helper.slot
		if route != module.routeKey {
			target, exists := plan.operationByRoute[route]
			if !exists {
				return "", fmt.Errorf("operation helper target %q has no module", route)
			}
			specifier, err := plan.relativeModuleSpecifier(module.path, target)
			if err != nil {
				return "", err
			}
			contract = "import(" + quoteTS(specifier) + ")." + helper.slot
		}
		var replacement string
		switch helper.kind {
		case "contract":
			replacement = contract
		case "public":
			replacement = "OperationPublicType<" + contract + ">"
		case "resource":
			replacement = "OperationResourceRawCapability<" + contract + ", " + source[argument:end] + ">"
			if route == module.routeKey {
				replacement = "ResourceRawCapability<" + source[argument:end] + ">"
			}
		case "identity":
			replacement = "(" + contract + " & RouteTypeIdentity<" + source[argument:end] + ">)"
		}
		output.WriteString(source[cursor:start])
		output.WriteString(replacement)
		cursor = close + 1
		index = cursor
	}
	if cursor == 0 {
		return source, nil
	}
	output.WriteString(source[cursor:])
	return output.String(), nil
}

func operationHelperIdentifierStart(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || value == '_' || value == '$' || value >= 0x80
}

func operationHelperIdentifierPart(value byte) bool {
	return operationHelperIdentifierStart(value) || value >= '0' && value <= '9'
}

func operationHelperSkipSpace(source string, index int) int {
	for index < len(source) && (source[index] == ' ' || source[index] == '\n' || source[index] == '\r' || source[index] == '\t') {
		index++
	}
	return index
}
