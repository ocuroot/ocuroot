package tuiwork

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/ocuroot/ocuroot/client/tui"
	librelease "github.com/ocuroot/ocuroot/lib/release"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
)

var (
	updateMark = lipgloss.NewStyle().Foreground(lipgloss.Color("48")).SetString("+++")
	deleteMark = lipgloss.NewStyle().Foreground(lipgloss.Color("160")).SetString("---")
)

func initCustomStateEvent(ref refs.Ref, t tui.Tui, store refstore.Store) *CustomStateTaskEvent {
	var out *CustomStateTaskEvent = &CustomStateTaskEvent{}

	ttr, found := t.GetTaskByID(ref.String())
	if found {
		out.Old, _ = ttr.(*CustomStateTask)
	}

	if out.Old != nil {
		newTask := *out.Old
		out.New = &newTask

		if store != nil {
			out.Old.Store = store
			out.New.Store = store
		}
	}
	if out.New == nil {
		name := ref.SubPath

		out.New = &CustomStateTask{
			Ref:     ref,
			Name:    name,
			Store:   store,
			Created: time.Now(),
		}
	}

	// Check if this ref still exists
	matches, err := store.Match(context.TODO(), ref.String())
	if err != nil {
		if true {
			panic(err)
		}
		return out
	}
	out.New.Exists = len(matches) > 0

	return out
}

func tuiCustomStateChange(ctx context.Context, store refstore.Store, tuiWork tui.Tui) func(ref refs.Ref) {
	return func(ref refs.Ref) {
		runRef := librelease.ReduceToRunRef(ref)

		out := initCustomStateEvent(runRef, tuiWork, store)
		tuiWork.UpdateTask(out)
	}
}

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

	Created time.Time

	Exists bool
}

func (t *CustomStateTask) SortKey() string {
	return t.Ref.SubPath
}

func (t *CustomStateTask) ID() string {
	return t.Ref.String()
}

func (t *CustomStateTask) StartTime() time.Time {
	return t.Created
}

func (t *CustomStateTask) Hierarchy() []string {
	if t.Ref.Global {
		if t.Ref.SubPathType == refs.SubPathTypeCustom {
			return []string{
				"Custom",
			}
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
	mark := updateMark
	if !task.Exists {
		mark = deleteMark
	}
	if task.Ref.SubPathType == refs.SubPathTypeCustom {
		return strings.Repeat("  ", depth) + mark.String() + " " + task.Ref.String() + "\n"
	}
	return strings.Repeat("  ", depth) + mark.String() + " " + task.Name + "\n"
}
