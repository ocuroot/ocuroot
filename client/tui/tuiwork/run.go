package tuiwork

import (
	"context"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client/tui"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/store/models"

	librelease "github.com/ocuroot/ocuroot/lib/release"
)

var (
	checkMark   = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("✓")
	errorMark   = lipgloss.NewStyle().Foreground(lipgloss.Color("160")).SetString("✗")
	pendingMark = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).SetString("›")
	updateMark  = lipgloss.NewStyle().Foreground(lipgloss.Color("48")).SetString("+")
)

func initRunStateEvent(ref refs.Ref, t tui.Tui, store refstore.Store) *RunTaskEvent {
	ctx := context.TODO()
	var err error

	runRef := librelease.ReduceToRunRef(ref)

	var out *RunTaskEvent = &RunTaskEvent{}

	ttr, found := t.GetTaskByID(runRef.String())
	if found {
		out.Old, _ = ttr.(*RunTask)
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
		name := strings.Split(runRef.SubPath, "/")[0]
		if runRef.SubPathType == refs.SubPathTypeDeploy {
			var run models.Run
			if store != nil {
				err = store.Get(ctx, runRef.String(), &run)
				if err != nil {
					log.Error("failed to get run", "error", err)
				} else {
					if run.Type == models.RunTypeDown {
						name = fmt.Sprintf("remove from %s", name)
					} else {
						name = fmt.Sprintf("deploy to %s", name)
					}
				}
			} else {
				log.Error("failed to get run", "error", "no store")
			}
		}
		name += fmt.Sprintf(" [%s]", path.Base(runRef.SubPath))

		out.New = &RunTask{
			RunRef:       runRef,
			Name:         name,
			Status:       WorkStatusPending,
			CreationTime: time.Now(),

			Store:  store,
			JobRef: runRef,
		}
	}

	return out
}

func tuiRunStatusChange(ctx context.Context, store refstore.Store, tuiWork tui.Tui) func(ref refs.Ref) {
	return func(ref refs.Ref) {
		runRef := librelease.ReduceToRunRef(ref)

		out := initRunStateEvent(runRef, tuiWork, store)
		updateStatus(ctx, store, runRef, out)

		if out.New.Status == WorkStatusRunning {
			if out.Old == nil || out.Old.Status != WorkStatusRunning {
				out.New.StartTime = time.Now()
			}
		}
		if out.New.Status == WorkStatusDone || out.New.Status == WorkStatusFailed {
			if out.Old == nil || (out.Old.Status != WorkStatusDone && out.Old.Status != WorkStatusFailed) {
				out.New.EndTime = time.Now()
			}
		}

		tuiWork.UpdateTask(out)
	}
}

type RunTaskEvent struct {
	Old *RunTask
	New *RunTask
}

func (e *RunTaskEvent) Task() tui.Task {
	return e.New
}

func (e *RunTaskEvent) Description() (string, bool) {
	fullName := fmt.Sprintf("%v > %v", strings.Join(e.New.Hierarchy(), " > "), e.New.Name)
	if e.Old == nil {
		return fmt.Sprintf("%v: %v", fullName, e.New.Status), true
	}
	if e.New.Status != e.Old.Status {
		if e.New.Status == WorkStatusDone {
			return fmt.Sprintf("%v> %v -> %v (%v)", fullName, e.Old.Status, e.New.Status, e.New.EndTime.Sub(e.New.StartTime)), true
		} else {
			return fmt.Sprintf("%v> %v -> %v", fullName, e.Old.Status, e.New.Status), true
		}
	}
	if len(e.New.Logs) > len(e.Old.Logs) {
		return fmt.Sprintf("%v> %v", fullName, e.New.Logs[len(e.New.Logs)-1]), true
	}
	return "", false
}

var _ tui.Task = (*RunTask)(nil)

type RunTask struct {
	RunRef refs.Ref

	CreationTime time.Time
	StartTime    time.Time
	EndTime      time.Time

	Name   string
	Status WorkStatus
	Error  error
	Logs   []string

	Store  refstore.Store
	JobRef refs.Ref
}

func (t *RunTask) SortKey() string {
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

	var keyTime string
	if t.StartTime.IsZero() {
		keyTime = fmt.Sprintf("%d", t.CreationTime.UnixNano())
	} else {
		keyTime = fmt.Sprintf("%d", t.StartTime.UnixNano())
	}
	return fmt.Sprintf("%d-%s", statusSort, keyTime)
}

func (t *RunTask) ID() string {
	return t.RunRef.String()
}

func (t *RunTask) Hierarchy() []string {
	return []string{
		t.RunRef.Repo,
		t.RunRef.Filename,
	}
}

func (task *RunTask) Render(depth int, spinner spinner.Model, final bool) string {
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
		s += fmt.Sprintf("%s%s%v (%v)\n", strings.Repeat("  ", depth), prefix, task.Name, duration)
	} else {
		s += fmt.Sprintf("%s%s%v\n", strings.Repeat("  ", depth), prefix, task.Name)
	}

	if task.Error != nil {
		s += fmt.Sprintf("%s%s\n", strings.Repeat("  ", depth+1), task.Error)
	}
	msg := task.message()
	if msg != "" {
		for _, line := range strings.Split(msg, "\n") {
			s += fmt.Sprintf("%s%s\n", strings.Repeat("  ", depth+1), line)
		}
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

func (t *RunTask) message() string {
	ctx := context.TODO()

	var message string
	if t.Status != WorkStatusDone && t.Status != WorkStatusPending {
		return ""
	}

	// Get job outputs and render as a message
	var jobWork *models.Run
	if err := t.Store.Get(ctx, t.JobRef.String(), &jobWork); err != nil {
		log.Error("failed to get job", "ref", t.JobRef.String(), "error", err)
		return "failed to get job: " + t.JobRef.String() + "\n" + err.Error()
	}

	if t.Status == WorkStatusDone && len(jobWork.Outputs) > 0 {
		outputs := tree.Root("Outputs")
		keys := make([]string, 0, len(jobWork.Outputs))
		for k := range jobWork.Outputs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := jobWork.Outputs[k]
			outputs = outputs.Child(
				tree.Root(
					fmt.Sprintf("%s#output/%s", t.ID(), k),
				).Child(v),
			)
		}
		message += outputs.String()
	}

	if t.Status == WorkStatusPending {
		fn := jobWork.Functions[0]
		hasPending := false
		pendingInputs := tree.Root("Pending Inputs")
		keys := make([]string, 0, len(fn.Inputs))
		for k := range fn.Inputs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := fn.Inputs[k]
			retrieved, err := librelease.RetrieveInput(ctx, t.Store, v)
			if err != nil {
				log.Error("failed to retrieve input", "ref", v.Ref.String(), "error", err)
				return "failed to retrieve input"
			}

			if retrieved.Default == nil && retrieved.Value == nil {
				hasPending = true
				pendingInputs = pendingInputs.Child(v.Ref)
			}
		}
		if hasPending {
			message += pendingInputs.String()
		}
	}
	return message
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
