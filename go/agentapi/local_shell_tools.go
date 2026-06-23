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
type LocalShellIsolationMode string

const (
	LocalShellAccessApproval LocalShellAccessMode = "approval"
	LocalShellAccessFull     LocalShellAccessMode = "full"
)

const (
	LocalShellIsolationNone     LocalShellIsolationMode = "none"
	LocalShellIsolationAuto     LocalShellIsolationMode = "auto"
	LocalShellIsolationRequired LocalShellIsolationMode = "required"
)

type LocalShellIsolationGuarantees struct {
	Filesystem string `json:"filesystem"`
	Network    string `json:"network"`
	User       string `json:"user"`
	Process    string `json:"process"`
	Resources  string `json:"resources"`
}

type LocalShellIsolationResourceOptions struct {
	MemoryMB *int `json:"memoryMb,omitempty"`
	CPUCount *int `json:"cpuCount,omitempty"`
}

type LocalShellIsolationOptions struct {
	Filesystem string                             `json:"filesystem"`
	Network    string                             `json:"network"`
	Env        string                             `json:"env"`
	Resources  LocalShellIsolationResourceOptions `json:"resources"`
}

type LocalShellIsolationStatus struct {
	Executor   string                        `json:"executor"`
	Driver     string                        `json:"driver"`
	Isolated   bool                          `json:"isolated"`
	Fallback   bool                          `json:"fallback"`
	Requested  LocalShellIsolationOptions    `json:"requested"`
	Guarantees LocalShellIsolationGuarantees `json:"guarantees"`
	Warnings   []string                      `json:"warnings"`
}

type LocalShellIsolatorStatusResult struct {
	Version string                     `json:"version,omitempty"`
	Driver  string                     `json:"driver"`
	Status  LocalShellIsolationStatus  `json:"status"`
	Drivers []LocalShellIsolatorDriver `json:"drivers,omitempty"`
}

type LocalShellIsolatorDriver struct {
	Name      string   `json:"name"`
	Platform  string   `json:"platform"`
	Available bool     `json:"available"`
	Warnings  []string `json:"warnings,omitempty"`
}

type LocalShellRequest struct {
	Command     string
	Description string
	Workdir     string
	TimeoutMS   int
	Env         map[string]string
}

type LocalShellResult struct {
	OK          bool                      `json:"ok"`
	Action      string                    `json:"action"`
	Command     string                    `json:"command"`
	Description string                    `json:"description,omitempty"`
	CWD         string                    `json:"cwd"`
	ExitCode    *int                      `json:"exit_code"`
	Signal      string                    `json:"signal,omitempty"`
	Stdout      string                    `json:"stdout"`
	Stderr      string                    `json:"stderr"`
	Output      string                    `json:"output"`
	DurationMS  int64                     `json:"duration_ms"`
	TimedOut    bool                      `json:"timed_out"`
	Truncated   bool                      `json:"truncated"`
	Isolation   LocalShellIsolationStatus `json:"shell_isolation"`
}

type LocalCommandRunner interface {
	Run(context.Context, LocalShellRequest) (LocalShellResult, error)
}

type LocalShellIsolationReporter interface {
	IsolationStatus() LocalShellIsolationStatus
}

type HostLocalShellRunnerOptions struct {
	CWD              string
	Shell            string
	TimeoutMS        int
	MaxOutputBytes   int
	Env              map[string]string
	Isolation        LocalShellIsolationStatus
	IsolationOptions LocalShellIsolationOptions
}

type HostLocalShellRunner struct {
	CWD            string
	Shell          string
	TimeoutMS      int
	MaxOutputBytes int
	Env            map[string]string
	Isolation      LocalShellIsolationStatus
}

type IsolatorLocalShellRunnerOptions struct {
	ExecutablePath   string
	Driver           string
	CWD              string
	TimeoutMS        int
	MaxOutputBytes   int
	IsolationOptions LocalShellIsolationOptions
}

type IsolatorLocalShellRunner struct {
	ExecutablePath   string
	Driver           string
	CWD              string
	TimeoutMS        int
	MaxOutputBytes   int
	IsolationOptions LocalShellIsolationOptions
	StatusResult     LocalShellIsolatorStatusResult
	Isolation        LocalShellIsolationStatus
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
		Isolation:      directIsolationStatus(false, opts.IsolationOptions),
	}, nil
}

func (r *HostLocalShellRunner) IsolationStatus() LocalShellIsolationStatus {
	return r.Isolation
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
		Isolation:   r.IsolationStatus(),
	}, nil
}

func NewIsolatorLocalShellRunner(opts IsolatorLocalShellRunnerOptions) (*IsolatorLocalShellRunner, error) {
	executablePath := strings.TrimSpace(opts.ExecutablePath)
	if executablePath == "" {
		executablePath = strings.TrimSpace(os.Getenv("AGENT_ISOLATOR_PATH"))
	}
	if executablePath == "" {
		return nil, fmt.Errorf("agent-isolator executable path is not configured; pass IsolatorOptions.ExecutablePath or set AGENT_ISOLATOR_PATH")
	}
	driver := strings.TrimSpace(opts.Driver)
	if driver == "" {
		driver = "auto"
	}
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
	status, err := isolatorStatusSync(executablePath, driver)
	if err != nil {
		return nil, err
	}
	return &IsolatorLocalShellRunner{
		ExecutablePath:   executablePath,
		Driver:           driver,
		CWD:              abs,
		TimeoutMS:        timeout,
		MaxOutputBytes:   maxOutput,
		IsolationOptions: opts.IsolationOptions,
		StatusResult:     status,
		Isolation:        status.Status,
	}, nil
}

func (r *IsolatorLocalShellRunner) IsolationStatus() LocalShellIsolationStatus {
	return r.Isolation
}

func (r *IsolatorLocalShellRunner) Run(ctx context.Context, req LocalShellRequest) (LocalShellResult, error) {
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
	envelope := isolatorRequestEnvelope{
		ID:     fmt.Sprintf("run_%d", time.Now().UnixMilli()),
		Method: "run",
		Params: map[string]any{
			"command":          command,
			"description":      req.Description,
			"cwd":              cwd,
			"timeout_ms":       timeout,
			"max_output_bytes": r.MaxOutputBytes,
			"env":              req.Env,
			"isolation":        r.IsolationOptions,
		},
	}
	response, err := isolatorRequest(ctx, r.ExecutablePath, r.Driver, envelope)
	if err != nil {
		return LocalShellResult{}, err
	}
	var result LocalShellResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return LocalShellResult{}, err
	}
	if result.Command == "" {
		return LocalShellResult{}, fmt.Errorf("agent-isolator run result must include command")
	}
	return result, nil
}

type LocalShellToolRegistryOptions struct {
	AccessMode       LocalShellAccessMode
	Isolation        LocalShellIsolationMode
	IsolationOptions LocalShellIsolationOptions
	UseIsolator      bool
	IsolatorOptions  IsolatorLocalShellRunnerOptions
	ToolName         string
	Runner           LocalCommandRunner
	CWD              string
	Shell            string
	TimeoutMS        int
	MaxOutputBytes   int
}

type LocalShellToolHandler func(map[string]any) (map[string]any, error)

type LocalShellToolRegistry struct {
	Driver     *LocalShellDriver
	AccessMode LocalShellAccessMode
	ToolName   string
}

type LocalShellDriver struct {
	AccessMode      LocalShellAccessMode
	Runner          LocalCommandRunner
	IsolationStatus LocalShellIsolationStatus
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
	runner, isolationStatus, err := resolveShellRunner(opts)
	if err != nil {
		return nil, err
	}
	return &LocalShellToolRegistry{
		Driver:     &LocalShellDriver{AccessMode: accessMode, Runner: runner, IsolationStatus: isolationStatus},
		AccessMode: accessMode,
		ToolName:   toolName,
	}, nil
}

func (r *LocalShellToolRegistry) Definitions() []Tool {
	opts := LocalShellToolPresentationOptions{AccessMode: r.AccessMode, IsolationStatus: r.Driver.IsolationStatus}
	if host, ok := r.Driver.Runner.(*HostLocalShellRunner); ok {
		opts.CWD = host.CWD
		opts.Shell = host.Shell
		opts.TimeoutMS = host.TimeoutMS
		opts.MaxOutputBytes = host.MaxOutputBytes
	}
	if isolator, ok := r.Driver.Runner.(*IsolatorLocalShellRunner); ok {
		opts.CWD = isolator.CWD
		opts.TimeoutMS = isolator.TimeoutMS
		opts.MaxOutputBytes = isolator.MaxOutputBytes
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
		return shellApprovalRequired(req, d.IsolationStatus), nil
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
	AccessMode       LocalShellAccessMode
	IsolationStatus  LocalShellIsolationStatus
	IsolationOptions LocalShellIsolationOptions
	CWD              string
	Shell            string
	Platform         string
	TimeoutMS        int
	MaxOutputBytes   int
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
	isolation := presentation.IsolationStatus
	if isolation.Driver == "" {
		isolation = directIsolationStatus(false, presentation.IsolationOptions)
	}
	shell := shellDisplayName(presentation.Shell)
	parts := []string{
		"Run a local shell command through one model-facing primitive.",
		"Prefer local_workdir for file discovery, reading, and editing. Use local_shell for package managers, tests, build commands, git, and other process-level tasks.",
		fmt.Sprintf("Execution environment: platform=%s; shell=%s; access_mode=%s; isolation_driver=%s; isolated=%t; fallback=%t; default_timeout_ms=%d; max_output_bytes=%d.", platform, shell, accessMode, isolation.Driver, isolation.Isolated, isolation.Fallback, timeout, maxOutput),
		isolationRequestDescription(isolation.Requested),
		isolationWarning(isolation),
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

func resolveShellRunner(opts LocalShellToolRegistryOptions) (LocalCommandRunner, LocalShellIsolationStatus, error) {
	if opts.Runner != nil {
		status, ok := runnerIsolationStatus(opts.Runner)
		if opts.Isolation == LocalShellIsolationRequired && (!ok || !status.Isolated) {
			return nil, LocalShellIsolationStatus{}, fmt.Errorf("local_shell isolation is required, but the configured runner does not report isolation")
		}
		if ok {
			return opts.Runner, status, nil
		}
		return opts.Runner, directIsolationStatus(opts.Isolation == LocalShellIsolationAuto, opts.IsolationOptions), nil
	}
	var isolatorFallbackWarning string
	if opts.Isolation == LocalShellIsolationAuto || opts.Isolation == LocalShellIsolationRequired || opts.UseIsolator {
		isolatorOptions := opts.IsolatorOptions
		if isolatorOptions.CWD == "" {
			isolatorOptions.CWD = opts.CWD
		}
		if isolatorOptions.TimeoutMS <= 0 {
			isolatorOptions.TimeoutMS = opts.TimeoutMS
		}
		if isolatorOptions.MaxOutputBytes <= 0 {
			isolatorOptions.MaxOutputBytes = opts.MaxOutputBytes
		}
		if isolatorOptions.IsolationOptions == (LocalShellIsolationOptions{}) {
			isolatorOptions.IsolationOptions = opts.IsolationOptions
		}
		runner, err := NewIsolatorLocalShellRunner(isolatorOptions)
		if err == nil {
			if opts.Isolation == LocalShellIsolationRequired && !runner.IsolationStatus().Isolated {
				return nil, LocalShellIsolationStatus{}, fmt.Errorf("local_shell isolation is required, but agent-isolator selected non-isolated driver %s", runner.IsolationStatus().Driver)
			}
			if opts.Isolation != LocalShellIsolationNone || opts.UseIsolator {
				return runner, runner.IsolationStatus(), nil
			}
		} else if opts.Isolation == LocalShellIsolationRequired {
			return nil, LocalShellIsolationStatus{}, err
		} else {
			isolatorFallbackWarning = err.Error()
		}
	}
	if opts.Isolation == LocalShellIsolationRequired {
		return nil, LocalShellIsolationStatus{}, fmt.Errorf("local_shell isolation is required, but no isolating runner was configured")
	}
	runner, err := NewHostLocalShellRunner(HostLocalShellRunnerOptions{
		CWD:              opts.CWD,
		Shell:            opts.Shell,
		TimeoutMS:        opts.TimeoutMS,
		MaxOutputBytes:   opts.MaxOutputBytes,
		IsolationOptions: opts.IsolationOptions,
	})
	if err != nil {
		return nil, LocalShellIsolationStatus{}, err
	}
	if opts.Isolation == LocalShellIsolationAuto {
		runner.Isolation = directIsolationStatus(true, opts.IsolationOptions)
		if isolatorFallbackWarning != "" {
			runner.Isolation.Warnings = append(runner.Isolation.Warnings, "Isolator unavailable: "+isolatorFallbackWarning)
		}
	}
	return runner, runner.IsolationStatus(), nil
}

func runnerIsolationStatus(runner LocalCommandRunner) (LocalShellIsolationStatus, bool) {
	reporter, ok := runner.(LocalShellIsolationReporter)
	if !ok {
		return LocalShellIsolationStatus{}, false
	}
	status := reporter.IsolationStatus()
	return status, status.Driver != ""
}

type isolatorRequestEnvelope struct {
	ID     string         `json:"id"`
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

type isolatorResponseEnvelope struct {
	ID     string          `json:"id,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    string `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"error,omitempty"`
}

func isolatorStatusSync(executablePath string, driver string) (LocalShellIsolatorStatusResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	response, err := isolatorRequest(ctx, executablePath, driver, isolatorRequestEnvelope{
		ID:     "status",
		Method: "status",
		Params: map[string]any{},
	})
	if err != nil {
		return LocalShellIsolatorStatusResult{}, err
	}
	var result LocalShellIsolatorStatusResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return LocalShellIsolatorStatusResult{}, err
	}
	if result.Driver == "" || result.Status.Driver == "" {
		return LocalShellIsolatorStatusResult{}, fmt.Errorf("agent-isolator status result must include driver and status")
	}
	return result, nil
}

func isolatorRequest(ctx context.Context, executablePath string, driver string, envelope isolatorRequestEnvelope) (isolatorResponseEnvelope, error) {
	payload, err := json.Marshal(envelope)
	if err != nil {
		return isolatorResponseEnvelope{}, err
	}
	cmd := exec.CommandContext(ctx, executablePath, "--once", "--driver="+driver)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		details := strings.TrimSpace(firstNonEmpty(stderr.String(), stdout.String(), err.Error()))
		return isolatorResponseEnvelope{}, fmt.Errorf("%s", details)
	}
	if strings.TrimSpace(stdout.String()) == "" {
		return isolatorResponseEnvelope{}, fmt.Errorf("agent-isolator returned an empty response")
	}
	var response isolatorResponseEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		details := strings.TrimSpace(stderr.String())
		if details != "" {
			return isolatorResponseEnvelope{}, fmt.Errorf("%w: %s", err, details)
		}
		return isolatorResponseEnvelope{}, err
	}
	if response.Error != nil {
		message := firstNonEmpty(response.Error.Message, response.Error.Code, "agent-isolator request failed")
		return isolatorResponseEnvelope{}, fmt.Errorf("%s", message)
	}
	return response, nil
}

func directIsolationStatus(fallback bool, opts LocalShellIsolationOptions) LocalShellIsolationStatus {
	requested := normalizeIsolationOptions(opts)
	return LocalShellIsolationStatus{
		Executor:  "direct",
		Driver:    "direct",
		Isolated:  false,
		Fallback:  fallback,
		Requested: requested,
		Guarantees: LocalShellIsolationGuarantees{
			Filesystem: "none",
			Network:    "allowed",
			User:       "host-user",
			Process:    "host-process-tree",
			Resources:  "timeout-only",
		},
		Warnings: directIsolationWarnings(fallback, requested),
	}
}

func normalizeIsolationOptions(opts LocalShellIsolationOptions) LocalShellIsolationOptions {
	if opts.Filesystem == "" {
		opts.Filesystem = "host"
	}
	if opts.Network == "" {
		opts.Network = "allowed"
	}
	if opts.Env == "" {
		opts.Env = "inherit"
	}
	return opts
}

func directIsolationWarnings(fallback bool, requested LocalShellIsolationOptions) []string {
	warning := "Direct host execution has no OS-level isolation."
	if fallback {
		warning = "No local shell isolator is configured; falling back to direct host execution."
	}
	warnings := []string{warning}
	if requested.Filesystem != "host" {
		warnings = append(warnings, fmt.Sprintf("Requested filesystem isolation (%s) is not enforced by direct execution.", requested.Filesystem))
	}
	if requested.Network == "blocked" {
		warnings = append(warnings, "Requested network blocking is not enforced by direct execution.")
	}
	if requested.Env == "minimal" {
		warnings = append(warnings, "Requested minimal environment is not enforced by direct execution.")
	}
	if requested.Resources.MemoryMB != nil || requested.Resources.CPUCount != nil {
		warnings = append(warnings, "Requested CPU or memory limits are not enforced by direct execution.")
	}
	return warnings
}

func isolationRequestDescription(opts LocalShellIsolationOptions) string {
	parts := []string{fmt.Sprintf("Requested isolation: filesystem=%s; network=%s; env=%s", opts.Filesystem, opts.Network, opts.Env)}
	if opts.Resources.MemoryMB != nil {
		parts = append(parts, fmt.Sprintf("memory_mb=%d", *opts.Resources.MemoryMB))
	}
	if opts.Resources.CPUCount != nil {
		parts = append(parts, fmt.Sprintf("cpu_count=%d", *opts.Resources.CPUCount))
	}
	return strings.Join(parts, "; ") + "."
}

func isolationWarning(status LocalShellIsolationStatus) string {
	if len(status.Warnings) == 0 {
		return ""
	}
	return "Isolation warning: " + strings.Join(status.Warnings, " ")
}

func shellApprovalRequired(req LocalShellRequest, isolation LocalShellIsolationStatus) map[string]any {
	return map[string]any{
		"ok":                false,
		"requires_approval": true,
		"action":            "run",
		"command":           req.Command,
		"description":       req.Description,
		"workdir":           req.Workdir,
		"timeout_ms":        req.TimeoutMS,
		"shell_isolation":   shellIsolationMap(isolation),
		"message":           "local_shell command execution requires approval",
	}
}

func shellIsolationMap(status LocalShellIsolationStatus) map[string]any {
	return map[string]any{
		"executor": status.Executor,
		"driver":   status.Driver,
		"isolated": status.Isolated,
		"fallback": status.Fallback,
		"requested": map[string]any{
			"filesystem": status.Requested.Filesystem,
			"network":    status.Requested.Network,
			"env":        status.Requested.Env,
			"resources": map[string]any{
				"memoryMb": status.Requested.Resources.MemoryMB,
				"cpuCount": status.Requested.Resources.CPUCount,
			},
		},
		"guarantees": map[string]any{
			"filesystem": status.Guarantees.Filesystem,
			"network":    status.Guarantees.Network,
			"user":       status.Guarantees.User,
			"process":    status.Guarantees.Process,
			"resources":  status.Guarantees.Resources,
		},
		"warnings": status.Warnings,
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
