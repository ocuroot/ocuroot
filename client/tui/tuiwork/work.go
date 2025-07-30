package tuiwork

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/ocuroot/ocuroot/client/tui"
)

var (
	checkMark   = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("✓")
	errorMark   = lipgloss.NewStyle().Foreground(lipgloss.Color("160")).SetString("✗")
	pendingMark = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).SetString("›")
)

type TaskEvent struct {
	Old *Task
	New *Task
}

func (e *TaskEvent) Task() tui.Task {
	return e.New
}

func (e *TaskEvent) Description() (string, bool) {
	if e.Old == nil {
		return fmt.Sprintf("%v: %v", e.New.Name, e.New.Status), true
	}
	if e.New.Status != e.Old.Status {
		if e.New.Status == WorkStatusDone {
			return fmt.Sprintf("%v> %v -> %v (%v)", e.New.Name, e.Old.Status, e.New.Status, e.New.EndTime.Sub(e.New.StartTime)), true
		} else {
			return fmt.Sprintf("%v: %v -> %v", e.New.Name, e.Old.Status, e.New.Status), true
		}
	}
	if len(e.New.Logs) > len(e.Old.Logs) {
		return fmt.Sprintf("%v> %v", e.New.Name, e.New.Logs[len(e.New.Logs)-1]), true
	}
	return "", false
}

var _ tui.Task = (*Task)(nil)

type Task struct {
	TaskID string

	StartTime time.Time
	EndTime   time.Time

	Name    string
	Status  WorkStatus
	Error   error
	Message string
	Logs    []string
}

func (t *Task) SortKey() string {
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

	return fmt.Sprintf("%d-%d", statusSort, t.StartTime.UnixNano())
}

func (t *Task) ID() string {
	return t.TaskID
}

func (task *Task) Render(spinner spinner.Model, final bool) string {
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

	duration := "N/A"
	if !task.StartTime.IsZero() {
		endTime := task.EndTime
		if endTime.IsZero() {
			endTime = time.Now()
		}
		duration = endTime.Sub(task.StartTime).String()
		s += fmt.Sprintf("%v%s (%v)\n", prefix, task.Name, duration)
	} else {
		s += fmt.Sprintf("%v%s\n", prefix, task.Name)
	}

	if task.Error != nil {
		s += fmt.Sprintf("  %s\n", task.Error)
	}
	if task.Message != "" {
		for _, line := range strings.Split(task.Message, "\n") {
			s += fmt.Sprintf("  %s\n", line)
		}
	}
	if task.Status == WorkStatusDone {
		return s
	}

	logs := task.Logs
	if task.Status != WorkStatusFailed && len(logs) > 4 {
		logs = logs[len(logs)-4:]
	}
	for _, log := range logs {
		s += fmt.Sprintf("  %s\n", log)
	}
	return s
}

type WorkStatus int

func (w WorkStatus) String() string {
	switch w {
	case WorkStatusPending:
		return "pending"
	case WorkStatusRunning:
		return "running"
	case WorkStatusFailed:
		return "failed"
	case WorkStatusDone:
		return "done"
	default:
		return "unknown"
	}
}

const (
	WorkStatusPending WorkStatus = iota
	WorkStatusRunning
	WorkStatusFailed
	WorkStatusDone
	WorkStatusUnknown
)
