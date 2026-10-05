package main

import (
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"openapi-sdkgen/internal/diagnostic"
)

var errorSecretName = regexp.MustCompile(`(?i)token|secret|password|passwd|credential|private[_-]?key|api[_-]?key|authorization|cookie|task_context`)
var errorURL = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^\s"'<>]+`)
var errorAuthorization = regexp.MustCompile(`(?i)\b(?:bearer|basic)\s+[a-zA-Z0-9._~+/=-]+`)
var errorPrivateKey = regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]+-----.*?(?:-----END [A-Z0-9 ]+-----|$)`)
var errorCredentialPrefix = regexp.MustCompile(`(?i)["']?[a-z0-9_-]*(?:token|secret|password|passwd|credential|private[_-]?key|api[_-]?key|authorization|cookie)[a-z0-9_-]*["']?\s*[:=]\s*`)
var errorCredentialValue = regexp.MustCompile(`^(?:"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|[^\s,;}\]]+)`)

type errorRedactor struct {
	secrets []string
}

func newErrorRedactor(environment []string) errorRedactor {
	redactor := errorRedactor{}
	for _, entry := range environment {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || !errorSecretName.MatchString(key) || len(value) < 4 {
			continue
		}
		values := append([]string{value}, strings.Fields(value)...)
		for _, secret := range values {
			if len(secret) < 4 {
				continue
			}
			encoded, _ := json.Marshal(secret)
			redactor.secrets = append(redactor.secrets, secret, string(encoded[1:len(encoded)-1]), url.QueryEscape(secret), url.PathEscape(secret))
		}
	}
	sort.Slice(redactor.secrets, func(i, j int) bool { return len(redactor.secrets[i]) > len(redactor.secrets[j]) })
	return redactor
}

func (redactor errorRedactor) sanitize(message string, limit int) string {
	for _, secret := range redactor.secrets {
		message = strings.ReplaceAll(message, secret, "[REDACTED]")
	}
	message = errorPrivateKey.ReplaceAllString(message, "[REDACTED PRIVATE KEY]")
	message = errorURL.ReplaceAllStringFunc(message, diagnostic.SafeSourceDisplay)
	message = errorAuthorization.ReplaceAllString(message, "[REDACTED AUTHORIZATION]")
	// A JSON decoder also consumes arrays and objects in credential fields.
	// Plain key=value and single-quoted values use the bounded lexical fallback.
	var result strings.Builder
	for {
		match := errorCredentialPrefix.FindStringIndex(message)
		if match == nil {
			result.WriteString(message)
			break
		}
		result.WriteString(message[:match[1]])
		message = message[match[1]:]
		decoder := json.NewDecoder(strings.NewReader(message))
		var value any
		length := 0
		if decoder.Decode(&value) == nil {
			length = int(decoder.InputOffset())
		} else if strings.HasPrefix(message, "{") || strings.HasPrefix(message, "[") || strings.HasPrefix(message, "\"") {
			// A partial quoted/container value may contain spaces and nested
			// secrets. Consume the remainder if its structure cannot be decoded.
			length = len(message)
		} else if strings.HasPrefix(message, "'") && !strings.Contains(message[1:], "'") {
			length = len(message)
		} else if valueMatch := errorCredentialValue.FindStringIndex(message); valueMatch != nil {
			length = valueMatch[1]
		}
		result.WriteString("[REDACTED]")
		message = message[length:]
	}
	message = strings.Map(func(value rune) rune {
		if unicode.IsControl(value) || unicode.IsSpace(value) {
			return ' '
		}
		return value
	}, result.String())
	message = strings.Join(strings.Fields(message), " ")
	return limitErrorMessage(message, limit)
}

func limitErrorMessage(message string, limit int) string {
	runes := []rune(message)
	if len(runes) > limit {
		message = string(runes[:limit]) + "…"
	}
	return message
}
