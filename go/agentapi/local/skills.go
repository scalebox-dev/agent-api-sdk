package local

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/scalebox-dev/agent-api-sdk/go/agentapi"
)

type SkillStore struct {
	Files *FileStore
}

type SkillDiscoveryOptions struct {
	Roots     []string
	Recursive bool
	MaxDepth  int
	Directory agentapi.LocalSkillDirectoryOptions
}

func (s *SkillStore) FromDirectory(root string, opts agentapi.LocalSkillDirectoryOptions) (*agentapi.LocalSkillDescriptor, error) {
	return agentapi.LocalSkillFromDirectory(root, opts)
}

func (s *SkillStore) Discover(opts SkillDiscoveryOptions) ([]agentapi.LocalSkillDescriptor, error) {
	roots := opts.Roots
	if len(roots) == 0 {
		roots = []string{s.Files.Root}
	}
	seen := map[string]bool{}
	var dirs []string
	for _, root := range roots {
		if err := discoverSkillDirs(root, opts.Recursive, opts.MaxDepth, seen, &dirs); err != nil {
			return nil, err
		}
	}
	sort.Strings(dirs)
	out := make([]agentapi.LocalSkillDescriptor, 0, len(dirs))
	for _, dir := range dirs {
		desc, err := agentapi.LocalSkillFromDirectory(dir, opts.Directory)
		if err != nil {
			return nil, err
		}
		out = append(out, *desc)
	}
	return out, nil
}

func discoverSkillDirs(root string, recursive bool, maxDepth int, seen map[string]bool, out *[]string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	info, err := os.Stat(rootAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	depthLimit := 1
	if recursive {
		depthLimit = maxDepth
	}
	var walk func(string, int) error
	walk = func(dir string, depth int) error {
		if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil && !seen[dir] {
			seen[dir] = true
			*out = append(*out, dir)
		}
		if depthLimit > 0 && depth >= depthLimit {
			return nil
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !entry.IsDir() || entry.Name() == ".git" || entry.Name() == "node_modules" || entry.Name() == "__pycache__" {
				continue
			}
			if err := walk(filepath.Join(dir, entry.Name()), depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(rootAbs, 0)
}
