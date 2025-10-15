package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/x/term"
	"github.com/ocuroot/ocuroot/client/release"
	"github.com/ocuroot/ocuroot/client/work"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/spf13/cobra"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

var ReplCmd = &cobra.Command{
	Use:   "repl [file]",
	Short: "Start a Starlark REPL with the Ocuroot SDK",
	Long: `Start an interactive Read-Eval-Print Loop (REPL) for the Ocuroot SDK in the Starlark language.
This allows you to interactively test, explore, and develop with the SDK.

The REPL automatically loads the repo.ocu.star file to provide access to the environment
list and state store. If a .ocu.star file is provided, it will be executed before starting 
the REPL, and its globals will be available in the REPL session.

Examples:
  ocuroot repl
  ocuroot repl package.ocu.star
  ocuroot repl release.ocu.star
  ocuroot repl -c "print('hello')"
  ocuroot repl release.ocu.star -c "host.shell('ls')"
`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		command, _ := cmd.Flags().GetString("command")
		filePath := ""
		if len(args) > 0 {
			filePath = args[0]
		}

		if command != "" {
			// Execute single command mode
			return runSingleCommand(ctx, filePath, command)
		} else {
			// Interactive REPL mode
			return runStarlarkReplWithFile(ctx, filePath)
		}
	},
}

// runSingleCommand executes a single Starlark command and exits
func runSingleCommand(ctx context.Context, filePath string, command string) error {
	// Get tracker config to load repo.ocu.star and set up state store
	w, err := work.NewWorker(ctx, refs.Ref{
		Repo:     "preview.git",
		Filename: filePath,
	})
	if err != nil {
		return fmt.Errorf("failed to create worker: %w", err)
	}
	// We don't need the TUI after this
	w.Cleanup()

	if filePath != "" {
		w.Tracker.Ref = refs.Ref{
			Repo:     w.Tracker.Ref.Repo,
			Filename: filePath,
		}
	}
	// Create a backend for SDK operations
	backend, _ := release.NewBackend(w.Tracker)

	var globals starlark.StringDict

	// If a file is specified, load it and get combined globals (SDK + user functions)
	if filePath != "" {
		fmt.Printf("Loading file: %s\n", filePath)

		// Load the .ocu.star file using sdk.LoadConfig to get user-defined functions
		config, err := sdk.LoadConfig(
			ctx,
			sdk.NewFSResolver(os.DirFS(w.Tracker.RepoPath)),
			filePath,
			backend,
			func(thread *starlark.Thread, msg string) {
				fmt.Printf("[%s] %s\n", filePath, msg)
			},
		)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Get SDK version for evaluation
		versions := sdk.AvailableVersions()
		if len(versions) == 0 {
			return fmt.Errorf("no SDK versions available")
		}
		latestVersion := versions[len(versions)-1]

		// Create globals that combine SDK builtins with user-defined functions
		globals, err = createGlobalsWithUserFunctions(ctx, backend, latestVersion, config)
		if err != nil {
			return fmt.Errorf("failed to create globals: %w", err)
		}

		fmt.Printf("File loaded successfully\n")
	} else {
		// No file specified, just use SDK builtins
		versions := sdk.AvailableVersions()
		if len(versions) == 0 {
			return fmt.Errorf("no SDK versions available")
		}
		latestVersion := versions[len(versions)-1]

		// Get just SDK builtins
		_, globals, err = sdk.EvalWithGlobals(ctx, backend, latestVersion, "None", make(starlark.StringDict))
		if err != nil {
			return fmt.Errorf("failed to get SDK builtins: %w", err)
		}
	}

	// Parse the command
	fmt.Printf("Executing: %s\n", command)
	expr, err := syntax.ParseExpr("<stdin>", command, 0)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	// Create a thread for execution
	thread := &starlark.Thread{
		Name: "single-command",
		Print: func(thread *starlark.Thread, msg string) {
			fmt.Println(msg)
		},
	}

	// Execute the command using the combined globals
	result, err := starlark.EvalExpr(thread, expr, globals)
	if err != nil {
		return fmt.Errorf("error executing command: %w", err)
	}

	// Print the result
	if result != starlark.None {
		fmt.Printf("Result: %v\n", result)
	} else {
		fmt.Println("Command executed successfully")
	}

	return nil
}

func runStarlarkReplWithFile(ctx context.Context, filePath string) error {
	// Get tracker config to load repo.ocu.star and set up state store
	w, err := work.NewWorker(ctx, refs.Ref{
		Repo:     "preview.git",
		Filename: filePath,
	})
	if err != nil {
		return fmt.Errorf("failed to create worker: %w", err)
	}
	// We don't need the TUI after this
	w.Cleanup()

	if filePath == "" {
		filePath = "repo.ocu.star"
	}
	w.Tracker.Ref = refs.Ref{
		Repo:     w.Tracker.Ref.Repo,
		Filename: filePath,
	}
	// Create a backend for SDK operations
	backend, _ := release.NewBackend(w.Tracker)

	// Load the .ocu.star file using sdk.LoadConfig to get user-defined functions
	config, err := sdk.LoadConfig(
		ctx,
		sdk.NewFSResolver(os.DirFS(w.Tracker.RepoPath)),
		filePath,
		backend,
		func(thread *starlark.Thread, msg string) {
			fmt.Printf("[%s] %s\n", filePath, msg)
		},
	)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get SDK version for evaluation
	versions := sdk.AvailableVersions()
	if len(versions) == 0 {
		return fmt.Errorf("no SDK versions available")
	}
	latestVersion := versions[len(versions)-1]

	// Create globals that combine SDK builtins with user-defined functions
	globals, err := createGlobalsWithUserFunctions(ctx, backend, latestVersion, config)
	if err != nil {
		return fmt.Errorf("failed to create globals: %w", err)
	}

	fmt.Println("Starting Starlark REPL with Ocuroot SDK")
	fmt.Println("Ctrl+C or Escape to exit")
	fmt.Println("Type 'help()' to see available globals")
	fmt.Println()

	// Use the combined globals for REPL that includes user-defined functions
	return runCustomREPLWithGlobals(globals)
}

// createGlobalsWithUserFunctions creates a globals dict that combines SDK builtins with user-defined functions
func createGlobalsWithUserFunctions(ctx context.Context, backend sdk.Backend, sdkVersion string, config *sdk.Config) (starlark.StringDict, error) {
	// Get SDK builtins using EvalWithGlobals
	_, globals, err := sdk.EvalWithGlobals(ctx, backend, sdkVersion, "None", make(starlark.StringDict))
	if err != nil {
		return nil, fmt.Errorf("failed to get SDK builtins: %w", err)
	}

	// Add user-defined functions from the config
	userFuncs := config.GlobalFuncs()
	for name, fn := range userFuncs {
		// Extract the function name from the definition string
		// The name in the map is the full definition, but we want just the function name
		if fn != nil {
			// Try to extract function name from the function itself
			if fnName := fn.Name(); fnName != "" {
				globals[fnName] = fn
			} else {
				// Fallback: use the key name (though it might be a full definition)
				globals[name] = fn
			}
		}
	}

	return globals, nil
}

// runCustomREPLWithGlobals implements a REPL that uses pre-loaded globals
func runCustomREPLWithGlobals(globals starlark.StringDict) error {
	m := newReplModel(globals)
	p := tea.NewProgram(m)
	m.p = p
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
	return nil
}

func init() {
	RootCmd.AddCommand(ReplCmd)
	AddRefFlags(ReplCmd, true)
	ReplCmd.Flags().StringP("command", "c", "", "Execute a single command and exit (non-interactive mode)")
}

var _ tea.Model = &replModel{}

func newReplModel(globals starlark.StringDict) *replModel {
	// Get the terminal width
	terminalWidth, _, err := term.GetSize(uintptr(os.Stdout.Fd()))
	if err != nil {
		terminalWidth = 80
	}

	ti := textinput.New()
	ti.Placeholder = "Enter your statement"
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = terminalWidth - 4
	ti.ShowSuggestions = true

	var suggestions []string
	for g := range globals {
		suggestions = append(suggestions, g)
	}
	ti.SetSuggestions(suggestions)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &replModel{
		textInput:     ti,
		globals:       globals,
		spinner:       s,
		terminalWidth: terminalWidth,
	}
}

type replModel struct {
	p *tea.Program

	spinner     spinner.Model
	command     string
	log         string
	returnValue any
	errMsg      string

	running bool

	textInput textinput.Model

	globals starlark.StringDict

	terminalWidth int
}

// Init implements tea.Model.
func (r *replModel) Init() tea.Cmd {
	return textinput.Blink
}

func (r *replModel) execute(line string) tea.Msg {
	r.command = line
	r.running = true
	r.log = ""
	r.returnValue = nil
	r.errMsg = ""
	if line == "help()" {
		for name, g := range r.globals {
			switch gt := g.(type) {
			case *starlark.Function:
				r.p.Send(execLine(fmt.Sprintf("%s: %s\n", name, gt)))
			default:
				r.p.Send(execLine(fmt.Sprintf("%s: %s\n", name, gt)))
			}
		}
		r.running = false
		return nil
	}

	// Create a new thread for this evaluation
	thread := &starlark.Thread{
		Name: "repl",
		Print: func(thread *starlark.Thread, msg string) {
			r.p.Send(execLine(msg))
		},
	}

	opts := syntax.FileOptions{}
	opts.LoadBindsGlobally = true

	readline := func() ([]byte, error) {
		return []byte(line + "\n"), nil
	}

	// Try to evaluate the expression
	go func() {
		f, err := opts.ParseCompoundStmt("<stdin>", readline)
		if err != nil {
			r.p.Send(execError(err))
			return
		}

		if expr := soleExpr(f); expr != nil {
			// eval
			v, err := starlark.EvalExprOptions(f.Options, thread, expr, r.globals)
			if err != nil {
				r.p.Send(execError(err))
				return
			}

			// store the result in "_" variable to hold the value of last expression, similar to Python REPL
			r.globals["_"] = v

			// print
			if v != starlark.None {
				r.p.Send(execReturnValue{content: v})
			}
		} else if err := starlark.ExecREPLChunk(f, thread, r.globals); err != nil {
			r.p.Send(execError(err))
			return
		}

		r.p.Send(execFinished{})
	}()

	return r.spinner.Tick
}

func soleExpr(f *syntax.File) syntax.Expr {
	if len(f.Stmts) == 1 {
		if stmt, ok := f.Stmts[0].(*syntax.ExprStmt); ok {
			return stmt.X
		}
	}
	return nil
}

// Update implements tea.Model.
func (r *replModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case execLine:
		r.log += string(msg) + "\n"
		return r, nil
	case execError:
		r.running = false
		r.errMsg += msg.Error() + "\n"
		return r, tea.Println(r.renderResult())
	case execReturnValue:
		r.returnValue = msg.content
		return r, nil
	case execFinished:
		r.running = false
		return r, tea.Println(r.renderResult())
	case spinner.TickMsg:
		r.spinner, cmd = r.spinner.Update(msg)
		return r, cmd
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if !r.running {
				r.execute(r.textInput.Value())
				r.textInput.Reset()
				return r, r.spinner.Tick
			}
		case tea.KeyCtrlC, tea.KeyEsc:
			return r, tea.Quit
		}
	}

	if r.running {
		return r, nil
	}
	r.textInput, cmd = r.textInput.Update(msg)
	return r, cmd
}

func (r *replModel) renderResult() string {
	out := strings.Builder{}
	if r.running {
		out.WriteString(r.spinner.View())
	} else if r.errMsg != "" {
		out.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000")).Render("*"))
	} else {
		out.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")).Render(">"))
	}
	out.WriteString(" ")
	out.WriteString(r.command)
	out.WriteString("\n")

	var style = lipgloss.NewStyle().
		Padding(1).
		BorderStyle(lipgloss.NormalBorder()).Width(r.terminalWidth - 4)

	// Print logs with a white border
	if len(r.log) > 0 {
		out.WriteString(
			style.BorderForeground(
				lipgloss.Color("#ffffff"),
			).Render(
				strings.TrimSpace(r.log),
			),
		)
		out.WriteString("\n")
	}

	// Print return value with a green border
	if r.returnValue != nil {
		out.WriteString(
			style.BorderForeground(
				lipgloss.Color("#00ff00"),
			).BorderStyle(
				lipgloss.ThickBorder(),
			).Render(
				strings.TrimSpace(fmt.Sprint(r.returnValue)),
			),
		)
		out.WriteString("\n")
	}

	// Print error message with a red border
	if len(r.errMsg) > 0 {
		out.WriteString(
			style.
				BorderForeground(
					lipgloss.Color("#ff0000"),
				).
				BorderStyle(
					lipgloss.ThickBorder(),
				).
				Render(
					strings.TrimSpace(r.errMsg),
				),
		)
		out.WriteString("\n")
	}

	return out.String()
}

// View implements tea.Model.
func (r *replModel) View() string {
	out := strings.Builder{}
	if r.running {
		out.WriteString(r.renderResult())
	} else {
		out.WriteString(r.textInput.View())
	}

	return out.String()
}

type execReturnValue struct {
	content any
}

type execLine string

type execFinished struct{}

type execError error
