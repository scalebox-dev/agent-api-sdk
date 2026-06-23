//go:build windows

package driver

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/scalebox-dev/agent-api-sdk/isolator/internal/protocol"
)

type WindowsJob struct{}

func DiscoverWindowsJob(context.Context) Discovery {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return Discovery{Driver: WindowsJob{}, Available: false, Warnings: []string{"CreateJobObject failed: " + err.Error()}}
	}
	windows.CloseHandle(job)
	return Discovery{Driver: WindowsJob{}, Available: true}
}

func (WindowsJob) Name() string {
	return "windows-job"
}

func (WindowsJob) Status(opts protocol.IsolationOptions) protocol.IsolationStatus {
	return windowsJobStatus(false, opts)
}

func (WindowsJob) Run(ctx context.Context, req protocol.RunRequest) (protocol.RunResult, error) {
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

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return protocol.RunResult{}, err
	}
	defer windows.CloseHandle(job)
	if err := configureJob(job, req.Isolation); err != nil {
		return protocol.RunResult{}, err
	}

	started := time.Now()
	cmd := exec.CommandContext(runCtx, "cmd.exe", "/C", command)
	cmd.Dir = cwd
	cmd.Env = windowsJobEnv(req)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return protocol.RunResult{}, err
	}
	if err := windows.AssignProcessToJobObject(job, windows.Handle(cmd.Process.Pid)); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return protocol.RunResult{}, err
	}
	err = cmd.Wait()
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
		Isolation:   windowsJobStatus(false, req.Isolation),
	}, nil
}

func configureJob(job windows.Handle, opts protocol.IsolationOptions) error {
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	requested := protocol.NormalizeIsolationOptions(opts)
	if requested.Resources.MemoryMB != nil && *requested.Resources.MemoryMB > 0 {
		info.ProcessMemoryLimit = uintptr(*requested.Resources.MemoryMB) * 1024 * 1024
		info.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_PROCESS_MEMORY
	}
	_, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	return err
}

func windowsJobEnv(req protocol.RunRequest) []string {
	requested := protocol.NormalizeIsolationOptions(req.Isolation)
	env := os.Environ()
	if requested.Env == "minimal" {
		env = []string{`PATH=C:\Windows\System32;C:\Windows`, `USERPROFILE=%TEMP%`}
	}
	for key, value := range req.Env {
		env = append(env, key+"="+value)
	}
	return env
}

func windowsJobStatus(fallback bool, opts protocol.IsolationOptions) protocol.IsolationStatus {
	requested := protocol.NormalizeIsolationOptions(opts)
	resources := "timeout-only"
	if requested.Resources.MemoryMB != nil {
		resources = "cpu-memory-limits"
	}
	return protocol.IsolationStatus{
		Executor:  "isolator",
		Driver:    "windows-job",
		Isolated:  true,
		Fallback:  fallback,
		Requested: requested,
		Guarantees: protocol.IsolationGuarantees{
			Filesystem: "none",
			Network:    "allowed",
			User:       "host-user",
			Process:    "child-contained",
			Resources:  resources,
		},
		Warnings: windowsJobWarnings(requested),
	}
}

func windowsJobWarnings(requested protocol.IsolationOptions) []string {
	warnings := []string{"windows-job driver contains the process tree with a native Job Object, but does not enforce filesystem or network isolation yet."}
	if requested.Filesystem != "host" {
		warnings = append(warnings, "Requested filesystem isolation is not enforced by the windows-job driver yet.")
	}
	if requested.Network == "blocked" {
		warnings = append(warnings, "Requested network blocking is not enforced by the windows-job driver yet.")
	}
	if requested.Resources.CPUCount != nil {
		warnings = append(warnings, "Requested CPU count limit is not enforced by the windows-job driver yet.")
	}
	return warnings
}
