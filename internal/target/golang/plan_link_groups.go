package golang

import (
	"bytes"
	"fmt"
)

func emitTaggedLinkGroup(output, exports *bytes.Buffer, group linkGroupPlan) {
	name := group.Name
	fmt.Fprintf(output, "type %sInput struct{branch int\n", name)
	for _, leaf := range group.Links {
		fmt.Fprintf(output, "input%s %s\n", leaf.StatusName, linkOverrideType(leaf.Target))
	}
	fmt.Fprintln(output, "}")
	fmt.Fprintf(output, "type %sOptions struct{branch int\n", name)
	for _, leaf := range group.Links {
		fmt.Fprintf(output, "options%s %sLinkOptions\n", leaf.StatusName, leaf.Target.Name)
	}
	fmt.Fprintln(output, "}")
	for _, leaf := range group.Links {
		inputType := linkOverrideType(leaf.Target)
		fmt.Fprintf(output, "func %sInputFor%s(input %s)%sInput{return %sInput{branch:%d,input%s:input}}\n", name, leaf.StatusName, inputType, name, name, linkLeafIndex(group, leaf), leaf.StatusName)
		fmt.Fprintf(output, "func %sOptionsFor%s(options %sLinkOptions)%sOptions{return %sOptions{branch:%d,options%s:options}}\n", name, leaf.StatusName, leaf.Target.Name, name, name, linkLeafIndex(group, leaf), leaf.StatusName)
		fmt.Fprintf(exports, "func %sInputFor%s(input %s)%sInput{return operations.%sInputFor%s(input)}\n", name, leaf.StatusName, inputType, name, name, leaf.StatusName)
		fmt.Fprintf(exports, "func %sOptionsFor%s(options %sLinkOptions)%sOptions{return operations.%sOptionsFor%s(options)}\n", name, leaf.StatusName, leaf.Target.Name, name, name, leaf.StatusName)
	}
	fmt.Fprintf(output, "type %sInvocation struct{SourceInput types.Optional[%s];Input types.Optional[%sInput];Options ", name, linkInputType(group.Source), name)
	if group.OptionsRequired {
		fmt.Fprintf(output, "%sOptions", name)
	} else {
		fmt.Fprintf(output, "types.Optional[%sOptions]", name)
	}
	fmt.Fprintln(output, "}")
	for _, suffix := range []string{"Input", "Options", "Invocation", "Request", "Output", "RawResponse"} {
		fmt.Fprintf(exports, "type %s%s = operations.%s%s\n", name, suffix, name, suffix)
	}
	fmt.Fprintf(output, "type %sRequest struct{branch int;err error\n", name)
	for _, leaf := range group.Links {
		fmt.Fprintf(output, "call%s %sCall\n", leaf.StatusName, leaf.Target.Name)
	}
	fmt.Fprintln(output, "}")
	fmt.Fprintf(output, "type %sOutput struct{branch int\n", name)
	for _, leaf := range group.Links {
		fmt.Fprintf(output, "output%s %s\n", leaf.StatusName, linkOutputType(leaf.Target))
	}
	fmt.Fprintln(output, "}")
	fmt.Fprintf(output, "type %sRawResponse struct{branch int\n", name)
	for _, leaf := range group.Links {
		fmt.Fprintf(output, "raw%s *%sRawResponse\n", leaf.StatusName, leaf.Target.Name)
	}
	fmt.Fprintln(output, "}")
	for _, leaf := range group.Links {
		fmt.Fprintf(output, "func(value %sOutput)As%s()(%s,bool){return value.output%s,value.branch==%d}\n", name, leaf.StatusName, linkOutputType(leaf.Target), leaf.StatusName, linkLeafIndex(group, leaf))
		fmt.Fprintf(output, "func(value %sRawResponse)As%s()(*%sRawResponse,bool){return value.raw%s,value.branch==%d}\n", name, leaf.StatusName, leaf.Target.Name, leaf.StatusName, linkLeafIndex(group, leaf))
	}
	fmt.Fprintf(output, "func(request %sRequest)Do(ctx context.Context)(%sOutput,error){if request.err!=nil{return %sOutput{},request.err};switch request.branch{\n", name, name, name)
	for _, leaf := range group.Links {
		if leaf.Target.CanDo {
			fmt.Fprintf(output, "case %d:value,err:=request.call%s.Do(ctx);return %sOutput{branch:%d,output%s:value},err\n", linkLeafIndex(group, leaf), leaf.StatusName, name, linkLeafIndex(group, leaf), leaf.StatusName)
		}
	}
	fmt.Fprintf(output, "};return %sOutput{},core.LinkFailure(\"selected Link target requires a streaming terminal\")}\n", name)
	fmt.Fprintf(output, "func(request %sRequest)Raw(ctx context.Context)(%sRawResponse,error){if request.err!=nil{return %sRawResponse{},request.err};switch request.branch{\n", name, name, name)
	for _, leaf := range group.Links {
		fmt.Fprintf(output, "case %d:value,err:=request.call%s.Raw(ctx);return %sRawResponse{branch:%d,raw%s:value},err\n", linkLeafIndex(group, leaf), leaf.StatusName, name, linkLeafIndex(group, leaf), leaf.StatusName)
	}
	fmt.Fprintf(output, "};return %sRawResponse{},core.LinkFailure(\"no Link target was selected\")}\n", name)
	argument := "invocations ..." + name + "Invocation"
	initial := "var invocation " + name + "Invocation;if len(invocations)>1{return " + name + "Request{err:core.LinkFailure(\"expected zero or one Link invocation\")}};if len(invocations)==1{invocation=invocations[0]}"
	if group.OptionsRequired {
		argument = "invocation " + name + "Invocation"
		initial = ""
	}
	fmt.Fprintf(output, "func(link %s)Follow(source %sLinkSource,%s)%sRequest{%s;if source==nil{return %sRequest{err:core.LinkFailure(\"Link source is nil\")}};response:=core.CaptureLinkSource(func()core.LinkSource{return source.linkSourceFor%s()});selected,best:=0,0\n", name, group.Source.Name, argument, name, initial, name, group.Source.Name)
	for _, leaf := range group.Links {
		fmt.Fprintf(output, "if score:=core.LinkStatusScore(response.Status,%q);score>best{selected,best=%d,score}\n", leaf.Status, linkLeafIndex(group, leaf))
	}
	fmt.Fprintln(output, "input,hasInput:=invocation.Input.Get()")
	if group.OptionsRequired {
		fmt.Fprintln(output, "options:=invocation.Options;hasOptions:=options.branch!=0")
	} else {
		fmt.Fprintln(output, "options,hasOptions:=invocation.Options.Get()")
	}
	fmt.Fprintf(output, "if hasInput&&input.branch!=selected||hasOptions&&options.branch!=selected{return %sRequest{err:core.LinkFailure(\"Link invocation tag does not match the selected definition\")}};switch selected{\n", name)
	for _, leaf := range group.Links {
		fmt.Fprintf(output, "case %d:invocationFor%s:=%sInvocationFor%s{SourceInput:invocation.SourceInput};if hasInput{invocationFor%s.Input=types.Some(input.input%s)}\n", linkLeafIndex(group, leaf), leaf.StatusName, name, leaf.StatusName, leaf.StatusName, leaf.StatusName)
		if leaf.Target.LinkTarget.OptionsRequired {
			fmt.Fprintf(output, "if !hasOptions{return %sRequest{err:core.LinkFailure(\"selected Link target requires options\")}};invocationFor%s.Options=options.options%s\n", name, leaf.StatusName, leaf.StatusName)
		} else {
			fmt.Fprintf(output, "if hasOptions{invocationFor%s.Options=types.Some(options.options%s)}\n", leaf.StatusName, leaf.StatusName)
		}
		fmt.Fprintf(output, "return %sRequest{branch:%d,call%s:link.ByStatus%s.Follow(source,invocationFor%s)}\n", name, linkLeafIndex(group, leaf), leaf.StatusName, leaf.StatusName, leaf.StatusName)
	}
	fmt.Fprintf(output, "};return %sRequest{err:core.LinkFailure(\"no Link definition matches the source status\")}}\n", name)
	emitTaggedLinkStream(output, exports, group)
}
func linkLeafIndex(group linkGroupPlan, leaf linkLeafPlan) int {
	for index, item := range group.Links {
		if item.Status == leaf.Status {
			return index + 1
		}
	}
	return 0
}
func linkOutputType(operation operationPlan) string {
	if operation.OutputUnion {
		return operation.Name + "Output"
	}
	return operation.OutputType
}
func emitTaggedLinkStream(output, exports *bytes.Buffer, group linkGroupPlan) {
	var leaves []linkLeafPlan
	for _, leaf := range group.Links {
		if leaf.Target.HasTypedStream {
			leaves = append(leaves, leaf)
		}
	}
	if len(leaves) == 0 {
		return
	}
	name := group.Name
	fmt.Fprintf(output, "type %sStreamItem struct{branch int\n", name)
	for _, leaf := range leaves {
		fmt.Fprintf(output, "item%s %s\n", leaf.StatusName, leaf.Target.StreamType)
	}
	fmt.Fprintln(output, "}")
	fmt.Fprintf(exports, "type %sStreamItem = operations.%sStreamItem\n", name, name)
	for _, leaf := range leaves {
		fmt.Fprintf(output, "func(value %sStreamItem)As%s()(%s,bool){return value.item%s,value.branch==%d}\n", name, leaf.StatusName, leaf.Target.StreamType, leaf.StatusName, linkLeafIndex(group, leaf))
	}
	fmt.Fprintf(output, "func(request %sRequest)Stream(ctx context.Context)*types.Stream[%sStreamItem]{return types.NewStream(ctx,func(ctx context.Context)(types.Source[%sStreamItem],*http.Response,error){if request.err!=nil{return nil,nil,request.err};switch request.branch{\n", name, name, name)
	for _, leaf := range leaves {
		fmt.Fprintf(output, "case %d:stream:=request.call%s.Stream(ctx);mapped:=types.MapSource(ctx,stream,func(ctx context.Context,item %s)(%sStreamItem,error){return %sStreamItem{branch:%d,item%s:item},nil});response,err:=stream.Response();return mapped,response,err\n", linkLeafIndex(group, leaf), leaf.StatusName, leaf.Target.StreamType, name, name, linkLeafIndex(group, leaf), leaf.StatusName)
	}
	fmt.Fprintln(output, "};return nil,nil,core.LinkFailure(\"selected Link target has no typed stream\")})}")
}
