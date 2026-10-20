//go:build !linux

package main

import "os/exec"

// Provider measurements use Linux; retain the platform's default cancellation
// for optional diagnostic invocations elsewhere.
func configureCompilerProcess(command *exec.Cmd) {}
