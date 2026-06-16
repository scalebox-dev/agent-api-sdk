package main

import (
	"fmt"
	"os"

	"github.com/scalebox-dev/agent-api-sdk/go/agentapi"
)

func main() {
	routes := agentapi.SupportedRoutes()
	if len(routes) != 47 {
		fmt.Fprintf(os.Stderr, "expected 47 routes, got %d\n", len(routes))
		os.Exit(1)
	}
	seen := map[string]bool{}
	for _, route := range routes {
		if route.Symbol == "" || route.Method == "" || route.Path == "" {
			fmt.Fprintf(os.Stderr, "incomplete route: %+v\n", route)
			os.Exit(1)
		}
		if seen[route.Symbol] {
			fmt.Fprintf(os.Stderr, "duplicate route symbol: %s\n", route.Symbol)
			os.Exit(1)
		}
		seen[route.Symbol] = true
	}
	fmt.Printf("ok: %d Go SDK routes covered\n", len(routes))
}
