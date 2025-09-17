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

func GetConfigEvent(ref refs.Ref, t tui.Tui, status WorkStatus, config *sdk.Config) *ConfigEvent {
	var out *ConfigEvent = &ConfigEvent{}
	id := ref.String()

	ttr, found := t.GetTaskByID(id)
	if found {
		out.Old, _ = ttr.(*Config)
	}

	if out.Old != nil {
		newTask := *out.Old
		out.New = &newTask
	}
	if out.New == nil {
		out.New = &Config{
			Ref: ref,

			Created: time.Now(),
		}
	}
	out.New.Status = status
	if config != nil {
		out.New.Config = config
	}

	return out
}

func TuiLoggerForConfig(tuiWork tui.Tui, ref refs.Ref, config *sdk.Config) func(msg sdk.Log) {
	return func(msg sdk.Log) {
		out := GetConfigEvent(ref, tuiWork, WorkStatusRunning, config)
		out.New.Logs = append(out.New.Logs, msg.Message)
		tuiWork.UpdateTask(out)
	}
}

type ConfigEvent struct {
	Old *Config
	New *Config
}

func (e *ConfigEvent) Task() tui.Task {
	return e.New
}

func (e *ConfigEvent) Description() (string, bool) {
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

type Config struct {
	Ref refs.Ref

	Config *sdk.Config

	Created time.Time

	Status WorkStatus
	Logs   []string
}

func (t *Config) SortKey() string {
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

func (t *Config) ID() string {
	return t.Ref.String()
}

func (t *Config) Hierarchy() []string {
	return []string{
		t.Ref.Repo,
		t.Ref.Filename,
	}
}

func (t *Config) StartTime() time.Time {
	return t.Created
}

func (task *Config) Render(depth int, spinner spinner.Model, final bool) string {
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

	if task.Status != WorkStatusDone {
		s += fmt.Sprintf("%s%s%v\n", strings.Repeat("  ", depth), prefix, "Load config")
	} else {
		msg := "Load config"
		if task.Config != nil {
			if task.Config.Package == nil {
				msg = "Loaded config: No phases"
			} else {
				var tasks int
				var environments int
				for _, phase := range task.Config.Package.Phases {
					for _, t := range phase.Tasks {
						if t.Task != nil {
							tasks++
						}
						if t.Deployment != nil {
							environments++
						}
					}
				}

				msg = fmt.Sprintf("Loaded config: %d tasks, deploy to %d environments", tasks, environments)
			}
		}
		s += fmt.Sprintf("%s%s%v\n", strings.Repeat("  ", depth), prefix, msg)
	}

	// Only show streaming for incomplete jobs
	// Unless in debug mode
	if task.Status != WorkStatusDone || os.Getenv("OCUROOT_DEBUG") == "" {

		logs := task.Logs
		if task.Status != WorkStatusFailed && len(logs) > 4 {
			logs = logs[len(logs)-4:]
		}
		for _, log := range logs {
			s += fmt.Sprintf("%s%s\n", strings.Repeat("  ", depth+1), log)
		}
	}

	return s
}
