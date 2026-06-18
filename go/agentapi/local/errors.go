package local

import "fmt"

type Error struct {
	Code string
	Path string
	Msg  string
}

func (e *Error) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s: %s: %s", e.Code, e.Path, e.Msg)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Msg)
}

func pathError(msg, path string) *Error {
	return &Error{Code: "local_path_error", Path: path, Msg: msg}
}

func ignoredPathError(path string) *Error {
	return &Error{Code: "local_ignored_path", Path: path, Msg: "local workdir path is ignored"}
}

func fileTooLargeError(path string) *Error {
	return &Error{Code: "local_file_too_large", Path: path, Msg: "local file is too large"}
}

func notTextFileError(path string) *Error {
	return &Error{Code: "local_not_text_file", Path: path, Msg: "local file must be text"}
}

func editConflictError(path string) *Error {
	return &Error{Code: "local_edit_conflict", Path: path, Msg: "local file changed before edit"}
}
