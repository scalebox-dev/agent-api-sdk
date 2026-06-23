package driver

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/scalebox-dev/agent-api-sdk/isolator/internal/protocol"
)

type Direct struct{}

func (Direct) Name() string {
	return "direct"
}

func (Direct) Status(opts protocol.IsolationOptions) protocol.IsolationStatus {
	return DirectStatus(false, opts)
}

func (Direct) Run(ctx context.Context, req protocol.RunRequest) (protocol.RunResult, error) {
	command := strings.TrimSpace(req.Command)
	if command == "" {
		return protocol.RunResult{}, fmt.Errorf("command must be a non-empty string")
	}
	cwd, err := resolveCWD(req.CWD, req.Workdir)
	if err != nil {
		return protocol.RunResult{}, err
	}
	timeout := req.TimeoutMS
	if timeout <= 0 {
		timeout = 120_000
	}
	maxOutput := req.MaxOutputBytes
	if maxOutput <= 0 {
		maxOutput = 128 * 1024
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	started := time.Now()
	cmd := shellCommand(runCtx, command)
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	for key, value := range req.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	timedOut := runCtx.Err() == context.DeadlineExceeded
	exitCode := 0
	exitCodePtr := &exitCode
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			code := exit.ExitCode()
			exitCodePtr = &code
		} else if !timedOut {
			return protocol.RunResult{}, err
		} else {
			exitCodePtr = nil
		}
	}
	out, outTruncated := boundedText(stdout.Bytes(), maxOutput)
	errText, errTruncated := boundedText(stderr.Bytes(), maxOutput)
	output := strings.TrimRight(strings.Join(nonEmpty(out, errText), "\n"), "\n")
	if output == "" {
		output = "(no output)"
	}
	return protocol.RunResult{
		OK:          true,
		Action:      "run",
		Command:     command,
		Description: req.Description,
		CWD:         cwd,
		ExitCode:    exitCodePtr,
		Stdout:      out,
		Stderr:      errText,
		Output:      output,
		DurationMS:  time.Since(started).Milliseconds(),
		TimedOut:    timedOut,
		Truncated:   outTruncated || errTruncated,
		Isolation:   DirectStatus(false, req.Isolation),
	}, nil
}

func resolveCWD(root string, child string) (string, error) {
	if strings.TrimSpace(root) == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(child) == "" {
		return abs, nil
	}
	if filepath.IsAbs(child) {
		return "", fmt.Errorf("workdir must be relative to cwd")
	}
	resolved := filepath.Clean(filepath.Join(abs, child))
	rel, err := filepath.Rel(abs, resolved)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("workdir must stay inside cwd")
	}
	return resolved, nil
}

func shellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd.exe", "/C", command)
	}
	return exec.CommandContext(ctx, "/bin/sh", "-c", command)
}

func boundedText(value []byte, maxBytes int) (string, bool) {
	if maxBytes <= 0 {
		maxBytes = 128 * 1024
	}
	if len(value) <= maxBytes {
		return string(value), false
	}
	return "...output truncated...\n" + string(value[len(value)-maxBytes:]), true
}

func nonEmpty(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
