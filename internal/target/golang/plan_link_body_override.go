package golang

import (
	"bytes"
	"fmt"
)

// A media body has two independently optional keys. Typed value methods allow
// replacing value while keeping a Link-derived contentType (and vice versa).
func emitLinkBodyOverrideFields(output *bytes.Buffer, operation operationPlan) {
	if !operation.BodyUnion {
		return
	}
	fmt.Fprintln(output, "bodyContentType types.Optional[string]\nbodyValue func(context.Context)(types.MultipartData,types.Optional[types.SourceFactory[types.MultipartData]],error)")
}
func emitLinkBodyOverrideMethods(output *bytes.Buffer, operation operationPlan) {
	if !operation.BodyUnion {
		return
	}
	root := operation.LinkTarget.Name
	fmt.Fprintf(output, "func(value %s)WithBodyContentType(contentType string)%s{value.bodyContentType=types.Some(contentType);return value}\n", root, root)
	fmt.Fprintf(output, "func(value %s)LinkBodyOverride(ctx context.Context)(types.MultipartData,types.Optional[types.SourceFactory[types.MultipartData]],bool,error){\n", root)
	fmt.Fprintln(output, "fields:=make(map[string]types.Value);var source types.Optional[types.SourceFactory[types.MultipartData]];result:=types.MultipartData{};present:=false;if contentType,hasType:=value.bodyContentType.Get();hasType{present=true;encoded,err:=types.ValueFrom(contentType);if err!=nil{return result,source,false,err};fields[\"contentType\"]=encoded};if value.bodyValue!=nil{present=true;payload,factory,err:=value.bodyValue(ctx);if err!=nil{return result,source,false,err};source=factory;fields[\"value\"]=payload.Value;for path,binary:=range payload.Slots{if result.Slots==nil{result.Slots=make(map[string]types.Binary)};result.Slots[\"/value\"+path]=binary}};var err error;result.Value,err=types.ValueFrom(fields);return result,source,present,err}")
	for _, body := range operation.RequestBodies {
		fmt.Fprintf(output, "func(value %s)With%sBodyValue(item %s)%s{value.bodyValue=func(ctx context.Context)(types.MultipartData,types.Optional[types.SourceFactory[types.MultipartData]],error){\n", root, body.Name, body.Model, root)
		emitLinkBodyValue(output, body)
		fmt.Fprintln(output, "};return value}")
		if body.Open {
			fmt.Fprintf(output, "func(value %s)With%sBinaryBodyValue(item types.Binary)%s{value.bodyValue=func(ctx context.Context)(types.MultipartData,types.Optional[types.SourceFactory[types.MultipartData]],error){logical,err:=types.ValueFrom(map[string]types.Value{});return types.MultipartData{Value:logical,Slots:map[string]types.Binary{\"\":item}},types.Optional[types.SourceFactory[types.MultipartData]]{},err};return value}\n", root, body.Name, root)
		}
	}
}
func emitLinkBodyValue(output *bytes.Buffer, body bodyVariantPlan) {
	none := "types.Optional[types.SourceFactory[types.MultipartData]]{}"
	if body.Binary {
		fmt.Fprintf(output, "logical,err:=types.ValueFrom(map[string]types.Value{});return types.MultipartData{Value:logical,Slots:map[string]types.Binary{\"\":item}},%s,err\n", none)
		return
	}
	if body.Incremental {
		fmt.Fprintln(output, "logical,err:=types.ValueFrom(map[string]types.Value{});if err!=nil{return types.MultipartData{},types.Optional[types.SourceFactory[types.MultipartData]]{},err}")
		fmt.Fprintln(output, "var factory types.SourceFactory[types.MultipartData];if item!=nil{factory=func(ctx context.Context)(types.Source[types.MultipartData],error){source,err:=types.OpenSourceFactory(ctx,item);if err!=nil{return nil,err};return types.MapSource(ctx,source,")
		if body.Multipart {
			fmt.Fprintf(output, "EncodeMedia%s", body.Stream.ItemModel)
		} else {
			fmt.Fprintf(output, "func(ctx context.Context,item %s)(types.MultipartData,error){value,err:=Encode%s(ctx,item);return types.MultipartData{Value:value},err}", body.Stream.ItemModel, body.Stream.ItemModel)
		}
		fmt.Fprintln(output, "),nil}};return types.MultipartData{Value:logical},types.Some(factory),nil")
		return
	}
	if body.Multipart {
		fmt.Fprintf(output, "state:=&modelState{ctx:ctx,active:make(map[any]bool),binaries:make(map[string]types.Binary)};encoded,err:=encode%s(item,state);return types.MultipartData{Value:encoded,Slots:state.binaries},%s,err\n", body.Model, none)
	} else {
		fmt.Fprintf(output, "encoded,err:=encode%s(item,&modelState{ctx:ctx,active:make(map[any]bool)});return types.MultipartData{Value:encoded},%s,err\n", body.Model, none)
	}
}
