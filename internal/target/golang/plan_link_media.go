package golang

import (
	"bytes"
	"fmt"
)

func emitLinkSourceField(output *bytes.Buffer, source operationPlan, field inputFieldPlan, key string) {
	if field.Required {
		fmt.Fprintf(output, "{item:=input.%s;", field.Name)
	} else {
		fmt.Fprintf(output, "if item,present:=input.%s.Get();present{", field.Name)
	}
	pointer := "/" + key
	switch {
	case field.Binary:
		fmt.Fprintf(output, "sourceFields[%q]=linkLiteral(\"{}\");core.PutLinkBinary(&sourceData,%q,item)\n", key, pointer)
	case field.Union:
		fmt.Fprintln(output, "contentType,err:=item.MediaType();if err!=nil{return err};logical:=linkLiteral(\"{}\")")
		fmt.Fprintf(output, "if binary,present:=item.Binary();present{core.PutLinkBinary(&sourceData,%q,binary)}else{handled:=false\n", pointer+"/value")
		for _, body := range source.RequestBodies {
			if body.Incremental {
				fmt.Fprintf(output, "if factory,present:=item.As%s();present{handled=true;core.PutLinkFactory(&sourceData,%q,%s)}\n", body.Name, pointer+"/value", linkSourceFactoryExpression(body.Stream, "factory"))
			}
		}
		fmt.Fprint(output, "if !handled{")
		if source.HasMultipart {
			fmt.Fprintf(output, "media,present,err:=item.MultipartBound(ctx);if err!=nil{return err};if present{logical=media.Value;core.PutLinkMultipart(&sourceData,%q,media)}else{logical,err=item.EncodeBound(ctx);if err!=nil{return err}}\n", pointer+"/value")
		} else {
			fmt.Fprintln(output, "logical,err=item.EncodeBound(ctx);if err!=nil{return err}")
		}
		fmt.Fprintf(output, "}};encodedType,err:=types.ValueFrom(contentType);if err!=nil{return err};sourceFields[%q],err=types.ValueFrom(map[string]types.Value{\"contentType\":encodedType,\"value\":logical});if err!=nil{return err}\n", key)
	case field.Incremental:
		fmt.Fprintf(output, "sourceFields[%q]=linkLiteral(\"{}\");core.PutLinkFactory(&sourceData,%q,%s)\n", key, pointer, linkSourceFactoryExpression(field.Stream, "item"))
	case field.Multipart:
		fmt.Fprintf(output, "value,err:=models.EncodeBoundMedia%s(ctx,item);if err!=nil{return err};sourceFields[%q]=value.Value;core.PutLinkMultipart(&sourceData,%q,value)\n", field.Type, key, pointer)
	default:
		fmt.Fprintf(output, "value,err:=models.EncodeBound%s(ctx,item);if err!=nil{return err};sourceFields[%q]=value\n", field.Type, key)
	}
	fmt.Fprintln(output, "}")
}
func linkSourceFactoryExpression(stream *requestStreamPlan, factory string) string {
	if stream.Multipart {
		return fmt.Sprintf("core.LinkSourceFactory(%s,models.EncodeBoundMedia%s)", factory, stream.ItemModel)
	}
	return fmt.Sprintf("core.LinkSourceFactory(%s,func(ctx context.Context,item models.%s)(types.MultipartData,error){value,err:=models.EncodeBound%s(ctx,item);return types.MultipartData{Value:value},err})", factory, stream.ItemModel, stream.ItemModel)
}
func linkTargetFactoryExpression(stream *requestStreamPlan, factory string) string {
	if stream.Multipart {
		return fmt.Sprintf("core.LinkTargetFactory(%s,models.DecodeMedia%s)", factory, stream.ItemModel)
	}
	return fmt.Sprintf("core.LinkTargetFactory(%s,func(ctx context.Context,item types.MultipartData)(models.%s,error){if len(item.Slots)!=0{var zero models.%s;return zero,core.LinkFailure(\"binary Link item cannot be represented by this JSON DTO\")};return models.Decode%s(ctx,item.Value)})", factory, stream.ItemModel, stream.ItemModel, stream.ItemModel)
}
func emitLinkOverrideField(output *bytes.Buffer, field linkOverrideFieldPlan) {
	if field.Native {
		return
	}
	if field.Multipart {
		fmt.Fprintf(output, "if hasOverride{if item,present:=override.%s.Get();present{value,err:=models.EncodeBoundMedia%s(ctx,item);if err!=nil{return err};overrideFields[%q]=value.Value;core.PutLinkMultipart(&overrideData,%q,value)}}\n", field.Name, field.Type, field.Key, "/"+field.Key)
	} else {
		fmt.Fprintf(output, "if hasOverride{if item,present:=override.%s.Get();present{value,err:=models.EncodeBound%s(ctx,item);if err!=nil{return err};overrideFields[%q]=value}}\n", field.Name, field.Type, field.Key)
	}
}
func emitLinkTargetField(output *bytes.Buffer, target operationPlan, field linkOverrideFieldPlan) {
	if field.Native {
		fmt.Fprintf(output, "{provided:=false;if hasOverride{if item,present:=override.%s.Get();present{provided=true;call.input.%s=", field.Name, field.Name)
		linkAssign(output, field.Original, "item")
		fmt.Fprintln(output, "}}")
		fmt.Fprintf(output, "if !provided{data,present,err:=core.LinkDataAt(resolvedData,%q);if err!=nil{return err};if present{\n", "/"+field.Key)
		switch {
		case field.Original.Binary:
			fmt.Fprint(output, "if binary,present:=data.Binaries[\"\"];present{call.input.")
			fmt.Fprint(output, field.Name, "=")
			linkAssign(output, field.Original, "binary")
			fmt.Fprintln(output, "}else{call.linkBodyValue=types.Some(data.Value)}")
		case field.Original.Union:
			emitLinkUnionBody(output, target, field.Original)
		case field.Original.Incremental:
			fmt.Fprintln(output, "factory,present:=data.Sources[\"\"];if !present{factory,err=core.LinkArrayFactory(data);if err!=nil{return err}}")
			fmt.Fprintf(output, "call.input.%s=", field.Name)
			linkAssign(output, field.Original, linkTargetFactoryExpression(field.Original.Stream, "factory"))
			fmt.Fprintln(output)
		}
		if field.Original.Required {
			fmt.Fprintln(output, "}else{return core.LinkFailure(\"required Link input section is missing\")}")
		} else {
			fmt.Fprintln(output, "}")
		}
		fmt.Fprintln(output, "}}")
		return
	}
	if field.Original.Required {
		fmt.Fprintf(output, "{value,present:=resolved.Get(%q);if !present{return core.LinkFailure(\"required Link input section is missing\")};", field.Key)
	} else {
		fmt.Fprintf(output, "if value,present:=resolved.Get(%q);present{", field.Key)
	}
	if field.Multipart {
		fmt.Fprintf(output, "data,_,err:=core.LinkDataAt(resolvedData,%q);if err!=nil{return err};decoded,err:=models.DecodeMedia%s(ctx,types.MultipartData{Value:value,Slots:data.Binaries});", "/"+field.Key, field.Original.Type)
	} else {
		fmt.Fprintf(output, "decoded,err:=models.Decode%s(ctx,value);", field.Original.Type)
	}
	fmt.Fprintf(output, "if err!=nil{return err};call.input.%s=", field.Name)
	linkAssign(output, field.Original, "decoded")
	fmt.Fprintln(output, "}")
}
func linkAssign(output *bytes.Buffer, field inputFieldPlan, value string) {
	if field.Required {
		fmt.Fprint(output, value)
	} else {
		fmt.Fprintf(output, "types.Some(%s)", value)
	}
}
func emitLinkUnionBody(output *bytes.Buffer, target operationPlan, field inputFieldPlan) {
	fmt.Fprintln(output, "contentTypeValue,hasType:=data.Value.Get(\"contentType\");contentType,isString:=contentTypeValue.String();payload,hasPayload,err:=core.LinkDataAt(data,\"/value\");if err!=nil{return err};if !hasType||!isString||!hasPayload{return core.LinkFailure(\"Link media body requires contentType and value\")};selected,best:=0,0")
	for _, body := range target.RequestBodies {
		if body.Stream != nil && body.Stream.Complete && body.Stream.ItemValidation != "" {
			fmt.Fprintln(output, "{_,factoryBody:=payload.Sources[\"\"];")
			if body.Incremental {
				fmt.Fprintln(output, "if factoryBody{")
			} else {
				fmt.Fprintln(output, "if !factoryBody{")
			}
		}
		fmt.Fprintf(output, "if score:=core.ResponseScore(200,contentType,\"200\",%q);score>best{selected,best=%d,score}\n", body.MediaType, body.Index)
		if body.Stream != nil && body.Stream.Complete && body.Stream.ItemValidation != "" {
			fmt.Fprintln(output, "}}")
		}
	}
	fmt.Fprintln(output, "switch selected{")
	for _, body := range target.RequestBodies {
		fmt.Fprintf(output, "case %d:\n", body.Index)
		if body.Incremental {
			fmt.Fprintln(output, "factory,present:=payload.Sources[\"\"];if !present{factory,err=core.LinkArrayFactory(payload);if err!=nil{return err}}")
			fmt.Fprintf(output, "decoded:=%s\n", linkTargetFactoryExpression(body.Stream, "factory"))
		} else if body.Binary {
			fmt.Fprintln(output, "decoded,present:=payload.Binaries[\"\"];if !present{call.linkBodyValue=types.Some(payload.Value)}")
		} else if body.Open {
			fmt.Fprintf(output, "if binary,present:=payload.Binaries[\"\"];present{call.input.%s=", field.Name)
			linkAssign(output, field, fmt.Sprintf("models.New%s%sBinaryBody(contentType,binary)", target.Name, body.Name))
			fmt.Fprintln(output, ";break}")
			fmt.Fprintf(output, "decoded,err:=models.Decode%s(ctx,payload.Value);if err!=nil{return err}\n", body.Model)
		} else if body.Multipart {
			fmt.Fprintf(output, "decoded,err:=models.DecodeMedia%s(ctx,types.MultipartData{Value:payload.Value,Slots:payload.Binaries});if err!=nil{return err}\n", body.Model)
		} else {
			fmt.Fprintf(output, "decoded,err:=models.Decode%s(ctx,payload.Value);if err!=nil{return err}\n", body.Model)
		}
		argument := "decoded"
		if body.Wildcard {
			argument = "contentType," + argument
		}
		fmt.Fprintf(output, "call.input.%s=", field.Name)
		linkAssign(output, field, fmt.Sprintf("models.New%s%sBody(%s).WithContentType(contentType)", target.Name, body.Name, argument))
		fmt.Fprintln(output)
	}
	fmt.Fprintln(output, "default:return core.LinkFailure(\"Link media type is not declared by the target\")}")
}
