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
	store, err := NewFileStore(rootAbs, "")
	if err != nil {
		return err
	}
	stats, _, err := store.ListWithWarnings(".", ListOptions{
		Recursive:          recursive,
		IncludeDirectories: true,
		MaxDepth:           maxDepth,
		Ignore:             []IgnoreRule{IgnoreName(".git"), IgnoreName("node_modules"), IgnoreName("__pycache__")},
	})
	if err != nil {
		return err
	}
	candidates := []string{rootAbs}
	for _, stat := range stats {
		if stat.Type == FileTypeDirectory {
			candidates = append(candidates, stat.FullPath)
		}
	}
	for _, dir := range candidates {
		if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil && !seen[dir] {
			seen[dir] = true
			*out = append(*out, dir)
		}
	}
	return nil
}
