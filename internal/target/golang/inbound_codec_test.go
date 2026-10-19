package golang

import (
	"testing"

	"openapi-sdkgen/internal/generator"
)

func TestGeneratedInboundCodecBorrowedBodyBoundary(t *testing.T) {
	_, prepared := healthPlan(t)
	value, err := prepared.Value("golang")
	if err != nil {
		t.Fatal(err)
	}
	plan := value.(sourcePlan)
	plan.Files = append([]filePlan(nil), plan.Files...)
	files, err := planFiles(generator.GoOptions{ImportPath: "example.test/consumer/api", Package: "api"}, nil, false, false, "media-inbound")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if file.Path == "internal/media/inbound.go" {
			plan.Files = append(plan.Files, file)
		}
	}
	testGeneratedModule(t, generator.NewPlan("golang", plan), map[string]string{"api/internal/media/inbound_test.go": inboundCodecConsumer})
}

const inboundCodecConsumer = `package media
import("context";"errors";"io";"net/http";"strings";"sync/atomic";"testing";"example.test/consumer/api/types")
type inboundProbe struct{*strings.Reader;reads,closes atomic.Int32}
func(body *inboundProbe)Read(buffer []byte)(int,error){body.reads.Add(1);return body.Reader.Read(buffer)}
func(body *inboundProbe)Close()error{body.closes.Add(1);return nil}
func TestInboundCodecBorrowsBodyAndSnapshotsHTTP(t *testing.T){
 body:=&inboundProbe{Reader:strings.NewReader("payload")};request,err:=http.NewRequest("POST","https://example.test/inbound?key=value",body);if err!=nil{t.Fatal(err)};request.Header.Set("X-Token","original");request.Trailer=http.Header{"X-Trailer":{"trailer"}};request.Proto="HTTP/2.0"
 codec:=types.MediaCodec{DecodeInbound:func(ctx context.Context,borrowed *http.Request,metadata types.MediaContext)(types.Value,error){
 if body.reads.Load()!=0||metadata.ContentType!="application/x-custom; version=1"||metadata.Request==nil||borrowed.Body!=metadata.Request.Body||metadata.Response!=nil||borrowed.Method!="POST"||borrowed.URL.RawQuery!="key=value"||metadata.Request.Proto!="HTTP/2.0"||ctx.Err()!=nil{t.Fatal("inbound metadata or eager read",metadata)}
 borrowed.Header.Set("X-Token","changed");borrowed.URL.Host="invalid.test";if metadata.Request.Header.Get("X-Token")!="original"||metadata.Request.URL.Host!="example.test"{t.Fatal("hook snapshots alias each other")};metadata.Request.Trailer.Set("X-Trailer","changed")
 data,err:=io.ReadAll(metadata.Request.Body);if err!=nil{return types.Value{},err};return types.ValueFrom(string(data))
 }}
 value,err:=DecodeInboundCodec(context.Background(),request,"application/x-custom; version=1",codec);if err!=nil{t.Fatal(err)};text,_:=value.String();if text!="payload"||body.closes.Load()!=0||request.Header.Get("X-Token")!="original"||request.URL.Host!="example.test"||request.Trailer.Get("X-Trailer")!="trailer"{t.Fatal("inbound body ownership/snapshot",text,body.closes.Load(),request)}
 if err:=request.Body.Close();err!=nil{t.Fatal(err)};if body.closes.Load()!=1{t.Fatal("owner close count",body.closes.Load())}
}
func TestInboundCodecMissingFailureAndCancellation(t *testing.T){
 request,_:=http.NewRequest("POST","https://example.test/inbound",nil)
 if _,err:=DecodeInboundCodec(context.Background(),request,"application/custom",types.MediaCodec{});err==nil{t.Fatal("missing direction accepted")}
 sentinel:=errors.New("decode failed");codec:=types.MediaCodec{DecodeInbound:func(context.Context,*http.Request,types.MediaContext)(types.Value,error){return types.Value{},sentinel}}
 if _,err:=DecodeInboundCodec(context.Background(),request,"application/custom",codec);!errors.Is(err,sentinel){t.Fatal("cause lost",err)}
 ctx,cancel:=context.WithCancel(context.Background());cancel();if _,err:=DecodeInboundCodec(ctx,request,"application/custom",codec);!errors.Is(err,context.Canceled){t.Fatal("pre-hook cancellation lost",err)}
 ctx,cancel=context.WithCancel(context.Background());defer cancel();codec.DecodeInbound=func(context.Context,*http.Request,types.MediaContext)(types.Value,error){cancel();return types.ValueFrom("value")}
 if _,err:=DecodeInboundCodec(ctx,request,"application/custom",codec);!errors.Is(err,context.Canceled){t.Fatal("post-hook cancellation lost",err)}
 if _,err:=DecodeInboundCodec(context.Background(),nil,"application/custom",codec);err==nil{t.Fatal("nil request accepted")}
}
`
