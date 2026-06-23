from __future__ import annotations

from collections.abc import Callable, Mapping, Sequence
from dataclasses import dataclass
from typing import Any, Literal, cast

from agent_api.types.tools import FunctionTool, Tool

from .context import create_local_context_package
from .workdir import LocalWorkdir

LocalWorkdirAccessMode = Literal["approval", "full"]
LocalWorkdirAction = Literal[
    "summarize",
    "list",
    "search",
    "grep",
    "read",
    "read_lines",
    "context",
    "snapshot",
    "classify_path",
    "preview_edits",
    "apply_edits",
    "write",
    "mkdir",
    "delete",
]

LOCAL_WORKDIR_ACTIONS: tuple[LocalWorkdirAction, ...] = (
    "summarize",
    "list",
    "search",
    "grep",
    "read",
    "read_lines",
    "context",
    "snapshot",
    "classify_path",
    "preview_edits",
    "apply_edits",
    "write",
    "mkdir",
    "delete",
)

MUTATING_LOCAL_WORKDIR_ACTIONS: frozenset[LocalWorkdirAction] = frozenset(
    {"apply_edits", "write", "mkdir", "delete"}
)

LOCAL_WORKDIR_TOOL_DESCRIPTION = " ".join(
    [
        "Inspect and modify the selected local workdir through one model-facing primitive.",
        "Use action=list/search/grep/summarize/context to discover files, read/read_lines for file content, preview_edits before edits, and apply_edits/write/mkdir/delete only when mutation is intended.",
        "grep searches file contents for a literal substring; path may be omitted, a file path, or a directory subtree.",
        "In approval mode, mutating actions return requires_approval with a safe preview instead of changing files. In full mode, mutating actions execute immediately.",
        "Paths are relative to the selected local workdir; never use absolute paths.",
    ]
)


@dataclass(frozen=True)
class LocalWorkdirToolRegistry:
    workdir: LocalWorkdir
    driver: "LocalWorkdirDriver"
    tool_name: str = "local_workdir"

    @property
    def access_mode(self) -> LocalWorkdirAccessMode:
        return self.driver.access_mode

    def definitions(self) -> list[Tool]:
        return [cast(Tool, dict(local_workdir_tool_definition(self.tool_name)))]

    def handlers(self) -> dict[str, Callable[[Mapping[str, Any]], dict[str, Any]]]:
        return {self.tool_name: self.driver.dispatch}

    def execute(self, name: str, args: Mapping[str, Any]) -> dict[str, Any]:
        if name != self.tool_name:
            raise ValueError(f"unknown local workdir tool: {name}")
        return self.driver.dispatch(args)

    def requires_approval(self, name: str, args: Mapping[str, Any] | None = None) -> bool:
        return name == self.tool_name and self.driver.requires_approval(args or {})


class LocalWorkdirDriver:
    def __init__(self, workdir: LocalWorkdir, *, access_mode: LocalWorkdirAccessMode = "approval") -> None:
        self.workdir = workdir
        self.access_mode = access_mode

    def dispatch(self, args: Mapping[str, Any]) -> dict[str, Any]:
        action = _safe_workdir_action(args)
        try:
            return self._dispatch(args)
        except Exception as exc:
            return _local_tool_error_result(action, exc)

    def _dispatch(self, args: Mapping[str, Any]) -> dict[str, Any]:
        action = _workdir_action(args)
        if action == "summarize":
            return _local_tool_result(action, self.workdir.summarize(**_summary_args(args)))
        if action == "list":
            return _local_tool_result(action, self.workdir.list_entries(_optional_string_arg(args, "path") or ".", **_list_args(args)))
        if action == "search":
            return _local_tool_result(action, self.workdir.search_entries(**_search_entries_args(args)))
        if action == "grep":
            return _local_tool_result(action, self.workdir.grep(**_grep_args(args)))
        if action == "read":
            return _local_tool_result(action, self.workdir.read_file(_string_arg(args, "path"), **_read_file_args(args)))
        if action == "read_lines":
            return _local_tool_result(action, self.workdir.read_lines(_string_arg(args, "path"), **_read_lines_args(args)))
        if action == "context":
            return _local_tool_result(action, create_local_context_package(self.workdir, **_context_package_args(args)))
        if action == "snapshot":
            return _local_tool_result(action, self.workdir.snapshot(**_snapshot_args(args)))
        if action == "classify_path":
            return _local_tool_result(action, self.workdir.classify_path(_string_arg(args, "path")))
        if action == "preview_edits":
            return _local_tool_result(action, self.workdir.preview_edits(_edits_arg(args)))
        if action == "apply_edits":
            return self._dispatch_apply_edits(args)
        if action == "write":
            return self._dispatch_write(args)
        if action == "mkdir":
            return self._dispatch_mkdir(args)
        if action == "delete":
            return self._dispatch_delete(args)
        raise ValueError(f"unsupported local_workdir action: {action}")

    def requires_approval(self, args: Mapping[str, Any]) -> bool:
        if self.access_mode == "full":
            return False
        return _safe_workdir_action(args) in MUTATING_LOCAL_WORKDIR_ACTIONS

    def _dispatch_apply_edits(self, args: Mapping[str, Any]) -> dict[str, Any]:
        edits = _edits_arg(args)
        if self.access_mode != "full":
            return _approval_required("apply_edits", args, self.workdir.preview_edits(edits))
        return _local_tool_result("apply_edits", self.workdir.apply_edits(edits))

    def _dispatch_write(self, args: Mapping[str, Any]) -> dict[str, Any]:
        if self.access_mode != "full":
            return _approval_required("write", args)
        return _local_tool_result("write", self.workdir.write_file(_string_arg(args, "path"), _string_arg(args, "content")))

    def _dispatch_mkdir(self, args: Mapping[str, Any]) -> dict[str, Any]:
        if self.access_mode != "full":
            return _approval_required("mkdir", args)
        return _local_tool_result("mkdir", self.workdir.create_directory(_string_arg(args, "path")))

    def _dispatch_delete(self, args: Mapping[str, Any]) -> dict[str, Any]:
        if self.access_mode != "full":
            return _approval_required("delete", args)
        return _local_tool_result("delete", self.workdir.delete_path(_string_arg(args, "path")))


def create_local_workdir_tool_registry(
    workdir: LocalWorkdir,
    *,
    access_mode: LocalWorkdirAccessMode = "approval",
    tool_name: str = "local_workdir",
) -> LocalWorkdirToolRegistry:
    return LocalWorkdirToolRegistry(
        workdir=workdir,
        driver=LocalWorkdirDriver(workdir, access_mode=access_mode),
        tool_name=tool_name,
    )


def local_workdir_tool_definition(name: str = "local_workdir") -> FunctionTool:
    return {
        "type": "function",
        "name": name,
        "description": LOCAL_WORKDIR_TOOL_DESCRIPTION,
        "parameters": _local_workdir_tool_parameters(),
        "strict": False,
    }


def local_workdir_tool_instructions() -> str:
    return LOCAL_WORKDIR_TOOL_DESCRIPTION


def _local_workdir_tool_parameters() -> dict[str, Any]:
    return _object_schema(
        {
            "action": {
                "type": "string",
                "enum": list(LOCAL_WORKDIR_ACTIONS),
                "description": "Workdir operation. Prefer summarize/list/search/grep before reading or editing. Prefer read_lines and apply_edits for source changes.",
            },
            "path": _string_schema("Relative path. File path for read/write/delete/edit actions. For grep, path may be a file or a directory subtree. For list/search/summarize/context/snapshot, path is a directory base."),
            "query": _string_schema("Path/name query for search, or optional context query."),
            "pattern": _string_schema("Literal text pattern for grep."),
            "content": _string_schema("Text content for write."),
            "start_line": _integer_schema("1-based start line for read_lines and edit entries."),
            "end_line": _integer_schema("1-based inclusive end line; omit or 0 for EOF when supported."),
            "replacement": _string_schema("Replacement text for simple single edit flows."),
            "edits": {
                "type": "array",
                "minItems": 1,
                "description": "Line edits for preview_edits/apply_edits.",
                "items": _object_schema(
                    {
                        "path": _string_schema("Relative file path."),
                        "start_line": _integer_schema("1-based start line."),
                        "end_line": _integer_schema("1-based inclusive end line."),
                        "replacement": _string_schema("Replacement text. Empty string deletes the line range."),
                        "expected_sha256": _string_schema("Optional expected SHA-256 for conflict detection."),
                    },
                    ["path", "start_line"],
                ),
            },
            "options": _object_schema(
                {
                    "recursive": _boolean_schema("List recursively."),
                    "include_directories": _boolean_schema("Include directories in list results."),
                    "max_depth": _integer_schema("Maximum recursive list depth."),
                    "limit": _integer_schema("Maximum entries or matches."),
                    "max_files": _integer_schema("Maximum files to scan or package."),
                    "max_bytes": _integer_schema("Maximum total bytes to read/package."),
                    "max_bytes_per_file": _integer_schema("Maximum bytes per file."),
                    "max_previews": _integer_schema("Maximum summary previews."),
                    "include_content": _boolean_schema("Include file contents in context packages."),
                    "include_summary": _boolean_schema("Include workdir summary in context packages."),
                    "include_search": _boolean_schema("Include grep results in context packages when query is set."),
                    "include_secrets": _boolean_schema("Include likely secret file contents in context packages."),
                    "hash": _boolean_schema("Include SHA-256 hashes in snapshots."),
                }
            ),
        },
        ["action"],
    )


def _workdir_action(args: Mapping[str, Any]) -> LocalWorkdirAction:
    value = _string_arg(args, "action").strip().lower()
    if value not in LOCAL_WORKDIR_ACTIONS:
        raise ValueError(f"unsupported local_workdir action: {value}")
    return cast(LocalWorkdirAction, value)


def _safe_workdir_action(args: Mapping[str, Any]) -> LocalWorkdirAction | Literal["unknown"]:
    value = args.get("action")
    if isinstance(value, str) and value.strip().lower() in LOCAL_WORKDIR_ACTIONS:
        return cast(LocalWorkdirAction, value.strip().lower())
    return "unknown"


def _summary_args(args: Mapping[str, Any]) -> dict[str, Any]:
    return _strip_none(
        {
            "path": _optional_string_arg(args, "path"),
            "max_files": _optional_number_arg(args, "maxFiles", "max_files"),
            "max_previews": _optional_number_arg(args, "maxPreviews", "max_previews"),
        }
    )


def _grep_args(args: Mapping[str, Any]) -> dict[str, Any]:
    return _strip_none(
        {
            "pattern": _string_arg(args, "pattern"),
            "path": _optional_string_arg(args, "path"),
            "limit": _optional_number_arg(args, "limit"),
            "max_files": _optional_number_arg(args, "maxFiles", "max_files"),
        }
    )


def _list_args(args: Mapping[str, Any]) -> dict[str, Any]:
    return _strip_none(
        {
            "recursive": _optional_boolean_arg(args, "recursive"),
            "include_directories": _optional_boolean_arg(args, "includeDirectories", "include_directories"),
            "max_depth": _optional_number_arg(args, "maxDepth", "max_depth"),
        }
    )


def _search_entries_args(args: Mapping[str, Any]) -> dict[str, Any]:
    return _strip_none(
        {
            "query": _string_arg(args, "query"),
            "path": _optional_string_arg(args, "path"),
            "limit": _optional_number_arg(args, "limit"),
        }
    )


def _read_file_args(args: Mapping[str, Any]) -> dict[str, Any]:
    return _strip_none({"max_bytes": _optional_number_arg(args, "maxBytes", "max_bytes")})


def _read_lines_args(args: Mapping[str, Any]) -> dict[str, Any]:
    return _strip_none(
        {
            "start_line": _number_arg(args, "startLine", "start_line"),
            "end_line": _optional_number_arg(args, "endLine", "end_line"),
            "max_bytes": _optional_number_arg(args, "maxBytes", "max_bytes"),
        }
    )


def _snapshot_args(args: Mapping[str, Any]) -> dict[str, Any]:
    out = _strip_none(
        {
            "path": _optional_string_arg(args, "path"),
            "max_bytes_per_file": _optional_number_arg(args, "maxBytesPerFile", "max_bytes_per_file"),
        }
    )
    hash_files = _optional_boolean_arg(args, "hash")
    if hash_files is not None:
        out["hash"] = hash_files
    return out


def _context_package_args(args: Mapping[str, Any]) -> dict[str, Any]:
    return _strip_none(
        {
            "path": _optional_string_arg(args, "path"),
            "query": _optional_string_arg(args, "query"),
            "max_files": _optional_number_arg(args, "maxFiles", "max_files"),
            "max_bytes": _optional_number_arg(args, "maxBytes", "max_bytes"),
            "max_bytes_per_file": _optional_number_arg(args, "maxBytesPerFile", "max_bytes_per_file"),
            "include_content": _optional_boolean_arg(args, "includeContent", "include_content"),
            "include_summary": _optional_boolean_arg(args, "includeSummary", "include_summary"),
            "include_search": _optional_boolean_arg(args, "includeSearch", "include_search"),
            "include_secrets": _optional_boolean_arg(args, "includeSecrets", "include_secrets"),
        }
    )


def _edits_arg(args: Mapping[str, Any]) -> list[dict[str, Any]]:
    edits = args.get("edits")
    if isinstance(edits, Sequence) and not isinstance(edits, (str, bytes, bytearray)) and len(edits) > 0:
        return [_edit_arg(cast(Mapping[str, Any], edit)) for edit in edits]
    if isinstance(args.get("path"), str) and _has_number(args, "startLine", "start_line"):
        return [_edit_arg(args)]
    raise ValueError("edits must be a non-empty array")


def _edit_arg(record: Mapping[str, Any]) -> dict[str, Any]:
    return _strip_none(
        {
            "path": _string_arg(record, "path"),
            "start_line": _number_arg(record, "startLine", "start_line"),
            "end_line": _optional_number_arg(record, "endLine", "end_line"),
            "replacement": record["replacement"] if isinstance(record.get("replacement"), str) else "",
            "expected_sha256": _optional_string_arg(record, "expectedSha256", "expected_sha256"),
        }
    )


def _local_tool_result(action: LocalWorkdirAction, value: Any) -> dict[str, Any]:
    if isinstance(value, Mapping):
        return {"ok": True, "action": action, **dict(value)}
    return {"ok": True, "action": action, "result": value}


def _local_tool_error_result(action: LocalWorkdirAction | Literal["unknown"], exc: Exception) -> dict[str, Any]:
    return {
        "ok": False,
        "action": action,
        "error": str(exc) or "local_workdir action failed",
        "code": getattr(exc, "code", None) or getattr(exc, "errno", None),
        "path": getattr(exc, "path", None) or getattr(exc, "filename", None),
    }


def _approval_required(action: LocalWorkdirAction, args: Mapping[str, Any], preview: Any | None = None) -> dict[str, Any]:
    return {
        "ok": False,
        "action": action,
        "requires_approval": True,
        "arguments": dict(args),
        "preview": preview,
        "message": f"local_workdir action {action} requires approval",
    }


def _string_arg(args: Mapping[str, Any], key: str, alternate_key: str | None = None) -> str:
    value = _arg_value(args, key, alternate_key)
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{key} must be a non-empty string")
    return value


def _optional_string_arg(args: Mapping[str, Any], key: str, alternate_key: str | None = None) -> str | None:
    value = _arg_value(args, key, alternate_key)
    if value is None or value == "":
        return None
    if not isinstance(value, str):
        raise ValueError(f"{key} must be a string")
    return value


def _number_arg(args: Mapping[str, Any], key: str, alternate_key: str | None = None) -> int:
    value = _arg_value(args, key, alternate_key)
    if not isinstance(value, (int, float)) or isinstance(value, bool):
        raise ValueError(f"{key} must be a number")
    return int(value)


def _optional_number_arg(args: Mapping[str, Any], key: str, alternate_key: str | None = None) -> int | None:
    value = _arg_value(args, key, alternate_key)
    if value is None:
        return None
    if not isinstance(value, (int, float)) or isinstance(value, bool):
        raise ValueError(f"{key} must be a number")
    return int(value)


def _optional_boolean_arg(args: Mapping[str, Any], key: str, alternate_key: str | None = None) -> bool | None:
    value = _arg_value(args, key, alternate_key)
    if value is None:
        return None
    if not isinstance(value, bool):
        raise ValueError(f"{key} must be a boolean")
    return value


def _arg_value(args: Mapping[str, Any], key: str, alternate_key: str | None = None) -> Any:
    if key in args:
        return args[key]
    if alternate_key is not None and alternate_key in args:
        return args[alternate_key]
    options = args.get("options")
    if isinstance(options, Mapping):
        if key in options:
            return options[key]
        if alternate_key is not None and alternate_key in options:
            return options[alternate_key]
    return None


def _has_number(args: Mapping[str, Any], key: str, alternate_key: str | None = None) -> bool:
    value = _arg_value(args, key, alternate_key)
    return isinstance(value, (int, float)) and not isinstance(value, bool)


def _strip_none(value: Mapping[str, Any]) -> dict[str, Any]:
    return {key: item for key, item in value.items() if item is not None}


def _object_schema(properties: dict[str, Any], required: list[str] | None = None) -> dict[str, Any]:
    return {
        "type": "object",
        "properties": properties,
        "required": required or [],
        "additionalProperties": False,
    }


def _string_schema(description: str) -> dict[str, str]:
    return {"type": "string", "description": description}


def _integer_schema(description: str) -> dict[str, str]:
    return {"type": "integer", "description": description}


def _boolean_schema(description: str) -> dict[str, str]:
    return {"type": "boolean", "description": description}
