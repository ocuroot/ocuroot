package tuiwork

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/ocuroot/ocuroot/client/release"
	"github.com/ocuroot/ocuroot/client/tui"
	"github.com/ocuroot/ocuroot/sdk"
)

func triggerID(repo, commit string) string {
	return "@trigger" + "/" + repo + "/" + commit
}

func GetTriggerEvent(repo, commit string, t tui.Tui, status TriggerStatus, tc release.TrackerConfig) *TriggerEvent {
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
		name := fmt.Sprintf("%v [%s]", repo, commit)

		out.New = &Trigger{
			Repo:   repo,
			Commit: commit,

			Name:    name,
			Created: time.Now(),
		}
	}
	out.New.Status = status
	out.New.TC = &tc

	return out
}

func TuiLoggerForTrigger(tuiWork tui.Tui, repo, commit string, tc release.TrackerConfig) func(msg sdk.Log) {
	return func(msg sdk.Log) {
		out := GetTriggerEvent(repo, commit, tuiWork, TriggerStatusRunning, tc)
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

	Created time.Time

	Name   string
	Status TriggerStatus
	Error  error
	Logs   []string

	TC *release.TrackerConfig
}

func (t *Trigger) SortKey() string {
	var statusSort = 0
	switch t.Status {
	case TriggerStatusPending:
		statusSort = 2
	case TriggerStatusRunning:
		statusSort = 1
	case TriggerStatusFailed:
		statusSort = 0
	case TriggerStatusDone:
		statusSort = 0
	}

	return fmt.Sprintf("%d-%d", statusSort, t.Created.UnixNano())
}

func (t *Trigger) ID() string {
	return triggerID(t.Repo, t.Commit)
}

func (t *Trigger) Hierarchy() []string {
	return []string{
		"Other commits",
	}
}

func (t *Trigger) StartTime() time.Time {
	return t.Created
}

func (task *Trigger) Render(depth int, spinner spinner.Model, final bool) string {
	var s string
	if task.Status == TriggerStatusPending && !final {
		return pendingMark.String() + " "
	}
	var prefix any = pendingMark.String() + " "
	if task.Status == TriggerStatusDone {
		prefix = checkMark.String() + " "
	}
	if task.Error != nil || task.Status == TriggerStatusFailed {
		prefix = errorMark.String() + " "
	}
	if task.Status == TriggerStatusRunning {
		prefix = spinner.View()
	}
	if task.Status == TriggerStatusNoTrigger {
		prefix = pendingMark.String() + " "
	}

	s += fmt.Sprintf("%s%s%v\n", strings.Repeat("  ", depth), prefix, task.Name)

	if task.Error != nil {
		s += fmt.Sprintf("%s%s\n", strings.Repeat("  ", depth+1), task.Error)
	}

	// Only show streaming for incomplete jobs
	// Unless in debug mode
	if task.Status == TriggerStatusDone && os.Getenv("OCUROOT_DEBUG") == "" {
		return s
	}

	logs := task.Logs
	if task.Status != TriggerStatusFailed && len(logs) > 4 {
		logs = logs[len(logs)-4:]
	}
	for _, log := range logs {
		s += fmt.Sprintf("%s%s\n", strings.Repeat("  ", depth+1), log)
	}

	if task.Status == TriggerStatusNoTrigger {
		var messages []string
		if task.Repo != task.TC.Ref.Repo || task.Commit != task.TC.Commit {
			messages = []string{
				fmt.Sprintf("To execute this work, open the %q repo and run:", task.Repo),
				fmt.Sprintf(" git checkout %v", task.Commit),
				" ocuroot work any",
			}
		} else {
			messages = []string{
				"You are currently on this commit.",
				"To execute this work, run:",
				" ocuroot work any",
			}
		}

		for _, msg := range messages {
			s += fmt.Sprintf("%s%s\n", strings.Repeat("  ", depth+1), msg)
		}
	}

	return s
}

type TriggerStatus int

func (w TriggerStatus) String() string {
	switch w {
	case TriggerStatusPending:
		return "pending"
	case TriggerStatusRunning:
		return "running"
	case TriggerStatusFailed:
		return "failed"
	case TriggerStatusDone:
		return "done"
	case TriggerStatusNoTrigger:
		return "no trigger"
	default:
		return "unknown"
	}
}

const (
	TriggerStatusPending TriggerStatus = iota
	TriggerStatusRunning
	TriggerStatusFailed
	TriggerStatusDone
	TriggerStatusNoTrigger
	TriggerStatusUnknown
)
