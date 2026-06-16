from __future__ import annotations

import hashlib
import json
import re
from pathlib import Path
from typing import Any

from agent_api.local_functions import function_call_output_input, pending_function_calls
from agent_api.types import AgentResponse, FunctionCallOutputInput
from agent_api.types.skills import LocalSkillDescriptor

DEFAULT_FOCUS_MANIFEST_CHARS = 16000
DEFAULT_FOCUS_FILE_CHARS = 12000


def local_skill_from_directory(
    root_dir: str | Path,
    *,
    id: str | None = None,
    name: str | None = None,
    description: str | None = None,
    max_manifest_chars: int = DEFAULT_FOCUS_MANIFEST_CHARS,
    metadata: dict[str, Any] | None = None,
) -> LocalSkillDescriptor:
    root = Path(root_dir).expanduser().resolve()
    digest = _directory_digest(root)
    local_id = id or root.name
    manifest, manifest_truncated = _read_local_manifest(root, max_manifest_chars)
    return {
        "local_skill_id": local_id,
        "skill_ref": _skill_ref_for_local(local_id, digest),
        "name": name or root.name,
        "description": description or "",
        "root_hint": str(root),
        "digest": digest,
        "manifest": manifest,
        "manifest_truncated": manifest_truncated,
        "metadata": metadata or {},
    }


def pending_local_skill_calls(response: AgentResponse) -> list[dict[str, Any]]:
    return [call for call in pending_function_calls(response) if call.get("name") == "skill_focus"]


async def run_local_skill_handlers(
    response: AgentResponse,
    local_skills: list[LocalSkillDescriptor],
) -> list[FunctionCallOutputInput]:
    outputs: list[FunctionCallOutputInput] = []
    by_ref = {_descriptor_skill_ref(skill): skill for skill in local_skills}
    for call in pending_local_skill_calls(response):
        raw_args = call.get("arguments") or "{}"
        args = json.loads(raw_args) if isinstance(raw_args, str) else dict(raw_args)
        payload = _focus_local_skills(args, by_ref)
        outputs.append(function_call_output_input(str(call.get("call_id", "")), payload))
    return outputs


def _focus_local_skills(args: dict[str, Any], by_ref: dict[str, LocalSkillDescriptor]) -> dict[str, Any]:
    max_manifest_chars = _manifest_char_limit(args)
    max_file_chars = _file_char_limit(args)
    items = args.get("skills") or []
    data: list[dict[str, Any]] = []
    if not isinstance(items, list):
        return {"data": [{"ok": False, "error": {"code": "invalid_skill_focus", "message": "skills must be an array"}}]}
    for item in items:
        if not isinstance(item, dict):
            data.append({"ok": False, "error": {"code": "invalid_skill_focus", "message": "skill item must be an object"}})
            continue
        skill_ref = str(item.get("skill_ref") or "").strip()
        result: dict[str, Any] = {"ok": False, "skill_ref": skill_ref, "branch": "main"}
        descriptor = by_ref.get(skill_ref)
        if descriptor is None:
            result["error"] = {"code": "skill_ref_not_found", "message": "skill_ref was not registered with the SDK"}
            data.append(result)
            continue
        try:
            result["skill"] = _focused_local_skill(descriptor, max_manifest_chars, _paths_arg(item), max_file_chars, _include_manifest(item))
            result["ok"] = True
        except Exception as exc:
            result["error"] = {"code": "local_skill_focus_failed", "message": str(exc)}
        data.append(result)
    return {"data": data}


def _focused_local_skill(descriptor: LocalSkillDescriptor, max_manifest_chars: int, paths: list[str], max_file_chars: int, include_manifest: bool) -> dict[str, Any]:
    root = Path(str(descriptor.get("root_hint", ""))).expanduser().resolve()
    if not root.is_dir():
        raise FileNotFoundError(f"local skill root does not exist: {root}")
    manifest_path = root / "SKILL.md"
    manifest = ""
    manifest_truncated = False
    if include_manifest and manifest_path.is_file():
        manifest = manifest_path.read_text(encoding="utf-8")
        if max_manifest_chars > 0 and len(manifest) > max_manifest_chars:
            manifest = manifest[:max_manifest_chars]
            manifest_truncated = True
    entries = []
    for path in sorted(root.iterdir(), key=lambda p: p.name):
        if path.name in {".git", "__pycache__", "node_modules"}:
            continue
        stat = path.stat()
        entries.append(
            {
                "path": path.name,
                "is_dir": path.is_dir(),
                "size": 0 if path.is_dir() else stat.st_size,
                "modified_at": int(stat.st_mtime),
            }
        )
    root_hint = str(root)
    return {
        "skill_ref": _descriptor_skill_ref(descriptor),
        "name": descriptor.get("name", ""),
        "description": descriptor.get("description", ""),
        "branch": "SKILL_BRANCH_MAIN",
        "digest": descriptor.get("digest", ""),
        "manifest": manifest,
        "manifest_truncated": manifest_truncated,
        "entries": entries,
        "files": [_focused_local_file(root, path, max_file_chars) for path in paths],
    }


def _manifest_char_limit(args: dict[str, Any]) -> int:
    raw = args.get("max_manifest_chars", DEFAULT_FOCUS_MANIFEST_CHARS)
    try:
        return int(raw)
    except (TypeError, ValueError):
        return DEFAULT_FOCUS_MANIFEST_CHARS


def _file_char_limit(args: dict[str, Any]) -> int:
    raw = args.get("max_file_chars", DEFAULT_FOCUS_FILE_CHARS)
    try:
        return int(raw)
    except (TypeError, ValueError):
        return DEFAULT_FOCUS_FILE_CHARS


def _paths_arg(item: dict[str, Any]) -> list[str]:
    raw = item.get("paths") or []
    if not isinstance(raw, list):
        return []
    return [str(path).strip() for path in raw if str(path).strip()]


def _include_manifest(item: dict[str, Any]) -> bool:
    raw = item.get("include_manifest")
    return raw if isinstance(raw, bool) else True


def _focused_local_file(root: Path, rel_path: str, max_file_chars: int) -> dict[str, Any]:
    result: dict[str, Any] = {"path": rel_path, "branch": "SKILL_BRANCH_MAIN", "content": "", "truncated": False, "size": 0}
    try:
        path = (root / rel_path).resolve()
        path.relative_to(root)
        if not path.is_file():
            result["error"] = {"type": "skill_error", "code": "skill_file_not_found", "message": "skill file was not found"}
            return result
        stat = path.stat()
        content = path.read_text(encoding="utf-8")
        truncated = False
        if max_file_chars > 0 and len(content) > max_file_chars:
            content = content[:max_file_chars]
            truncated = True
        result.update({"content": content, "truncated": truncated, "size": stat.st_size})
        return result
    except ValueError:
        result["error"] = {"type": "skill_error", "code": "invalid_skill_file_path", "message": "path must stay inside the local skill root"}
        return result
    except UnicodeDecodeError:
        result["error"] = {"type": "skill_error", "code": "invalid_skill_file_utf8", "message": "skill file must be valid UTF-8"}
        return result
    except Exception as exc:
        result["error"] = {"type": "skill_error", "code": "skill_file_read_failed", "message": str(exc)}
        return result


def _directory_digest(root: Path) -> str:
    h = hashlib.sha256()
    for path in sorted(p for p in root.rglob("*") if p.is_file() and ".git" not in p.parts and "__pycache__" not in p.parts):
        rel = path.relative_to(root).as_posix()
        h.update(rel.encode("utf-8"))
        h.update(b"\0")
        h.update(path.read_bytes())
        h.update(b"\0")
    return "sha256:" + h.hexdigest()


def _descriptor_skill_ref(descriptor: LocalSkillDescriptor) -> str:
    ref = str(descriptor.get("skill_ref") or "").strip()
    if ref:
        return ref
    return _skill_ref_for_local(str(descriptor.get("local_skill_id") or ""), str(descriptor.get("digest") or ""))


def _skill_ref_for_local(local_id: str, digest: str) -> str:
    slug = _skill_ref_slug(local_id) or "local-skill"
    digest_part = _skill_ref_digest_part(digest) or "unknown"
    return f"local::{slug}@{digest_part}::main"


def _skill_ref_slug(raw: str) -> str:
    slug = raw.strip().lower().replace("::", "-").replace("@", "-")
    slug = re.sub(r"[^a-z0-9_-]+", "-", slug).strip("-_")
    return slug[:64].strip("-_")


def _skill_ref_digest_part(raw: str) -> str:
    digest = raw.strip().lower().split(":")[-1]
    return re.sub(r"[^a-z0-9_-]+", "", digest)[:16]


def _read_local_manifest(root: Path, max_manifest_chars: int) -> tuple[str, bool]:
    manifest_path = root / "SKILL.md"
    if not manifest_path.is_file():
        return "", False
    manifest = manifest_path.read_text(encoding="utf-8")
    if max_manifest_chars > 0 and len(manifest) > max_manifest_chars:
        return manifest[:max_manifest_chars], True
    return manifest, False
