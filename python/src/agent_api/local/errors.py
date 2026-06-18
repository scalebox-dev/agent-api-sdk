from __future__ import annotations


class LocalError(Exception):
    code: str
    path: str | None

    def __init__(self, code: str, message: str, *, path: str | None = None) -> None:
        super().__init__(message)
        self.code = code
        self.path = path


class LocalPathError(LocalError):
    def __init__(self, message: str, path: str | None = None) -> None:
        super().__init__("local_path_error", message, path=path)


class LocalIgnoredPathError(LocalError):
    def __init__(self, path: str) -> None:
        super().__init__("local_ignored_path", f"local workdir path is ignored: {path}", path=path)


class LocalFileTooLargeError(LocalError):
    def __init__(self, path: str) -> None:
        super().__init__("local_file_too_large", f"local file is too large: {path}", path=path)


class LocalNotTextFileError(LocalError):
    def __init__(self, path: str) -> None:
        super().__init__("local_not_text_file", f"local file must be text: {path}", path=path)


class LocalConfigError(LocalError):
    def __init__(self, message: str, path: str | None = None) -> None:
        super().__init__("local_config_error", message, path=path)
