package main

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

type generateProjectConfig struct {
	Source            *string                     `toml:"source"`
	Target            *string                     `toml:"target"`
	Output            *string                     `toml:"output"`
	Addons            []string                    `toml:"addons"`
	Incremental       *bool                       `toml:"incremental"`
	DiagnosticsFormat *string                     `toml:"diagnostics_format"`
	Input             generateProjectInputConfig  `toml:"input"`
	References        generateProjectRefConfig    `toml:"references"`
	Schema            generateProjectSchemaConfig `toml:"schema"`
}

type generateProjectInputConfig struct {
	Base           *string           `toml:"base"`
	HeadersFromEnv map[string]string `toml:"headers_from_env"`
	TLSClientCert  *string           `toml:"tls_client_cert"`
	TLSClientKey   *string           `toml:"tls_client_key"`
	TLSCAFile      *string           `toml:"tls_ca_file"`
}

type generateProjectRefConfig struct {
	Allow   []string `toml:"allow"`
	Lock    *string  `toml:"lock"`
	Offline *bool    `toml:"offline"`
}

type generateProjectSchemaConfig struct {
	Extensions []string `toml:"extensions"`
}

func loadGenerateProjectConfig(path string) (generateProjectConfig, string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return generateProjectConfig{}, "", fmt.Errorf("resolve --config %q: %w", path, err)
	}
	file, err := os.Open(absolute)
	if err != nil {
		return generateProjectConfig{}, "", fmt.Errorf("open --config %q: %w", absolute, err)
	}
	defer file.Close()

	var config generateProjectConfig
	if err := toml.NewDecoder(file).DisallowUnknownFields().Decode(&config); err != nil {
		var strict *toml.StrictMissingError
		if errors.As(err, &strict) {
			err = errors.New(strict.String())
		}
		return generateProjectConfig{}, "", fmt.Errorf("decode --config %q: %w", absolute, err)
	}
	return config, filepath.Dir(absolute), nil
}

func applyGenerateProjectConfig(
	config generateProjectConfig,
	base string,
	values *generateFlagValues,
	visited map[string]bool,
) {
	if !visited["input"] && config.Source != nil {
		*values.input = resolveConfigSource(base, *config.Source)
	}
	if !visited["target"] && config.Target != nil {
		*values.targetName = *config.Target
	}
	if !visited["output"] && config.Output != nil {
		*values.output = resolveConfigPath(base, *config.Output)
	}
	if !visited["with"] && config.Addons != nil {
		values.with = append(values.with[:0], config.Addons...)
	}
	if !visited["incremental"] && config.Incremental != nil {
		*values.incremental = *config.Incremental
	}
	if !visited["diagnostics-format"] && config.DiagnosticsFormat != nil {
		*values.diagnosticsFormat = *config.DiagnosticsFormat
	}
	if !visited["input-base"] && config.Input.Base != nil {
		*values.inputBase = resolveConfigSource(base, *config.Input.Base)
	}
	if !visited["http-header-env"] && config.Input.HeadersFromEnv != nil {
		headers := make([]string, 0, len(config.Input.HeadersFromEnv))
		for header := range config.Input.HeadersFromEnv {
			headers = append(headers, header)
		}
		sort.Strings(headers)
		values.httpHeaderEnv = values.httpHeaderEnv[:0]
		for _, header := range headers {
			values.httpHeaderEnv = append(values.httpHeaderEnv, header+"="+config.Input.HeadersFromEnv[header])
		}
	}
	if !visited["tls-client-cert"] && config.Input.TLSClientCert != nil {
		*values.tlsClientCert = resolveConfigPath(base, *config.Input.TLSClientCert)
	}
	if !visited["tls-client-key"] && config.Input.TLSClientKey != nil {
		*values.tlsClientKey = resolveConfigPath(base, *config.Input.TLSClientKey)
	}
	if !visited["tls-ca-file"] && config.Input.TLSCAFile != nil {
		*values.tlsCAFile = resolveConfigPath(base, *config.Input.TLSCAFile)
	}
	if !visited["allow-remote-ref"] && config.References.Allow != nil {
		values.remoteRefs = append(values.remoteRefs[:0], config.References.Allow...)
	}
	if !visited["ref-lock"] && config.References.Lock != nil {
		*values.refLock = resolveConfigPath(base, *config.References.Lock)
	}
	if !visited["offline"] && config.References.Offline != nil {
		*values.offline = *config.References.Offline
	}
	if !visited["schema-extension"] && config.Schema.Extensions != nil {
		values.schemaExtensions = values.schemaExtensions[:0]
		for _, extension := range config.Schema.Extensions {
			values.schemaExtensions = append(values.schemaExtensions, resolveConfigPath(base, extension))
		}
	}
}

func visitedGenerateFlags(flags interface{ Visit(func(*flag.Flag)) }) map[string]bool {
	visited := make(map[string]bool)
	flags.Visit(func(value *flag.Flag) {
		visited[value.Name] = true
	})
	return visited
}

func resolveConfigPath(base, value string) string {
	if value == "" || filepath.IsAbs(value) {
		return value
	}
	return filepath.Clean(filepath.Join(base, value))
}

func resolveConfigSource(base, value string) string {
	if value == "" || value == "-" || filepath.IsAbs(value) {
		return value
	}
	parsed, err := url.Parse(value)
	if err == nil && parsed.Scheme != "" {
		return value
	}
	if strings.HasPrefix(value, "//") {
		return value
	}
	return filepath.Clean(filepath.Join(base, value))
}
