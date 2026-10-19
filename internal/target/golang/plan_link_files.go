package golang

import (
	"bytes"
	"fmt"
	"openapi-sdkgen/internal/generator"
	"strings"
)

func appendLinkFiles(config generator.GoOptions, files []filePlan, sources []linkSourcePlan, operations []operationPlan) []filePlan {
	if len(sources) == 0 {
		return files
	}
	for index := range files {
		if files[index].Path == "client.go" {
			files[index].LinkSources = sources
		}
	}
	var declaration, exports bytes.Buffer
	fmt.Fprintln(&declaration, "type Links struct{")
	for _, source := range sources {
		if source.Operation.DependencyOnly {
			continue
		}
		fmt.Fprintf(&declaration, "%s %sLinks\n", source.Operation.Name, source.Operation.Name)
	}
	fmt.Fprintln(&declaration, "}")
	fmt.Fprintln(&declaration, "func NewLinks(client *core.Client)Links{return Links{")
	for _, source := range sources {
		if source.Operation.DependencyOnly {
			continue
		}
		fmt.Fprintf(&declaration, "%s:%sLinks{", source.Operation.Name, source.Operation.Name)
		for _, group := range source.Groups {
			fmt.Fprintf(&declaration, "%s:%s{client:client,", group.Member, group.Name)
			for _, leaf := range group.Links {
				fmt.Fprintf(&declaration, "ByStatus%s:%s{client:client},", leaf.StatusName, leaf.Name)
			}
			fmt.Fprint(&declaration, "},")
		}
		fmt.Fprintln(&declaration, "},")
	}
	fmt.Fprintln(&declaration, "}}")
	fmt.Fprintln(&declaration, "func linkLiteral(text string)types.Value{value,err:=types.ParseValue([]byte(text));if err!=nil{panic(err)};return value}")
	for _, operation := range operations {
		if operation.LinkTarget == nil {
			continue
		}
		if operation.LinkTarget.Security {
			fmt.Fprintf(&declaration, "type %sLinkOptions struct{types.RequestOptions;SecurityRequirement %sSecurityRequirement}\n", operation.Name, operation.Name)
		} else {
			fmt.Fprintf(&declaration, "type %sLinkOptions = types.RequestOptions\n", operation.Name)
		}
		fmt.Fprintf(&exports, "type %sLinkOptions = operations.%sLinkOptions\n", operation.Name, operation.Name)
		if operation.HasInput {
			var models bytes.Buffer
			fmt.Fprintf(&models, "type %s struct{\n", operation.LinkTarget.Name)
			for _, field := range operation.LinkTarget.Fields {
				fmt.Fprintf(&models, "%s types.Optional[%s]\n", field.Name, field.Type)
				if !field.Native {
					fmt.Fprintf(&exports, "type %s = models.%s\n", field.Type, field.Type)
				}
			}
			emitLinkBodyOverrideFields(&models, operation)
			fmt.Fprintln(&models, "}")
			emitLinkBodyOverrideMethods(&models, operation)
			imports := []importPlan{{Path: config.ImportPath + "/types"}}
			if operation.BodyUnion {
				imports = append(imports, importPlan{Path: "context"})
			}
			files = append(files, filePlan{Path: namedSourceFilePath("models", "link_override", operation.Name), Package: "models", Template: "templates/prepared.go.tmpl", Imports: imports, Prepared: models.String()})
			fmt.Fprintf(&exports, "type %s = models.%s\n", operation.LinkTarget.Name, operation.LinkTarget.Name)
		}
	}
	for _, source := range sources {
		fmt.Fprintf(&declaration, "type %sLinks struct{\n", source.Operation.Name)
		for _, group := range source.Groups {
			fmt.Fprintf(&declaration, "%s %s\n", group.Member, group.Name)
		}
		fmt.Fprintln(&declaration, "}")
		fmt.Fprintf(&exports, "type %sLinkSource = operations.%sLinkSource\n", source.Operation.Name, source.Operation.Name)
		for _, group := range source.Groups {
			fmt.Fprintf(&declaration, "type %s struct{client *core.Client\n", group.Name)
			for _, leaf := range group.Links {
				fmt.Fprintf(&declaration, "ByStatus%s %s\n", leaf.StatusName, leaf.Name)
			}
			fmt.Fprintln(&declaration, "}")
			fmt.Fprintf(&exports, "type %s = operations.%s\n", group.Name, group.Name)
			for _, leaf := range group.Links {
				emitLinkLeaf(&declaration, &exports, group, leaf)
			}
			if group.DifferentTargets {
				emitDifferentTargetLinkGroup(&declaration, &exports, group)
			} else {
				emitSingleTargetLinkGroup(&declaration, &exports, group)
			}
		}
	}
	imports := []importPlan{{Path: "context"}, {Path: config.ImportPath + "/internal/core"}, {Path: config.ImportPath + "/types"}}
	if strings.Contains(declaration.String(), "models.") {
		imports = append(imports, importPlan{Path: config.ImportPath + "/models"})
	}
	if strings.Contains(declaration.String(), "http.") {
		imports = append(imports, importPlan{Path: "net/http"})
	}
	files = append(files, filePlan{Path: "internal/operations/links.go", Package: "operations", Template: "templates/prepared.go.tmpl", Imports: imports, Prepared: declaration.String()})
	exportImports := []importPlan{{Path: config.ImportPath + "/internal/operations"}}
	if strings.Contains(exports.String(), "models.") {
		exportImports = append(exportImports, importPlan{Path: config.ImportPath + "/models"})
	}
	if strings.Contains(exports.String(), "types.") {
		exportImports = append(exportImports, importPlan{Path: config.ImportPath + "/types"})
	}
	files = append(files, filePlan{Path: "links.go", Package: config.Package, Template: "templates/prepared.go.tmpl", Imports: exportImports, Prepared: exports.String()})
	return files
}
func linkInputType(operation operationPlan) string {
	if operation.HasInput {
		return "models." + operation.Name + "Input"
	}
	return "types.NoInput"
}
func linkOverrideType(operation operationPlan) string {
	if operation.HasInput {
		return "models." + operation.LinkTarget.Name
	}
	return "types.NoInput"
}
func linkInvocationType(source, target operationPlan) string {
	base := "types.LinkInvocation"
	if target.LinkTarget.OptionsRequired {
		base = "types.RequiredLinkInvocation"
	}
	return fmt.Sprintf("%s[%s,%s,%sLinkOptions]", base, linkInputType(source), linkOverrideType(target), target.Name)
}
func emitLinkLeaf(output, exports *bytes.Buffer, group linkGroupPlan, leaf linkLeafPlan) {
	source, target := group.Source, leaf.Target
	invocation := group.Name + "InvocationFor" + leaf.StatusName
	fmt.Fprintf(output, "type %s = %s\n", invocation, linkInvocationType(source, target))
	fmt.Fprintf(exports, "type %s = operations.%s\n", invocation, invocation)
	fmt.Fprintf(output, "type %s struct{client *core.Client}\n", leaf.Name)
	argument, initial := "invocations ..."+invocation, "var invocation "+invocation+";if len(invocations)>1{call.err=core.LinkFailure(\"expected zero or one Link invocation\");return call};if len(invocations)==1{invocation=invocations[0]}"
	if target.LinkTarget.OptionsRequired {
		argument = "invocation " + invocation
		initial = ""
	}
	fmt.Fprintf(output, "func(link %s)Follow(source %sLinkSource,%s)%sCall{call:=%sCall{client:link.client};%s\n", leaf.Name, source.Name, argument, target.Name, target.Name, initial)
	if target.LinkTarget.OptionsRequired {
		fmt.Fprintln(output, "options:=invocation.Options")
	} else {
		fmt.Fprintln(output, "options,_:=invocation.Options.Get()")
	}
	if target.LinkTarget.Security {
		fmt.Fprintln(output, "call.options=options.RequestOptions;call.options.SecurityRequirement=options.SecurityRequirement")
	} else {
		fmt.Fprintln(output, "call.options=options")
	}
	fmt.Fprintf(output, "call.prepareLink=func(ctx context.Context,call *%sCall)error{\n", target.Name)
	fmt.Fprintf(output, "if source==nil{return core.LinkFailure(\"Link source is nil\")};response:=core.CaptureLinkSource(func()core.LinkSource{return source.linkSourceFor%s()});if !response.Valid{return core.LinkFailure(\"Link requires an actual HTTP response\")}\n", source.Name)
	if target.LinkTarget.OptionsRequired {
		fmt.Fprintln(output, "if options.SecurityRequirement==\"\"{return core.LinkFailure(\"Link invocation requires a target security selection\")}")
	}
	fmt.Fprintln(output, "sourceFields:=make(map[string]types.Value);sourceData:=core.LinkData{}")
	if source.HasInput && len(leaf.SourceSections) != 0 {
		fmt.Fprintln(output, "if input,present:=invocation.SourceInput.Get();present{_=input")
		for _, field := range source.InputFields {
			key := strings.ToLower(field.Name)
			if leaf.SourceSections[key] {
				emitLinkSourceField(output, source, field, key)
			}
		}
		fmt.Fprintln(output, "}")
	}
	fmt.Fprintln(output, "sourceInput,err:=types.ValueFrom(sourceFields);if err!=nil{return err};sourceData.Value=sourceInput;overrideFields:=make(map[string]types.Value);overrideData:=core.LinkData{}")
	if target.HasInput {
		fmt.Fprintln(output, "override,hasOverride:=invocation.Input.Get();_=override;_=hasOverride")
		for _, field := range target.LinkTarget.Fields {
			emitLinkOverrideField(output, field)
		}
		if target.BodyUnion {
			fmt.Fprintln(output, "if hasOverride{media,factory,present,err:=override.LinkBodyOverride(ctx);if err!=nil{return err};if present{if override.Body.IsSet(){return core.LinkFailure(\"whole media body and partial media body cannot be combined\")};overrideFields[\"body\"]=media.Value;core.PutLinkMultipart(&overrideData,\"/body\",media);if source,present:=factory.Get();present{core.PutLinkFactory(&overrideData,\"/body/value\",source)}}}")
		}
	}
	fmt.Fprintf(output, "overrideInput,err:=types.ValueFrom(overrideFields);if err!=nil{return err};overrideData.Value=overrideInput;resolvedData,err:=core.ResolveLinkData(ctx,response,%s,sourceData,overrideData);if err!=nil{return err};resolved:=resolvedData.Value;_=resolved\n", leaf.Definition)
	if target.HasInput {
		for _, field := range target.LinkTarget.Fields {
			emitLinkTargetField(output, target, field)
		}
	}

	if leaf.ServerURL != "" {
		fmt.Fprintf(output, "server,err:=core.LinkURL(response,%q);if err!=nil{return err};call.options.BaseURL=types.Some(server)\n", leaf.ServerURL)
	}
	fmt.Fprintln(output, "return nil};return call}")
}
func emitSingleTargetLinkGroup(output, exports *bytes.Buffer, group linkGroupPlan) {
	target := group.Links[0].Target
	invocation := group.Name + "Invocation"
	fmt.Fprintf(output, "type %s = %s\n", invocation, linkInvocationType(group.Source, target))
	fmt.Fprintf(exports, "type %s = operations.%s\n", invocation, invocation)
	argument, initial, forward := "invocations ..."+invocation, "if len(invocations)>1{return "+target.Name+"Call{client:link.client,err:core.LinkFailure(\"expected zero or one Link invocation\")}}", "invocations..."
	if target.LinkTarget.OptionsRequired {
		argument = "invocation " + invocation
		initial = ""
		forward = "invocation"
	}
	fmt.Fprintf(output, "func(link %s)Follow(source %sLinkSource,%s)%sCall{%s;if source==nil{return %sCall{client:link.client,err:core.LinkFailure(\"Link source is nil\")}};response:=core.CaptureLinkSource(func()core.LinkSource{return source.linkSourceFor%s()});selected,best:=0,0\n", group.Name, group.Source.Name, argument, target.Name, initial, target.Name, group.Source.Name)
	for index, leaf := range group.Links {
		fmt.Fprintf(output, "if score:=core.LinkStatusScore(response.Status,%q);score>best{selected,best=%d,score}\n", leaf.Status, index+1)
	}
	fmt.Fprintln(output, "switch selected{")
	for index, leaf := range group.Links {
		fmt.Fprintf(output, "case %d:return link.ByStatus%s.Follow(source,%s)\n", index+1, leaf.StatusName, forward)
	}
	fmt.Fprintf(output, "};return %sCall{client:link.client,err:core.LinkFailure(\"no Link definition matches the source status\")}}\n", target.Name)
}

func emitDifferentTargetLinkGroup(output, exports *bytes.Buffer, group linkGroupPlan) {
	// Status-specific leaves remain concrete; the group union is emitted below.
	emitTaggedLinkGroup(output, exports, group)
}
