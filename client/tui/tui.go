package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
)

type Tui interface {
	GetTaskByID(id string) (Task, bool)
	UpdateTask(ev TaskEvent)
	Cleanup() error
}

type WorkTui struct {
	model   *WorkModel
	program *tea.Program
	tuiDone chan struct{}
}

func (w *WorkTui) GetTaskByID(id string) (Task, bool) {
	return w.model.GetTaskByID(id)
}

func (w *WorkTui) UpdateTask(ev TaskEvent) {
	w.program.Send(ev)
}

func (w *WorkTui) Cleanup() error {
	// Only run this process once
	select {
	case <-w.tuiDone:
		return w.program.ReleaseTerminal()
	default:
	}

	w.program.Send(DoneEvent{})
	<-w.tuiDone
	return w.program.ReleaseTerminal()
}

type NonTTYTui struct {
	tasks map[string]Task
}

func (n *NonTTYTui) GetTaskByID(id string) (Task, bool) {
	t, found := n.tasks[id]
	return t, found
}

func (n *NonTTYTui) UpdateTask(ev TaskEvent) {
	description, show := ev.Description()
	if show {
		fmt.Println(description)
	}

	n.tasks[ev.Task().ID()] = ev.Task()
}

func (n *NonTTYTui) Cleanup() error {
	return nil
}

type NullTui struct {
}

func (n *NullTui) GetTaskByID(id string) (Task, bool) {
	return nil, false
}

func (n *NullTui) UpdateTask(ev TaskEvent) {
}

func (n *NullTui) Cleanup() error {
	return nil
}

func StartWorkTui() Tui {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return &NonTTYTui{tasks: make(map[string]Task)}
	}

	model := NewWorkModel()

	p := tea.NewProgram(model)

	var tuiDone = make(chan struct{})
	go func() {
		if _, err := p.Run(); err != nil {
			fmt.Printf("TUI error: %v", err)
			os.Exit(1)
		}

		close(tuiDone)
	}()

	return &WorkTui{
		model:   model,
		program: p,
		tuiDone: tuiDone,
	}
}
