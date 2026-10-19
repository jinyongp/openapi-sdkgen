package golang

import (
	"bytes"
	"fmt"
)

func inboundHeaderGroup(operation operationPlan, status string) responseHeaderPlan {
	for _, group := range operation.Headers {
		if group.Status == status {
			return group
		}
	}
	return responseHeaderPlan{}
}

func emitInboundHeaders(body *bytes.Buffer, entry inboundOperationPlan, group responseHeaderPlan) {
	fmt.Fprintf(body, "func encode%s%sHeaders(execution *runtime.Execution,value models.%s,headers http.Header)(http.Header,error){logical,err:=models.Encode%s(execution.Context,value);if err!=nil{return nil,err};return runtime.EncodeHeaders(execution,logical,[]runtime.Header{\n", entry.PublicName, group.Name, group.Model, group.Model)
	for _, field := range group.Fields {
		decoder := "nil"
		if isXMLMedia(field.ContentType) {
			decoder = "schema.DecodeXMLHTTP__" + field.Validation + "__"
		}
		fmt.Fprintf(body, "{Name:%q,ContentType:%q,Explode:%t,Encode:%s,Decode:%s,Schema:headerShapesFor%s()[%q]},\n", field.Name, field.ContentType, field.Explode, inboundXMLEncoder(entry, "header:"+group.Status+":"+field.Name), decoder, entry.PublicName, group.Status+":"+field.Name)
	}
	fmt.Fprintln(body, "},headers)}")
}
