from __future__ import annotations

import re

import pytest

from agent_api.local import (
    LocalError,
    LocalFileStore,
    LocalIgnoredPathError,
    LocalWorkdir,
    classify_local_path_sensitivity,
    create_local_context_package,
    create_local_runtime,
    local_app_dirs,
)


def test_local_app_dirs_follows_xdg_directories_on_linux() -> None:
    dirs = local_app_dirs(
        app_name="Agent Studio",
        platform_name="linux",
        env={
            "HOME": "/home/dev",
            "XDG_DATA_HOME": "/xdg/data",
            "XDG_CONFIG_HOME": "/xdg/config",
            "XDG_CACHE_HOME": "/xdg/cache",
            "XDG_STATE_HOME": "/xdg/state",
        },
    )

    assert dirs.data == "/xdg/data/agent-studio"
    assert dirs.config == "/xdg/config/agent-studio"
    assert dirs.cache == "/xdg/cache/agent-studio"
    assert dirs.logs == "/xdg/state/agent-studio/logs"


def test_local_file_store_rejects_traversal_and_absolute_paths(tmp_path) -> None:
    store = LocalFileStore(tmp_path)

    with pytest.raises(Exception, match="inside the store root|relative"):
        store.resolve_path("../outside.txt")
    with pytest.raises(Exception, match="relative"):
        store.resolve_path("/tmp/outside.txt")

    store.write_text("notes/hello.txt", "hello")
    assert store.read_text("notes/hello.txt") == "hello"


def test_local_runtime_reads_and_writes_config_json(tmp_path) -> None:
    runtime = create_local_runtime(app_name="agent-studio", base_dir=tmp_path)
    runtime.ensure()

    runtime.config.write("settings.json", {"theme": "dark"})
    assert runtime.config.read("settings.json") == {"theme": "dark"}

    runtime.config.set("settings.json", "apiBaseURL", "https://agent.test")
    assert runtime.config.get("settings.json", "apiBaseURL") == "https://agent.test"
    assert '"theme": "dark"' in (tmp_path / "config" / "settings.json").read_text()


def test_local_file_store_workbench_operations(tmp_path) -> None:
    store = LocalFileStore(tmp_path)
    store.write_text("notes/hello.md", "# Hello\nneedle one\n")
    store.write_bytes("assets/blob.bin", bytes([0, 1, 2, 3]))

    entries = store.list_entries(".", recursive=True)
    assert any(entry["path"] == "notes/hello.md" and not entry["is_dir"] for entry in entries["entries"])

    search = store.search_entries(query="hello")
    assert [entry["path"] for entry in search["entries"]] == ["notes/hello.md"]

    text = store.read_file("notes/hello.md")
    assert text["encoding"] == "text"
    assert "needle one" in text["content"]

    binary = store.read_file("assets/blob.bin")
    assert binary["encoding"] == "base64"
    assert binary["content_base64"] == "AAECAw=="


def test_local_file_store_reads_patches_greps_and_summarizes(tmp_path) -> None:
    store = LocalFileStore(tmp_path)
    store.write_text("src/a.py", "a\nneedle\nc\n")
    store.write_text("src/b.py", "again needle\n")
    store.write_bytes("src/blob.bin", bytes([0, 110, 101, 101, 100, 108, 101]))

    lines = store.read_lines("src/a.py", start_line=2, end_line=3)
    assert lines["lines"] == ["needle", "c"]

    patch = store.patch_lines("src/a.py", start_line=2, end_line=2, replacement="NEEDLE\nNEEDLE2")
    assert patch["total_lines"] == 4
    assert store.read_text("src/a.py") == "a\nNEEDLE\nNEEDLE2\nc\n"

    grep = store.grep(pattern="needle", path="src", limit=10)
    assert grep["files_scanned"] == 2
    assert [match["path"] for match in grep["matches"]] == ["src/b.py"]

    summary = store.summarize(max_previews=2)
    assert summary["file_count"] == 3
    assert any("src/a.py" in item for item in summary["top_paths_by_size"])
    assert any(preview["path"] == "src/a.py" for preview in summary["text_previews"])


def test_local_file_store_summarize_honors_max_depth(tmp_path) -> None:
    store = LocalFileStore(tmp_path)
    store.write_text("README.md", "# Project\n")
    store.write_text("src/a.py", "print('hello')\n")

    summary = store.summarize(max_depth=1)
    assert summary["file_count"] == 1
    assert any("README.md" in item for item in summary["top_paths_by_size"])
    assert not any("src/a.py" in item for item in summary["top_paths_by_size"])


def test_local_file_store_reports_non_fatal_child_scan_warnings(tmp_path) -> None:
    store = LocalFileStore(tmp_path)
    store.write_text("README.md", "# Project\n")
    blocked = tmp_path / "blocked"
    blocked.mkdir()
    blocked.chmod(0)
    try:
        summary = store.summarize()
        assert summary["file_count"] == 1
        if summary.get("scan_warnings"):
            assert summary["scan_warnings"][0]["path"] == "blocked"
    finally:
        blocked.chmod(0o700)


def test_local_file_store_skips_broken_symlinks_during_recursive_scans(tmp_path) -> None:
    store = LocalFileStore(tmp_path)
    store.write_text("README.md", "# Project\nneedle\n")
    (tmp_path / "SingletonCookie").symlink_to(tmp_path / "missing-target")

    files = store.list(".", recursive=True)
    assert [(item.path, item.type) for item in files] == [("README.md", "file"), ("SingletonCookie", "symlink")]

    summary = store.summarize()
    assert summary["file_count"] == 1

    grep = store.grep(pattern="needle")
    assert [match["path"] for match in grep["matches"]] == ["README.md"]


def test_local_workdir_ignores_gitignore_and_applies_edits(tmp_path) -> None:
    workdir = LocalWorkdir(tmp_path, name="Demo", trusted=True)
    workdir.write_text(".gitignore", "ignored-dir/\n*.tmp\n")
    workdir.files.write_text("ignored-dir/a.txt", "hidden\n")
    workdir.files.write_text("keep.tmp", "hidden\n")
    workdir.write_text("src/index.py", "a\nb\nc\n")
    workdir.load_ignore_files()

    entries = workdir.list_entries(".", recursive=True)
    assert [entry["path"] for entry in entries["entries"]] == [".gitignore", "src", "src/index.py"]
    with pytest.raises(LocalIgnoredPathError):
        workdir.resolve_path("ignored-dir/a.txt")

    preview = workdir.preview_patch_lines("src/index.py", start_line=2, end_line=2, replacement="B")
    assert preview["before"] == ["b"]
    assert preview["after"] == ["B"]

    before = workdir.snapshot()
    src_hash = next(file["sha256"] for file in before["files"] if file["path"] == "src/index.py")
    plan = workdir.preview_edits(
        [{"path": "src/index.py", "start_line": 2, "end_line": 2, "replacement": "B", "expected_sha256": src_hash}]
    )
    assert plan["previews"][0]["before"] == ["b"]

    result = workdir.apply_edits(plan["edits"])
    assert result["applied"][0]["path"] == "src/index.py"
    assert result["changed_files"] == ["src/index.py"]
    assert result["edit_count"] == 1
    assert "backups" not in result
    assert workdir.read_text("src/index.py") == "a\nB\nc\n"


def test_local_workdir_snapshots_diffs_and_rolls_back(tmp_path) -> None:
    workdir = LocalWorkdir(tmp_path)
    workdir.write_text("a.txt", "a\n")
    workdir.write_text("b.txt", "b\n")
    before = workdir.snapshot()

    workdir.write_text("a.txt", "changed\n")
    workdir.delete_path("b.txt")
    workdir.write_text("c.txt", "c\n")
    after = workdir.snapshot()
    diff = workdir.diff(before, after)

    assert [file["path"] for file in diff["added"]] == ["c.txt"]
    assert [file["path"] for file in diff["deleted"]] == ["b.txt"]
    assert [item["after"]["path"] for item in diff["modified"]] == ["a.txt"]

    workdir.write_text("a.txt", "a\n")
    workdir.write_text("b.txt", "b\n")
    with pytest.raises(ValueError, match="invalid line range"):
        workdir.apply_edits(
            [
                {"path": "a.txt", "start_line": 1, "end_line": 1, "replacement": "A"},
                {"path": "b.txt", "start_line": 99, "end_line": 99, "replacement": "B"},
            ]
        )
    assert workdir.read_text("a.txt") == "a\n"
    assert workdir.read_text("b.txt") == "b\n"

    stale = workdir.snapshot()["files"][0]["sha256"]
    workdir.write_text("a.txt", "changed\n")
    with pytest.raises(LocalError, match="local file changed"):
        workdir.apply_edits([{"path": "a.txt", "start_line": 1, "replacement": "A", "expected_sha256": stale}])


def test_local_path_sensitivity_classification() -> None:
    assert classify_local_path_sensitivity(".env")["sensitivity"] == "secret"
    assert classify_local_path_sensitivity("config/service-token.json")["sensitivity"] == "sensitive"
    assert classify_local_path_sensitivity("src/index.py")["sensitivity"] == "normal"


def test_local_context_packages_budget_workdir_files_for_agent_handoff(tmp_path) -> None:
    workdir = LocalWorkdir(tmp_path, name="Context Demo", ignore=[re.compile(r"ignored")])
    workdir.write_text("README.md", "# Demo\nneedle\n")
    workdir.write_text("src/index.py", "print('needle')\n")
    workdir.write_text(".env", "TOKEN=secret\n")
    workdir.files.write_text("ignored.txt", "needle\n")

    manifest = create_local_context_package(workdir, query="needle", include_search=True, max_files=10, max_bytes=1024)

    assert manifest["object"] == "local_context_manifest"
    assert manifest["workdir_name"] == "Context Demo"
    assert manifest["file_count"] == 3
    assert manifest["summary"]
    assert len(manifest["search"]["matches"]) >= 2

    env_file = next(file for file in manifest["files"] if file["path"] == ".env")
    assert env_file["sensitivity"] == "secret"
    assert env_file["omitted_reason"] == "secret_path"
    assert "content" not in env_file

    readme = next(file for file in manifest["files"] if file["path"] == "README.md")
    assert readme["encoding"] == "text"
    assert "needle" in readme["content"]
    assert re.match(r"^[a-f0-9]{64}$", readme["sha256"])


def test_local_context_packages_can_include_explicit_secret_content(tmp_path) -> None:
    workdir = LocalWorkdir(tmp_path)
    workdir.write_text(".env", "TOKEN=secret\n")

    manifest = create_local_context_package(workdir, include_secrets=True, include_summary=False)
    env_file = next(file for file in manifest["files"] if file["path"] == ".env")

    assert env_file["sensitivity"] == "secret"
    assert "omitted_reason" not in env_file
    assert "TOKEN=secret" in env_file["content"]
