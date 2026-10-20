package golang

import (
 "testing"
 "openapi-sdkgen/internal/generator"
)

func TestGeneratedXMLWhitespacePreservation(t *testing.T){
 files,err:=planFiles(generator.GoOptions{ImportPath:"example.test/consumer/api",Package:"api"},nil,false,false,"wire-xml-output");if err!=nil{t.Fatal(err)}
 testGeneratedModule(t,generator.NewPlan("golang",sourcePlan{Files:files}),map[string]string{"api/internal/wire/whitespace_test.go":`package wire
import("context";"encoding/xml";"testing")
func TestXMLWhitespace(t *testing.T){value:=func(text string)Value{return Text(text)};root,_:=Parse([]byte("{}"));mapping:=&XMLMapping{Value:root,Fields:[]*XMLMapping{{Value:value("a\nb\tc\rd"),Metadata:XMLOutputMetadata{Name:"a",Attribute:true,Defined:XMLDefinedName|XMLDefinedAttribute}},{Value:value("z\rw]]>q"),Metadata:XMLOutputMetadata{NodeType:"cdata"}},{Value:value("x\ry"),Metadata:XMLOutputMetadata{Name:"text",Defined:XMLDefinedName}}}};data,err:=RenderXML(context.Background(),mapping,"root");if err!=nil{t.Fatal(err)};want:="<root a=\"a&#10;b&#9;c&#13;d\"><![CDATA[z]]>&#13;<![CDATA[w]]]]><![CDATA[>q]]><text>x&#13;y</text></root>";if string(data)!=want{t.Fatal(string(data))};var decoded struct{A string `+"`xml:\"a,attr\"`"+`;Text string `+"`xml:\"text\"`"+`;CDATA string `+"`xml:\",chardata\"`"+`};if err:=xml.Unmarshal(data,&decoded);err!=nil||decoded.A!="a\nb\tc\rd"||decoded.Text!="x\ry"||decoded.CDATA!="z\rw]]>q"{t.Fatal(decoded,err)}}
`})
}
