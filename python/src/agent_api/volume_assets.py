from __future__ import annotations

import re

_SUPPORTED_VOLUME_IMAGE_EXTENSIONS = {
    ".avif",
    ".bmp",
    ".gif",
    ".jpeg",
    ".jpg",
    ".png",
    ".svg",
    ".webp",
}


def normalize_volume_asset_path(src: str | None) -> str:
    value = (src or "").strip()
    if not value or _is_external_asset_target(value):
        return ""
    path = value.split("#", 1)[0].split("?", 1)[0].strip()
    path = re.sub(r"^/agent-volume/+", "", path, flags=re.IGNORECASE)
    path = re.sub(r"^/+", "", path)
    path = re.sub(r"/+", "/", path)
    if not path or path == "." or ".." in path:
        return ""
    return path


def is_supported_volume_image_path(src: str | None) -> bool:
    path = normalize_volume_asset_path(src).lower()
    return any(path.endswith(ext) for ext in _SUPPORTED_VOLUME_IMAGE_EXTENSIONS)


def is_supported_volume_image_content_type(content_type: str | None) -> bool:
    return (content_type or "").split(";", 1)[0].strip().lower().startswith("image/")


def _is_external_asset_target(src: str) -> bool:
    return bool(re.match(r"^(?:[a-z][a-z0-9+.-]*:|//)", src, flags=re.IGNORECASE))
