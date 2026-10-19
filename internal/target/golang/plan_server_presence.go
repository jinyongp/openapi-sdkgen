package golang

import (
	"bytes"
	"fmt"
)

// Returns whether an optional body's decoding block must be closed afterward.
func emitInboundBodyPresence(body *bytes.Buffer, field inputFieldPlan) bool {
	if field.Stream != nil {
		if !field.Required && !field.Incremental {
			fmt.Fprintln(body, "if execution.HasBody(){")
			return true
		}
		return false
	}
	if field.Multipart {
		if !field.Required {
			fmt.Fprintln(body, "present:=true")
		}
		return false
	}
	if field.Binary {
		return false
	}
	if field.Required {
		fmt.Fprintln(body, "if _,err:=runtime.BodyPresent(execution,contentType,true);err!=nil{return runtime.Response{},err}")
		return false
	}
	fmt.Fprintln(body, "present,err:=runtime.BodyPresent(execution,contentType,false);if err!=nil{return runtime.Response{},err};if present{")
	return true
}
