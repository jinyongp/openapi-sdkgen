package main

import (
	"context"
	"fmt"
	"os"
	"time"

	api "example.com/sdkgen-go-file-conversion/generated/api"
	"example.com/sdkgen-go-file-conversion/generated/api/types"
)

func run() error {
	input, err := os.ReadFile("contacts.json")
	if err != nil {
		return err
	}
	baseURL := os.Getenv("CONVERSION_API_BASE_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:18798"
	}
	client, err := api.NewClient(api.ClientOptions{BaseURL: baseURL})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	file, err := client.Operations.ConvertContacts(api.ConvertContactsInput{Body: types.BinaryFromBytes(input)}).Do(ctx)
	if err != nil {
		return err
	}
	if err := os.WriteFile("contacts.ndjson", file, 0o600); err != nil {
		return err
	}
	fmt.Println("Saved: contacts.ndjson")
	fmt.Print(string(file))
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
