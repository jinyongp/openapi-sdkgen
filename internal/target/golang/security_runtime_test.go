package golang

import (
	"encoding/json"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedSecurityAndServers(t *testing.T) {
	schemes := map[string]any{
		"bearer":        map[string]any{"type": "http", "scheme": "bearer", "bearerFormat": "JWT"},
		"basic":         map[string]any{"type": "http", "scheme": "basic"},
		"custom":        map[string]any{"type": "http", "scheme": "Digest"},
		"key":           map[string]any{"type": "apiKey", "in": "header", "name": "X-Key"},
		"query":         map[string]any{"type": "apiKey", "in": "query", "name": "token"},
		"cookie":        map[string]any{"type": "apiKey", "in": "cookie", "name": "session"},
		"csrf":          map[string]any{"type": "apiKey", "in": "header", "name": "X-CSRF-Token"},
		"authorization": map[string]any{"type": "apiKey", "in": "header", "name": "Authorization"},
		"mtls":          map[string]any{"type": "mutualTLS"},
		"oauth":         map[string]any{"type": "oauth2", "oauth2MetadataUrl": "https://issuer.test/metadata", "deprecated": true, "flows": map[string]any{"authorizationCode": map[string]any{"authorizationUrl": "https://issuer.test/authorize", "tokenUrl": "https://issuer.test/token", "refreshUrl": "https://issuer.test/refresh", "scopes": map[string]any{"read": "Read"}}}},
		"oidc":          map[string]any{"type": "openIdConnect", "openIdConnectUrl": "https://issuer.test/openid"},
	}
	paths := make(map[string]any)
	for _, entry := range []struct {
		name         string
		requirements []any
	}{
		{"Inherited", nil}, {"Public", []any{}}, {"Either", []any{map[string]any{"bearer": []string{}}, map[string]any{}}},
		{"And", []any{map[string]any{"bearer": []string{}, "key": []string{}}}},
		{"All", []any{map[string]any{"key": []string{}, "query": []string{}, "cookie": []string{}, "csrf": []string{}}}},
		{"Basic", []any{map[string]any{"basic": []string{}}}}, {"Custom", []any{map[string]any{"custom": []string{}}}},
		{"OAuth", []any{map[string]any{"oauth": []string{"read"}}}}, {"OIDC", []any{map[string]any{"oidc": []string{}}}},
		{"MTLS", []any{map[string]any{"mtls": []string{}}}}, {"Cookie", []any{map[string]any{"cookie": []string{}}}},
		{"Authorization", []any{map[string]any{"authorization": []string{}}}},
		{"URI", []any{map[string]any{"#/components/securitySchemes/bearer": []string{}}}},
	} {
		operation := map[string]any{"operationId": "get" + entry.name, "responses": map[string]any{"204": map[string]any{"description": "Done"}}}
		if entry.requirements != nil {
			operation["security"] = entry.requirements
		}
		paths["/"+entry.name] = map[string]any{"get": operation}
	}
	paths["/Server"] = map[string]any{"servers": []any{map[string]any{"url": "/path"}}, "get": map[string]any{"operationId": "getServer", "security": []any{}, "responses": map[string]any{"204": map[string]any{"description": "Done"}}}}
	paths["/Override"] = map[string]any{"servers": []any{map[string]any{"url": "/path"}}, "get": map[string]any{"operationId": "getOverride", "security": []any{}, "servers": []any{map[string]any{"url": "/operation"}, map[string]any{"url": "/alternative/{version}", "variables": map[string]any{"version": map[string]any{"default": "v1", "enum": []string{"v1", "v2"}}}}}, "responses": map[string]any{"204": map[string]any{"description": "Done"}}}}
	raw := map[string]any{"openapi": "3.2.0", "info": map[string]any{"title": "Security", "version": "1"}, "components": map[string]any{"securitySchemes": schemes}, "security": []any{map[string]any{"bearer": []string{}}}, "servers": []any{map[string]any{"url": "/root/{version}", "variables": map[string]any{"version": map[string]any{"default": "v1", "enum": []string{"v1", "v2"}}}}}, "paths": paths}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	document, err := compiler.Compile(data)
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) > 0 {
		t.Fatalf("prepare: %v %#v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"security_test.go": securityConsumer})
}

const securityConsumer = `package consumer_test
import("context";"crypto/tls";"crypto/x509";"encoding/base64";"encoding/json";"errors";"net/http";"net/http/cookiejar";"net/http/httptest";"net/url";"strings";"sync";"sync/atomic";"testing";"time";sdk "example.test/consumer/api";"example.test/consumer/api/types")
func assertSecurityCode(t *testing.T,err error,code string){t.Helper();var detail *types.SDKError;var failure *types.APIError;if !errors.As(err,&detail)||detail.Code!=code||!errors.As(err,&failure){t.Fatalf("wanted %s, got %v",code,err)};encoded,_:=json.Marshal(err);if strings.Contains(string(encoded),"SECRET")||strings.Contains(err.Error(),"SECRET"){t.Fatal("credential leaked")}}
func TestSelectionDirectCredentialsAndProvider(t *testing.T){
 var requests,providers atomic.Int32
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){requests.Add(1);if request.URL.Path!="/Either"&&request.URL.Path!="/Public"&&request.Header.Get("Authorization")!="Bearer SECRET"{t.Error("missing bearer",request.URL.Path)};writer.WriteHeader(204)}));defer server.Close()
 provider:=func(ctx context.Context,info types.SecurityCredentialContext)(types.SecurityCredentials,error){providers.Add(1);if info.Origin!=server.URL||info.Operation.Method!="GET"||info.Operation.Route!="GET /Inherited"&&info.Operation.Route!="GET /And"{t.Error("provider context",info.Operation,info.Origin)};result:=types.SecurityCredentials{};for _,scheme:=range info.Requirement.Schemes{switch scheme.Name{case "bearer":result[scheme.Name]=types.SecurityCredential{Kind:"http-bearer",Token:"SECRET"};case "key":result[scheme.Name]=types.SecurityCredential{Kind:"api-key",Value:"key"}}};return result,nil}
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,SecurityProvider:provider});if err!=nil{t.Fatal(err)}
 if _,err:=client.Operations.GetInherited().Do(context.Background());err!=nil{t.Fatal(err)}
 if _,err:=client.Operations.GetAnd().WithOptions(types.RequestOptions{Authorization:types.Some("Bearer SECRET")}).Do(context.Background());err!=nil{t.Fatal(err)}
 before:=providers.Load();if _,err:=client.Operations.GetInherited().WithOptions(types.RequestOptions{Authorization:types.Some("Bearer SECRET")}).Do(context.Background());err!=nil{t.Fatal(err)};if providers.Load()!=before{t.Fatal("called provider for satisfied requirement")}
 if _,err:=client.Operations.GetPublic().Do(context.Background());err!=nil{t.Fatal(err)}
 sent:=requests.Load();_,err=client.Operations.GetEither().Do(context.Background());assertSecurityCode(t,err,types.SecurityRequirementRequired)
 if _,err:=client.Operations.GetEither().WithSecurityRequirement(sdk.GetEitherSecurityAnonymous).Do(context.Background());err!=nil{t.Fatal(err)}
 if requests.Load()!=sent+1||providers.Load()!=before{t.Fatal("selection unexpectedly called provider or HTTP")}
 _,err=client.Operations.GetInherited().WithOptions(types.RequestOptions{SecurityRequirement:sdk.GetEitherSecurityAnonymous}).Do(context.Background());assertSecurityCode(t,err,types.SecurityRequirementInvalid)
 _,err=client.Operations.GetPublic().WithOptions(types.RequestOptions{SecurityRequirement:sdk.GetEitherSecurityAnonymous}).Do(context.Background());assertSecurityCode(t,err,types.SecurityRequirementInvalid)
 _,err=client.Operations.GetInherited().WithOptions(types.RequestOptions{Authorization:types.Some("")}).Do(context.Background());assertSecurityCode(t,err,types.SecurityCredentialsInvalid)
 noProvider,_:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});_,err=noProvider.Operations.GetInherited().Do(context.Background());assertSecurityCode(t,err,types.SecurityCredentialsRequired)
}
func TestCredentialKindsAndMetadata(t *testing.T){
 var called atomic.Int32
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){called.Add(1);switch request.URL.Path{
 case "/All":if request.Header.Get("X-Key")!="key"||request.URL.Query().Get("token")!="a+b&c"||request.Header.Get("Cookie")!="session=s%20e"||request.Header.Get("X-CSRF-Token")!="csrf"{t.Error("api key wire",request.URL,request.Header)}
 case "/Basic":if request.Header.Get("Authorization")!="Basic "+base64.StdEncoding.EncodeToString([]byte("ü:p")){t.Error("basic UTF-8")}
 case "/Custom":if request.Header.Get("Authorization")!="digest opaque"{t.Error("custom HTTP")}
 case "/OAuth","/OIDC":if request.Header.Get("Authorization")!="Bearer SECRET"{t.Error("oauth bearer")}
 case "/Authorization":if request.Header.Get("Authorization")!="opaque"{t.Error("authorization apiKey")}
 };writer.WriteHeader(204)}));defer server.Close()
 provider:=func(ctx context.Context,info types.SecurityCredentialContext)(types.SecurityCredentials,error){result:=types.SecurityCredentials{};for _,scheme:=range info.Requirement.Schemes{credential:=types.SecurityCredential{Kind:"api-key",Value:scheme.Name};switch scheme.Name{
 case "query":credential.Value="a+b&c";case "cookie":credential.Value="s e"
 case "basic":credential=types.SecurityCredential{Kind:"http-basic",Username:"ü",Password:"p"}
 case "custom":credential=types.SecurityCredential{Kind:"http",Value:"opaque"}
 case "oauth":if scheme.OAuth2MetadataURL!="https://issuer.test/metadata"||!scheme.Deprecated||len(scheme.Scopes)!=1||scheme.Scopes[0]!="read"||scheme.Flows.Kind()==types.ValueNull{t.Error("lost oauth metadata")};credential=types.SecurityCredential{Kind:"oauth2",Token:"SECRET"}
 case "oidc":if scheme.OpenIDConnectURL!="https://issuer.test/openid"{t.Error("lost OIDC metadata")};credential=types.SecurityCredential{Kind:"openIdConnect",Token:"SECRET"}
 case "authorization":credential.Value="opaque"
 };result[scheme.Name]=credential};return result,nil}
 client,_:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,SecurityProvider:provider})
 for _,call:=range []func(context.Context)(types.NoContent,error){client.Operations.GetAll().Do,client.Operations.GetBasic().Do,client.Operations.GetCustom().Do,client.Operations.GetOAuth().Do,client.Operations.GetOIDC().Do,client.Operations.GetAuthorization().Do}{if _,err:=call(context.Background());err!=nil{t.Fatal(err)}}
 if called.Load()!=6{t.Fatal("credential cases missing")}
}
func TestProviderFailuresCollisionsCancellationAndNoRetries(t *testing.T){
 var requests atomic.Int32;server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){requests.Add(1);writer.WriteHeader(204)}));defer server.Close()
 cause:=errors.New("SECRET provider failure")
 for _,test:=range []struct{credentials types.SecurityCredentials;err error;code string}{
  {nil,nil,types.SecurityCredentialsInvalid},{types.SecurityCredentials{},nil,types.SecurityCredentialsInvalid},
  {types.SecurityCredentials{"other":{Kind:"http-bearer",Token:"SECRET"}},nil,types.SecurityCredentialsInvalid},
  {types.SecurityCredentials{"bearer":{Kind:"api-key",Value:"SECRET"}},nil,types.SecurityCredentialsInvalid},
  {types.SecurityCredentials{"bearer":{Kind:"http-bearer",Token:"SECRET\r\ninjected"}},nil,types.SecurityCredentialsInvalid},
  {nil,cause,""},
 }{var calls int;client,_:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,SecurityProvider:func(context.Context,types.SecurityCredentialContext)(types.SecurityCredentials,error){calls++;return test.credentials,test.err}});_,err:=client.Operations.GetInherited().Do(context.Background());if err==nil||calls!=1{t.Fatal("provider failure/retry",err,calls)};if test.code!=""{assertSecurityCode(t,err,test.code)}else if !errors.Is(err,cause){t.Fatal("lost provider cause")}}
 if requests.Load()!=0{t.Fatal("invalid credentials reached HTTP")}
 for _,header:=range []http.Header{{"X-Key":{"existing"}},{"Cookie":{"existing=value"}}}{client,_:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,Headers:header,SecurityProvider:func(context.Context,types.SecurityCredentialContext)(types.SecurityCredentials,error){return types.SecurityCredentials{"key":{Kind:"api-key",Value:"key"},"query":{Kind:"api-key",Value:"query"},"cookie":{Kind:"api-key",Value:"cookie"},"csrf":{Kind:"api-key",Value:"csrf"}},nil}});_,err:=client.Operations.GetAll().Do(context.Background());assertSecurityCode(t,err,types.SecurityCredentialsInvalid)}
 client,_:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,Timeout:types.Some(20*time.Millisecond),SecurityProvider:func(ctx context.Context,_ types.SecurityCredentialContext)(types.SecurityCredentials,error){<-ctx.Done();return nil,ctx.Err()}})
 _,err:=client.Operations.GetInherited().Do(context.Background());var failure *types.APIError;if !errors.Is(err,context.DeadlineExceeded)||!errors.As(err,&failure)||failure.Code!=types.RequestTimeout{t.Fatal("provider timeout",err)}
 ctx,cancel:=context.WithCancel(context.Background());cancel();_,err=client.Operations.GetInherited().Do(ctx);if !errors.Is(err,context.Canceled){t.Fatal("provider cancellation",err)}
}
func TestProviderConcurrency(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.WriteHeader(204)}));defer server.Close()
 var active,maximum atomic.Int32
 release:=make(chan struct{});started:=make(chan struct{},2)
 client,_:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,SecurityProvider:func(ctx context.Context,info types.SecurityCredentialContext)(types.SecurityCredentials,error){n:=active.Add(1);for previous:=maximum.Load();n>previous&&!maximum.CompareAndSwap(previous,n);previous=maximum.Load(){};started<-struct{}{};select{case <-release:case <-ctx.Done():return nil,ctx.Err()};active.Add(-1);info.Requirement.Schemes[0].Name="changed";return types.SecurityCredentials{"bearer":{Kind:"http-bearer",Token:"SECRET"}},nil}})
 var group sync.WaitGroup;for index:=0;index<2;index++{group.Add(1);go func(){defer group.Done();if _,err:=client.Operations.GetInherited().Do(context.Background());err!=nil{t.Error(err)}}()}
 for index:=0;index<2;index++{select{case <-started:case <-time.After(time.Second):close(release);t.Fatal("provider serialized")}};close(release);group.Wait();if maximum.Load()!=2{t.Fatal("no concurrent providers")}
}
func TestServerInheritanceSelectionAndCopies(t *testing.T){
 var path string;server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){path=request.URL.Path;writer.WriteHeader(204)}));defer server.Close()
 variables:=map[string]string{"version":"v2"};client,err:=sdk.NewClient(sdk.ClientOptions{Origin:server.URL,Server:types.ServerSelection{Variables:variables},Authorization:types.Some("Bearer SECRET")});if err!=nil{t.Fatal(err)};variables["version"]="invalid"
 if _,err:=client.Operations.GetInherited().Do(context.Background());err!=nil||path!="/root/v2/Inherited"{t.Fatal("root server",path,err)}
 if _,err:=client.Operations.GetServer().Do(context.Background());err!=nil||path!="/path/Server"{t.Fatal("path server",path,err)}
 if _,err:=client.Operations.GetOverride().Do(context.Background());err!=nil||path!="/operation/Override"{t.Fatal("operation server",path,err)}
 selected,_:=sdk.NewClient(sdk.ClientOptions{Origin:server.URL,Server:types.ServerSelection{ID:"#/paths/~1Override/get/servers/1",Variables:map[string]string{"version":"v2"}}})
 if _,err:=selected.Operations.GetOverride().Do(context.Background());err!=nil||path!="/alternative/v2/Override"{t.Fatal("server alternative",path,err)}
 if _,err:=selected.Operations.GetServer().Do(context.Background());err==nil{t.Fatal("foreign server accepted")}
 if _,err:=selected.Operations.GetServer().WithOptions(types.RequestOptions{BaseURL:types.Some(server.URL+"/explicit")}).Do(context.Background());err!=nil||path!="/explicit/Server"{t.Fatal("request BaseURL precedence",path,err)}
 explicit,_:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL+"/client",Server:types.ServerSelection{ID:"unknown"}});if _,err:=explicit.Operations.GetServer().Do(context.Background());err!=nil||path!="/client/Server"{t.Fatal("client BaseURL precedence",path,err)}
 invalid,_:=sdk.NewClient(sdk.ClientOptions{Origin:server.URL,Server:types.ServerSelection{Variables:map[string]string{"version":"invalid"}}});if _,err:=invalid.Operations.GetPublic().Do(context.Background());err==nil{t.Fatal("invalid variable enum")}
 relative,_:=sdk.NewClient(sdk.ClientOptions{});if _,err:=relative.Operations.GetPublic().Do(context.Background());err==nil{t.Fatal("relative server without origin")}
}
func TestSecurityRedirectAndAmbientCookie(t *testing.T){
 var redirected atomic.Int32
 target:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){redirected.Add(1);writer.WriteHeader(204)}));defer target.Close()
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){http.Redirect(writer,request,target.URL,302)}));defer server.Close()
 client,_:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,SecurityProvider:func(context.Context,types.SecurityCredentialContext)(types.SecurityCredentials,error){return types.SecurityCredentials{"bearer":{Kind:"http-bearer",Token:"SECRET"}},nil}})
 if _,err:=client.Operations.GetInherited().Do(context.Background());err==nil||redirected.Load()!=0{t.Fatal("credential redirect permitted",err)}
 jar,_:=cookiejar.New(nil);origin,_:=url.Parse(target.URL);jar.SetCookies(origin,[]*http.Cookie{{Name:"session",Value:"ambient",Path:"/"}})
 var providerCalls atomic.Int32
 ambient,_:=sdk.NewClient(sdk.ClientOptions{BaseURL:target.URL,HTTPClient:&http.Client{Jar:jar},Credentials:types.Include,SecurityProvider:func(context.Context,types.SecurityCredentialContext)(types.SecurityCredentials,error){providerCalls.Add(1);return nil,errors.New("unexpected provider")}})
 if _,err:=ambient.Operations.GetCookie().Do(context.Background());err!=nil||providerCalls.Load()!=0{t.Fatal("ambient include cookie",err)}
}
func TestLocalURISecurityIdentityAndFinalOrigin(t *testing.T){
 var called bool
 host:=&securityHostTransport{}
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:"HTTP://EXAMPLE.TEST:80/v1",Transport:host,SecurityProvider:func(ctx context.Context,info types.SecurityCredentialContext)(types.SecurityCredentials,error){called=true;if info.Origin!="http://example.test"||info.Requirement.ID!="#/components/securitySchemes/bearer"||info.Requirement.Schemes[0].Name!="bearer"||info.Requirement.Schemes[0].BearerFormat!="JWT"{t.Error("URI identity or final origin",info)};return types.SecurityCredentials{"bearer":{Kind:"http-bearer",Token:"SECRET"}},nil}})
 if err!=nil{t.Fatal(err)};if _,err:=client.Operations.GetURI().Do(context.Background());err!=nil||!called{t.Fatal("URI security",err)}
}
type securityHostTransport struct{}
func(*securityHostTransport)Capabilities()types.TransportCapabilities{return types.TransportCapabilities{}}
func(*securityHostTransport)Do(request *http.Request,options types.TransportOptions)(*http.Response,error){if options.AllowRedirects{return nil,errors.New("security redirect allowed")};return &http.Response{StatusCode:204,Header:make(http.Header),Body:http.NoBody,Request:request},nil}
func TestActualMutualTLS(t *testing.T){
 server:=httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){if len(request.TLS.PeerCertificates)==0{t.Error("missing client certificate")};writer.WriteHeader(204)}));server.TLS=&tls.Config{ClientAuth:tls.RequireAnyClientCert};server.StartTLS();defer server.Close()
 roots:=x509.NewCertPool();roots.AddCert(server.Certificate());host:=server.Client();transport:=host.Transport.(*http.Transport).Clone();transport.TLSClientConfig=&tls.Config{RootCAs:roots,Certificates:server.TLS.Certificates};host.Transport=transport
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,HTTPClient:host});if err!=nil{t.Fatal(err)};if _,err:=client.Operations.GetMTLS().Do(context.Background());err!=nil{t.Fatal(err)}
 unsupported,_:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,SecurityProvider:func(context.Context,types.SecurityCredentialContext)(types.SecurityCredentials,error){return types.SecurityCredentials{"mtls":{Kind:"mutual-tls"}},nil}});_,err=unsupported.Operations.GetMTLS().Do(context.Background());assertSecurityCode(t,err,types.TransportCapabilityRequired)
}
`
