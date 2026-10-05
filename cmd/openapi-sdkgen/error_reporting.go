package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const internalCauseLimit = 8192
const internalCauseCount = 8

type internalErrorReport struct {
	Version string   `json:"version"`
	Stage   string   `json:"stage"`
	Causes  []string `json:"causes"`
}

// reportCLIError is the process boundary: diagnostic reports have already been
// rendered, while unexpected failures still need an actionable, sanitized cause.
func reportCLIError(output io.Writer, err error, directory string, environment []string) {
	if err == nil || errors.Is(err, errReportedDiagnostics) {
		return
	}
	var internal *internalGenerationError
	if !errors.As(err, &internal) {
		fmt.Fprintf(output, "openapi-sdkgen: %v\n", err)
		return
	}
	redactor := newErrorRedactor(environment)
	report := internalErrorReport{
		Version: redactor.sanitize(resolvedVersion(), 256),
		Stage:   redactor.sanitize(internal.label, 256),
		Causes:  []string{},
	}
	collectInternalCauses(internal.cause, redactor, &report.Causes)
	fmt.Fprintf(output, "openapi-sdkgen: %s\n", report.Stage)
	if len(report.Causes) != 0 {
		fmt.Fprintf(output, "cause: %s\n", limitErrorMessage(report.Causes[0], 1000))
	}
	path, writeErr := writeInternalErrorReport(directory, report)
	if writeErr != nil {
		fmt.Fprintf(output, "could not save internal error diagnostics: %s\n", redactor.sanitize(writeErr.Error(), 1000))
		return
	}
	fmt.Fprintf(output, "internal error diagnostics: %s\n", redactor.sanitize(path, 1000))
}

func collectInternalCauses(err error, redactor errorRedactor, causes *[]string) {
	if err == nil || len(*causes) >= internalCauseCount {
		return
	}
	*causes = append(*causes, redactor.sanitize(err.Error(), internalCauseLimit))
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range joined.Unwrap() {
			collectInternalCauses(child, redactor, causes)
		}
		return
	}
	collectInternalCauses(errors.Unwrap(err), redactor, causes)
}

func writeInternalErrorReport(directory string, report internalErrorReport) (string, error) {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", err
	}
	runDirectory, err := os.MkdirTemp(directory, "internal-error-")
	if err != nil {
		return "", err
	}
	path := filepath.Join(runDirectory, "diagnostic.json")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		os.RemoveAll(runDirectory)
		return "", err
	}
	encodeErr := json.NewEncoder(file).Encode(report)
	closeErr := file.Close()
	if err := errors.Join(encodeErr, closeErr); err != nil {
		os.RemoveAll(runDirectory)
		return "", err
	}
	return path, nil
}
