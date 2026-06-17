package local

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

type IgnoreRule func(string) bool

func AppDirsFor(appName string, opts AppDirsOptions) (AppDirs, error) {
	name := strings.TrimSpace(appName)
	if name == "" {
		return AppDirs{}, &Error{Code: "local_config_error", Msg: "appName is required"}
	}
	env := opts.Env
	if env == nil {
		env = getenvMap()
	}
	platform := opts.Platform
	if platform == "" {
		platform = runtime.GOOS
	}
	home := firstNonEmpty(env["HOME"], env["USERPROFILE"])
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return AppDirs{}, err
		}
	}
	home, _ = filepath.Abs(home)
	author := sanitizeSegment(firstNonEmpty(opts.AppAuthor, name))
	app := sanitizeSegment(name)
	base := ""
	if opts.BaseDir != "" {
		base, _ = filepath.Abs(opts.BaseDir)
	}
	defaults := map[string]string{}
	switch {
	case base != "":
		defaults = map[string]string{"data": filepath.Join(base, "data"), "config": filepath.Join(base, "config"), "cache": filepath.Join(base, "cache"), "logs": filepath.Join(base, "logs"), "temp": filepath.Join(base, "tmp")}
	case platform == "darwin":
		defaults = map[string]string{"data": filepath.Join(home, "Library", "Application Support", app), "config": filepath.Join(home, "Library", "Application Support", app), "cache": filepath.Join(home, "Library", "Caches", app), "logs": filepath.Join(home, "Library", "Logs", app), "temp": filepath.Join(os.TempDir(), app)}
	case platform == "windows":
		roaming := firstNonEmpty(env["APPDATA"], filepath.Join(home, "AppData", "Roaming"))
		local := firstNonEmpty(env["LOCALAPPDATA"], filepath.Join(home, "AppData", "Local"))
		defaults = map[string]string{"data": filepath.Join(roaming, author, app), "config": filepath.Join(roaming, author, app), "cache": filepath.Join(local, author, app, "Cache"), "logs": filepath.Join(local, author, app, "Logs"), "temp": filepath.Join(os.TempDir(), app)}
	default:
		defaults = map[string]string{"data": filepath.Join(firstNonEmpty(env["XDG_DATA_HOME"], filepath.Join(home, ".local", "share")), app), "config": filepath.Join(firstNonEmpty(env["XDG_CONFIG_HOME"], filepath.Join(home, ".config")), app), "cache": filepath.Join(firstNonEmpty(env["XDG_CACHE_HOME"], filepath.Join(home, ".cache")), app), "logs": filepath.Join(firstNonEmpty(env["XDG_STATE_HOME"], filepath.Join(home, ".local", "state")), app, "logs"), "temp": filepath.Join(os.TempDir(), app)}
	}
	dirs := opts.Dirs
	if dirs == nil {
		dirs = map[string]string{}
	}
	return AppDirs{
		Home:   home,
		Data:   absOrDefault(dirs["data"], defaults["data"]),
		Config: absOrDefault(dirs["config"], defaults["config"]),
		Cache:  absOrDefault(dirs["cache"], defaults["cache"]),
		Logs:   absOrDefault(dirs["logs"], defaults["logs"]),
		Temp:   absOrDefault(dirs["temp"], defaults["temp"]),
	}, nil
}

type AppDirsOptions struct {
	AppAuthor string
	BaseDir   string
	Dirs      map[string]string
	Env       map[string]string
	Platform  string
}

func NormalizeRelativePath(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "." {
		return ".", nil
	}
	if filepath.IsAbs(trimmed) {
		return "", pathError("local path must be relative", value)
	}
	return filepath.ToSlash(filepath.Clean(trimmed)), nil
}

func ResolveInside(root, rel string) (string, string, error) {
	clean, err := NormalizeRelativePath(rel)
	if err != nil {
		return "", "", err
	}
	full, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(clean)))
	if err != nil {
		return "", "", err
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	relative, err := filepath.Rel(rootAbs, full)
	if err != nil {
		return "", "", err
	}
	if strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == ".." || filepath.IsAbs(relative) {
		return "", "", pathError("local path must stay inside the store root", full)
	}
	return full, filepath.ToSlash(relative), nil
}

func IgnoreName(name string) IgnoreRule {
	return func(rel string) bool {
		clean := strings.Trim(strings.ReplaceAll(name, "\\", "/"), "/")
		return rel == clean || strings.HasPrefix(rel, clean+"/") || strings.HasSuffix(rel, "/"+clean) || strings.Contains(rel, "/"+clean+"/")
	}
}

func IgnoreGlob(pattern string) IgnoreRule {
	return func(rel string) bool {
		pattern = strings.TrimLeft(strings.ReplaceAll(pattern, "\\", "/"), "/")
		ok, _ := filepath.Match(pattern, rel)
		if ok {
			return true
		}
		ok, _ = filepath.Match(pattern, filepath.Base(rel))
		return ok
	}
}

func IgnoreRegexp(re *regexp.Regexp) IgnoreRule {
	return func(rel string) bool { return re.MatchString(rel) }
}

func DefaultWorkspaceIgnoreRules() []IgnoreRule {
	return []IgnoreRule{
		IgnoreName(".git"), IgnoreName("node_modules"), IgnoreName("__pycache__"), IgnoreName(".DS_Store"),
		IgnoreName("dist"), IgnoreName("build"), IgnoreName("coverage"), IgnoreName(".next"), IgnoreName(".turbo"), IgnoreName(".cache"),
		IgnoreRegexp(regexp.MustCompile(`\.pyc$`)), IgnoreRegexp(regexp.MustCompile(`\.pyo$`)), IgnoreRegexp(regexp.MustCompile(`\.class$`)), IgnoreRegexp(regexp.MustCompile(`\.log$`)),
	}
}

func ParseIgnoreFile(text string) []IgnoreRule {
	var out []IgnoreRule
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		line = strings.Trim(strings.TrimLeft(strings.ReplaceAll(line, "\\", "/"), "/"), "/")
		if strings.Contains(line, "*") {
			out = append(out, IgnoreGlob(line))
		} else {
			out = append(out, IgnoreName(line))
		}
	}
	return out
}

func Ignored(rel string, rules []IgnoreRule) bool {
	for _, rule := range rules {
		if rule != nil && rule(rel) {
			return true
		}
	}
	return false
}

func ClassifyPathSensitivity(rel string) PathSensitivityInfo {
	clean, err := NormalizeRelativePath(rel)
	if err != nil {
		clean = rel
	}
	lower := strings.ToLower(clean)
	base := filepath.Base(lower)
	if base == ".env" || strings.HasPrefix(base, ".env.") || strings.Contains(lower, "id_rsa") || strings.Contains(lower, "id_ed25519") || strings.HasSuffix(lower, ".pem") || strings.HasSuffix(lower, ".key") || strings.HasSuffix(lower, ".p12") || strings.HasSuffix(lower, ".pfx") {
		return PathSensitivityInfo{Path: clean, Sensitivity: SensitivitySecret, Reason: "path commonly contains credentials or private keys"}
	}
	if strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "credential") || strings.Contains(lower, "password") || strings.HasSuffix(lower, ".crt") || strings.HasSuffix(lower, ".cert") {
		return PathSensitivityInfo{Path: clean, Sensitivity: SensitivitySensitive, Reason: "path name suggests sensitive material"}
	}
	return PathSensitivityInfo{Path: clean, Sensitivity: SensitivityNormal}
}

func sanitizeSegment(value string) string {
	lower := strings.ToLower(strings.TrimSpace(value))
	re := regexp.MustCompile(`[^a-z0-9._-]+`)
	out := strings.Trim(re.ReplaceAllString(lower, "-"), "-")
	if out == "" {
		return "agent-api"
	}
	return out
}

func getenvMap() map[string]string {
	out := map[string]string{}
	for _, item := range os.Environ() {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) == 2 {
			out[parts[0]] = parts[1]
		}
	}
	return out
}

func absOrDefault(value, fallback string) string {
	if value == "" {
		value = fallback
	}
	out, _ := filepath.Abs(value)
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
