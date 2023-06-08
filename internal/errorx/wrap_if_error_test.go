package errorx

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWrapIfError_hasError(t *testing.T) {

	actErr := func() (err error) {
		defer WrapIfError("errorWithMyFunc", &err)
		return errors.New("initialError")
	}()

	assert.Error(t, actErr)
	assert.Equal(t, actErr.Error(), "errorWithMyFunc: initialError")
}

func TestWrapIfError_noError(t *testing.T) {

	actErr := func() (err error) {
		defer WrapIfError("errorWithMyFunc", &err)
		return nil
	}()

	assert.NoError(t, actErr)
}

func myFuncThatMightError(throw bool) (err error) {
	defer WrapWithFuncNameIfError(&err)
	if throw {
		return errors.New("it threw an error")
	}
	return nil
}

func TestWrapWithFuncNameIfError_hasError(t *testing.T) {
	actErr := myFuncThatMightError(true)

	assert.Error(t, actErr)
	assert.Equal(t, actErr.Error(), "errorx.myFuncThatMightError: it threw an error")
}

func TestWrapWithFuncNameIfError_noError(t *testing.T) {
	actErr := myFuncThatMightError(false)

	assert.NoError(t, actErr)
}
