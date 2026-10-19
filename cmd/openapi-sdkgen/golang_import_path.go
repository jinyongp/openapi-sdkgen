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
	// Keep parent segments until symlinks have been resolved. Abs and Dir clean
	// them lexically, which can select a different module from the actual output.
	directory := output
	separators := string(filepath.Separator)
	if filepath.Separator == '\\' {
		separators += "/"
	}
	var missing []string
	for {
		_, err := os.Lstat(directory)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(directory)
			if err != nil {
				return "", err
			}
			resolved, err = filepath.Abs(resolved)
			if err != nil {
				return "", err
			}
			// A relative existing prefix may still inherit a symlink from PWD.
			resolved, err = filepath.EvalSymlinks(resolved)
			if err != nil {
				return "", err
			}
			info, err := os.Stat(resolved)
			if err != nil {
				return "", err
			}
			if !info.IsDir() {
				return "", fmt.Errorf("output ancestor %s is not a directory", directory)
			}
			for index := len(missing) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, missing[index])
			}
			return resolved, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent, name := filepath.Split(directory)
		if name == "" {
			trimmed := strings.TrimRight(directory, separators)
			if trimmed == directory || trimmed == "" || trimmed == filepath.VolumeName(trimmed) {
				return "", err
			}
			directory = trimmed
			continue
		}
		if name == "." || name == ".." {
			return "", err
		}
		missing = append(missing, name)
		directory = parent
		if directory == "" {
			directory = "."
		}
	}
}

// Read only the module directive, including quoted and parenthesized forms.
// Dependency directives are not interpreted and no Go toolchain is invoked.
func goModulePath(data string) (string, error) {
	// text/scanner silently discards an initial BOM, but Go module files reject it.
	if strings.HasPrefix(data, "\ufeff") {
		return "", errors.New("byte order mark is not valid in go.mod")
	}
	var lexer scanner.Scanner
	lexer.Init(strings.NewReader(data))
	lexer.Mode = scanner.ScanIdents | scanner.ScanStrings | scanner.ScanRawStrings | scanner.ScanComments
	lexer.Whitespace = 1<<' ' | 1<<'\t' | 1<<'\r'
	lexer.IsIdentRune = func(char rune, _ int) bool {
		return char > ' ' && !strings.ContainsRune("/()\"`", char)
	}
	var scanErr error
	lexer.Error = func(_ *scanner.Scanner, message string) { scanErr = errors.New(message) }
	var module, block string
	var declarations int
	setModule := func(value string) error {
		if strings.HasPrefix(value, "\"") {
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				return err
			}
			module = unquoted
			return nil
		}
		if strings.ContainsAny(value, "\"'`") {
			return errors.New("invalid quoted module path")
		}
		module = value
		return nil
	}
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
			return setModule(tokens[0])
		}
		if tokens[0] == "module" {
			declarations++
			if declarations > 1 || len(tokens) != 2 {
				return errors.New("expected exactly one module path")
			}
			if tokens[1] != "(" {
				return setModule(tokens[1])
			}
		}
		if len(tokens) == 2 && tokens[1] == "(" {
			block = tokens[0]
		}
		return nil
	}
	var line []string
	var previousEnd int
	var previousUnquoted bool
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
			previousUnquoted = false
			if token == scanner.EOF {
				break
			}
			continue
		}
		value := lexer.TokenText()
		unquoted := token != scanner.String && token != scanner.RawString && token != '(' && token != ')'
		// Slash tokens belong to an adjacent unquoted module path, but keeping
		// them out of identifiers lets the scanner recognize attached // comments.
		// Quotes adjoining an identifier must not create a separate keyword.
		if previousUnquoted && token != '(' && token != ')' && lexer.Position.Offset == previousEnd {
			line[len(line)-1] += value
		} else {
			line = append(line, value)
		}
		previousEnd = lexer.Pos().Offset
		previousUnquoted = unquoted
	}
	if declarations != 1 || module == "" || block == "module" {
		return "", errors.New("missing or incomplete module directive")
	}
	return module, nil
}
