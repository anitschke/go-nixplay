package errorx

import (
	"fmt"
	"path/filepath"
	"runtime"
)

func WrapIfError(msg string, err *error) {
	if *err != nil {
		*err = fmt.Errorf("%s: %w", msg, *err)
	}
}

func WrapWithFuncNameIfError(err *error) {
	if *err != nil {
		pc, _, _, ok := runtime.Caller(1)
		details := runtime.FuncForPC(pc)
		if ok && details != nil {
			*err = fmt.Errorf("%s: %w", filepath.Base(details.Name()), *err)
		}
	}
}
