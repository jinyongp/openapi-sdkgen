package golang

import "testing"

func TestGeneratedBinarySourceOwnership(t *testing.T) {
	_, plan := healthPlan(t)
	testGeneratedModule(t, plan, map[string]string{"api/types/binary_test.go": binarySourceConsumer})
}

const binarySourceConsumer = `package types
import("bytes";"context";"errors";"io";"strings";"sync";"sync/atomic";"testing")
type binaryTestReader struct{io.Reader;closed atomic.Int32;closeError error}
func(reader *binaryTestReader)Close()error{reader.closed.Add(1);return reader.closeError}
type binaryTestSource struct{metadata BinaryMetadata;open func(context.Context)(io.ReadCloser,error)}
func(source *binaryTestSource)Metadata()BinaryMetadata{return source.metadata}
func(source *binaryTestSource)Open(ctx context.Context)(io.ReadCloser,error){return source.open(ctx)}
func TestBinaryBytesAndZero(t *testing.T){
 ctx:=context.Background()
 var zero Binary
 opened,err:=zero.Open(ctx);if err!=nil{t.Fatal(err)};data,err:=io.ReadAll(opened);if err!=nil||len(data)!=0{t.Fatal("zero binary is not empty",err)};opened.Close()
 if size,ok:=zero.Metadata().Size.Get();!ok||size!=0{t.Fatal("empty byte metadata")}
 input:=[]byte("abc");value:=BinaryFromBytes(input);input[0]='z'
 for repeat:=0;repeat<2;repeat++{reader,err:=value.Open(ctx);if err!=nil{t.Fatal(err)};data,err:=io.ReadAll(reader);reader.Close();if err!=nil||!bytes.Equal(data,[]byte("zbc")){t.Fatal("bytes not borrowed or replayable",err)}}
}
func TestOneShotCopiesShareConsumption(t *testing.T){
 reader:=&binaryTestReader{Reader:strings.NewReader("hello")}
 first:=BinaryFromReader(reader,BinaryMetadata{Filename:"a.bin",ContentType:"application/octet-stream",Size:Some(int64(5))});second:=first
 var workers sync.WaitGroup;var successes atomic.Int32;var failures atomic.Int32
 for _,value:=range []Binary{first,second}{workers.Go(func(){opened,err:=value.Open(context.Background());if err!=nil{var detail *SDKError;if !errors.As(err,&detail)||detail.Code!=RequestBodyConsumed{t.Error(err)};failures.Add(1);return};successes.Add(1);data,err:=io.ReadAll(opened);if err!=nil||string(data)!="hello"{t.Error(err)};opened.Close();opened.Close()})};workers.Wait()
 if successes.Load()!=1||failures.Load()!=1||reader.closed.Load()!=1{t.Fatalf("one-shot ownership %d %d %d",successes.Load(),failures.Load(),reader.closed.Load())}
}
func TestBinaryOpenFailuresAndLengths(t *testing.T){
 ctx:=context.Background();cause:=errors.New("open failure");closeCause:=errors.New("close failure")
 acquired:=&binaryTestReader{Reader:strings.NewReader("unused"),closeError:closeCause}
 source:=&binaryTestSource{open:func(context.Context)(io.ReadCloser,error){return acquired,cause}}
 if _,err:=BinaryFromSource(source).Open(ctx);!errors.Is(err,cause)||!errors.Is(err,closeCause)||acquired.closed.Load()!=1{t.Fatalf("reader+error cleanup: %v",err)}
 var typedNil *binaryTestSource
 for _,value:=range []Binary{BinaryFromSource(nil),BinaryFromSource(typedNil),BinaryFromReader(nil,BinaryMetadata{}),BinaryFromSource(&binaryTestSource{open:func(context.Context)(io.ReadCloser,error){return nil,nil}})}{if _,err:=value.Open(ctx);err==nil{t.Fatal("nil source/reader accepted")}}
 opened:=false
 invalid:=BinaryFromSource(&binaryTestSource{metadata:BinaryMetadata{Size:Some(int64(-1))},open:func(context.Context)(io.ReadCloser,error){opened=true;return nil,nil}})
 if _,err:=invalid.Open(ctx);err==nil||opened{t.Fatal("negative size acquired a resource")}
 for _,test:=range []struct{data string;size int64;valid bool}{{"abc",3,true},{"abc",2,false},{"abc",4,false},{"",0,true},{"a",0,false}}{
  raw:=&binaryTestReader{Reader:strings.NewReader(test.data)}
  value:=BinaryFromReader(raw,BinaryMetadata{Size:Some(test.size)});reader,err:=value.Open(ctx);if err!=nil{t.Fatal(err)};_,err=io.ReadAll(reader);reader.Close();if (err==nil)!=test.valid||raw.closed.Load()!=1{t.Errorf("length %d/%d: %v",len(test.data),test.size,err)}
 }
 canceled,cancel:=context.WithCancel(ctx);cancel();reusable:=BinaryFromReader(&binaryTestReader{Reader:strings.NewReader("")},BinaryMetadata{})
 if _,err:=reusable.Open(canceled);err!=context.Canceled{t.Fatal(err)}
 reader,err:=reusable.Open(ctx);if err!=nil{t.Fatal("canceled execution consumed source",err)};reader.Close()
}
func TestAPIErrorPreservesCausesWithoutFormattingSensitiveData(t *testing.T){
 cause:=errors.New("secret token and body")
 detail:=&SDKError{Code:ValidationLimitExceeded,Cause:cause}
 failure:=&APIError{Code:ResponseDecodeFailed,Route:"GET /items",Cause:detail}
 var typed *SDKError
 if !errors.Is(failure,cause)||!errors.As(failure,&typed)||typed.Code!=ValidationLimitExceeded{t.Fatal("error chain lost its cause")}
 if strings.Contains(failure.Error(),"secret"){t.Fatal("error formatted sensitive cause")}
}
`
