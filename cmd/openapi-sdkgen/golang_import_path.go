package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/scanner"

	"openapi-sdkgen/internal/generator"
)

// Only the CLI discovers consumer modules. Generators receive a resolved import root.
func resolveGoImportPath(output, configured string, explicit bool) (string, error) {
	if explicit || configured != "" {
		return configured, nil
	}
	fail := func(err error) (string, error) {
		return "", fmt.Errorf("cannot infer Go import path: %w; set --go-import-path or golang.import_path", err)
	}
	if output == "" {
		return fail(errors.New("--output is not specified"))
	}
	destination, err := physicalGoOutputPath(output)
	if err != nil {
		return fail(err)
	}
	for directory := destination; ; directory = filepath.Dir(directory) {
		file := filepath.Join(directory, "go.mod")
		data, err := os.ReadFile(file)
		if err == nil {
			module, err := goModulePath(string(data))
			if err != nil {
				return fail(fmt.Errorf("read module from %s: %w", file, err))
			}
			if _, err := generator.CanonicalGoOptions(&generator.GoOptions{ImportPath: module}); err != nil {
				return fail(fmt.Errorf("read module from %s: %w", file, err))
			}
			relative, err := filepath.Rel(directory, destination)
			if err != nil {
				return fail(err)
			}
			if relative == "." {
				return module, nil
			}
			return module + "/" + filepath.ToSlash(relative), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fail(fmt.Errorf("read %s: %w", file, err))
		}
		if filepath.Dir(directory) == directory {
			return fail(fmt.Errorf("no go.mod above output %s", output))
		}
	}
}

// Resolve the existing prefix without creating any part of the output directory.
func physicalGoOutputPath(output string) (string, error) {
	directory, err := filepath.Abs(output)
	if err != nil {
		return "", err
	}
	var missing []string
	for {
		info, err := os.Lstat(directory)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(directory)
			if err != nil {
				return "", err
			}
			if !info.IsDir() {
				info, err = os.Stat(resolved)
				if err != nil {
					return "", err
				}
				if !info.IsDir() {
					return "", fmt.Errorf("output ancestor %s is not a directory", directory)
				}
			}
			for index := len(missing) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, missing[index])
			}
			return resolved, nil
		}
		if !errors.Is(err, os.ErrNotExist) || filepath.Dir(directory) == directory {
			return "", err
		}
		missing = append(missing, filepath.Base(directory))
		directory = filepath.Dir(directory)
	}
}

// Read only the module directive, including quoted and parenthesized forms.
// Dependency directives are not interpreted and no Go toolchain is invoked.
func goModulePath(data string) (string, error) {
	var lexer scanner.Scanner
	lexer.Init(strings.NewReader(data))
	lexer.Mode = scanner.ScanIdents | scanner.ScanStrings | scanner.ScanRawStrings | scanner.ScanComments
	lexer.Whitespace = 1<<' ' | 1<<'\t' | 1<<'\r'
	lexer.IsIdentRune = func(char rune, position int) bool {
		return char > ' ' && !strings.ContainsRune("()\"`", char) && (char != '/' || position > 0)
	}
	var scanErr error
	lexer.Error = func(_ *scanner.Scanner, message string) { scanErr = errors.New(message) }
	var module, block string
	var declarations int
	consume := func(tokens []string) error {
		if len(tokens) == 0 {
			return nil
		}
		if block != "" {
			if len(tokens) == 1 && tokens[0] == ")" {
				if block == "module" && module == "" {
					return errors.New("empty module directive")
				}
				block = ""
				return nil
			}
			if block != "module" {
				return nil
			}
			if len(tokens) != 1 || module != "" {
				return errors.New("invalid module directive")
			}
			module = tokens[0]
			return nil
		}
		if tokens[0] == "module" {
			declarations++
			if declarations > 1 || len(tokens) != 2 {
				return errors.New("expected exactly one module path")
			}
			if tokens[1] != "(" {
				module = tokens[1]
				return nil
			}
		}
		if len(tokens) == 2 && tokens[1] == "(" {
			block = tokens[0]
		}
		return nil
	}
	var line []string
	for {
		token := lexer.Scan()
		if scanErr != nil {
			return "", scanErr
		}
		if token == scanner.Comment {
			if strings.HasPrefix(lexer.TokenText(), "/*") {
				return "", errors.New("block comments are not valid in go.mod")
			}
			continue
		}
		if token == scanner.EOF || token == '\n' {
			if err := consume(line); err != nil {
				return "", err
			}
			line = nil
			if token == scanner.EOF {
				break
			}
			continue
		}
		value := lexer.TokenText()
		if token == scanner.String || token == scanner.RawString {
			var err error
			value, err = strconv.Unquote(value)
			if err != nil {
				return "", err
			}
		}
		line = append(line, value)
	}
	if declarations != 1 || module == "" || block == "module" {
		return "", errors.New("missing or incomplete module directive")
	}
	return module, nil
}
