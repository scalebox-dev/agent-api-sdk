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

type SandboxExec struct {
	Path string
}

func DiscoverSandboxExec(ctx context.Context) Discovery {
	if runtime.GOOS != "darwin" {
		return Discovery{Driver: SandboxExec{}, Available: false, Warnings: []string{"sandbox-exec driver is only supported on macOS."}}
	}
	path, err := exec.LookPath("sandbox-exec")
	if err != nil {
		return Discovery{Driver: SandboxExec{}, Available: false, Warnings: []string{"sandbox-exec executable was not found on PATH."}}
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, path, "-p", "(version 1)(allow default)", "/usr/bin/true")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return Discovery{Driver: SandboxExec{Path: path}, Available: false, Warnings: []string{"sandbox-exec probe failed: " + message}}
	}
	return Discovery{Driver: SandboxExec{Path: path}, Available: true}
}

func (s SandboxExec) Name() string {
	return "sandbox-exec"
}

func (s SandboxExec) Status(opts protocol.IsolationOptions) protocol.IsolationStatus {
	return sandboxExecStatus(false, opts)
}

func (s SandboxExec) Run(ctx context.Context, req protocol.RunRequest) (protocol.RunResult, error) {
	path := s.Path
	if strings.TrimSpace(path) == "" {
		var err error
		path, err = exec.LookPath("sandbox-exec")
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
	profile, cleanup, err := writeSandboxProfile(req.Isolation, cwd)
	if err != nil {
		return protocol.RunResult{}, err
	}
	defer cleanup()
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	started := time.Now()
	cmd := exec.CommandContext(runCtx, path, "-f", profile, "/bin/sh", "-c", command)
	cmd.Dir = cwd
	cmd.Env = sandboxExecEnv(req)
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
		Isolation:   sandboxExecStatus(false, req.Isolation),
	}, nil
}

func writeSandboxProfile(opts protocol.IsolationOptions, cwd string) (string, func(), error) {
	requested := protocol.NormalizeIsolationOptions(opts)
	rules := []string{
		"(version 1)",
		"(allow default)",
		"(deny file-write*)",
		`(allow file-write* (subpath "/tmp") (subpath "/private/tmp"))`,
	}
	if requested.Filesystem != "workdir-readonly" {
		rules = append(rules, `(allow file-write* (subpath "`+sandboxProfileString(cwd)+`"))`)
	}
	if requested.Network == "blocked" {
		rules = append(rules, "(deny network*)")
	}
	file, err := os.CreateTemp("", "agent-isolator-sandbox-*.sb")
	if err != nil {
		return "", nil, err
	}
	defer file.Close()
	if _, err := file.WriteString(strings.Join(rules, "\n") + "\n"); err != nil {
		os.Remove(file.Name())
		return "", nil, err
	}
	return file.Name(), func() { _ = os.Remove(file.Name()) }, nil
}

func sandboxExecEnv(req protocol.RunRequest) []string {
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

func sandboxExecStatus(fallback bool, opts protocol.IsolationOptions) protocol.IsolationStatus {
	requested := protocol.NormalizeIsolationOptions(opts)
	network := "allowed"
	if requested.Network == "blocked" {
		network = "blocked"
	}
	return protocol.IsolationStatus{
		Executor:  "isolator",
		Driver:    "sandbox-exec",
		Isolated:  true,
		Fallback:  fallback,
		Requested: requested,
		Guarantees: protocol.IsolationGuarantees{
			Filesystem: "policy-enforced",
			Network:    network,
			User:       "host-user",
			Process:    "child-contained",
			Resources:  "timeout-only",
		},
		Warnings: sandboxExecWarnings(requested),
	}
}

func sandboxExecWarnings(requested protocol.IsolationOptions) []string {
	warnings := []string{
		"sandbox-exec is a macOS system utility with version-dependent behavior; validate on the target macOS release.",
	}
	if requested.Resources.MemoryMB != nil || requested.Resources.CPUCount != nil {
		warnings = append(warnings, "Requested CPU or memory limits are not enforced by the sandbox-exec driver yet.")
	}
	if requested.Filesystem == "host" {
		warnings = append(warnings, "Requested filesystem=host still runs with file write restrictions applied by the generated sandbox profile.")
	}
	return warnings
}

func sandboxProfileString(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`)
}
