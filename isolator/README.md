# Agent API Local Isolator

`agent-isolator` is the local execution boundary for agentic shell commands.

The SDKs expose `local_shell`; this kit is the shared implementation point for
platform-native isolation drivers. It is intentionally separate from the JS,
Python, and Go SDK surfaces so each SDK can use the same local protocol instead
of reimplementing OS-specific sandbox logic.

## Contract

The isolator accepts JSON requests over stdin/stdout. Each line is one request
and one response:

```json
{"id":"req_1","method":"run","params":{"command":"npm test","cwd":"/repo"}}
```

Methods:

- `status`: report driver availability and current guarantees.
- `run`: execute one command and return stdout, stderr, exit status, and
  `shell_isolation`.

Run one request and exit:

```bash
/absolute/path/to/agent-isolator --once --driver=auto
```

SDKs do not search `PATH` for this binary. Apps should install or download the
binary to an app-managed location, then pass the absolute path through SDK
options or `AGENT_ISOLATOR_PATH`.

Run the native-driver smoke test on a target machine:

```bash
cd sdk/isolator
AGENT_ISOLATOR_NATIVE_SMOKE=1 go test ./cmd/agent-isolator -run TestAgentIsolatorNativeSmoke -v
```

Expected native drivers are `bwrap` on Linux, `sandbox-exec` on macOS, and
`windows-job` on Windows. Override the expectation when intentionally testing a
specific driver:

```bash
AGENT_ISOLATOR_NATIVE_SMOKE=1 AGENT_ISOLATOR_EXPECT_DRIVER=direct go test ./cmd/agent-isolator -run TestAgentIsolatorNativeSmoke -v
```

Driver selection:

- `--driver=auto`: prefer the first available native isolator, otherwise direct.
- `--driver=direct`: force direct host execution.
- `--driver=bwrap`: require the Linux bubblewrap driver.
- `--driver=sandbox-exec`: require the macOS sandbox-exec driver.
- `--driver=windows-job`: require the Windows Job Object driver.

The requested options are intentionally lean:

- `filesystem`: `host`, `workdir-readonly`, `workdir-readwrite`
- `network`: `allowed`, `blocked`
- `env`: `inherit`, `minimal`
- `resources.memoryMb`
- `resources.cpuCount`

The result always reports actual guarantees. Unsupported options should be
reported as warnings unless the caller uses SDK-level `isolation: "required"`
and no real isolating runner is available.

## Current Drivers

`direct` has no OS-level isolation and exists as the baseline/fallback behavior
already supported by the SDKs.

`bwrap` is the first Linux-native driver. It probes real runtime availability,
not just whether the executable exists. Many hosts disable unprivileged user
namespaces; in that case the driver is reported as unavailable and `auto`
falls back to `direct`.

The current `bwrap` driver mounts the host root read-only so common shells and
tools can run, gives write access to tmpfs paths and the requested workdir, and
can request network namespace isolation. CPU and memory limits are not enforced
yet and are reported as warnings.

`sandbox-exec` is the macOS-native driver. It probes `sandbox-exec`, generates
a temporary sandbox profile, can restrict file writes to the requested workdir
and temp paths, and can request network denial. It should be validated on the
target macOS release because sandbox profile behavior is OS-version sensitive.

`windows-job` is the Windows-native driver. It uses Job Objects to contain the
process tree and kill child processes when the job closes. It can apply a
process memory limit when requested. It does not yet enforce filesystem or
network isolation, and reports those limitations in `shell_isolation.warnings`.

Planned native drivers should live behind the same protocol:

- Linux: deeper namespace/cgroup/seccomp driver if `bwrap` is not enough.
- macOS: deeper sandbox profiles or lower-level helpers if `sandbox-exec` is not enough.
- Windows: restricted token/AppContainer/filesystem policy on top of Job Objects.
- Docker/container: optional fallback, not the first-class target.
