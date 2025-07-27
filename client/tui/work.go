package tui

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func NewWorkModel() WorkModel {
	buf := new(bytes.Buffer)

	return WorkModel{
		Spinner: func() spinner.Model {
			s := spinner.New()
			s.Spinner = spinner.Dot
			s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
			return s
		}(),
		logBuf: buf,
	}
}

var (
	checkMark   = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("✓")
	errorMark   = lipgloss.NewStyle().Foreground(lipgloss.Color("160")).SetString("✗")
	pendingMark = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).SetString("›")
)

type TaskEvent struct {
	ID      string
	Name    string
	Status  WorkStatus
	Error   error
	Message string // Message to be shown below the task
}

type LogEvent struct {
	TaskID string
	Log    string
}

type DoneEvent struct{}

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
)

type Task struct {
	ID      string
	Name    string
	Status  WorkStatus
	Error   error
	Message string
	Logs    []string
}

type WorkModel struct {
	Tasks []*Task

	// Shared Spinner
	// TODO: Separate spinners could be tracked by ID
	Spinner spinner.Model

	Done bool

	logBuf *bytes.Buffer

	logMode bool
}

func (w *WorkModel) GetTaskByID(id string) (*Task, bool) {
	for _, task := range w.Tasks {
		if task.ID == id {
			return task, true
		}
	}
	return nil, false
}

func (m WorkModel) Init() tea.Cmd {
	return m.Spinner.Tick
}

func (m WorkModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {

	// Is it a key press?
	case tea.KeyMsg:

		// Cool, what was the actual key pressed?
		switch msg.String() {

		case "l":
			m.logMode = !m.logMode
			return m, nil

		// These keys should exit the program.
		case "ctrl+c":
			return m, tea.Quit
		}
	case TaskEvent:
		task, ok := m.GetTaskByID(msg.ID)
		if !ok {
			task = &Task{
				ID:     msg.ID,
				Name:   msg.Name,
				Status: msg.Status,
				Error:  msg.Error,
			}
			m.Tasks = append(m.Tasks, task)
		} else if task.Status == msg.Status {
			return m, nil
		}

		task.Status = msg.Status
		if msg.Message != "" {
			if task.Message == "" {
				task.Message = msg.Message
			} else {
				task.Message = strings.Join([]string{task.Message, msg.Message}, "\n")
			}
		}
		task.Error = msg.Error
	case LogEvent:
		task, ok := m.GetTaskByID(msg.TaskID)
		if !ok {
			return m, nil
		}
		task.Logs = append(task.Logs, msg.Log)
	case DoneEvent:
		currentView := m.view(true)
		currentView = strings.TrimRight(currentView, "\n")

		m.Done = true
		return m, tea.Sequence(tea.Printf("%v", currentView), tea.Quit)
	case spinner.TickMsg:
		s, cmd := m.Spinner.Update(msg)
		m.Spinner = s
		return m, cmd
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}

func (m WorkModel) View() string {
	return m.view(false)
}

func (m WorkModel) view(finished bool) string {
	if m.Done {
		return ""
	}

	if m.logMode {
		return m.logBuf.String()
	}

	var s string

	// Iterate over incomplete tasks
	for _, task := range m.Tasks {
		if task.Status == WorkStatusPending && !finished {
			continue
		}
		var prefix any = pendingMark.String() + " "
		if task.Status == WorkStatusDone {
			prefix = checkMark.String() + " "
		}
		if task.Error != nil || task.Status == WorkStatusFailed {
			prefix = errorMark.String() + " "
		}
		if task.Status == WorkStatusRunning {
			prefix = m.Spinner.View()
		}
		s += fmt.Sprintf("%v%s\n", prefix, task.Name)
		if task.Error != nil {
			s += fmt.Sprintf("  %s\n", task.Error)
		}
		if task.Message != "" {
			for _, line := range strings.Split(task.Message, "\n") {
				s += fmt.Sprintf("  %s\n", line)
			}
		}
		if task.Status == WorkStatusDone {
			continue
		}

		logs := task.Logs
		if task.Status != WorkStatusFailed && len(logs) > 4 {
			logs = logs[len(logs)-4:]
		}
		for _, log := range logs {
			s += fmt.Sprintf("  %s\n", log)
		}

	}

	// Send the UI for rendering
	return s
}
