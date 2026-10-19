package golang

import (
	"bytes"
	"fmt"
	"strings"

	"openapi-sdkgen/internal/generator"
	schemaplan "openapi-sdkgen/internal/schema/plan"
)

// Generic compatibility validation uses the same prepared format algorithms as
// typed schemas. The typed request path never dispatches through this helper.
func prepareInboundGenericFormats(config generator.GoOptions, schemas *schemaPlanner) (filePlan, error) {
	var body bytes.Buffer
	fmt.Fprintln(&body, "func validateNormalizedFormat(ctx context.Context,value types.Value,format string)error{switch strings.ToLower(format){")
	for _, format := range []string{"email", "idn-email", "hostname", "duration", "uuid", "json-pointer", "relative-json-pointer", "date", "time", "date-time", "ipv4", "ipv6", "regex", "uri", "uri-reference", "iri", "iri-reference", "uri-template", "idn-hostname"} {
		name := "ValidateInboundFormat__" + strings.ReplaceAll(format, "-", "_") + "__"
		start := len(schemas.programs)
		if _, err := schemas.node(name, &schemaplan.Node{Fields: []schemaplan.Field{{Name: "formatAssertion", Value: schemaplan.Literal{Data: true}}, {Name: "format", Value: schemaplan.Literal{Data: format}}}}); err != nil {
			return filePlan{}, err
		}
		for index := start; index < len(schemas.programs); index++ {
			schemas.programs[index].Owner = name
			schemas.programs[index].Public = true
		}
		fmt.Fprintf(&body, "case %q:_,err:=schema.%s(ctx,value,\"\");return err\n", format, name)
	}
	fmt.Fprintln(&body, "};return nil}")
	return filePlan{Path: "server/runtime/schema_formats.go", Package: "runtime", Template: "templates/prepared.go.tmpl", Prepared: body.String(), Imports: []importPlan{{Path: "context"}, {Path: "strings"}, {Path: config.ImportPath + "/internal/schema"}, {Path: config.ImportPath + "/types"}}}, nil
}
