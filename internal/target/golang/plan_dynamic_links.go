package golang

import (
	"bytes"
	"fmt"
)

func emitDynamicLinks(output *bytes.Buffer, source linkSourcePlan) {
	fmt.Fprintf(output, "func dynamicLinksFor%s(client *core.Client)DynamicLinks{return DynamicLinks{names:[]string{", source.Operation.Name)
	for _, group := range source.Groups {
		fmt.Fprintf(output, "%q,", group.OriginalName)
	}
	fmt.Fprintf(output, "},follow:func(name,status string,source *DynamicRawResponse,invocation DynamicLinkInvocation)DynamicCall{if source==nil||source.sourceOperation!=%q||!source.source.Valid{return DynamicCall{failure:dynamicFailure(\"Link requires a response from this operation\")}};switch name{\n", source.Operation.Route)
	for _, group := range source.Groups {
		fmt.Fprintf(output, "case %q:selected,best:=0,0\n", group.OriginalName)
		for index, leaf := range group.Links {
			fmt.Fprintf(output, "if status==%q{selected,best=%d,4}else if status==\"\"{if score:=core.LinkStatusScore(source.source.Status,%q);score>best{selected,best=%d,score}}\n", leaf.Status, index+1, leaf.Status, index+1)
		}
		fmt.Fprintln(output, "_=best;switch selected{")
		for index, leaf := range group.Links {
			fmt.Fprintf(output, "case %d:options,_:=invocation.Options.Get();return DynamicCall{options:core.CloneOptions(options),prepare:func(ctx context.Context,call *DynamicCall)(failure error){defer func(){if failure!=nil{failure=core.RequestFailure(ctx,types.OperationInfo{OperationID:%q,Route:%q,Method:%q,Path:%q},types.RequestEncodeFailed,failure,nil,nil)}}();\n", index+1, leaf.Target.OperationID, leaf.Target.Route, leaf.Target.Method, leaf.Target.Path)
			if leaf.Target.LinkTarget.OptionsRequired {
				fmt.Fprintln(output, "if call.options.SecurityRequirement==nil{return dynamicFailure(\"Link requires a target security selection\")}")
			}
			fmt.Fprintf(output, "input,_:=invocation.SourceInput.Get();override,_:=invocation.Input.Get();sourceData:=core.LinkData{Value:input};if native,present:=invocation.SourceNative.Get();present{sourceData=native};overrideData:=core.LinkData{Value:override};if native,present:=invocation.InputNative.Get();present{overrideData=native};resolved,err:=core.ResolveLinkData(ctx,source.source,%s,sourceData,overrideData);if err!=nil{return err};target:=newDynamic%sData(client,resolved);target.options=call.options;\n", leaf.Definition, leaf.Target.Name)
			if leaf.ServerURL != "" {
				fmt.Fprintf(output, "server,err:=core.LinkURL(source.source,%q);if err!=nil{return err};target.options.BaseURL=types.Some(server)\n", leaf.ServerURL)
			}
			fmt.Fprintln(output, "*call=target;return nil}}")
		}
		fmt.Fprintln(output, "};return DynamicCall{failure:dynamicFailure(\"no Link definition matches the selected status\")}")
	}
	fmt.Fprintln(output, "};return DynamicCall{failure:dynamicFailure(\"unknown response Link\")}}}}")
}
