package typescript

import "fmt"

// operationInputSection is semantic request presence, not an emitted type name.
// Requiredness and buffered-call capability are tracked separately.
type operationInputSection string

const (
	inputSectionPath        operationInputSection = "path"
	inputSectionQuery       operationInputSection = "query"
	inputSectionQuerystring operationInputSection = "querystring"
	inputSectionHeader      operationInputSection = "header"
	inputSectionCookie      operationInputSection = "cookie"
	inputSectionBody        operationInputSection = "body"
)

type operationInputSectionList []operationInputSection

// hasInput subtracts the bound path section without assuming it is present or
// inferring remaining input from the number of sections.
func (sections operationInputSectionList) hasInput(pathBound bool) bool {
	for _, section := range sections {
		if !pathBound || section != inputSectionPath {
			return true
		}
	}
	return false
}

type requestInputSectionDescriptor struct {
	section            operationInputSection
	suffix             string
	sectionKey         string
	inputProperty      string
	publicHelperSuffix string
	parameterLocation  bool
}

var requestInputSectionDescriptors = []requestInputSectionDescriptor{
	{section: inputSectionPath, suffix: "PathInput", sectionKey: "path", inputProperty: "path", publicHelperSuffix: "Path", parameterLocation: true},
	{section: inputSectionQuery, suffix: "QueryInput", sectionKey: "query", inputProperty: "query", publicHelperSuffix: "Query", parameterLocation: true},
	{section: inputSectionQuerystring, suffix: "QuerystringInput", sectionKey: "querystring", inputProperty: "querystring", publicHelperSuffix: "Querystring", parameterLocation: true},
	{section: inputSectionHeader, suffix: "HeaderInput", sectionKey: "header", inputProperty: "headerParams", publicHelperSuffix: "Headers", parameterLocation: true},
	{section: inputSectionCookie, suffix: "CookieInput", sectionKey: "cookie", inputProperty: "cookieParams", publicHelperSuffix: "Cookies", parameterLocation: true},
	{section: inputSectionBody, suffix: "BodyInput", sectionKey: "body", inputProperty: "body", publicHelperSuffix: "Body"},
}

func requestInputSection(section operationInputSection) (requestInputSectionDescriptor, error) {
	for _, descriptor := range requestInputSectionDescriptors {
		if descriptor.section == section {
			return descriptor, nil
		}
	}
	return requestInputSectionDescriptor{}, fmt.Errorf("unsupported operation input section %q", section)
}
