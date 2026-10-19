package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type contact struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func problem(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": message})
}
func convert(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" || r.URL.Path != "/convert/contacts" {
		http.NotFound(w, r)
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64*1024))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			problem(w, 413, "Upload exceeds 64 KiB")
		} else {
			problem(w, 400, "Cannot read upload")
		}
		return
	}
	var contacts []contact
	if err := json.Unmarshal(data, &contacts); err != nil || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		problem(w, 400, "Expected a JSON array of contacts")
		return
	}
	for _, item := range contacts {
		if strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.Email) == "" {
			problem(w, 400, "Each contact needs a name and email")
			return
		}
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	for _, item := range contacts {
		if err := encoder.Encode(item); err != nil {
			problem(w, 400, "Cannot encode contact")
			return
		}
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="contacts.ndjson"`)
	_, _ = w.Write(output.Bytes())
}
func main() {
	port := os.Getenv("CONVERSION_API_PORT")
	if port == "" {
		port = "18798"
	}
	server := &http.Server{Addr: "127.0.0.1:" + port, Handler: http.HandlerFunc(convert), ReadHeaderTimeout: 5 * time.Second}
	log.Print("Conversion API listening on ", server.Addr)
	log.Fatal(server.ListenAndServe())
}
