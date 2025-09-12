package tuiwork

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/ocuroot/ocuroot/client/tui"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
)

type CustomStateTaskEvent struct {
	Old *CustomStateTask
	New *CustomStateTask
}

func (e *CustomStateTaskEvent) Task() tui.Task {
	return e.New
}

func (e *CustomStateTaskEvent) Description() (string, bool) {
	return fmt.Sprintf("Updated %v", e.New.Ref.String()), true
}

var _ tui.Task = (*CustomStateTask)(nil)

type CustomStateTask struct {
	Ref  refs.Ref
	Name string

	Store refstore.Store
}

func (t *CustomStateTask) SortKey() string {
	return t.Ref.SubPath
}

func (t *CustomStateTask) ID() string {
	return t.Ref.String()
}

func (t *CustomStateTask) Hierarchy() []string {
	if t.Ref.Global {
		if t.Ref.SubPathType == refs.SubPathTypeCustom {
			return []string{}
		}
		if t.Ref.SubPathType == refs.SubPathTypeEnvironment {
			return []string{
				"environments",
			}
		}
	}

	return []string{
		t.Ref.Repo,
		t.Ref.Filename,
	}
}

func (task *CustomStateTask) Render(depth int, spinner spinner.Model, final bool) string {
	return strings.Repeat("  ", depth) + updateMark.String() + " " + task.Name + "\n"
}
