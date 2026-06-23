package protocol

type Envelope struct {
	ID     string         `json:"id,omitempty"`
	Method string         `json:"method,omitempty"`
	Params map[string]any `json:"params,omitempty"`
	Result any            `json:"result,omitempty"`
	Error  *Error         `json:"error,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type RunRequest struct {
	Command        string            `json:"command"`
	Description    string            `json:"description,omitempty"`
	CWD            string            `json:"cwd,omitempty"`
	Workdir        string            `json:"workdir,omitempty"`
	TimeoutMS      int               `json:"timeout_ms,omitempty"`
	MaxOutputBytes int               `json:"max_output_bytes,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Isolation      IsolationOptions  `json:"isolation,omitempty"`
}

type RunResult struct {
	OK          bool            `json:"ok"`
	Action      string          `json:"action"`
	Command     string          `json:"command"`
	Description string          `json:"description,omitempty"`
	CWD         string          `json:"cwd"`
	ExitCode    *int            `json:"exit_code"`
	Signal      string          `json:"signal,omitempty"`
	Stdout      string          `json:"stdout"`
	Stderr      string          `json:"stderr"`
	Output      string          `json:"output"`
	DurationMS  int64           `json:"duration_ms"`
	TimedOut    bool            `json:"timed_out"`
	Truncated   bool            `json:"truncated"`
	Isolation   IsolationStatus `json:"shell_isolation"`
}

type StatusResult struct {
	Version string            `json:"version"`
	Driver  string            `json:"driver"`
	Status  IsolationStatus   `json:"status"`
	Drivers []DriverDiscovery `json:"drivers"`
}

type DriverDiscovery struct {
	Name      string   `json:"name"`
	Platform  string   `json:"platform"`
	Available bool     `json:"available"`
	Warnings  []string `json:"warnings,omitempty"`
}

type IsolationOptions struct {
	Filesystem string                   `json:"filesystem,omitempty"`
	Network    string                   `json:"network,omitempty"`
	Env        string                   `json:"env,omitempty"`
	Resources  IsolationResourceOptions `json:"resources,omitempty"`
}

type IsolationResourceOptions struct {
	MemoryMB *int `json:"memoryMb,omitempty"`
	CPUCount *int `json:"cpuCount,omitempty"`
}

type IsolationGuarantees struct {
	Filesystem string `json:"filesystem"`
	Network    string `json:"network"`
	User       string `json:"user"`
	Process    string `json:"process"`
	Resources  string `json:"resources"`
}

type IsolationStatus struct {
	Executor   string              `json:"executor"`
	Driver     string              `json:"driver"`
	Isolated   bool                `json:"isolated"`
	Fallback   bool                `json:"fallback"`
	Requested  IsolationOptions    `json:"requested"`
	Guarantees IsolationGuarantees `json:"guarantees"`
	Warnings   []string            `json:"warnings"`
}

func NormalizeIsolationOptions(opts IsolationOptions) IsolationOptions {
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
