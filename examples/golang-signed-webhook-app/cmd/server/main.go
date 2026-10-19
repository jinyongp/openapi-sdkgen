package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"example.com/sdkgen-go-signed-webhook/generated/api/models"
	"example.com/sdkgen-go-signed-webhook/generated/api/server/webhooks"
	"example.com/sdkgen-go-signed-webhook/internal/signature"
)

func main() {
	secret := os.Getenv("WEBHOOK_SECRET")
	if secret == "" {
		log.Fatal("Set WEBHOOK_SECRET on the receiver and sender.")
	}
	var mu sync.Mutex
	deliveries := make(map[string]models.WebhookOrderPaidPostBody)
	orders := make([]string, 0)
	receiver, err := webhooks.NewOrderPaidPostHandler("/hooks/orders/paid", webhooks.Options{}, func(_ context.Context, input webhooks.OrderPaidPostInput) (webhooks.OrderPaidPostResponse, error) {
		mu.Lock()
		defer mu.Unlock()
		id := input.Header.XWebhookID
		if previous, found := deliveries[id]; found {
			if previous.OrderID != input.Body.OrderID || previous.AmountCents != input.Body.AmountCents {
				return webhooks.NewOrderPaidPost409NoContentResponse(), nil
			}
			return webhooks.NewOrderPaidPost204NoContentResponse(), nil
		}
		deliveries[id] = input.Body
		orders = append(orders, input.Body.OrderID)
		return webhooks.NewOrderPaidPost204NoContentResponse(), nil
	})
	if err != nil {
		log.Fatal(err)
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/fulfillments" {
			mu.Lock()
			result := models.GetFulfillmentsOutput{Processed: float64(len(orders)), Orders: append([]string{}, orders...)}
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(result)
			return
		}
		if r.Method != "POST" || r.URL.Path != "/hooks/orders/paid" {
			http.NotFound(w, r)
			return
		}
		for _, name := range []string{"x-webhook-id", "x-webhook-timestamp", "x-webhook-signature"} {
			if len(r.Header.Values(name)) != 1 {
				w.WriteHeader(401)
				return
			}
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64*1024))
		if err != nil {
			w.WriteHeader(413)
			return
		}
		if !signature.Verify(secret, r.Header.Get("x-webhook-id"), r.Header.Get("x-webhook-timestamp"), r.Header.Get("x-webhook-signature"), body) {
			w.WriteHeader(401)
			return
		}
		// Restore the verified bytes for the generated JSON decoder.
		r.Body = io.NopCloser(bytes.NewReader(body))
		receiver.ServeHTTP(w, r)
	})
	port := os.Getenv("WEBHOOK_API_PORT")
	if port == "" {
		port = "18800"
	}
	server := &http.Server{Addr: "127.0.0.1:" + port, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	log.Print("Signed webhook API listening on ", server.Addr)
	log.Fatal(server.ListenAndServe())
}
