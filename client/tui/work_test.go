package tui

import (
	"encoding/json"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestWorkModelUpdate(t *testing.T) {
	initialSpinner := NewWorkModel().Spinner

	var tests = []struct {
		name string

		in       WorkModel
		msg      tea.Msg
		out      modelExample
		cmdCheck func(t *testing.T, cmd tea.Cmd)
	}{
		{
			name: "spinner tick",
			in:   NewWorkModel(),
			msg:  spinner.TickMsg{},
			out: modelExample{
				Spinner: "⣽ ",
			},
		},
		{
			name: "first mention of task",
			in:   NewWorkModel(),
			msg: TaskEvent{
				ID:     "1",
				Name:   "Task number 1",
				Status: WorkStatusPending,
			},
			out: modelExample{
				Spinner: "⣾ ",
				Tasks: []*Task{
					{
						ID:     "1",
						Name:   "Task number 1",
						Status: WorkStatusPending,
					},
				},
			},
		},
		{
			name: "update task status",
			in: WorkModel{
				Tasks: []*Task{
					{
						ID:     "1",
						Name:   "Task number 1",
						Status: WorkStatusPending,
					},
				},
				Spinner: initialSpinner,
			},
			msg: TaskEvent{
				ID:     "1",
				Name:   "Task number 1",
				Status: WorkStatusRunning,
			},
			out: modelExample{
				Spinner: "⣾ ",
				Tasks: []*Task{
					{
						ID:     "1",
						Name:   "Task number 1",
						Status: WorkStatusRunning,
					},
				},
			},
		},
		{
			name: "update task with same status",
			in: WorkModel{
				Tasks: []*Task{
					{
						ID:     "1",
						Name:   "Task number 1",
						Status: WorkStatusRunning,
					},
				},
				Spinner: initialSpinner,
			},
			msg: TaskEvent{
				ID:     "1",
				Name:   "Task number 1",
				Status: WorkStatusRunning,
			},
			out: modelExample{
				Spinner: "⣾ ",
				Tasks: []*Task{
					{
						ID:     "1",
						Name:   "Task number 1",
						Status: WorkStatusRunning,
					},
				},
			},
		},
		{
			name: "log event",
			in: WorkModel{
				Tasks: []*Task{
					{
						ID:     "1",
						Name:   "Task number 1",
						Status: WorkStatusRunning,
					},
				},
				Spinner: initialSpinner,
			},
			msg: LogEvent{
				TaskID: "1",
				Log:    "Log number 1",
			},
			out: modelExample{
				Spinner: "⣾ ",
				Tasks: []*Task{
					{
						ID:     "1",
						Name:   "Task number 1",
						Status: WorkStatusRunning,
						Logs:   []string{"Log number 1"},
					},
				},
			},
		},
		{
			name: "logs append",
			in: WorkModel{
				Tasks: []*Task{
					{
						ID:     "1",
						Name:   "Task number 1",
						Status: WorkStatusRunning,
						Logs:   []string{"Log number 1"},
					},
				},
				Spinner: initialSpinner,
			},
			msg: LogEvent{
				TaskID: "1",
				Log:    "Log number 2",
			},
			out: modelExample{
				Spinner: "⣾ ",
				Tasks: []*Task{
					{
						ID:     "1",
						Name:   "Task number 1",
						Status: WorkStatusRunning,
						Logs:   []string{"Log number 1", "Log number 2"},
					},
				},
			},
		},
		{
			name: "task done",
			in: WorkModel{
				Tasks: []*Task{
					{
						ID:     "1",
						Name:   "Task number 1",
						Status: WorkStatusRunning,
						Logs:   []string{"Log number 1", "Log number 2"},
					},
				},
				Spinner: initialSpinner,
			},
			msg: TaskEvent{
				ID:     "1",
				Name:   "Task number 1",
				Status: WorkStatusDone,
			},
			out: modelExample{
				Spinner: "⣾ ",
				Tasks: []*Task{
					{
						ID:     "1",
						Name:   "Task number 1",
						Status: WorkStatusDone,
						Logs:   []string{"Log number 1", "Log number 2"},
					},
				},
			},
		},
		{
			name: "ignore events for already done tasks",
			in: WorkModel{
				Tasks: []*Task{
					{
						ID:     "1",
						Name:   "Task number 1",
						Status: WorkStatusDone,
						Logs:   []string{"Log number 1", "Log number 2"},
					},
				},
				Spinner: initialSpinner,
			},
			msg: TaskEvent{
				ID:     "1",
				Name:   "Task number 1",
				Status: WorkStatusDone,
			},
			out: modelExample{
				Spinner: "⣾ ",
				Tasks: []*Task{
					{
						ID:     "1",
						Name:   "Task number 1",
						Status: WorkStatusDone,
						Logs:   []string{"Log number 1", "Log number 2"},
					},
				},
			},
			cmdCheck: func(t *testing.T, cmd tea.Cmd) {
				if cmd != nil {
					t.Errorf("expected nil cmd, got %v", cmd)
				}
			},
		},
		{
			name: "done event",
			in:   NewWorkModel(),
			msg:  DoneEvent{},
			out: modelExample{
				Spinner: "⣾ ",
				Done:    true,
			},
			cmdCheck: func(t *testing.T, cmd tea.Cmd) {
				res := cmd()
				if seq, ok := res.([]tea.Msg); ok {
					if len(seq) != 2 {
						t.Errorf("expected 2 msgs, got %v", len(seq))
					}
					if seq[1] != tea.Quit() {
						t.Errorf("expected tea.Quit, got %v", seq[1])
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, cmd := tt.in.Update(tt.msg)

			gotJSON, _ := json.MarshalIndent(got, "", "  ")
			outJSON, _ := json.MarshalIndent(tt.out, "", "  ")

			assert.Equal(t, string(gotJSON), string(outJSON))
			if tt.cmdCheck != nil {
				tt.cmdCheck(t, cmd)
			}
		})
	}

}

type modelExample struct {
	Tasks   []*Task
	Spinner string
	Done    bool
}

func (m WorkModel) MarshalJSON() ([]byte, error) {
	return json.Marshal(modelExample{
		Tasks:   m.Tasks,
		Spinner: m.Spinner.View(),
		Done:    m.Done,
	})
}
