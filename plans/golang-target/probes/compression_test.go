package probes

import (
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

// Isolated design prototype: HTTP content decoding, not a generated SDK adapter.
type decodedBody struct {
	io.Reader
	original io.Closer
	closers  []func()
	once     sync.Once
	closeErr error
}

func (body *decodedBody) Close() error {
	body.once.Do(func() {
		for index := len(body.closers) - 1; index >= 0; index-- {
			body.closers[index]()
		}
		body.closeErr = body.original.Close()
	})
	return body.closeErr
}

func decompressResponse(response *http.Response, method string) error {
	if method == "HEAD" || response.StatusCode == 204 || response.StatusCode == 304 {
		return nil
	}
	codings := strings.Split(response.Header.Get("Content-Encoding"), ",")
	for _, coding := range codings {
		switch strings.TrimSpace(coding) {
		case "gzip", "x-gzip", "deflate", "br", "zstd":
		default:
			return nil // Fetch preserves the entire body if any coding is unknown.
		}
	}
	body := &decodedBody{Reader: response.Body, original: response.Body}
	response.Body = body
	for index := len(codings) - 1; index >= 0; index-- {
		switch strings.TrimSpace(codings[index]) {
		case "gzip", "x-gzip":
			reader, err := gzip.NewReader(body.Reader)
			if err != nil {
				_ = body.Close()
				return err
			}
			body.Reader = reader
			body.closers = append(body.closers, func() { _ = reader.Close() })
		case "deflate":
			buffer := bufio.NewReader(body.Reader)
			prefix, err := buffer.Peek(1)
			if err != nil {
				_ = body.Close()
				return err
			}
			var reader io.ReadCloser
			if prefix[0]&15 == 8 {
				reader, err = zlib.NewReader(buffer)
			} else {
				reader = flate.NewReader(buffer)
			}
			if err != nil {
				_ = body.Close()
				return err
			}
			body.Reader = reader
			body.closers = append(body.closers, func() { _ = reader.Close() })
		case "br":
			body.Reader = brotli.NewReader(body.Reader)
		case "zstd":
			reader, err := zstd.NewReader(body.Reader, zstd.WithDecoderConcurrency(1))
			if err != nil {
				_ = body.Close()
				return err
			}
			body.Reader = reader
			body.closers = append(body.closers, reader.Close)
		}
	}
	response.Body = body
	response.ContentLength = -1
	response.Uncompressed = true
	return nil
}

func compressFixture(t *testing.T, coding string, input []byte) []byte {
	t.Helper()
	var output bytes.Buffer
	var writer io.WriteCloser
	switch coding {
	case "gzip", "x-gzip":
		writer = gzip.NewWriter(&output)
	case "deflate":
		writer = zlib.NewWriter(&output)
	case "raw-deflate":
		writer, _ = flate.NewWriter(&output, flate.DefaultCompression)
	case "br":
		writer = brotli.NewWriter(&output)
	case "zstd":
		writer, _ = zstd.NewWriter(&output, zstd.WithEncoderConcurrency(1))
	default:
		t.Fatal("unknown fixture coding")
	}
	if _, err := writer.Write(input); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestCompressedFetchParity(t *testing.T) {
	const plain = `{"id":"todo-1"}`
	cases := []string{"gzip", "x-gzip", "deflate", "raw-deflate", "br", "zstd", "gzip, br", "unknown"}
	for _, coding := range cases {
		t.Run(coding, func(t *testing.T) {
			var encoded []byte
			if coding == "unknown" {
				encoded = []byte(plain)
			} else if coding == "gzip, br" {
				encoded = compressFixture(t, "br", compressFixture(t, "gzip", []byte(plain)))
			} else {
				encoded = compressFixture(t, coding, []byte(plain))
			}
			header := coding
			if coding == "raw-deflate" {
				header = "deflate"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				w.Header().Set("Content-Encoding", header)
				w.Header().Set("Content-Length", strconv.Itoa(len(encoded)))
				_, _ = w.Write(encoded)
			}))
			defer server.Close()
			request, _ := http.NewRequest("GET", server.URL, nil)
			request.Header.Set("Accept-Encoding", "gzip, deflate") // Prevent Go's implicit gzip decoder.
			response, err := server.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			if err := decompressResponse(response, request.Method); err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if err != nil || string(body) != plain {
				t.Fatalf("Go body: %s %v", body, err)
			}
			output, err := exec.Command("node", "-e", `fetch(process.argv[1]).then(async r=>console.log(JSON.stringify({body:await r.text(),encoding:r.headers.get("content-encoding"),length:r.headers.get("content-length")}))).catch(e=>{console.error(e);process.exit(1)})`, server.URL).CombinedOutput()
			if err != nil {
				t.Fatalf("Node fetch: %v %s", err, output)
			}
			var want struct{ Body, Encoding, Length string }
			if err := json.Unmarshal(output, &want); err != nil {
				t.Fatal(err)
			}
			if want.Body != string(body) || want.Encoding != response.Header.Get("Content-Encoding") || want.Length != response.Header.Get("Content-Length") {
				t.Fatalf("Node=%+v Go=%s %v", want, body, response.Header)
			}
		})
	}
	t.Logf("OBSERVATION %d compressed-response fixtures preserve decoded bytes and original headers against Node Fetch", len(cases))
}

func TestCompressionOwnershipAndBodyless(t *testing.T) {
	for _, status := range []int{204, 304} {
		response := &http.Response{StatusCode: status, Header: http.Header{"Content-Encoding": {"gzip"}}, Body: http.NoBody}
		if err := decompressResponse(response, "GET"); err != nil || response.Uncompressed {
			t.Fatalf("bodyless %d: %v", status, err)
		}
	}
	body := &countedBody{Reader: bytes.NewReader(compressFixture(t, "gzip", []byte("data")))}
	response := &http.Response{StatusCode: 200, Header: http.Header{"Content-Encoding": {"gzip"}}, Body: body}
	if err := decompressResponse(response, "GET"); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	_ = response.Body.Close()
	if body.closes != 1 {
		t.Fatal(fmt.Sprintf("original body closed %d times", body.closes))
	}
	broken := &countedBody{Reader: strings.NewReader("invalid compressed data")}
	failed := &http.Response{StatusCode: 200, Header: http.Header{"Content-Encoding": {"gzip"}}, Body: broken}
	if err := decompressResponse(failed, "GET"); err == nil {
		t.Fatal("malformed gzip accepted")
	}
	_ = failed.Body.Close()
	if broken.closes != 1 {
		t.Fatal("decoder initialization failure did not close exactly once")
	}
}

func TestStandardGzipHeaderDifference(t *testing.T) {
	encoded := compressFixture(t, "gzip", []byte(`{"id":1}`))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Encoding", "gzip")
		writer.Header().Set("Content-Length", strconv.Itoa(len(encoded)))
		_, _ = writer.Write(encoded)
	}))
	defer server.Close()
	response, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if !response.Uncompressed || response.Header.Get("Content-Encoding") != "" || response.Header.Get("Content-Length") != "" {
		t.Fatal("standard Go automatic gzip observation changed")
	}
	t.Log("OBSERVATION standard Go automatic gzip decoding removes original encoding/length headers; explicit Accept-Encoding prevents this")
}

type countedBody struct {
	io.Reader
	closes int
}

func (body *countedBody) Close() error { body.closes++; return nil }
