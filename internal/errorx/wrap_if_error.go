package errorx

import (
	"fmt"
	"path/filepath"
	"runtime"
)

// WrapIfError wraps the provided error with an additional message if it is not
// nil. This is intended to be used inside a defer to wrap the returned error
func WrapIfError(msg string, err *error) {
	if *err != nil {
		*err = fmt.Errorf("%s: %w", msg, *err)
	}
}

// WrapIfError wraps the provided error with the name of the caller if the error
// is not nil. This is intended to be used inside a defer to wrap the returned
// error
func WrapWithFuncNameIfError(err *error) {
	if *err != nil {
		pc, _, _, ok := runtime.Caller(1)
		details := runtime.FuncForPC(pc)
		if ok && details != nil {
			*err = fmt.Errorf("%s: %w", filepath.Base(details.Name()), *err)
		}
	}
}
