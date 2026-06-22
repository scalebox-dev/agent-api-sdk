package agentapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type LocalShellAccessMode string

const (
	LocalShellAccessApproval LocalShellAccessMode = "approval"
	LocalShellAccessFull     LocalShellAccessMode = "full"
)

type LocalShellRequest struct {
	Command     string
	Description string
	Workdir     string
	TimeoutMS   int
	Env         map[string]string
}

type LocalShellResult struct {
	OK          bool   `json:"ok"`
	Action      string `json:"action"`
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
	CWD         string `json:"cwd"`
	ExitCode    *int   `json:"exit_code"`
	Signal      string `json:"signal,omitempty"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	Output      string `json:"output"`
	DurationMS  int64  `json:"duration_ms"`
	TimedOut    bool   `json:"timed_out"`
	Truncated   bool   `json:"truncated"`
}

type LocalCommandRunner interface {
	Run(context.Context, LocalShellRequest) (LocalShellResult, error)
}

type HostLocalShellRunnerOptions struct {
	CWD            string
	Shell          string
	TimeoutMS      int
	MaxOutputBytes int
	Env            map[string]string
}

type HostLocalShellRunner struct {
	CWD            string
	Shell          string
	TimeoutMS      int
	MaxOutputBytes int
	Env            map[string]string
}

func NewHostLocalShellRunner(opts HostLocalShellRunnerOptions) (*HostLocalShellRunner, error) {
	cwd := opts.CWD
	if strings.TrimSpace(cwd) == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}
	timeout := opts.TimeoutMS
	if timeout <= 0 {
		timeout = 120_000
	}
	maxOutput := opts.MaxOutputBytes
	if maxOutput <= 0 {
		maxOutput = 128 * 1024
	}
	return &HostLocalShellRunner{
		CWD:            abs,
		Shell:          opts.Shell,
		TimeoutMS:      timeout,
		MaxOutputBytes: maxOutput,
		Env:            opts.Env,
	}, nil
}

func (r *HostLocalShellRunner) Run(ctx context.Context, req LocalShellRequest) (LocalShellResult, error) {
	command := strings.TrimSpace(req.Command)
	if command == "" {
		return LocalShellResult{}, fmt.Errorf("command must be a non-empty string")
	}
	cwd, err := resolveContainedShellPath(r.CWD, req.Workdir)
	if err != nil {
		return LocalShellResult{}, err
	}
	timeout := r.TimeoutMS
	if req.TimeoutMS > 0 {
		timeout = req.TimeoutMS
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()
	started := time.Now()
	cmd := shellCommand(runCtx, r.Shell, command)
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	for key, value := range r.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
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
	signal := ""
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			code := exit.ExitCode()
			exitCodePtr = &code
		} else if !timedOut {
			return LocalShellResult{}, err
		} else {
			exitCodePtr = nil
		}
	}
	out, outTruncated := boundedText(stdout.Bytes(), r.MaxOutputBytes)
	errText, errTruncated := boundedText(stderr.Bytes(), r.MaxOutputBytes)
	output := strings.TrimRight(strings.Join(nonEmptyStrings(out, errText), "\n"), "\n")
	if output == "" {
		output = "(no output)"
	}
	return LocalShellResult{
		OK:          true,
		Action:      "run",
		Command:     command,
		Description: req.Description,
		CWD:         cwd,
		ExitCode:    exitCodePtr,
		Signal:      signal,
		Stdout:      out,
		Stderr:      errText,
		Output:      output,
		DurationMS:  time.Since(started).Milliseconds(),
		TimedOut:    timedOut,
		Truncated:   outTruncated || errTruncated,
	}, nil
}

type LocalShellToolRegistryOptions struct {
	AccessMode     LocalShellAccessMode
	ToolName       string
	Runner         LocalCommandRunner
	CWD            string
	Shell          string
	TimeoutMS      int
	MaxOutputBytes int
}

type LocalShellToolHandler func(map[string]any) (map[string]any, error)

type LocalShellToolRegistry struct {
	Driver     *LocalShellDriver
	AccessMode LocalShellAccessMode
	ToolName   string
}

type LocalShellDriver struct {
	AccessMode LocalShellAccessMode
	Runner     LocalCommandRunner
}

func CreateLocalShellToolRegistry(opts LocalShellToolRegistryOptions) (*LocalShellToolRegistry, error) {
	toolName := strings.TrimSpace(opts.ToolName)
	if toolName == "" {
		toolName = "local_shell"
	}
	accessMode := opts.AccessMode
	if accessMode == "" {
		accessMode = LocalShellAccessApproval
	}
	runner := opts.Runner
	if runner == nil {
		var err error
		runner, err = NewHostLocalShellRunner(HostLocalShellRunnerOptions{
			CWD:            opts.CWD,
			Shell:          opts.Shell,
			TimeoutMS:      opts.TimeoutMS,
			MaxOutputBytes: opts.MaxOutputBytes,
		})
		if err != nil {
			return nil, err
		}
	}
	return &LocalShellToolRegistry{
		Driver:     &LocalShellDriver{AccessMode: accessMode, Runner: runner},
		AccessMode: accessMode,
		ToolName:   toolName,
	}, nil
}

func (r *LocalShellToolRegistry) Definitions() []Tool {
	opts := LocalShellToolPresentationOptions{AccessMode: r.AccessMode}
	if host, ok := r.Driver.Runner.(*HostLocalShellRunner); ok {
		opts.CWD = host.CWD
		opts.Shell = host.Shell
		opts.TimeoutMS = host.TimeoutMS
		opts.MaxOutputBytes = host.MaxOutputBytes
	}
	return []Tool{LocalShellToolDefinition(r.ToolName, opts)}
}

func (r *LocalShellToolRegistry) Handlers() map[string]LocalShellToolHandler {
	return map[string]LocalShellToolHandler{r.ToolName: r.Driver.Dispatch}
}

func (r *LocalShellToolRegistry) Execute(name string, args map[string]any) (map[string]any, error) {
	return r.ExecuteContext(context.Background(), name, args)
}

func (r *LocalShellToolRegistry) ExecuteContext(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
	if name != r.ToolName {
		return nil, fmt.Errorf("unknown local shell tool: %s", name)
	}
	return r.Driver.DispatchContext(ctx, args)
}

func (r *LocalShellToolRegistry) RequiresApproval(name string, args map[string]any) bool {
	return name == r.ToolName && r.Driver.RequiresApproval(args)
}

func (d *LocalShellDriver) Dispatch(args map[string]any) (map[string]any, error) {
	return d.DispatchContext(context.Background(), args)
}

func (d *LocalShellDriver) DispatchContext(ctx context.Context, args map[string]any) (map[string]any, error) {
	req, err := shellRequest(args)
	if err != nil {
		return nil, err
	}
	if d.AccessMode != LocalShellAccessFull {
		return shellApprovalRequired(req), nil
	}
	result, err := d.Runner.Run(ctx, req)
	if err != nil {
		return nil, err
	}
	return shellToolResult(result)
}

func (d *LocalShellDriver) RequiresApproval(map[string]any) bool {
	return d.AccessMode != LocalShellAccessFull
}

type LocalShellToolPresentationOptions struct {
	AccessMode     LocalShellAccessMode
	CWD            string
	Shell          string
	Platform       string
	TimeoutMS      int
	MaxOutputBytes int
}

func LocalShellToolDefinition(name string, opts ...LocalShellToolPresentationOptions) Tool {
	if strings.TrimSpace(name) == "" {
		name = "local_shell"
	}
	strict := false
	presentation := LocalShellToolPresentationOptions{}
	if len(opts) > 0 {
		presentation = opts[0]
	}
	return Tool{
		Type:        "function",
		Name:        name,
		Description: LocalShellToolInstructions(presentation),
		Parameters:  localShellToolParameters(),
		Strict:      &strict,
	}
}

func LocalShellToolInstructions(opts ...LocalShellToolPresentationOptions) string {
	presentation := LocalShellToolPresentationOptions{}
	if len(opts) > 0 {
		presentation = opts[0]
	}
	accessMode := presentation.AccessMode
	if accessMode == "" {
		accessMode = LocalShellAccessApproval
	}
	platform := presentation.Platform
	if platform == "" {
		platform = runtime.GOOS
	}
	timeout := presentation.TimeoutMS
	if timeout <= 0 {
		timeout = 120_000
	}
	maxOutput := presentation.MaxOutputBytes
	if maxOutput <= 0 {
		maxOutput = 128 * 1024
	}
	shell := shellDisplayName(presentation.Shell)
	parts := []string{
		"Run a local shell command through one model-facing primitive.",
		"Prefer local_workdir for file discovery, reading, and editing. Use local_shell for package managers, tests, build commands, git, and other process-level tasks.",
		fmt.Sprintf("Execution environment: platform=%s; shell=%s; access_mode=%s; default_timeout_ms=%d; max_output_bytes=%d.", platform, shell, accessMode, timeout, maxOutput),
		"The workdir parameter must be a relative child path of the configured cwd. Use workdir instead of cd when possible.",
		"Absolute paths inside the command are permitted by the host OS if the user/process has permission; this tool is not a filesystem sandbox or security sandbox.",
		"Captured stdout/stderr may be truncated when output exceeds the advertised max_output_bytes.",
		"In approval mode, calls return requires_approval instead of executing. In full mode, commands execute immediately.",
		shellGuidance(platform, presentation.Shell),
	}
	if presentation.CWD != "" {
		parts = append(parts[:3], append([]string{fmt.Sprintf("Default cwd: %s. Relative command paths resolve from this cwd unless workdir is set.", presentation.CWD)}, parts[3:]...)...)
	} else {
		parts = append(parts[:3], append([]string{"Relative command paths resolve from the configured cwd unless workdir is set."}, parts[3:]...)...)
	}
	return strings.Join(parts, " ")
}

func localShellToolParameters() map[string]any {
	return objectSchema(
		map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "Shell command to execute. Keep it focused and quote paths containing spaces.",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Short human-readable description of why this command is being run.",
			},
			"workdir": map[string]any{
				"type":        "string",
				"description": "Optional relative working directory. Use this instead of cd. Absolute workdir values are rejected by the default host runner.",
			},
			"timeout_ms": map[string]any{
				"type":        "integer",
				"description": "Optional timeout in milliseconds.",
			},
		},
		[]string{"command"},
	)
}

func shellRequest(args map[string]any) (LocalShellRequest, error) {
	command, err := stringArg(args, "command")
	if err != nil {
		return LocalShellRequest{}, err
	}
	description, err := optionalStringArg(args, "description")
	if err != nil {
		return LocalShellRequest{}, err
	}
	workdir, err := optionalStringArg(args, "workdir")
	if err != nil {
		return LocalShellRequest{}, err
	}
	timeout, _, err := optionalIntArg(args, "timeoutMs", "timeout_ms")
	if err != nil {
		return LocalShellRequest{}, err
	}
	return LocalShellRequest{Command: command, Description: description, Workdir: workdir, TimeoutMS: timeout}, nil
}

func shellApprovalRequired(req LocalShellRequest) map[string]any {
	return map[string]any{
		"ok":                false,
		"requires_approval": true,
		"action":            "run",
		"command":           req.Command,
		"description":       req.Description,
		"workdir":           req.Workdir,
		"timeout_ms":        req.TimeoutMS,
		"message":           "local_shell command execution requires approval",
	}
}

func shellToolResult(result LocalShellResult) (map[string]any, error) {
	raw := map[string]any{}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func shellCommand(ctx context.Context, shell string, command string) *exec.Cmd {
	if strings.TrimSpace(shell) != "" {
		if isPowerShell(shell) {
			return exec.CommandContext(ctx, shell, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", command)
		}
		if isCmd(shell) {
			return exec.CommandContext(ctx, shell, "/C", command)
		}
		return exec.CommandContext(ctx, shell, "-c", command)
	}
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd.exe", "/C", command)
	}
	return exec.CommandContext(ctx, "/bin/sh", "-c", command)
}

func resolveContainedShellPath(root string, child string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("cwd is required")
	}
	if strings.TrimSpace(child) == "" {
		return root, nil
	}
	if filepath.IsAbs(child) {
		return "", fmt.Errorf("workdir must be relative to the configured local shell cwd")
	}
	resolved := filepath.Clean(filepath.Join(root, child))
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("workdir must stay inside the configured local shell cwd")
	}
	return resolved, nil
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

func nonEmptyStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func shellDisplayName(shell string) string {
	if strings.TrimSpace(shell) != "" {
		return shell
	}
	if runtime.GOOS == "windows" {
		return "cmd.exe"
	}
	return "/bin/sh"
}

func shellGuidance(platform string, shell string) string {
	name := strings.ToLower(filepath.Base(shellDisplayName(shell)))
	if strings.HasPrefix(platform, "win") || strings.Contains(name, "powershell") || name == "pwsh" {
		return "On Windows/PowerShell, prefer PowerShell-compatible commands and quote paths with spaces."
	}
	if name == "cmd" || name == "cmd.exe" {
		return "On cmd.exe, prefer cmd-compatible syntax and use && only for dependent command chaining."
	}
	return "On POSIX shells, prefer portable shell syntax, quote paths with spaces, and use && only when later commands depend on earlier success."
}

func isPowerShell(shell string) bool {
	name := strings.ToLower(filepath.Base(shell))
	return name == "pwsh" || name == "powershell" || name == "powershell.exe" || name == "pwsh.exe"
}

func isCmd(shell string) bool {
	name := strings.ToLower(filepath.Base(shell))
	return name == "cmd" || name == "cmd.exe"
}
