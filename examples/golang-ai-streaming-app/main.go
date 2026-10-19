package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	api "example.com/sdkgen-go-stream-example/generated/api"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	interrupt, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(interrupt, 120*time.Second)
	defer cancel()
	baseURL := os.Getenv("AI_API_BASE_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:18794"
	}
	prompt := "Explain what an OpenAPI SDK does in one sentence."
	if len(os.Args) > 1 {
		prompt = os.Args[1]
	}
	client, err := api.NewClient(api.ClientOptions{BaseURL: baseURL})
	if err != nil {
		return err
	}
	events := client.Operations.Generate(api.GenerateInput{
		Body: api.GenerateBody{Prompt: prompt},
	}).Stream(ctx)
	defer events.Close()
	for events.Next() {
		fmt.Print(events.Item().Data)
	}
	if err := events.Err(); err != nil {
		return err
	}
	fmt.Println()
	return nil
}
