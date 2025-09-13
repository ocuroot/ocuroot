package tuiwork

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/ocuroot/ocuroot/client/tui"
	"github.com/ocuroot/ocuroot/sdk"
)

func triggerID(repo, commit string) string {
	return "@trigger" + "/" + repo + "/" + commit
}

func GetTriggerEvent(repo, commit string, t tui.Tui, status WorkStatus) *TriggerEvent {
	var out *TriggerEvent = &TriggerEvent{}
	id := triggerID(repo, commit)

	ttr, found := t.GetTaskByID(id)
	if found {
		out.Old, _ = ttr.(*Trigger)
	}

	if out.Old != nil {
		newTask := *out.Old
		out.New = &newTask
	}
	if out.New == nil {
		name := fmt.Sprintf("Trigger work at %s", commit)

		out.New = &Trigger{
			Repo:   repo,
			Commit: commit,

			Name:         name,
			CreationTime: time.Now(),
		}
	}
	out.New.Status = status

	return out
}

func TuiLoggerForTrigger(tuiWork tui.Tui, repo, commit string) func(msg sdk.Log) {
	return func(msg sdk.Log) {
		out := GetTriggerEvent(repo, commit, tuiWork, WorkStatusRunning)
		out.New.Logs = append(out.New.Logs, msg.Message)
		tuiWork.UpdateTask(out)
	}
}

type TriggerEvent struct {
	Old *Trigger
	New *Trigger
}

func (e *TriggerEvent) Task() tui.Task {
	return e.New
}

func (e *TriggerEvent) Description() (string, bool) {
	if e.Old == nil {
		return fmt.Sprintf("%v: %v", e.New.Name, e.New.Status), true
	}
	if e.New.Status != e.Old.Status {
		return fmt.Sprintf("%v> %v -> %v", e.New.Name, e.Old.Status, e.New.Status), true
	}
	if len(e.New.Logs) > len(e.Old.Logs) {
		return fmt.Sprintf("%v> %v", e.New.Name, e.New.Logs[len(e.New.Logs)-1]), true
	}
	return "", false
}

var _ tui.Task = (*Trigger)(nil)

type Trigger struct {
	Repo   string
	Commit string

	CreationTime time.Time

	Name   string
	Status WorkStatus
	Error  error
	Logs   []string
}

func (t *Trigger) SortKey() string {
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

	return fmt.Sprintf("%d-%d", statusSort, t.CreationTime.UnixNano())
}

func (t *Trigger) ID() string {
	return triggerID(t.Repo, t.Commit)
}

func (t *Trigger) Hierarchy() []string {
	return []string{
		t.Repo,
	}
}

func (task *Trigger) Render(depth int, spinner spinner.Model, final bool) string {
	var s string
	if task.Status == WorkStatusPending && !final {
		return ""
	}
	var prefix any = pendingMark.String() + " "
	if task.Status == WorkStatusDone {
		prefix = checkMark.String() + " "
	}
	if task.Error != nil || task.Status == WorkStatusFailed {
		prefix = errorMark.String() + " "
	}
	if task.Status == WorkStatusRunning {
		prefix = spinner.View()
	}

	s += fmt.Sprintf("%s%s%v\n", strings.Repeat("  ", depth), prefix, task.Name)

	if task.Error != nil {
		s += fmt.Sprintf("%s%s\n", strings.Repeat("  ", depth+1), task.Error)
	}

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
