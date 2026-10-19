package golang

import "testing"

func TestGeneratedSecurityWireAndCollisionRules(t *testing.T) {
	_, plan := healthPlan(t)
	testGeneratedModule(t, plan, map[string]string{"api/internal/security/security_test.go": securityEngineConsumer})
}

const securityEngineConsumer = `package security
import("context";"errors";"net/http";"testing";"example.test/consumer/api/types")
func TestQueryFormEncodingAndOrder(t *testing.T){
 actual,err:=appendQueryCredential("z=%2B&x=*&space=a+b&semi=a;b&empty&z=last","key/~","a+b&c~*")
 want:="z=%2B&x=*&space=a+b&semi=a%3Bb&empty=&z=last&key%2F%7E=a%2Bb%26c%7E*"
 if err!=nil||actual!=want{t.Fatalf("query %q %v, want %q",actual,err,want)}
 for _,raw:=range []string{"key%2f~=existing","key%2F%7e=existing","key/~="}{if _,err:=appendQueryCredential(raw,"key/~","next");err==nil{t.Fatal("encoded duplicate escaped collision",raw)}}
}
func TestMixedDirectAndProviderCollisionRules(t *testing.T){
 requirement:=[]types.SecurityRequirementDefinition{ {ID:"both",Schemes:[]types.SecuritySchemeDefinition{ {Name:"bearer",Type:"http",Scheme:"bearer"},{Name:"key",Type:"apiKey",Location:"header",ParameterName:"X-Key"}}}}
 for _,test:=range []struct{token string;pass bool}{{"same",true},{"different",false}}{
  request,_:=http.NewRequest("GET","https://example.test",nil);request.Header.Set("Authorization","Bearer same")
  _,err:=Apply(context.Background(),request,types.OperationInfo{},types.RequestOptions{Authorization:types.Some("Bearer same")},func(context.Context,types.SecurityCredentialContext)(types.SecurityCredentials,error){return types.SecurityCredentials{"bearer":{Kind:"http-bearer",Token:test.token},"key":{Kind:"api-key",Value:"key"}},nil},types.TransportCapabilities{},requirement)
  if (err==nil)!=test.pass{t.Fatal("direct/provider collision",test.token,err)}
 }
 request,_:=http.NewRequest("GET","https://example.test",nil)
 _,err:=Apply(context.Background(),request,types.OperationInfo{},types.RequestOptions{Credentials:types.Include},nil,types.TransportCapabilities{CookieJar:false},[]types.SecurityRequirementDefinition{ {ID:"cookie",Schemes:[]types.SecuritySchemeDefinition{ {Name:"cookie",Type:"apiKey",Location:"cookie",ParameterName:"session"}}}})
 var detail *types.SDKError;if !errors.As(err,&detail)||detail.Code!=types.SecurityCredentialsRequired{t.Fatal("include falsely claimed absent jar",err)}
}
`
