package agentapi

import (
	"fmt"
	"strings"
)

func validateUniqueToolNames(tools []Tool) error {
	seen := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			return fmt.Errorf("tools[].name is required")
		}
		if _, ok := seen[name]; ok {
			return fmt.Errorf("duplicate tools[].name: %s", name)
		}
		seen[name] = struct{}{}
	}
	return nil
}
