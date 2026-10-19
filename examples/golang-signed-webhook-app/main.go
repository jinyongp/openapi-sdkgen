package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	api "example.com/sdkgen-go-signed-webhook/generated/api"
	"example.com/sdkgen-go-signed-webhook/internal/signature"
)

func run() error {
	secret := os.Getenv("WEBHOOK_SECRET")
	if secret == "" {
		return fmt.Errorf("set WEBHOOK_SECRET on the receiver and sender")
	}
	baseURL := os.Getenv("WEBHOOK_API_BASE_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:18800"
	}
	id, timestamp := rand.Text(), strconv.FormatInt(time.Now().Unix(), 10)
	body := []byte(`{"orderID":"order-1001","amountCents":27000}`)
	signed := signature.Sign(secret, id, timestamp, body)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	deliver := func(payload []byte) (int, error) {
		request, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/hooks/orders/paid", bytes.NewReader(payload))
		if err != nil {
			return 0, err
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("x-webhook-id", id)
		request.Header.Set("x-webhook-timestamp", timestamp)
		request.Header.Set("x-webhook-signature", signed)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return 0, err
		}
		defer response.Body.Close()
		_, err = io.Copy(io.Discard, response.Body)
		return response.StatusCode, err
	}
	for index, payload := range [][]byte{body, body, []byte(`{"orderID":"order-1001","amountCents":1}`)} {
		status, err := deliver(payload)
		if err != nil {
			return err
		}
		fmt.Printf("%s: %d\n", []string{"First delivery", "Duplicate delivery", "Tampered delivery"}[index], status)
	}
	client, err := api.NewClient(api.ClientOptions{BaseURL: baseURL})
	if err != nil {
		return err
	}
	result, err := client.Operations.GetFulfillments().Do(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Processed orders: %g\n", result.Processed)
	if len(result.Orders) > 0 {
		fmt.Println("Order:", result.Orders[len(result.Orders)-1])
	}
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
