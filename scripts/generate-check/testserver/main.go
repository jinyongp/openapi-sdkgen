package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

func main() {
	flags := flag.NewFlagSet("generate-check-testserver", flag.ExitOnError)
	input := flags.String("input", "", "response body file")
	urlOutput := flags.String("url-output", "", "listening URL output")
	countOutput := flags.String("count-output", "", "request count output")
	reference := flags.String("reference", "", "optional relative reference response body file")
	requiredHeader := flags.String("required-header", "", "optional required Header=Value")
	redirect := flags.Bool("redirect", false, "publish a same-origin redirecting root URL")
	tlsCAOutput := flags.String("tls-ca-output", "", "optional CA PEM output; enables HTTPS")
	flags.Parse(os.Args[1:])
	if *input == "" || *urlOutput == "" || *countOutput == "" {
		panic(errors.New("--input, --url-output, and --count-output are required"))
	}
	body, err := os.ReadFile(*input)
	if err != nil {
		panic(err)
	}
	var referenceBody []byte
	if *reference != "" {
		referenceBody, err = os.ReadFile(*reference)
		if err != nil {
			panic(err)
		}
	}
	headerName, headerValue := "", ""
	if *requiredHeader != "" {
		headerName, headerValue, _ = strings.Cut(*requiredHeader, "=")
		if headerName == "" || headerValue == "" {
			panic(errors.New("--required-header must be Header=Value"))
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	serveListener := listener
	scheme := "http"
	if *tlsCAOutput != "" {
		certificate, caPEM, err := localTLSCertificate()
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(*tlsCAOutput, caPEM, 0o600); err != nil {
			panic(err)
		}
		serveListener = tls.NewListener(listener, &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{certificate},
		})
		scheme = "https"
	}
	var requests atomic.Int64
	handler := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		count := requests.Add(1)
		if err := os.WriteFile(*countOutput, []byte(strconv.FormatInt(count, 10)), 0o600); err != nil {
			http.Error(response, "count failed", http.StatusInternalServerError)
			return
		}
		if headerName != "" && request.Header.Get(headerName) != headerValue {
			http.Error(response, "missing required header", http.StatusUnauthorized)
			return
		}
		if *redirect && request.URL.Path == "/redirect/openapi.json" {
			http.Redirect(response, request, "/contracts/openapi.json", http.StatusFound)
			return
		}
		rootPath := "/openapi.json"
		referencePath := "/schemas/item.json"
		if *redirect {
			rootPath = "/contracts/openapi.json"
			referencePath = "/contracts/schemas/item.json"
		}
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case rootPath:
			_, _ = response.Write(body)
		case referencePath:
			if len(referenceBody) == 0 {
				http.NotFound(response, request)
				return
			}
			_, _ = response.Write(referenceBody)
		default:
			http.NotFound(response, request)
		}
	})
	server := &http.Server{Handler: handler}
	rootPath := "/openapi.json"
	if *redirect {
		rootPath = "/redirect/openapi.json"
	}
	if err := os.WriteFile(*urlOutput, []byte(fmt.Sprintf("%s://%s%s", scheme, listener.Addr(), rootPath)), 0o600); err != nil {
		panic(err)
	}
	if err := server.Serve(serveListener); !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

func localTLSCertificate() (tls.Certificate, []byte, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER})
	certificate, err := tls.X509KeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	return certificate, certificatePEM, nil
}
