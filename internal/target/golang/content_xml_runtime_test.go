package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

func TestAmbiguousXMLRepresentationIsTargetDiagnostic(t *testing.T) {
	const input = `{"openapi":"3.1.0","info":{"title":"Ambiguous XML","version":"1"},"paths":{},"components":{"schemas":{"Payload":{"type":"string","contentMediaType":"application/xml","contentSchema":{"allOf":[{"type":"object","xml":{"name":"first"}},{"type":"object","xml":{"name":"second"}}]}}}}}`
	document, err := compiler.Compile([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	_, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 1 {
		t.Fatalf("expected author diagnostic, got %v %v", err, diagnostics)
	}
	if got := diagnostics[0]; got.Target != "golang" || got.Phase != diagnostic.PhaseTarget || got.Code != "GO_XML_REPRESENTATION" || got.Location.Pointer == "" {
		t.Fatalf("incomplete XML diagnostic: %+v", got)
	}
}

func TestGeneratedXMLContentSchema(t *testing.T) {
	const input = `openapi: 3.2.0
info: {title: XML content, version: "1"}
paths: {}
components:
  schemas:
    Payload:
      type: string
      contentMediaType: application/xml
      contentSchema:
        allOf:
          - {xml: {x-annotation: {name: first}}}
          - {xml: {x-annotation: {name: second}}}
        type: object
        required: [id, message, items]
        properties:
          id: {type: integer, minimum: 1, xml: {attribute: true, name: code}}
          message: {type: string, minLength: 1, xml: {name: body, namespace: 'urn:message', prefix: m}}
          items:
            type: array
            xml: {name: items, wrapped: true}
            items: {type: boolean, xml: {name: item}}
    Text:
      type: string
      contentMediaType: application/example+xml
      contentSchema:
        type: object
        required: [text, tag]
        properties:
          text: {type: string, minLength: 1, xml: {nodeType: cdata}}
          tag: {type: string, xml: {nodeType: attribute}}
          hidden: {type: string, xml: {nodeType: none}}
    Choice:
      type: string
      contentMediaType: application/xml
      contentSchema:
        oneOf:
          - type: object
            required: [amount]
            properties: {amount: {type: integer, xml: {name: numeric}}}
          - type: object
            required: [message]
            properties: {message: {type: string, xml: {name: text}}}
    Conditional:
      type: string
      contentMediaType: application/xml
      contentSchema:
        type: object
        properties: {kind: {type: string, xml: {attribute: true}}}
        if: {required: [kind], properties: {kind: {const: number}}}
        then: {required: [value], properties: {value: {type: integer, xml: {name: numeric}}}}
        else: {required: [value], properties: {value: {type: string, xml: {name: text}}}}
    TreeContent:
      type: string
      contentMediaType: application/xml
      contentSchema: {$ref: '#/components/schemas/Tree'}
    Tree:
      type: object
      required: [value]
      properties:
        value: {type: integer}
        children:
          type: array
          xml: {wrapped: true}
          items: {$ref: '#/components/schemas/Tree', xml: {name: child}}
    Fallback:
      anyOf:
        - {type: string, contentMediaType: application/xml, contentSchema: {type: integer}}
        - {type: string, pattern: '^raw$'}
    DynamicContent:
      type: string
      contentMediaType: application/xml
      contentSchema: {$ref: '#/components/schemas/Strict'}
    Base:
      $id: https://schemas.example.test/xml/base
      $dynamicAnchor: node
      type: object
      required: [value]
      properties:
        value: {type: integer}
        children:
          type: array
          xml: {wrapped: true}
          items: {$dynamicRef: '#node', xml: {name: child}}
    Strict:
      $id: https://schemas.example.test/xml/strict
      $dynamicAnchor: node
      $ref: '#/components/schemas/Base'
      required: [extra]
      properties: {extra: {type: boolean}}
`
	document, err := compiler.Compile([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare: %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"api/models/xml_content_test.go": xmlContentConsumer, "api/internal/wire/xml_test.go": xmlParserConsumer})
}

const xmlContentConsumer = `package models
import("context";"testing";"example.test/consumer/api/types")
func TestXMLContent(t *testing.T){
 ctx:=context.Background()
 for _,test:=range []struct{source string;valid bool}{
  {"<root code='2' xmlns:other='urn:message'><other:body>Hello &amp; bye</other:body><items><item>true</item><item>false</item></items></root>",true},
  {"<root code='0' xmlns:m='urn:message'><m:body>Hello</m:body><items/></root>",false},
  {"<root code='2' xmlns:m='urn:wrong'><m:body>Hello</m:body><items/></root>",false},
  {"<root code='2' xmlns:m='urn:message'><m:body>Hello</m:body><items><item>1</item></items></root>",false},
 }{value,_:=types.ValueFrom(test.source);decoded,err:=DecodePayloadOutput(ctx,value);if (err==nil)!=test.valid{t.Errorf("%s: %v",test.source,err)};if err==nil&&string(decoded)!=test.source{t.Fatal("XML content changed outer string")}}
 for _,source:=range []string{"<root><numeric>2</numeric></root>","<root><text>message</text></root>"}{value,_:=types.ValueFrom(source);if _,err:=DecodeChoiceOutput(ctx,value);err!=nil{t.Fatal(source,err)}}
 invalid,_:=types.ValueFrom("<root><numeric>2</numeric><text>ambiguous</text></root>");if _,err:=DecodeChoiceOutput(ctx,invalid);err==nil{t.Fatal("ambiguous XML oneOf accepted")}
 for _,source:=range []string{"<root kind='number'><numeric>2</numeric></root>","<root kind='text'><text>value</text></root>"}{value,_:=types.ValueFrom(source);if _,err:=DecodeConditionalOutput(ctx,value);err!=nil{t.Fatal(source,err)}}
 text,_:=types.ValueFrom("<root tag='ok'><![CDATA[a < b]]></root>");if _,err:=DecodeTextOutput(ctx,text);err!=nil{t.Fatal(err)}
 tree,_:=types.ValueFrom("<root><value>1</value><children><child><value>2</value><children/></child></children></root>");if _,err:=DecodeTreeContentOutput(ctx,tree);err!=nil{t.Fatal(err)}
 raw,_:=types.ValueFrom("raw");if _,err:=DecodeFallbackOutput(ctx,raw);err!=nil{t.Fatal("XML mismatch prevented anyOf fallback",err)}
 for _,test:=range []struct{source string;valid bool}{
  {"<root><value>1</value><extra>true</extra><children><child><value>2</value><extra>false</extra><children/></child></children></root>",true},
  {"<root><value>1</value><extra>true</extra><children><child><value>2</value><children/></child></children></root>",false},
 }{value,_:=types.ValueFrom(test.source);_,err:=DecodeDynamicContentOutput(ctx,value);if (err==nil)!=test.valid{t.Errorf("XML dynamic scope %s: %v",test.source,err)}}
}
`

const xmlParserConsumer = `package wire
import("context";"testing")
func TestXMLParserBoundaries(t *testing.T){
 for _,source:=range []string{"<a/><b/>","<a></b>","text<a/>","<p:a/>","<a p:v='1'/>","<a x='1' x='2'/>","<a xmlns:p='urn:x' xmlns:q='urn:x' p:v='1' q:v='2'/>","<!DOCTYPE a><a/>","<a>&unknown;</a>","<a xmlns:xml='urn:wrong'/>","<a xmlns:p=''/>","<a>\x00</a>","<![CDATA[ ]]><a/>"}{if _,err:=ParseXML(context.Background(),source);err==nil{t.Errorf("accepted malformed XML %s",source)}}
 node,err:=ParseXML(context.Background(),"<?xml version='1.0'?><a xmlns='urn:x' xmlns:p='urn:y' p:v='1'>x<!-- comment --><![CDATA[y]]><b/>z</a>");if err!=nil{t.Fatal(err)}
 if node.Name.Namespace!="urn:x"||node.Attributes[0].Name.Namespace!="urn:y"||node.Text!="xyz"||node.Children[0].Name.Namespace!="urn:x"{t.Fatalf("XML namespace/text parsing: %+v",node)}
 normalized,err:=ParseXML(context.Background(),"<a v='a\tb\nc&#x9;d'><![CDATA[a\tb\nc]]></a>");if err!=nil{t.Fatal(err)}
 if normalized.Attributes[0].Value!="a b c\td"||normalized.Text!="a\tb\nc"{t.Fatalf("attribute whitespace normalization: %+v",normalized)}
 canceled,cancel:=context.WithCancel(context.Background());cancel();if _,err:=ParseXML(canceled,"<a/>");err!=context.Canceled{t.Fatal(err)}
}
`
