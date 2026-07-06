from __future__ import annotations

import base64
import hashlib
import time
from typing import Any

from .paths import classify_local_path_sensitivity, matches_path_filters, positive_int
from .workdir import LocalWorkdir


def create_local_context_package(workdir: LocalWorkdir, **params: Any) -> dict[str, Any]:
    base_path = params.get("path", ".")
    max_files = positive_int(params.get("max_files"), 80)
    max_bytes = positive_int(params.get("max_bytes"), 256 * 1024)
    max_bytes_per_file = positive_int(params.get("max_bytes_per_file"), 32 * 1024)
    max_depth = positive_int(params.get("max_depth"), 3)
    preview_bytes = positive_int(params.get("preview_bytes"), max_bytes_per_file)
    include_content = params.get("include_content", True)
    include_hashes = params.get("include_hashes", True)
    include_summary = params.get("include_summary", True)
    include_search = bool(params.get("include_search") and str(params.get("query", "")).strip())
    include_secrets = params.get("include_secrets", False)

    scan_stats, scan_warnings = workdir.list_with_warnings(base_path, recursive=True, max_depth=max_depth)
    stats = [
        item
        for item in scan_stats
        if item.type == "file" and matches_path_filters(item.path, params.get("include"), params.get("exclude"))
    ]
    stats.sort(key=lambda item: item.path)
    files: list[dict[str, Any]] = []
    included_bytes = 0
    truncated = len(stats) > max_files

    for item in stats[:max_files]:
        sensitivity = classify_local_path_sensitivity(item.path)
        packaged: dict[str, Any] = {
            "path": item.path,
            "size": item.size,
            "sensitivity": sensitivity["sensitivity"],
            "sensitivity_reason": sensitivity.get("reason"),
        }
        if not include_secrets and sensitivity["sensitivity"] == "secret":
            packaged["omitted_reason"] = "secret_path"
            files.append(packaged)
            continue
        if item.size > max_bytes_per_file:
            packaged["omitted_reason"] = "file_too_large"
            files.append(packaged)
            truncated = True
            continue
        if included_bytes >= max_bytes:
            packaged["omitted_reason"] = "package_budget_exceeded"
            files.append(packaged)
            truncated = True
            continue

        read_budget = min(preview_bytes, max_bytes_per_file, max_bytes - included_bytes)
        if read_budget <= 0:
            packaged["omitted_reason"] = "package_budget_exceeded"
            files.append(packaged)
            truncated = True
            continue

        delivered = workdir.read_file(item.path, max_bytes=read_budget)
        if delivered.get("encoding") == "text":
            included_bytes += len(delivered.get("content", "").encode("utf-8"))
        else:
            included_bytes += len(base64.b64decode(delivered.get("content_base64", "")))

        packaged.update(
            {
                "mime_type": delivered.get("mime_type"),
                "encoding": delivered.get("encoding"),
                "truncated": delivered.get("truncated") or None,
            }
        )
        if include_content:
            if "content" in delivered:
                packaged["content"] = delivered["content"]
            if "content_base64" in delivered:
                packaged["content_base64"] = delivered["content_base64"]
        if include_hashes:
            packaged["sha256"] = hashlib.sha256(workdir.files.resolve_path(item.path).read_bytes()).hexdigest()
        if delivered.get("truncated"):
            truncated = True
        files.append(packaged)

    manifest = {
        "object": "local_context_manifest",
        "root": str(workdir.root),
        "workdir_name": workdir.name,
        "generated_at_unix": int(time.time()),
        "base_path": str(base_path),
        "file_count": len(stats),
        "total_bytes": sum(item.size for item in stats),
        "included_bytes": included_bytes,
        "truncated": truncated,
        "files": files,
        "summary": workdir.summarize(path=base_path, max_files=max_files, max_depth=max_depth, preview_bytes=preview_bytes, ignore=params.get("exclude")) if include_summary else None,
        "search": workdir.grep(pattern=params["query"], path=base_path, limit=max_files, max_bytes_per_file=max_bytes_per_file, ignore=params.get("exclude")) if include_search else None,
    }
    if scan_warnings:
        manifest["scan_warnings"] = scan_warnings
    return manifest
