package starlarkerrors

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/log"
	"go.starlark.net/starlark"
)

var sdkFilePathRegex = regexp.MustCompile(`^sdk/\d+\.\d+\.\d+/.+\.star`)

func Wrap(err error) error {
	if err == nil {
		return nil
	}
	if evalErr, ok := err.(*starlark.EvalError); ok {
		bt := evalErr.Backtrace()
		lines := strings.Split(bt, "\n")
		for i := len(lines) - 2; i >= 0; i-- {
			pos := strings.TrimLeft(lines[i], " \t")
			log.Info("line", "line", lines[i])
			if !sdkFilePathRegex.MatchString(pos) {
				return fmt.Errorf("%s - %w", pos, err)
			}
		}
		lastLine := lines[len(lines)-1]
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
