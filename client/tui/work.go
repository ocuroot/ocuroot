package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func NewWorkModel() *WorkModel {
	return &WorkModel{
		Spinner: func() spinner.Model {
			s := spinner.New()
			s.Spinner = spinner.Dot
			s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
			return s
		}(),
	}
}

type TaskEvent interface {
	Task() Task
	Description() (string, bool)
}

type DoneEvent struct{}

type Task interface {
	ID() string
	Hierarchy() []string
	SortKey() string
	Render(depth int, spinner spinner.Model, final bool) string
}

type WorkModel struct {
	Tasks []Task

	// Shared Spinner
	Spinner spinner.Model

	Done bool
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

	// Sort tasks by sort key
	sort.Slice(m.Tasks, func(i, j int) bool {
		return m.Tasks[i].SortKey() < m.Tasks[j].SortKey()
	})

	hierarchy := &HierarchyNode{}
	for _, task := range m.Tasks {
		hierarchy.Add(task.Hierarchy(), task)
	}

	s := hierarchy.Render(0, m.Spinner, finished)

	// Send the UI for rendering
	return s
}

type HierarchyNode struct {
	Children map[string]*HierarchyNode
	Elems    map[string]Task
}

func (h *HierarchyNode) Render(depth int, spinner spinner.Model, finished bool) string {
	var s string
	var elemIDs []string
	for _, elem := range h.Elems {
		elemIDs = append(elemIDs, elem.ID())
	}
	sort.Slice(elemIDs, func(i, j int) bool {
		idI := h.Elems[elemIDs[i]]
		idJ := h.Elems[elemIDs[j]]
		return idI.SortKey() < idJ.SortKey()
	})

	for _, id := range elemIDs {
		s += h.Elems[id].Render(depth, spinner, finished)
	}
	for name, child := range h.Children {
		s += fmt.Sprintf("%s%s:\n", strings.Repeat("  ", depth), name)
		s += child.Render(depth+1, spinner, finished)
	}
	return s
}

func (h *HierarchyNode) Add(hierarchy []string, t Task) {
	if h.Elems == nil {
		h.Elems = make(map[string]Task)
	}

	if len(hierarchy) == 0 {
		h.Elems[t.ID()] = t
		return
	}

	child := hierarchy[0]
	hierarchy = hierarchy[1:]

	if h.Children == nil {
		h.Children = make(map[string]*HierarchyNode)
	}

	if _, ok := h.Children[child]; !ok {
		h.Children[child] = &HierarchyNode{}
	}

	h.Children[child].Add(hierarchy, t)
}
