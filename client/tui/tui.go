package tui

import (
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/mattn/go-isatty"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/sdk"
)

func StartWorkTui(startInLogMode bool) (update func(tea.Msg), cleanup func() error, err error) {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		currentStatuses := make(map[string]WorkStatus)

		log.SetOutput(io.Discard)
		return func(msg tea.Msg) {
			switch msg := msg.(type) {
			case TaskEvent:
				// Avoid repeating status updates
				if status, ok := currentStatuses[msg.Name]; ok && status == msg.Status {
					return
				}

				currentStatuses[msg.Name] = msg.Status
				fmt.Println(msg.Name + ": " + msg.Status.String())
			case LogEvent:
				fmt.Println("  " + msg.Log)
			}
		}, func() error { return nil }, nil
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

	return p.Send, func() error {
		// Only run this process once
		select {
		case <-tuiDone:
			return p.ReleaseTerminal()
		default:
		}

		p.Send(DoneEvent{})
		<-tuiDone
		return p.ReleaseTerminal()
	}, nil
}

func FunctionLogToEvent(fnRef refs.Ref, msg sdk.Log) LogEvent {
	return LogEvent{
		TaskID: string(fnRef.String()),
		Log:    msg.Message,
	}
}
