package tui

import (
	"bytes"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func NewWorkModel() *WorkModel {
	buf := new(bytes.Buffer)

	return &WorkModel{
		Spinner: func() spinner.Model {
			s := spinner.New()
			s.Spinner = spinner.Dot
			s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
			return s
		}(),
		logBuf: buf,
	}
}

type TaskEvent interface {
	Task() Task
	Description() (string, bool)
}

type DoneEvent struct{}

type Task interface {
	ID() string
	SortKey() string
	Render(spinner spinner.Model, final bool) string
}

type WorkModel struct {
	Tasks []Task

	// Shared Spinner
	Spinner spinner.Model

	Done bool

	logBuf *bytes.Buffer

	logMode bool
}

func (w *WorkModel) GetTaskByID(id string) (Task, bool) {
	for _, task := range w.Tasks {
		if task.ID() == id {
			return task, true
		}
	}
	return nil, false
}

func (m *WorkModel) Init() tea.Cmd {
	return m.Spinner.Tick
}

func (m *WorkModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

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
		// Replace existing task with new value if exists
		for index, task := range m.Tasks {
			if task.ID() == msg.Task().ID() {
				m.Tasks[index] = msg.Task()
				return m, nil
			}
		}

		// Add new task
		m.Tasks = append(m.Tasks, msg.Task())
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

func (m *WorkModel) View() string {
	return m.view(false)
}

func (m *WorkModel) view(finished bool) string {
	if m.Done {
		return ""
	}

	if m.logMode {
		return m.logBuf.String()
	}

	// Sort tasks by sort key
	sort.Slice(m.Tasks, func(i, j int) bool {
		return m.Tasks[i].SortKey() < m.Tasks[j].SortKey()
	})

	var s string

	// Iterate over incomplete tasks
	for _, task := range m.Tasks {
		s += task.Render(m.Spinner, finished)
	}

	// Send the UI for rendering
	return s
}
