package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCompilerCancellationStopsDescendants(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "sh", "-c", "sleep 60 & echo $!; wait")
	configureCompilerProcess(command)
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	stopped := false
	t.Cleanup(func() {
		if !stopped {
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		}
	})
	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil {
		cancel()
		_ = command.Wait()
		t.Fatal(err)
	}
	child, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || child <= 0 {
		cancel()
		_ = command.Wait()
		t.Fatal("missing child process", line, err)
	}
	cancel()
	if err := command.Wait(); err == nil {
		t.Fatal("cancelled compiler succeeded")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		err := syscall.Kill(child, 0)
		if errors.Is(err, syscall.ESRCH) {
			stopped = true
			return
		}
		// A killed orphan may await reaping by the host's init process.
		status, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", child))
		if errors.Is(err, os.ErrNotExist) || strings.Contains(string(status), "State:\tZ") {
			stopped = true
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("compiler child survived cancellation", child)
}
