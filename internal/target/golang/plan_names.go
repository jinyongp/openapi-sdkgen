package golang

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/token"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"openapi-sdkgen/internal/compiler/ir"
)

type namePlan struct {
	operations, routes, schemas   map[string]string
	resources, resourceParameters map[string]string
	resourceMembers               map[string]map[string]string
	linkGroups, linkMembers       map[string]string
	errorCodes, errorCategories   map[string]string
	reserved                      map[string]bool
}

func reservePublicName(used map[string]bool, raw, identity string, suffixes ...string) (string, error) {
	return reserveDerivedName(used, raw, identity, func(base string) []string {
		names := make([]string, 0, len(suffixes))
		for _, suffix := range suffixes {
			names = append(names, base+suffix)
		}
		return names
	})
}

func reserveDerivedName(used map[string]bool, raw, identity string, derivatives func(string) []string) (string, error) {
	base := exportedName(raw)
	first, _ := utf8.DecodeRuneInString(raw)
	forced := unicode.IsDigit(first) || token.Lookup(raw).IsKeyword()
	available := func(name string) bool {
		for _, derived := range derivatives(name) {
			if used[derived] {
				return false
			}
		}
		return true
	}
	name := base
	if forced || !available(name) {
		digest := sha256.Sum256([]byte(identity))
		encoded := hex.EncodeToString(digest[:])
		found := false
		for width := 8; width <= len(encoded); width += 4 {
			candidate := base + "_" + encoded[:width]
			if available(candidate) {
				name = candidate
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("cannot reserve a unique Go name for %q", identity)
		}
	}
	for _, derived := range derivatives(name) {
		used[derived] = true
	}
	return name, nil
}

func modelRootNames(base string) []string {
	names := []string{base, base + "Input", base + "Output", base + "Call", base + "RawResponse", base + "HTTPError", base + "SecurityRequirement"}
	for _, root := range []string{base + "Input", base + "Output"} {
		names = append(names, modelNames(root)...)
	}
	for _, section := range []string{"Path", "Query", "Header", "Cookie", "Querystring", "Body"} {
		names = append(names, modelNames(base+section)...)
	}
	return names
}

func modelNames(base string) []string {
	return []string{base, "Encode" + base, "Decode" + base, "DecodeResponse" + base, "EncodeMedia" + base, "DecodeMedia" + base, "EncodeBound" + base, "EncodeBoundMedia" + base, "DecodeValidated" + base, "Parse" + base, "Is" + base, base + "Values", base + "FromNullable"}
}

func sourceFilePath(directory, owner string) string {
	return namedSourceFilePath(directory, "source", owner)
}
func namedSourceFilePath(directory, prefix, owner string) string {
	digest := sha256.Sum256([]byte(owner))
	return directory + "/" + prefix + "_" + hex.EncodeToString(digest[:]) + ".go"
}

// Public identities are reserved across the whole document before selection.
func planNames(document *ir.Document) (namePlan, error) {
	result := namePlan{operations: make(map[string]string), routes: make(map[string]string), schemas: make(map[string]string)}
	used := map[string]bool{"Client": true, "ClientOptions": true, "RequestOptions": true, "APIError": true, "SDKError": true, "NewClient": true, "Operations": true, "IsAPIError": true, "GetErrorCode": true, "IsErrorCode": true, "GetRequestID": true, "ServerErrorCode": true, "ServerErrorCategory": true, "ServerErrorCodes": true, "ServerErrorCategories": true, "IsErrorCategory": true}
	for _, name := range []string{"Ref", "Reference", "ReferenceInfo", "DynamicCall", "DynamicOperation", "DynamicRawResponse", "DynamicLinks", "DynamicLinkInvocation"} {
		used[name] = true
	}
	used["OpenAPIVersion"], used["OpenAPIVersionLine"] = true, true
	schemaNames := make(map[string]bool)
	for name := range document.Schemas {
		schemaNames[name] = true
	}
	for name := range document.ComponentSchemas {
		schemaNames[name] = true
	}
	for _, sourceName := range sortedKeys(schemaNames) {
		identity := document.Schemas[sourceName].Pointer
		if identity == "" {
			identity = "#/components/schemas/" + strings.ReplaceAll(strings.ReplaceAll(sourceName, "~", "~0"), "/", "~1")
		}
		name, err := reserveDerivedName(used, sourceName, identity, modelRootNames)
		if err != nil {
			return namePlan{}, err
		}
		result.schemas[sourceName] = name
	}
	operations := append([]ir.Operation(nil), document.Operations...)
	sort.Slice(operations, func(i, j int) bool { return operations[i].RouteKey < operations[j].RouteKey })
	routeNames := make(map[string]bool)
	for _, operation := range operations {
		routeName, err := reservePublicName(routeNames, routeMethodName(operation), operation.Pointer+"/route", "")
		if err != nil {
			return namePlan{}, err
		}
		result.routes[operation.RouteKey] = routeName
		raw := operation.OperationID
		if raw == "" {
			raw = operation.Method + " " + operation.Path
		}
		identity := operation.Pointer
		if identity == "" {
			identity = operation.RouteKey
		}
		roots, factories, err := operationContractNames(operation, document.OpenAPIVersionLine == "3.0")
		if err != nil {
			return namePlan{}, err
		}
		// Reservation must not promote an omitted operation's security failure.
		// Effective, retained operations are validated during their lowering.
		security, _ := prepareOperationSecurity(document, operation)
		name, err := reserveDerivedName(used, raw, identity, func(base string) []string {
			result := modelRootNames(base)
			for _, requirement := range security {
				result = append(result, base+"Security"+requirement.Name)
			}
			for _, suffix := range roots {
				result = append(result, modelNames(base+suffix)...)
			}
			for _, suffix := range factories {
				result = append(result, "New"+base+suffix)
			}
			return result
		})
		if err != nil {
			return namePlan{}, err
		}
		result.operations[operation.RouteKey] = name
	}
	if err := prepareResourceNames(&result, operations, used); err != nil {
		return namePlan{}, err
	}
	if err := prepareLinkNames(&result, document, operations, used); err != nil {
		return namePlan{}, err
	}
	if err := prepareErrorNames(&result, document, used); err != nil {
		return namePlan{}, err
	}
	result.reserved = used
	return result, nil
}

func exportedName(value string) string {
	var out strings.Builder
	upper := true
	for _, char := range value {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) {
			upper = true
			continue
		}
		if out.Len() == 0 && unicode.IsDigit(char) {
			out.WriteString("N")
		}
		if upper {
			char = unicode.ToUpper(char)
			upper = false
		}
		out.WriteRune(char)
	}
	if out.Len() == 0 {
		return "Operation"
	}
	name := out.String()
	first, _ := utf8.DecodeRuneInString(name)
	if !unicode.IsUpper(first) {
		name = "X" + name
	}
	for _, initialism := range []string{"ID", "URL", "HTTP", "JSON", "XML", "API", "SDK", "UUID"} {
		mixed := initialism[:1] + strings.ToLower(initialism[1:])
		for start := 0; start < len(name); {
			relative := strings.Index(name[start:], mixed)
			if relative < 0 {
				break
			}
			index := start + relative
			end := index + len(mixed)
			next := rune(0)
			if end < len(name) {
				next, _ = utf8.DecodeRuneInString(name[end:])
			}
			if end == len(name) || unicode.IsUpper(next) || unicode.IsDigit(next) {
				name = name[:index] + initialism + name[end:]
			}
			start = end
		}
	}
	return name
}
