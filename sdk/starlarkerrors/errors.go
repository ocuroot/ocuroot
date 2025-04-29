package starlarkerrors

import (
	"fmt"
	"strings"

	"go.starlark.net/starlark"
)

func Wrap(err error) error {
	if err == nil {
		return nil
	}
	if evalErr, ok := err.(*starlark.EvalError); ok {
		bt := evalErr.Backtrace()
		lines := strings.Split(bt, "\n")
		lastLine := lines[len(lines)-2]
		return fmt.Errorf("%s - %w", lastLine, err)
	}
	return fmt.Errorf("%w", err)
}

func Render(err error) string {
	if err == nil {
		return ""
	}
	errMsg := err.Error()
	if evalErr, ok := err.(*starlark.EvalError); ok {
		errMsg = fmt.Sprintf("%s\n%s", errMsg, evalErr.Backtrace())
	}
	return errMsg
}
