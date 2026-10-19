package golang

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/generator"
)

type regexSample struct{ Name, Pattern, Input string }
type regexOracle struct{ Valid, Matches bool }

func TestGeneratedRegexNodeOracleAndExecutionErrors(t *testing.T) {
	data, err := os.ReadFile("testdata/regex-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []regexSample
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "node", "-e", `const fs=require('node:fs');const cases=JSON.parse(fs.readFileSync(0,'utf8'));console.log(JSON.stringify(cases.map(c=>{try{return {Valid:true,Matches:new RegExp(c.pattern,'u').test(c.input)}}catch{return {Valid:false,Matches:false}}})))`)
	command.Stdin = bytes.NewReader(data)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Node oracle: %v %s", err, output)
	}
	var oracle []regexOracle
	if err := json.Unmarshal(output, &oracle); err != nil {
		t.Fatal(err)
	}
	if len(oracle) != len(cases) {
		t.Fatal("incomplete Node oracle")
	}
	document := &ir.Document{OpenAPIVersionLine: "3.1"}
	var source strings.Builder
	source.WriteString("package schema\nimport(\"context\";\"errors\";\"strings\";\"sync\";\"testing\";\"example.test/consumer/api/internal/runtime/ecmaregexp\";\"example.test/consumer/api/internal/wire\")\nfunc TestOracle(t *testing.T){ctx:=context.Background()\n")
	operation := func(index int, schema any) ir.Operation {
		return ir.Operation{RouteKey: fmt.Sprintf("GET /regex%d", index), OperationID: fmt.Sprintf("regex%d", index), Method: "GET", Path: fmt.Sprintf("/regex%d", index), Responses: []ir.Response{{Status: "200", Content: []ir.MediaType{{ContentType: "application/json", Schema: schema}}}}}
	}
	for index, test := range cases {
		op := operation(index, map[string]any{"pattern": test.Pattern})
		if !oracle[index].Valid {
			_, diagnostics, err := (Generator{}).Prepare(&ir.Document{OpenAPIVersionLine: "3.1", Operations: []ir.Operation{op}}, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
			if err == nil && len(diagnostics) == 0 {
				t.Errorf("accepted invalid pattern %s", test.Name)
			}
			continue
		}
		document.Operations = append(document.Operations, op)
		fmt.Fprintf(&source, "t.Run(%q,func(t *testing.T){_,err:=ValidateHTTP__Regex%dOutput__(ctx,wire.Text(%q),\"\");if (err==nil)!=%t{t.Fatal(err)}})\n", test.Name, index, test.Input, oracle[index].Matches)
	}
	document.Operations = append(document.Operations, operation(1000, map[string]any{"pattern": `^\p{Script=Tai_Yo}$`}))
	document.Operations = append(document.Operations, operation(999, map[string]any{"anyOf": []any{map[string]any{"pattern": "^(a+)+$"}, true}}))
	source.WriteString(`
 var workers sync.WaitGroup
 for index:=0;index<16;index++{workers.Go(func(){for repeat:=0;repeat<20;repeat++{if _,err:=ValidateHTTP__Regex7Output__(ctx,wire.Text("go-go"),"");err!=nil{t.Error(err)}}})};workers.Wait()
 _,err:=ValidateHTTP__Regex999Output__(ctx,wire.Text(strings.Repeat("a",32)+"!"),"")
 if !errors.Is(err,ecmaregexp.ErrComplexity)||IsValidationError(err){t.Fatalf("lost regex execution error through anyOf: %v",err)}
 if _,err:=ValidateHTTP__Regex999Output__(ctx,wire.Text("aaa"),"");err!=nil{t.Fatal("pattern could not be reused after budget error",err)}
 canceled,cancel:=context.WithCancel(ctx);cancel();if _,err:=ValidateHTTP__Regex7Output__(canceled,wire.Text("go-go"),"");err!=context.Canceled{t.Fatal("lost cancellation",err)}
 if _,err:=ValidateHTTP__Regex1000Output__(ctx,wire.Text("\U0001e6c0"),"");err!=nil{t.Fatal("Unicode17 boundary",err)}
}
`)
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare: %v %v", err, diagnostics)
	}
	root := testGeneratedModule(t, plan, map[string]string{"api/internal/schema/regex_test.go": source.String(), "consumer_test.go": regexHTTPConsumer})
	pureContext, pureCancel := context.WithTimeout(context.Background(), time.Minute)
	defer pureCancel()
	pure := exec.CommandContext(pureContext, "go", "test", "-mod=readonly", "./api/internal/schema")
	pure.Dir = root
	pure.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local", "CGO_ENABLED=0")
	if output, err := pure.CombinedOutput(); err != nil {
		t.Fatalf("generated CGO=0 consumer: %v\n%s", err, output)
	}
}

const regexHTTPConsumer = `package consumer_test
import("context";"encoding/json";"errors";"net/http";"net/http/httptest";"strings";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types" )
func TestHTTPValidationLimitKeepsPhaseAndCause(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json");json.NewEncoder(w).Encode(strings.Repeat("a",32)+"!")}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 _,err=client.Operations.Regex999().Do(context.Background());var apiError *sdk.APIError;var detail *sdk.SDKError
 if !errors.As(err,&apiError)||apiError.Code!=types.ResponseDecodeFailed||!errors.As(err,&detail)||detail.Code!=types.ValidationLimitExceeded||detail.Cause==nil{t.Fatal("execution limit became a schema mismatch or lost its cause",err,detail)}
}
`
