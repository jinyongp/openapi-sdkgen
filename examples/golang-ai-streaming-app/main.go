package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	api "example.com/sdkgen-go-stream-example/generated/api"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: hello\n\n")
		w.(http.Flusher).Flush()
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := api.NewClient(api.ClientOptions{BaseURL: server.URL})
	if err != nil {
		return err
	}
	events := client.Operations.Generate(api.GenerateInput{
		Body: api.GenerateBody{Prompt: "Say hello"},
	}).Stream(ctx)
	defer events.Close()
	for events.Next() {
		fmt.Println(events.Item().Data)
	}
	if err := events.Err(); err != nil {
		return err
	}
	return nil
}
