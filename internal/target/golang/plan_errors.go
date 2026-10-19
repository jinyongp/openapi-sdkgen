package golang

import (
	"bytes"
	"encoding/json"
	"fmt"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/generator"
	"openapi-sdkgen/internal/http/extension"
	schemaplan "openapi-sdkgen/internal/schema/plan"
	"strconv"
	"strings"
)

type errorContractPlan struct {
	Code, Category, Name, Validation string
	Details                          []any
	Unknown                          bool
}

func goErrorContracts(document *ir.Document) map[string]*errorContractPlan {
	planner := extension.Planner{}
	reachable := planner.ReachableErrorSchemas(document)
	result := make(map[string]*errorContractPlan)
	for _, name := range sortedKeys(reachable) {
		schema := document.ComponentSchemas[name]
		for _, variant := range planner.ErrorEnvelopes(document, schema) {
			category := document.ErrorCategories[name]
			if wire, _, exact := planner.WireCategory(document, variant.ErrorSchema); exact {
				category = wire
			} else if declared, ok := schema["x-error-category"].(string); ok && declared != "" {
				category = declared
			}
			properties, _ := variant.ErrorSchema["properties"].(map[string]any)
			details, present := properties["details"].(map[string]any)
			for _, code := range variant.Codes {
				contract := result[code]
				if contract == nil {
					contract = &errorContractPlan{Code: code}
					result[code] = contract
				}
				if category != "" {
					contract.Category = category
				}
				if !present || len(details) == 0 {
					contract.Unknown = true
				} else {
					encoded, _ := json.Marshal(details)
					duplicate := false
					for _, item := range contract.Details {
						previous, _ := json.Marshal(item)
						duplicate = duplicate || string(previous) == string(encoded)
					}
					if !duplicate {
						contract.Details = append(contract.Details, details)
					}
				}
			}
		}
	}
	return result
}
func prepareErrorNames(names *namePlan, document *ir.Document, used map[string]bool) error {
	names.errorCodes = make(map[string]string)
	names.errorCategories = make(map[string]string)
	contracts := goErrorContracts(document)
	for _, code := range sortedKeys(contracts) {
		name, err := reserveDerivedName(used, "ServerError"+exportedName(code), "error-code/"+code, func(base string) []string {
			return append(modelNames(base+"Details"), base+"Code", "As"+base+"Details")
		})
		if err != nil {
			return err
		}
		names.errorCodes[code] = name
	}
	categories := make(map[string]bool)
	for _, contract := range contracts {
		if contract.Category != "" {
			categories[contract.Category] = true
		}
	}
	for _, category := range sortedKeys(categories) {
		name, err := reservePublicName(used, "ServerErrorCategory"+exportedName(category), "error-category/"+category, "")
		if err != nil {
			return err
		}
		names.errorCategories[category] = name
	}
	return nil
}
func prepareErrorContracts(document *ir.Document, names namePlan, schemas *schemaPlanner) ([]errorContractPlan, error) {
	contracts := goErrorContracts(document)
	var result []errorContractPlan
	for _, code := range sortedKeys(contracts) {
		contract := contracts[code]
		contract.Name = names.errorCodes[code]
		var schema any = map[string]any{}
		if !contract.Unknown && len(contract.Details) == 1 {
			schema = contract.Details[0]
		} else if !contract.Unknown && len(contract.Details) > 1 {
			schema = map[string]any{"anyOf": contract.Details}
		}
		validation, err := schemas.prepare(httpValidationName(contract.Name+"Details"), schema, schemaplan.Output, document.OpenAPIVersionLine == "3.0", schemaOccurrence{Source: "#/error-codes/" + pointerToken(code) + "/details"})
		if err != nil {
			return nil, err
		}
		contract.Validation = validation
		result = append(result, *contract)
	}
	return result, nil
}
func appendErrorFiles(config generator.GoOptions, files []filePlan, contracts []errorContractPlan, names namePlan) []filePlan {
	var output bytes.Buffer
	fmt.Fprintln(&output, "type ServerErrorCode string\ntype ServerErrorCategory string")
	for _, contract := range contracts {
		fmt.Fprintf(&output, "const %sCode = %q\ntype %sDetails = models.%sDetails\n", contract.Name, contract.Code, contract.Name, contract.Name)
	}
	for _, category := range sortedKeys(names.errorCategories) {
		fmt.Fprintf(&output, "const %s ServerErrorCategory = %q\n", names.errorCategories[category], category)
	}
	fmt.Fprintln(&output, "func ServerErrorCodes()[]ServerErrorCode{return []ServerErrorCode{")
	for _, contract := range contracts {
		fmt.Fprintf(&output, "%q,\n", contract.Code)
	}
	fmt.Fprintln(&output, "}}")
	fmt.Fprintln(&output, "func ServerErrorCategories()[]ServerErrorCategory{return []ServerErrorCategory{")
	for _, category := range sortedKeys(names.errorCategories) {
		fmt.Fprintf(&output, "%q,\n", category)
	}
	fmt.Fprintln(&output, "}}")
	fmt.Fprintln(&output, "func IsErrorCategory(cause error,category ServerErrorCategory)bool{code,present:=types.GetErrorCode(cause);if !present{return false};_=code;switch category{")
	for _, category := range sortedKeys(names.errorCategories) {
		var codes []string
		for _, contract := range contracts {
			if contract.Category == category {
				codes = append(codes, strconv.Quote(contract.Code))
			}
		}
		fmt.Fprintf(&output, "case %q:switch code{case %s:return true}\n", category, strings.Join(codes, ","))
	}
	fmt.Fprintln(&output, "};return false}")
	for _, contract := range contracts {
		fmt.Fprintf(&output, "func As%sDetails(cause error)(%sDetails,bool){var zero %sDetails;var failure *types.APIError;if !errors.As(cause,&failure)||failure==nil||failure.Code!=%q{return zero,false};value,present:=failure.Details.Get();if !present{return zero,false};decoded,err:=models.DecodeResponse%sDetails(context.Background(),value);return decoded,err==nil}\n", contract.Name, contract.Name, contract.Name, contract.Code, contract.Name)
	}
	imports := []importPlan{{Path: config.ImportPath + "/types"}}
	if len(contracts) != 0 {
		imports = append(imports, importPlan{Path: "context"}, importPlan{Path: "errors"}, importPlan{Path: config.ImportPath + "/models"})
	}
	return append(files, filePlan{Path: "errors.go", Package: config.Package, Template: "templates/prepared.go.tmpl", Imports: imports, Prepared: output.String()})
}
