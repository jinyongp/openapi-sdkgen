package golang

import (
	"context"
	"openapi-sdkgen/internal/generator"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestGeneratedCompressionHTTPHeadersAndRaw(t *testing.T) {
	document, _ := healthPlan(t)
	document.Operations[0].Responses[0].Raw["headers"] = map[string]any{
		"Content-Encoding": map[string]any{"required": true, "schema": map[string]any{"type": "string"}},
		"Content-Length":   map[string]any{"required": true, "schema": map[string]any{"type": "integer", "minimum": 0}},
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	root := testGeneratedModule(t, plan, map[string]string{"consumer_test.go": compressionHTTPConsumer})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "test", "-mod=mod", "./...")
	command.Dir = root
	command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOWORK=off", "GOTOOLCHAIN=local")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("CGO-disabled generated HTTP consumer: %v\n%s", err, output)
	}
}

const compressionHTTPConsumer = `package consumer_test
import("bytes";"compress/flate";"compress/gzip";"compress/zlib";"context";"errors";"io";"net/http";"net/http/httptest";"strconv";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types";"github.com/andybalholm/brotli";"github.com/klauspost/compress/zstd")
func encodeResponse(t *testing.T,data []byte,coding string)[]byte{
 t.Helper();var buffer bytes.Buffer;var writer io.WriteCloser
 switch coding{case "gzip","x-gzip":writer=gzip.NewWriter(&buffer);case "zlib":writer=zlib.NewWriter(&buffer);case "raw":var err error;writer,err=flate.NewWriter(&buffer,flate.DefaultCompression);if err!=nil{t.Fatal(err)};case "br":writer=brotli.NewWriter(&buffer);case "zstd":var err error;writer,err=zstd.NewWriter(&buffer,zstd.WithEncoderConcurrency(1));if err!=nil{t.Fatal(err)}}
 if _,err:=writer.Write(data);err!=nil{t.Fatal(err)};if err:=writer.Close();err!=nil{t.Fatal(err)};return buffer.Bytes()
}
func TestAllHTTPResponseCodings(t *testing.T){
 payload:=[]byte("{\"ok\":true}")
 for _,coding:=range []string{"gzip","x-gzip","zlib","raw","br","zstd","chain","identity","unknown"}{t.Run(coding,func(t *testing.T){
  encoded:=payload;header:=coding
  switch coding{case "identity":case "unknown":header="gzip, example";case "chain":encoded=encodeResponse(t,encodeResponse(t,payload,"gzip"),"br");header="gzip, br";default:encoded=encodeResponse(t,payload,coding);if coding=="zlib"||coding=="raw"{header="deflate"}}
  server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json");w.Header().Set("Content-Encoding",header);w.Header().Set("Content-Length",strconv.Itoa(len(encoded)));w.Write(encoded)}));defer server.Close()
  client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
  value,err:=client.Operations.GetHealth().Do(context.Background());if err!=nil||!value.Ok{t.Fatal("Do",value,err)}
  raw,err:=client.Operations.GetHealth().Raw(context.Background());if err!=nil{t.Fatal(err)}
  if !bytes.Equal(raw.Bytes(),payload)||raw.OriginalResponse.Body!=http.NoBody||raw.Headers.Get("Content-Encoding")!=header||raw.Headers.Get("Content-Length")!=strconv.Itoa(len(encoded)){t.Fatal("Raw body or original headers",raw)}
  typed,ok:=raw.As200Headers();if !ok||typed.ContentEncoding!=header||typed.ContentLength!=float64(len(encoded)){t.Fatal("typed original headers",typed,ok)}
 })}
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json");w.Header().Set("Content-Encoding","gzip");w.Header().Set("Content-Length","8");w.Write([]byte("not gzip"))}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 _,err=client.Operations.GetHealth().Raw(context.Background());var apiError *sdk.APIError;if !errors.As(err,&apiError)||apiError.Code!=types.ResponseDecodeFailed{t.Fatal("malformed compression",err)}
}
`
