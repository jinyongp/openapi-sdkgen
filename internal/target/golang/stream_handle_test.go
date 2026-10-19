package golang

import (
	"testing"

	"openapi-sdkgen/internal/generator"
)

func streamRuntimePlan(t *testing.T, roots ...string) generator.Plan {
	t.Helper()
	_, prepared := healthPlan(t)
	value, err := prepared.Value("golang")
	if err != nil {
		t.Fatal(err)
	}
	plan := value.(sourcePlan)
	plan.Files = append([]filePlan(nil), plan.Files...)
	files, err := planFiles(generator.GoOptions{ImportPath: "example.test/consumer/api", Package: "api"}, nil, false, false, roots...)
	if err != nil {
		t.Fatal(err)
	}
	present := make(map[string]bool)
	for _, file := range plan.Files {
		present[file.Path] = true
	}
	for _, file := range files {
		if !present[file.Path] {
			plan.Files = append(plan.Files, file)
			present[file.Path] = true
		}
	}
	return generator.NewPlan("golang", plan)
}

func TestGeneratedStreamHandleLifetime(t *testing.T) {
	testGeneratedModule(t, streamRuntimePlan(t, "types-stream", "stream-reader"), map[string]string{"stream_handle_test.go": streamHandleConsumer})
}

const streamHandleConsumer = `package consumer_test
import("context";"errors";"io";"net/http";"sync";"sync/atomic";"testing";"time";"example.test/consumer/api/types")
type handleSource struct{ctx context.Context;opens,closes atomic.Int32;entered,release chan struct{};once sync.Once;block bool;failure,closeFailure error;requireCancel bool}
func(source *handleSource)Next()bool{source.opens.Add(1);if source.block{source.once.Do(func(){close(source.entered)});select{case <-source.ctx.Done():case <-source.release:};return false};return source.opens.Load()==1}
func(*handleSource)Item()int{return 7}
func(source *handleSource)Err()error{if source.block{return source.ctx.Err()};return source.failure}
func(source *handleSource)Close()error{source.closes.Add(1);if source.requireCancel&&source.ctx.Err()==nil{return errors.New("source closed before cancellation")};return source.closeFailure}
func waitEntered(t *testing.T,entered <-chan struct{}){t.Helper();select{case <-entered:case <-time.After(3*time.Second):t.Fatal("stream did not enter source")}}
func TestLazyHeadersAndSequentialItems(t *testing.T){
 var starts atomic.Int32;source:=&handleSource{};response:=&http.Response{StatusCode:200,Header:http.Header{"X-Test":{"headers"}},Body:http.NoBody}
 stream:=types.NewStream(context.Background(),func(ctx context.Context)(types.Source[int],*http.Response,error){starts.Add(1);source.ctx=ctx;return source,response,nil})
 if starts.Load()!=0{t.Fatal("eager start")};var group sync.WaitGroup;for range 12{group.Go(func(){metadata,err:=stream.Response();if err!=nil||metadata!=response{t.Error("response",metadata,err)}})};group.Wait()
 if starts.Load()!=1||source.opens.Load()!=0{t.Fatal("headers consumed items",starts.Load(),source.opens.Load())};if !stream.Next()||stream.Item()!=7{t.Fatal("first item",stream.Err())};if stream.Next()||stream.Err()!=nil{t.Fatal("EOF",stream.Err())};if err:=stream.Close();err!=nil{t.Fatal(err)};if source.closes.Load()!=1{t.Fatal("close count",source.closes.Load())}
}
func TestCloseBeforeStartAndFactories(t *testing.T){
 var starts atomic.Int32;stream:=types.NewStream(context.Background(),func(context.Context)(types.Source[int],*http.Response,error){starts.Add(1);return nil,nil,io.EOF});if err:=stream.Close();err!=nil{t.Fatal(err)};if stream.Next()||stream.Err()!=nil||starts.Load()!=0{t.Fatal("pre-start close",stream.Err(),starts.Load())};_,err:=stream.Response();var detail *types.SDKError;if !errors.As(err,&detail)||detail.Code!=types.RequestAborted{t.Fatal("closed header error",err)}
 source:=&handleSource{};factory:=types.SourceOnce[int](source);copy:=factory;if _,err:=factory(context.Background());err!=nil{t.Fatal(err)};if _,err:=copy(context.Background());!errors.As(err,&detail)||detail.Code!=types.RequestBodyConsumed{t.Fatal("factory copy reused",err)}
}
func TestConcurrentConsumptionAndExplicitClose(t *testing.T){
 for _,overlap:=range []bool{true,false}{source:=&handleSource{block:true,entered:make(chan struct{}),release:make(chan struct{})};stream:=types.NewStream(context.Background(),func(ctx context.Context)(types.Source[int],*http.Response,error){source.ctx=ctx;return source,&http.Response{StatusCode:200},nil});done:=make(chan bool,1);go func(){done<-stream.Next()}();waitEntered(t,source.entered)
 if overlap{if stream.Next(){t.Fatal("overlapping Next succeeded")}}else{if err:=stream.Close();err!=nil{t.Fatal(err)}};select{case value:=<-done:if value{t.Fatal("closed Next produced an item")};case <-time.After(3*time.Second):t.Fatal("Next not canceled")};if source.closes.Load()!=1{t.Fatal("close count",source.closes.Load())};var detail *types.SDKError;if overlap{if !errors.As(stream.Err(),&detail)||detail.Code!=types.StreamAlreadyConsumed{t.Fatal("overlap error",stream.Err())}}else if stream.Err()!=nil{t.Fatal("explicit close created an error",stream.Err())};stream.Close()
 }
}
func TestExternalCancellationAndStartFailure(t *testing.T){
 ctx,cancel:=context.WithCancel(context.Background());source:=&handleSource{block:true,entered:make(chan struct{}),release:make(chan struct{})};stream:=types.NewStream(ctx,func(ctx context.Context)(types.Source[int],*http.Response,error){source.ctx=ctx;return source,&http.Response{StatusCode:200},nil});done:=make(chan bool,1);go func(){done<-stream.Next()}();waitEntered(t,source.entered);cancel();select{case <-done:case <-time.After(3*time.Second):t.Fatal("external cancel not observed")};if !errors.Is(stream.Err(),context.Canceled){t.Fatal("external error",stream.Err())};stream.Close()
 sentinel:=errors.New("start failed");failed:=types.NewStream(context.Background(),func(context.Context)(types.Source[int],*http.Response,error){return nil,&http.Response{StatusCode:500},sentinel});response,err:=failed.Response();if response==nil||!errors.Is(err,sentinel)||failed.Next(){t.Fatal("start cause",response,err)}
 nilSource:=types.NewStream(context.Background(),func(context.Context)(types.Source[int],*http.Response,error){var source *handleSource;return source,nil,nil});if nilSource.Next()||nilSource.Err()==nil{t.Fatal("typed nil accepted")}
}
func TestStartFailureCancelsBeforeCloseAndPreservesCleanupCause(t *testing.T){
 startError,closeError:=errors.New("start failed"),errors.New("cleanup failed");source:=&handleSource{requireCancel:true,closeFailure:closeError}
 stream:=types.NewStream(context.Background(),func(ctx context.Context)(types.Source[int],*http.Response,error){source.ctx=ctx;return source,&http.Response{StatusCode:500},startError})
 _,err:=stream.Response();if !errors.Is(err,startError)||!errors.Is(err,closeError)||!errors.Is(stream.Err(),closeError)||source.closes.Load()!=1{t.Fatal("start cleanup cause or ownership",err,stream.Err(),source.closes.Load())};stream.Close();if source.closes.Load()!=1{t.Fatal("cleanup repeated")}
}
`
