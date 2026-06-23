package driver

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/scalebox-dev/agent-api-sdk/isolator/internal/protocol"
)

type Bwrap struct {
	Path string
}

func DiscoverBwrap(ctx context.Context) Discovery {
	if runtime.GOOS != "linux" {
		return Discovery{Driver: Bwrap{}, Available: false, Warnings: []string{"bwrap driver is only supported on Linux."}}
	}
	path, err := exec.LookPath("bwrap")
	if err != nil {
		return Discovery{Driver: Bwrap{}, Available: false, Warnings: []string{"bwrap executable was not found on PATH."}}
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, path, "--ro-bind", "/", "/", "/bin/true")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return Discovery{Driver: Bwrap{Path: path}, Available: false, Warnings: []string{"bwrap probe failed: " + message}}
	}
	return Discovery{Driver: Bwrap{Path: path}, Available: true}
}

func (b Bwrap) Name() string {
	return "bwrap"
}

func (b Bwrap) Status(opts protocol.IsolationOptions) protocol.IsolationStatus {
	return bwrapStatus(false, opts)
}

func (b Bwrap) Run(ctx context.Context, req protocol.RunRequest) (protocol.RunResult, error) {
	path := b.Path
	if strings.TrimSpace(path) == "" {
		var err error
		path, err = exec.LookPath("bwrap")
		if err != nil {
			return protocol.RunResult{}, err
		}
	}
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

	args := bwrapArgs(req, cwd, command)
	started := time.Now()
	cmd := exec.CommandContext(runCtx, path, args...)
	cmd.Env = bwrapEnv(req)
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
		Isolation:   bwrapStatus(false, req.Isolation),
	}, nil
}

func bwrapArgs(req protocol.RunRequest, cwd string, command string) []string {
	requested := protocol.NormalizeIsolationOptions(req.Isolation)
	args := []string{
		"--die-with-parent",
		"--new-session",
		"--unshare-pid",
		"--unshare-ipc",
		"--unshare-uts",
		"--ro-bind", "/", "/",
		"--dev", "/dev",
		"--proc", "/proc",
		"--tmpfs", "/tmp",
	}
	if requested.Network == "blocked" {
		args = append(args, "--unshare-net")
	}
	if requested.Filesystem == "workdir-readonly" {
		args = append(args, "--ro-bind", cwd, cwd)
	} else {
		args = append(args, "--bind", cwd, cwd)
	}
	args = append(args, "--chdir", cwd, "/bin/sh", "-c", command)
	return args
}

func bwrapEnv(req protocol.RunRequest) []string {
	requested := protocol.NormalizeIsolationOptions(req.Isolation)
	env := os.Environ()
	if requested.Env == "minimal" {
		env = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=/tmp"}
	}
	for key, value := range req.Env {
		env = append(env, key+"="+value)
	}
	return env
}

func bwrapStatus(fallback bool, opts protocol.IsolationOptions) protocol.IsolationStatus {
	requested := protocol.NormalizeIsolationOptions(opts)
	network := "allowed"
	if requested.Network == "blocked" {
		network = "blocked"
	}
	return protocol.IsolationStatus{
		Executor:  "isolator",
		Driver:    "bwrap",
		Isolated:  true,
		Fallback:  fallback,
		Requested: requested,
		Guarantees: protocol.IsolationGuarantees{
			Filesystem: "workdir-mounted",
			Network:    network,
			User:       "namespace-user",
			Process:    "pid-namespace",
			Resources:  "timeout-only",
		},
		Warnings: bwrapWarnings(requested),
	}
}

func bwrapWarnings(requested protocol.IsolationOptions) []string {
	warnings := []string{
		"bwrap driver exposes the host root filesystem read-only so common shells and tools can run; write access is limited to tmpfs paths and the mounted workdir.",
	}
	if requested.Resources.MemoryMB != nil || requested.Resources.CPUCount != nil {
		warnings = append(warnings, "Requested CPU or memory limits are not enforced by the bwrap driver yet.")
	}
	if requested.Filesystem == "host" {
		warnings = append(warnings, "Requested filesystem=host still runs inside bwrap with a read-only host root and mounted cwd.")
	}
	return warnings
}
