package probes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

// JSON fixtures here are ordinary Unicode. The product WTF-8 tree is W02 work.
type immutableLiteral struct{ json string }

func objectLiteral() immutableLiteral { return immutableLiteral{`{"x":1,"items":[true,null]}`} }
func literalValues() []immutableLiteral {
	return []immutableLiteral{{`null`}, {`"a"`}, {`1`}, objectLiteral(), {`[1,"a"]`}}
}
func (value immutableLiteral) snapshot() any {
	var result any
	if err := json.Unmarshal([]byte(value.json), &result); err != nil {
		panic(err)
	}
	return result
}
func (value immutableLiteral) equal(other immutableLiteral) bool {
	return reflect.DeepEqual(value.snapshot(), other.snapshot())
}

func TestMixedCompositeEnumContract(t *testing.T) {
	want := []string{`null`, `"a"`, `1`, `{"x":1,"items":[true,null]}`, `[1,"a"]`}
	for index, value := range literalValues() {
		if value.json != want[index] {
			t.Fatalf("literal %d changed", index)
		}
	}
	object := objectLiteral()
	snapshot := object.snapshot().(map[string]any)
	snapshot["x"] = float64(99)
	snapshot["items"].([]any)[0] = false
	values := literalValues()
	values[3] = immutableLiteral{`false`}
	if !object.equal(immutableLiteral{`{"items":[true,null],"x":1}`}) || !literalValues()[3].equal(object) {
		t.Fatal("caller mutation changed enum or object order affected equality")
	}
	if object.equal(immutableLiteral{`{"x":1,"items":[null,true]}`}) || (immutableLiteral{`"1"`}).equal(immutableLiteral{`1`}) {
		t.Fatal("enum equality coerced values or ignored array ordering")
	}
	t.Log("OBSERVATION mixed null/string/number/object/array enum factories isolate mutable projections and compare structural values")
}

type createInput struct{ Body map[string]any }
type targetOverride struct{ ID optional[string] }
type targetOptions struct{ Headers http.Header }
type linkInvocation[S, I, O any] struct {
	SourceInput optional[S]
	Input       optional[I]
	Options     optional[O]
}
type linkMetadata struct {
	Status int
	Body   map[string]any
	Header http.Header
}
type createLinkSource interface{ linkSource() linkMetadata }
type createRaw struct{ metadata linkMetadata }
type createHTTPError struct{ metadata linkMetadata }

func (source createRaw) linkSource() linkMetadata        { return source.metadata }
func (source *createHTTPError) linkSource() linkMetadata { return source.metadata }
func (source *createHTTPError) Error() string            { return "HTTP error" }

func followPrototype(source createLinkSource, invocation linkInvocation[createInput, targetOverride, targetOptions], statusOverride int) (string, string, error) {
	metadata := source.linkSource()
	status := metadata.Status
	if statusOverride != 0 {
		status = statusOverride
	}
	target := "get"
	if status == 409 {
		target = "recover"
	}
	var id string
	if invocation.SourceInput.set {
		id, _ = invocation.SourceInput.value.Body["id"].(string)
	}
	if invocation.Input.set && invocation.Input.value.ID.set {
		id = invocation.Input.value.ID.value
	}
	if id == "" {
		return "", "", errors.New("missing required request expression")
	}
	return target, id, nil
}

func TestLinkInvocationContract(t *testing.T) {
	invocation := linkInvocation[createInput, targetOverride, targetOptions]{SourceInput: some(createInput{Body: map[string]any{"id": "source-id"}})}
	var original error = &createHTTPError{metadata: linkMetadata{Status: 409}}
	var source *createHTTPError
	if !errors.As(original, &source) {
		t.Fatal("HTTP error narrowing failed")
	}
	target, id, err := followPrototype(source, invocation, 0)
	if err != nil || target != "recover" || id != "source-id" {
		t.Fatalf("error Link: %s %s %v", target, id, err)
	}
	invocation.Input = some(targetOverride{ID: some("override-id")})
	target, id, err = followPrototype(source, invocation, 201)
	if err != nil || target != "get" || id != "override-id" {
		t.Fatalf("explicit status and override: %s %s %v", target, id, err)
	}
	if invocation.SourceInput.value.Body["id"] != "source-id" {
		t.Fatal("source input was mutated")
	}
	if _, _, err := followPrototype(createRaw{}, linkInvocation[createInput, targetOverride, targetOptions]{}, 0); err == nil {
		t.Fatal("missing required source input accepted")
	}
	if reflect.TypeFor[error]().Implements(reflect.TypeFor[createLinkSource]()) {
		t.Fatal("arbitrary errors must not implement source interface")
	}
	t.Log("OBSERVATION typed Link invocation preserves explicit source input, target overrides, HTTP error sources and per-status dispatch")
}

type binarySource interface {
	Open(context.Context) (io.ReadCloser, error)
}
type mediaContext struct{ ContentType string }
type mediaCodec struct {
	Encode          func(context.Context, immutableLiteral, mediaContext) (binarySource, error)
	Decode          func(context.Context, *http.Response, mediaContext) (immutableLiteral, error)
	EncodeParameter func(context.Context, immutableLiteral, mediaContext) (string, error)
	DecodeParameter func(context.Context, string, mediaContext) (immutableLiteral, error)
	DecodeInbound   func(context.Context, *http.Request, mediaContext) (immutableLiteral, error)
}

func TestMediaCallbackHTTPMetadata(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	body := &countedBody{Reader: bytes.NewBufferString(`{"x":1}`)}
	endpoint, _ := url.Parse("https://api.example.test/items")
	request := &http.Request{Method: "POST", URL: endpoint, Header: http.Header{"X-Key": {"key-1"}}}
	response := &http.Response{StatusCode: 409, Header: request.Header.Clone(), Request: request, Body: body}
	codec := mediaCodec{Decode: func(actual context.Context, response *http.Response, media mediaContext) (immutableLiteral, error) {
		if actual != ctx || response.StatusCode != 409 || response.Header.Get("X-Key") != "key-1" || response.Request.URL.Path != "/items" || media.ContentType != "application/example; v=1" {
			return immutableLiteral{}, errors.New("HTTP metadata was lost")
		}
		value, err := io.ReadAll(response.Body)
		return immutableLiteral{string(value)}, err
	}, DecodeInbound: func(actual context.Context, request *http.Request, media mediaContext) (immutableLiteral, error) {
		if request.Method != "POST" || request.Header.Get("X-Key") != "key-1" {
			return immutableLiteral{}, errors.New("inbound metadata lost")
		}
		return objectLiteral(), actual.Err()
	}}
	value, err := codec.Decode(ctx, response, mediaContext{"application/example; v=1"})
	if err != nil || value.json != `{"x":1}` || body.closes != 0 {
		t.Fatalf("decode/borrow contract: %v %v", value, err)
	}
	_ = response.Body.Close() // SDK owns the one close, callback only borrows.
	if body.closes != 1 {
		t.Fatal("body ownership failed")
	}
	cancel()
	if _, err := codec.DecodeInbound(ctx, request, mediaContext{}); !errors.Is(err, context.Canceled) {
		t.Fatal("codec context cancellation lost")
	}
	t.Log("OBSERVATION complete-media callbacks receive status, headers, URL/method and cancellation while SDK owns body closure")
}

type inboundRejection struct {
	Status  int
	Headers http.Header
	Body    binarySource
}
type inboundRequestContext struct {
	Request   *http.Request
	Operation string
	Params    map[string]map[string]any
	CloneBody func(context.Context) (io.ReadCloser, error)
}
type authenticate func(context.Context, inboundRequestContext) (*inboundRejection, error)

func TestInboundAuthenticationContract(t *testing.T) {
	const input = `{"id":"todo-1"}`
	request, _ := http.NewRequest("POST", "https://api.example.test/hooks", bytes.NewBufferString(input))
	view := request.Clone(context.Background())
	view.Body = http.NoBody
	var buffered []byte
	metadata := inboundRequestContext{Request: view, Operation: "todoCompleted", Params: map[string]map[string]any{"query": {"key": "value"}},
		CloneBody: func(ctx context.Context) (io.ReadCloser, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if buffered == nil {
				var err error
				buffered, err = io.ReadAll(request.Body)
				_ = request.Body.Close()
				if err != nil {
					return nil, err
				}
				request.Body = io.NopCloser(bytes.NewReader(buffered))
			}
			return io.NopCloser(bytes.NewReader(buffered)), nil
		}}
	var policy authenticate = func(ctx context.Context, metadata inboundRequestContext) (*inboundRejection, error) {
		clone, err := metadata.CloneBody(ctx)
		if err != nil {
			return nil, err
		}
		defer clone.Close()
		data, err := io.ReadAll(clone)
		if err != nil || string(data) != input || metadata.Request.Method != "POST" || metadata.Params["query"]["key"] != "value" {
			return nil, errors.New("authentication metadata/clone contract failed")
		}
		return &inboundRejection{Status: 403, Headers: http.Header{"X-Reason": {"policy"}}}, nil
	}
	denied, err := policy(context.Background(), metadata)
	remaining, readErr := io.ReadAll(request.Body)
	_ = request.Body.Close()
	if err != nil || readErr != nil || string(remaining) != input || denied.Status != 403 || denied.Headers.Get("X-Reason") != "policy" {
		t.Fatal("authentication consumed original input or lost rejection metadata")
	}
	t.Log("OBSERVATION inbound authentication can inspect an explicit body clone and reject with status/headers without consuming downstream body")
}
