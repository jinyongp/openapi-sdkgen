package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode"

	toml "github.com/pelletier/go-toml/v2"
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

type inspectFilter struct {
	search, methods, tags rawStrings
	deprecated            *string
	format                *string
}

type inspectOperation struct{ compiler.InventoryOperation }
type inspectDocument struct {
	Title          string `json:"title"`
	Version        string `json:"version"`
	OpenAPIVersion string `json:"openapiVersion"`
}
type inspectReport struct {
	SchemaVersion int                `json:"schemaVersion"`
	Document      inspectDocument    `json:"document"`
	Total         int                `json:"total"`
	Matched       int                `json:"matched"`
	DocumentsRead int                `json:"documentsRead"`
	Operations    []inspectOperation `json:"operations"`
}

// Reuse the generate flag values and metadata for input settings while exposing
// only inspect's read-only options. Configuration retains the same parser and
// path/override rules; generation settings do not implicitly narrow a catalog.
func newInspectFlagSet(registries cliRegistries) (*commandFlagSet, *generateFlagValues, *inspectFilter) {
	original, values := newGenerateFlagSet(registries)
	flags := newCommandFlagSet("inspect", "Input", "Filters", "Output", "Options")
	groups := map[string]int{"config": 0, "input": 0, "input-base": 0, "http-header-env": 0, "tls-client-cert": 0, "tls-client-key": 0, "tls-ca-file": 0, "allow-remote-ref": 0, "ref-lock": 0, "offline": 0, "operation": 1, "route": 1, "diagnostics-format": 3, "diagnostic-mode": 3, "help": 3}
	for _, group := range original.Groups {
		for _, option := range group.Options {
			index, ok := groups[option.Name]
			if !ok {
				continue
			}
			switch option.Name {
			case "config":
				option.Summary = "Reuse input and reference settings from an explicit TOML file"
			case "operation":
				option.Summary = "Match an exact operationId; combine with --route"
			case "route":
				option.Summary = "Match an exact METHOD and OpenAPI path template"
			}
			flags.addOption(index, option)
			flag := original.Flags.Lookup(option.Name)
			flags.Flags.Var(flag.Value, option.Name, option.Summary)
			if option.Short != "" {
				flags.Flags.Var(flag.Value, option.Short, option.Summary)
			}
		}
	}
	filter := &inspectFilter{}
	flags.Var(1, helpOption{Name: "search", Metavariable: "text", Summary: "Case-insensitive substring in path, operationId, or summary", Repeatable: true}, &filter.search)
	flags.Var(1, helpOption{Name: "method", Metavariable: "method", Summary: "Match an HTTP method; custom methods use exact case", Repeatable: true}, &filter.methods)
	flags.Var(1, helpOption{Name: "tag", Metavariable: "tag", Summary: "Match an exact tag", Repeatable: true}, &filter.tags)
	filter.deprecated = flags.String(1, helpOption{Name: "deprecated", Metavariable: "true|false", Summary: "Match deprecated status (undeclared is false)"}, "")
	filter.format = flags.String(2, helpOption{Name: "format", Metavariable: "format", Summary: "Output format", Available: func() []string { return []string{"table", "json", "selection"} }}, "table")
	return flags, values, filter
}

func writeInspectHelp(flags *commandFlagSet) error {
	return renderHelp(standardOutput, helpDocument{
		Description: "List API declarations and export routes for selected SDK generation.", Usage: "openapi-sdkgen inspect [options]", Groups: flags.Groups,
		Examples: []string{`openapi-sdkgen inspect --input ./openapi.yaml --search users --method GET`, `openapi-sdkgen inspect --input ./openapi.yaml --tag Users --format selection`},
		Footer:   "Repeated values in each filter match any value; different filters must all match.\nOperation IDs and routes form one combined selection filter. Configuration supplies\ninput settings; its generation selection, target, output, and add-ons do not narrow\nthe catalog. Selection output is a TOML fragment for your existing config.",
	})
}

func inspectUsageError(message string) error {
	return fmt.Errorf("%s\nTry \"openapi-sdkgen inspect --help\" for usage", message)
}

func inspectWithRegistries(args []string, runtime generationRuntime, registries cliRegistries) error {
	flags, values, filter := newInspectFlagSet(registries)
	if err := flags.Flags.Parse(args); err != nil {
		return inspectUsageError(err.Error())
	}
	if *values.help {
		return writeInspectHelp(flags)
	}
	if flags.Flags.NArg() != 0 {
		return inspectUsageError("unexpected positional arguments")
	}
	visited := visitedGenerateFlags(flags.Flags)
	if visited["deprecated"] && *filter.deprecated == "" {
		return inspectUsageError("--deprecated requires true or false")
	}
	if *values.config != "" {
		config, base, err := loadGenerateProjectConfig(*values.config)
		if err != nil {
			return inspectUsageError(err.Error())
		}
		// Selection and target are explicit inspect options, independent of the
		// project's generation scope. Reuse only the input-side settings.
		visited["operation"], visited["route"], visited["target"] = true, true, true
		applyGenerateProjectConfig(config, base, values, visited)
	}
	if *values.input == "" {
		return inspectUsageError("--input is required unless provided by --config")
	}
	format, err := resolveDiagnosticOutputFormat(*values.diagnosticsFormat)
	if err != nil {
		return inspectUsageError(err.Error())
	}
	mode, err := resolveDiagnosticMode(*values.diagnosticMode)
	if err != nil {
		return inspectUsageError(err.Error())
	}
	if err := filter.validate(values.operations, values.routes); err != nil {
		return inspectUsageError(err.Error())
	}
	options := compiler.CompileOptions{DiagnosticMode: mode, InputBase: *values.inputBase, InputReader: standardInput, RemoteRefAllowlist: values.remoteRefs, RefLockPath: *values.refLock, Offline: *values.offline, HTTPHeaderEnv: values.httpHeaderEnv, TLSClientCert: *values.tlsClientCert, TLSClientKey: *values.tlsClientKey, TLSCAFile: *values.tlsCAFile}
	compiled, err := compiler.InspectInputResult(*values.input, options)
	if err != nil {
		return internalFailure("internal inventory failure", err)
	}
	if err := writeDiagnostics(compiled.Diagnostics, nil, nil, mode, format); err != nil {
		return err
	}
	if diagnostic.HasErrors(compiled.Diagnostics) {
		return errReportedDiagnostics
	}
	if compiled.Inventory == nil {
		return internalFailure("internal inventory failure", fmt.Errorf("inventory is unavailable"))
	}
	report, err := filter.apply(compiled.Inventory, values.operations, values.routes)
	if err != nil {
		return inspectUsageError(err.Error())
	}
	return writeInspectReport(standardOutput, report, *filter.format)
}

func (f *inspectFilter) validate(operations, routes []string) error {
	switch *f.format {
	case "table", "json", "selection":
	default:
		return fmt.Errorf("unsupported --format %q (available: table, json, selection)", *f.format)
	}
	if *f.deprecated != "" && *f.deprecated != "true" && *f.deprecated != "false" {
		return fmt.Errorf("--deprecated requires true or false")
	}
	for _, values := range [][]string{f.search, f.tags, operations} {
		for _, value := range values {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("filter values must not be blank")
			}
		}
	}
	for _, method := range f.methods {
		if !validInspectMethod(method) {
			return fmt.Errorf("invalid --method %q", method)
		}
	}
	if len(operations)+len(routes) > 0 {
		_, err := (&generator.Selection{Operations: operations, Routes: routes}).Canonical()
		if err != nil {
			return err
		}
	}
	return nil
}

func validInspectMethod(method string) bool {
	return method != "" && !strings.ContainsFunc(method, func(r rune) bool {
		return r > 127 || !strings.ContainsRune("!#$%&'*+-.^_`|~0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", r)
	})
}

func normalizedInspectMethod(method string) string {
	switch strings.ToUpper(method) {
	case "GET", "PUT", "POST", "DELETE", "OPTIONS", "HEAD", "PATCH", "TRACE", "QUERY":
		return strings.ToUpper(method)
	}
	return method
}

func (f *inspectFilter) apply(inventory *compiler.Inventory, operations, routes []string) (inspectReport, error) {
	ids, routeNames := map[string]bool{}, map[string]bool{}
	for _, operation := range inventory.Operations {
		routeNames[operation.Route] = true
		if operation.OperationID != nil {
			ids[*operation.OperationID] = true
		}
	}
	for _, id := range operations {
		if !ids[id] {
			return inspectReport{}, fmt.Errorf("unknown operationId %q", id)
		}
	}
	for _, route := range routes {
		if !routeNames[route] {
			return inspectReport{}, fmt.Errorf("unknown route %q", route)
		}
	}
	report := inspectReport{SchemaVersion: 1, Document: inspectDocument{inventory.Title, inventory.Version, inventory.OpenAPIVersion}, Total: len(inventory.Operations), DocumentsRead: inventory.DocumentsRead, Operations: []inspectOperation{}}
	for _, operation := range inventory.Operations {
		if len(operations)+len(routes) > 0 {
			matches := containsInspectValue(routes, operation.Route) || (operation.OperationID != nil && containsInspectValue(operations, *operation.OperationID))
			if !matches {
				continue
			}
		}
		if len(f.methods) > 0 {
			matches := false
			for _, method := range f.methods {
				if normalizedInspectMethod(method) == operation.Method {
					matches = true
					break
				}
			}
			if !matches {
				continue
			}
		}
		if len(f.tags) > 0 {
			matches := false
			for _, tag := range operation.Tags {
				if containsInspectValue(f.tags, tag) {
					matches = true
					break
				}
			}
			if !matches {
				continue
			}
		}
		if *f.deprecated != "" && operation.Deprecated != (*f.deprecated == "true") {
			continue
		}
		if len(f.search) > 0 {
			matches := false
			for _, search := range f.search {
				needle := strings.ToLower(search)
				matches = strings.Contains(strings.ToLower(operation.Path), needle) || strings.Contains(strings.ToLower(operation.Summary), needle) || (operation.OperationID != nil && strings.Contains(strings.ToLower(*operation.OperationID), needle))
				if matches {
					break
				}
			}
			if !matches {
				continue
			}
		}
		report.Operations = append(report.Operations, inspectOperation{operation})
	}
	sort.Slice(report.Operations, func(i, j int) bool {
		a, b := report.Operations[i], report.Operations[j]
		if a.Path == b.Path {
			return a.Method < b.Method
		}
		return a.Path < b.Path
	})
	report.Matched = len(report.Operations)
	return report, nil
}

func containsInspectValue(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func writeInspectReport(output io.Writer, report inspectReport, format string) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	case "selection":
		if report.Matched == 0 {
			return fmt.Errorf("no operations matched; selection output requires at least one route")
		}
		routes := make([]string, 0, report.Matched)
		for _, operation := range report.Operations {
			routes = append(routes, operation.Route)
		}
		selection, err := (&generator.Selection{Routes: routes}).Canonical()
		if err != nil {
			return err
		}
		body := struct {
			Selection struct {
				Routes []string `toml:"routes"`
			} `toml:"selection"`
		}{}
		body.Selection.Routes = selection.Routes
		data, err := toml.Marshal(body)
		if err != nil {
			return err
		}
		_, err = output.Write(data)
		return err
	case "table":
		rows := [][]string{{"METHOD", "PATH", "OPERATION ID", "TAGS", "DEPRECATED", "SUMMARY"}}
		for _, operation := range report.Operations {
			id := "—"
			if operation.OperationID != nil {
				id = *operation.OperationID
			}
			summary := []rune(operation.Summary)
			if len(summary) > 60 {
				summary = append(summary[:59], '…')
			}
			rows = append(rows, []string{operation.Method, operation.Path, id, strings.Join(operation.Tags, ", "), strconv.FormatBool(operation.Deprecated), string(summary)})
		}
		widths := make([]int, len(rows[0]))
		for _, row := range rows {
			for col, text := range row {
				row[col] = safeInspectCell(text)
				if width := inspectCellWidth(row[col]); width > widths[col] {
					widths[col] = width
				}
			}
		}
		var result strings.Builder
		for _, row := range rows {
			for col, text := range row {
				result.WriteString(text)
				if col+1 < len(row) {
					result.WriteString(strings.Repeat(" ", widths[col]-inspectCellWidth(text)+2))
				}
			}
			result.WriteByte('\n')
		}
		fmt.Fprintf(&result, "\n%d / %d operations\n", report.Matched, report.Total)
		_, err := io.WriteString(output, result.String())
		return err
	default:
		return fmt.Errorf("unsupported inspect output format %q", format)
	}
}

func safeInspectCell(value string) string {
	var output strings.Builder
	for _, r := range value {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' || unicode.Is(unicode.Cf, r) {
			escaped := strconv.QuoteToASCII(string(r))
			output.WriteString(escaped[1 : len(escaped)-1])
		} else {
			output.WriteRune(r)
		}
	}
	return output.String()
}

func inspectCellWidth(value string) int {
	width := 0
	for _, r := range value {
		if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
			continue
		}
		if r >= 0x1100 && (r <= 0x115f || r >= 0x2e80 && r <= 0xa4cf || r >= 0xac00 && r <= 0xd7a3 || r >= 0xf900 && r <= 0xfaff || r >= 0xfe10 && r <= 0xfe6f || r >= 0xff01 && r <= 0xff60 || r >= 0xffe0 && r <= 0xffe6 || r >= 0x1f300 && r <= 0x1faff || r >= 0x20000) {
			width += 2
		} else {
			width++
		}
	}
	return width
}
