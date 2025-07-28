package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/mattn/go-isatty"
)

type WorkTui struct {
	model   *WorkModel
	program *tea.Program
	tuiDone chan struct{}
}

func (w *WorkTui) GetTaskByID(id string) (Task, bool) {
	return w.model.GetTaskByID(id)
}

func (w *WorkTui) UpdateTask(task Task) {
	w.program.Send(TaskEvent{Task: task})
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

func StartWorkTui(startInLogMode bool) *WorkTui {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		startInLogMode = true
		log.SetLevel(log.ErrorLevel)
	}

	model := NewWorkModel()
	model.logMode = startInLogMode

	log.SetOutput(model.logBuf)
	log.SetReportCaller(true)

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
