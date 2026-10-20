package golang

import (
	"fmt"
	"strings"

	schemaplan "openapi-sdkgen/internal/schema/plan"
)

func (planner *schemaPlanner) format(node *schemaplan.Node, format string, pattern func(string) (string, error)) (string, error) {
	assertion, _ := node.Get("formatAssertion").(schemaplan.Literal)
	if assertion.Data != true {
		return "", nil
	}
	format = strings.ToLower(format)
	if planner.formatRoots == nil {
		planner.formatRoots = make(map[string]bool)
	}
	fail := `return result,Failure("format",path)`
	matched := func(expression string) string {
		return "if text,ok:=value.String();ok {if !(" + expression + "){" + fail + "}}\n"
	}
	fixed := map[string]string{
		"duration":              `^[Pp](?!$)(?:[0-9]+[Yy])?(?:[0-9]+[Mm])?(?:[0-9]+[Dd])?(?:[Tt](?=[0-9])(?:[0-9]+[Hh])?(?:[0-9]+[Mm])?(?:[0-9]+(?:\.[0-9]+)?[Ss])?)?$(?![\s\S])`,
		"uuid":                  `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$(?![\s\S])`,
		"json-pointer":          `^(?:/(?:[^~/]|~[01])*)*$(?![\s\S])`,
		"relative-json-pointer": `^(?:0|[1-9][0-9]*)(?:#|(?:/(?:[^~/]|~[01])*)*)$(?![\s\S])`,
	}
	if format == "hostname" || format == "idn-hostname" {
		planner.formatRoots["format-hostname"] = true
		return matched(fmt.Sprintf("FormatHostname(text,%t)", format == "idn-hostname")), nil
	}
	if format == "email" || format == "idn-email" {
		planner.formatRoots["format-email"] = true
		return matched(fmt.Sprintf("FormatEmail(text,%t)", format == "idn-email")), nil
	}
	if source, ok := fixed[format]; ok {
		binding, err := pattern(source)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("if text,ok:=value.String();ok{matched,err:=%s.Match(ctx,text);if err!=nil{return result,err};if !matched{%s}}\n", binding, fail), nil
	}
	if format == "uri" || format == "uri-reference" || format == "iri" || format == "iri-reference" {
		planner.formatRoots["format-url"] = true
		return matched(fmt.Sprintf("FormatURL(text,%t,%t)", format == "uri" || format == "iri", format == "iri" || format == "iri-reference")), nil
	}
	switch format {
	case "date", "time", "date-time":
		planner.formatRoots["format-"+format] = true
		return matched(map[string]string{"date": "FormatDate(text)", "time": "FormatTime(text)", "date-time": "FormatDateTime(text)"}[format]), nil
	case "ipv4", "ipv6":
		planner.formatRoots["format-ip"] = true
		return matched(fmt.Sprintf("FormatIP(text,%t)", format == "ipv6")), nil
	case "regex":
		planner.formatRoots["regex-compiler"] = true
		return "if text,ok:=value.String();ok{matched,err:=FormatRegex(ctx,text);if err!=nil{return result,err};if !matched{" + fail + "}}\n", nil
	case "uri-template":
		guard, err := pattern(`[\u0000-\u001f\u007f\s]`)
		if err != nil {
			return "", err
		}
		planner.formatRoots["format-template"] = true
		return fmt.Sprintf("if text,ok:=value.String();ok{blocked,err:=%s.Match(ctx,text);if err!=nil{return result,err};if blocked||!FormatTemplate(text){%s}}\n", guard, fail), nil

	}
	return "", nil
}
