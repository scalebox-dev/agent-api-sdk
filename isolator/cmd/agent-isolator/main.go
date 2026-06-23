package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/scalebox-dev/agent-api-sdk/isolator/internal/driver"
	"github.com/scalebox-dev/agent-api-sdk/isolator/internal/protocol"
)

var version = "0.0.0-dev"

func main() {
	once := flag.Bool("once", false, "read one JSON request from stdin and write one JSON response")
	driverName := flag.String("driver", "auto", "isolation driver: auto, direct, bwrap, sandbox-exec, or windows-job")
	flag.Parse()

	runner, discoveries, err := selectDriver(context.Background(), *driverName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *once {
		if err := handleOnce(os.Stdin, os.Stdout, runner, discoveries); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := serveJSONL(context.Background(), os.Stdin, os.Stdout, runner, discoveries); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func handleOnce(in io.Reader, out io.Writer, runner driver.Driver, discoveries []protocol.DriverDiscovery) error {
	var req protocol.Envelope
	if err := json.NewDecoder(in).Decode(&req); err != nil {
		return err
	}
	resp := handle(context.Background(), req, runner, discoveries)
	return json.NewEncoder(out).Encode(resp)
}

func serveJSONL(ctx context.Context, in io.Reader, out io.Writer, runner driver.Driver, discoveries []protocol.DriverDiscovery) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	encoder := json.NewEncoder(out)
	for scanner.Scan() {
		var req protocol.Envelope
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			if encodeErr := encoder.Encode(protocol.Envelope{Error: &protocol.Error{Code: "bad_json", Message: err.Error()}}); encodeErr != nil {
				return encodeErr
			}
			continue
		}
		if err := encoder.Encode(handle(ctx, req, runner, discoveries)); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func handle(ctx context.Context, req protocol.Envelope, runner driver.Driver, discoveries []protocol.DriverDiscovery) protocol.Envelope {
	resp := protocol.Envelope{ID: req.ID}
	switch req.Method {
	case "status":
		resp.Result = protocol.StatusResult{
			Version: version,
			Driver:  runner.Name(),
			Status:  runner.Status(protocol.IsolationOptions{}),
			Drivers: discoveries,
		}
	case "run":
		runReq, err := decodeParams[protocol.RunRequest](req.Params)
		if err != nil {
			resp.Error = &protocol.Error{Code: "bad_params", Message: err.Error()}
			return resp
		}
		result, err := runner.Run(ctx, runReq)
		if err != nil {
			resp.Error = &protocol.Error{Code: "run_failed", Message: err.Error()}
			return resp
		}
		resp.Result = result
	default:
		resp.Error = &protocol.Error{Code: "unknown_method", Message: "unknown method: " + req.Method}
	}
	return resp
}

func selectDriver(ctx context.Context, name string) (driver.Driver, []protocol.DriverDiscovery, error) {
	bwrap := driver.DiscoverBwrap(ctx)
	sandboxExec := driver.DiscoverSandboxExec(ctx)
	windowsJob := driver.DiscoverWindowsJob(ctx)
	discoveries := []protocol.DriverDiscovery{
		{Name: "direct", Platform: runtime.GOOS, Available: true, Warnings: []string{"Direct driver has no OS-level isolation."}},
		discoveryInfo("bwrap", bwrap),
		discoveryInfo("sandbox-exec", sandboxExec),
		discoveryInfo("windows-job", windowsJob),
	}
	switch name {
	case "", "auto":
		switch runtime.GOOS {
		case "linux":
			if bwrap.Available {
				return bwrap.Driver, discoveries, nil
			}
		case "darwin":
			if sandboxExec.Available {
				return sandboxExec.Driver, discoveries, nil
			}
		case "windows":
			if windowsJob.Available {
				return windowsJob.Driver, discoveries, nil
			}
		}
		if bwrap.Available {
			return bwrap.Driver, discoveries, nil
		}
		if sandboxExec.Available {
			return sandboxExec.Driver, discoveries, nil
		}
		if windowsJob.Available {
			return windowsJob.Driver, discoveries, nil
		}
		return driver.Direct{}, discoveries, nil
	case "direct":
		return driver.Direct{}, discoveries, nil
	case "bwrap":
		if !bwrap.Available {
			return nil, discoveries, fmt.Errorf("bwrap driver is not available: %s", strings.Join(bwrap.Warnings, " "))
		}
		return bwrap.Driver, discoveries, nil
	case "sandbox-exec":
		if !sandboxExec.Available {
			return nil, discoveries, fmt.Errorf("sandbox-exec driver is not available: %s", strings.Join(sandboxExec.Warnings, " "))
		}
		return sandboxExec.Driver, discoveries, nil
	case "windows-job":
		if !windowsJob.Available {
			return nil, discoveries, fmt.Errorf("windows-job driver is not available: %s", strings.Join(windowsJob.Warnings, " "))
		}
		return windowsJob.Driver, discoveries, nil
	default:
		return nil, discoveries, fmt.Errorf("unknown driver %q", name)
	}
}

func discoveryInfo(name string, discovery driver.Discovery) protocol.DriverDiscovery {
	return protocol.DriverDiscovery{
		Name:      name,
		Platform:  runtime.GOOS,
		Available: discovery.Available,
		Warnings:  discovery.Warnings,
	}
}

func decodeParams[T any](params map[string]any) (T, error) {
	var out T
	data, err := json.Marshal(params)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	return out, nil
}
