package tuiwork

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/ocuroot/ocuroot/client/tui"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/sdk"
)

func GetRepoEvent(rootPath string, ref refs.Ref, t tui.Tui, status WorkStatus) *RepoEvent {
	var out *RepoEvent = &RepoEvent{}
	id := rootPath

	ttr, found := t.GetTaskByID(id)
	if found {
		out.Old, _ = ttr.(*Repo)
	}

	if out.Old != nil {
		newTask := *out.Old
		out.New = &newTask
	}
	if out.New == nil {
		out.New = &Repo{
			RootPath: rootPath,
			Created:  time.Now(),
		}
	}
	out.New.Status = status
	out.New.Ref = ref

	return out
}

func TuiLoggerForRepo(tuiWork tui.Tui, rootPath string, ref refs.Ref) func(msg sdk.Log) {
	return func(msg sdk.Log) {
		out := GetRepoEvent(rootPath, ref, tuiWork, WorkStatusRunning)
		out.New.Logs = append(out.New.Logs, msg.Message)
		tuiWork.UpdateTask(out)
	}
}

type RepoEvent struct {
	Old *Repo
	New *Repo
}

func (e *RepoEvent) Task() tui.Task {
	return e.New
}

func (e *RepoEvent) Description() (string, bool) {
	if e.Old == nil {
		return fmt.Sprintf("%v: %v", "Load Config", e.New.Status), true
	}
	if e.New.Status != e.Old.Status {
		return fmt.Sprintf("%v> %v -> %v", "Load Config", e.Old.Status, e.New.Status), true
	}
	if len(e.New.Logs) > len(e.Old.Logs) {
		return fmt.Sprintf("%v> %v", "Load Config", e.New.Logs[len(e.New.Logs)-1]), true
	}
	return "", false
}

var _ tui.Task = (*Config)(nil)

type Repo struct {
	RootPath string
	Ref      refs.Ref

	Created time.Time

	Status WorkStatus
	Logs   []string
}

func (t *Repo) SortKey() string {
	var statusSort = 0
	switch t.Status {
	case WorkStatusPending:
		statusSort = 2
	case WorkStatusRunning:
		statusSort = 1
	case WorkStatusFailed:
		statusSort = 0
	case WorkStatusDone:
		statusSort = 0
	}

	return fmt.Sprintf("%d-%d", statusSort, t.Created.UnixNano())
}

func (t *Repo) ID() string {
	return t.RootPath
}

func (t *Repo) Hierarchy() []string {
	if t.Ref.Repo == "" {
		return []string{
			t.RootPath,
		}
	}
	return []string{
		t.Ref.Repo,
	}
}

func (t *Repo) StartTime() time.Time {
	return t.Created
}

func (task *Repo) Render(depth int, spinner spinner.Model, final bool) string {
	var s string
	if task.Status == WorkStatusPending && !final {
		return pendingMark.String() + " "
	}
	var prefix any = pendingMark.String() + " "
	if task.Status == WorkStatusDone {
		prefix = checkMark.String() + " "
	}
	if task.Status == WorkStatusFailed {
		prefix = errorMark.String() + " "
	}
	if task.Status == WorkStatusRunning {
		prefix = spinner.View()
	}

	s += fmt.Sprintf("%s%s%v\n", strings.Repeat("  ", depth), prefix, "Load config")

	// Only show streaming for incomplete jobs
	// Unless in debug mode
	if task.Status == WorkStatusDone && os.Getenv("OCUROOT_DEBUG") == "" {
		return s
	}

	logs := task.Logs
	if task.Status != WorkStatusFailed && len(logs) > 4 {
		logs = logs[len(logs)-4:]
	}
	for _, log := range logs {
		s += fmt.Sprintf("%s%s\n", strings.Repeat("  ", depth+1), log)
	}

	return s
}
