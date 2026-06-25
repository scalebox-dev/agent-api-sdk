package agentapi

import "strings"

var supportedVolumeImageExtensions = map[string]struct{}{
	".avif": {},
	".bmp":  {},
	".gif":  {},
	".jpeg": {},
	".jpg":  {},
	".png":  {},
	".svg":  {},
	".webp": {},
}

func NormalizeVolumeAssetPath(src string) string {
	value := strings.TrimSpace(src)
	if value == "" || isExternalAssetTarget(value) {
		return ""
	}
	path := strings.SplitN(value, "#", 2)[0]
	path = strings.SplitN(path, "?", 2)[0]
	path = strings.TrimSpace(path)
	lower := strings.ToLower(path)
	if strings.HasPrefix(lower, "/agent-volume/") {
		path = path[len("/agent-volume/"):]
	}
	path = strings.TrimLeft(path, "/")
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}
	if path == "" || path == "." || strings.Contains(path, "..") {
		return ""
	}
	return path
}

func IsSupportedVolumeImagePath(src string) bool {
	path := strings.ToLower(NormalizeVolumeAssetPath(src))
	if path == "" {
		return false
	}
	dot := strings.LastIndex(path, ".")
	if dot < 0 {
		return false
	}
	_, ok := supportedVolumeImageExtensions[path[dot:]]
	return ok
}

func IsSupportedVolumeImageContentType(contentType string) bool {
	mime := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	return strings.HasPrefix(mime, "image/")
}

func isExternalAssetTarget(src string) bool {
	lower := strings.ToLower(src)
	if strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "blob:") || strings.HasPrefix(lower, "//") {
		return true
	}
	colon := strings.Index(lower, ":")
	slash := strings.Index(lower, "/")
	return colon > 0 && (slash < 0 || colon < slash)
}
