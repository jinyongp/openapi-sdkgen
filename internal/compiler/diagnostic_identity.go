package sdkgen

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"path/filepath"
	"strings"

	"openapi-sdkgen/internal/diagnostic"
)

type diagnosticIdentityContext struct {
	rootDisplay string
	rootFile    string
	localBase   string
}

func newDiagnosticIdentityContext(source inputSource) *diagnosticIdentityContext {
	rootFile := source.filePath
	if rootFile != "" {
		if resolved, err := filepath.EvalSymlinks(rootFile); err == nil {
			rootFile = resolved
		}
	}
	localBase := source.fileBase
	if localBase != "" {
		if resolved, err := filepath.EvalSymlinks(localBase); err == nil {
			localBase = resolved
		}
	}
	return &diagnosticIdentityContext{
		rootDisplay: source.display,
		rootFile:    rootFile,
		localBase:   localBase,
	}
}

func (context *diagnosticIdentityContext) apply(values []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	result := append([]diagnostic.Diagnostic(nil), values...)
	if context == nil {
		return result
	}
	for index := range result {
		if result[index].IdentitySource == "" {
			result[index].IdentitySource = context.identity(result[index].Location.Source)
		}
	}
	return result
}

func (context *diagnosticIdentityContext) identity(source string) string {
	if source == "" {
		return ""
	}
	if source == context.rootDisplay {
		return "root"
	}
	if context.rootFile != "" && filepath.IsAbs(source) &&
		filepath.Clean(source) == filepath.Clean(context.rootFile) {
		return "root"
	}
	if parsed, err := url.Parse(source); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		parsed.Scheme = strings.ToLower(parsed.Scheme)
		parsed.Host = strings.ToLower(parsed.Host)
		parsed.Fragment = ""
		sum := sha256.Sum256([]byte(parsed.String()))
		return "url-sha256:" + hex.EncodeToString(sum[:])
	}
	if filepath.IsAbs(source) {
		clean := filepath.Clean(source)
		if context.localBase != "" {
			relative, err := filepath.Rel(context.localBase, clean)
			if err == nil && relative != ".." &&
				!strings.HasPrefix(relative, ".."+string(filepath.Separator)) &&
				!filepath.IsAbs(relative) {
				return "file:" + filepath.ToSlash(relative)
			}
		}
		return "absolute:" + filepath.ToSlash(clean)
	}
	return "literal:" + filepath.ToSlash(filepath.Clean(source))
}
