package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type generateInput struct {
	Prompt string `json:"prompt"`
}

func generate(client openai.Client, model string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/generate" {
			http.NotFound(w, r)
			return
		}
		var input generateInput
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
		if err := decoder.Decode(&input); err != nil || strings.TrimSpace(input.Prompt) == "" {
			http.Error(w, "Expected a non-empty JSON prompt", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			http.Error(w, "Expected a single JSON prompt", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
		defer cancel()
		stream := client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
			Model:    model,
			Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage(input.Prompt)},
		})
		defer stream.Close()
		started := false
		for stream.Next() {
			chunk := stream.Current()
			if len(chunk.Choices) == 0 || chunk.Choices[0].Delta.Content == "" {
				continue
			}
			if !started {
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Cache-Control", "no-cache")
				started = true
			}
			// Each SSE data line preserves a line of the model's text.
			text := strings.ReplaceAll(strings.ReplaceAll(chunk.Choices[0].Delta.Content, "\r\n", "\n"), "\r", "\n")
			if _, err := fmt.Fprintf(w, "data: %s\n\n", strings.ReplaceAll(text, "\n", "\ndata: ")); err != nil {
				return
			}
			if err := http.NewResponseController(w).Flush(); err != nil {
				return
			}
		}
		if stream.Err() != nil || !started {
			if !started {
				http.Error(w, "AI generation failed", http.StatusBadGateway)
				return
			}
			// A truncated model response must not appear to finish successfully.
			panic(http.ErrAbortHandler)
		}
	}
}

func main() {
	model := os.Getenv("OPENAI_MODEL")
	if os.Getenv("OPENAI_API_KEY") == "" || model == "" {
		log.Fatal("Set OPENAI_API_KEY and OPENAI_MODEL before starting the server.")
	}
	opts := []option.RequestOption{option.WithMaxRetries(0)}
	if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	port := os.Getenv("AI_API_PORT")
	if port == "" {
		port = "18794"
	}
	server := &http.Server{
		Addr:              "127.0.0.1:" + port,
		Handler:           generate(openai.NewClient(opts...), model),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Print("AI API listening on ", server.Addr)
	log.Fatal(server.ListenAndServe())
}
