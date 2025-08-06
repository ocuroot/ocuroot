package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ocuroot/ocuroot/client/release"
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
			return runStarlarkReplWithFile(filePath)
		}
	},
}

// runSingleCommand executes a single Starlark command and exits
func runSingleCommand(ctx context.Context, filePath string, command string) error {
	// Get tracker config to load repo.ocu.star and set up state store
	tc, err := getTrackerConfigNoRef()
	if err != nil {
		return fmt.Errorf("failed to get tracker config: %w", err)
	}

	// Create a backend for SDK operations
	backend, _ := release.NewBackend(tc)

	var globals starlark.StringDict

	// If a file is specified, load it and get combined globals (SDK + user functions)
	if filePath != "" {
		fmt.Printf("Loading file: %s\n", filePath)

		// Load the .ocu.star file using sdk.LoadConfig to get user-defined functions
		config, err := sdk.LoadConfig(
			sdk.NewFSResolver(os.DirFS(tc.RepoPath)),
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
		globals, err = createGlobalsWithUserFunctions(backend, latestVersion, config)
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

func runStarlarkReplWithFile(filePath string) error {
	// Get tracker config to load repo.ocu.star and set up state store
	tc, err := getTrackerConfigNoRef()
	if err != nil {
		return fmt.Errorf("failed to get tracker config: %w", err)
	}

	// Create a backend for SDK operations
	backend, _ := release.NewBackend(tc)

	if filePath == "" {
		filePath = "repo.ocu.star"
	}

	// Load the .ocu.star file using sdk.LoadConfig to get user-defined functions
	config, err := sdk.LoadConfig(
		sdk.NewFSResolver(os.DirFS(tc.RepoPath)),
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
	globals, err := createGlobalsWithUserFunctions(backend, latestVersion, config)
	if err != nil {
		return fmt.Errorf("failed to create globals: %w", err)
	}

	fmt.Println("Starting Starlark REPL with Ocuroot SDK")
	fmt.Println("Type Ctrl+D to exit (Ctrl+C will interrupt the current operation)")
	fmt.Println("Type 'help()' to see available SDK modules")
	fmt.Printf("Loaded repo: %s\n", tc.Ref.Repo)
	fmt.Printf("Loaded file: %s\n", filePath)
	fmt.Printf("Available user functions: %d\n", len(config.GlobalFuncs()))
	fmt.Println()

	// Use the combined globals for REPL that includes user-defined functions
	return runCustomREPLWithGlobals(globals)
}

// createGlobalsWithUserFunctions creates a globals dict that combines SDK builtins with user-defined functions
func createGlobalsWithUserFunctions(backend sdk.Backend, sdkVersion string, config *sdk.Config) (starlark.StringDict, error) {
	// Get SDK builtins using EvalWithGlobals
	_, globals, err := sdk.EvalWithGlobals(context.Background(), backend, sdkVersion, "None", make(starlark.StringDict))
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
	// Create a scanner for reading input
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print(">>> ")
		if !scanner.Scan() {
			// EOF (Ctrl+D)
			fmt.Println()
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Handle special commands
		if line == "exit" || line == "quit" {
			break
		}

		// Create a new thread for this evaluation
		thread := &starlark.Thread{
			Name: "repl",
			Print: func(thread *starlark.Thread, msg string) {
				fmt.Println(msg)
			},
		}

		opts := syntax.FileOptions{}
		opts.LoadBindsGlobally = true

		// Parse the expression first
		expr, err := opts.ParseExpr("<stdin>", line, syntax.RetainComments)
		if err != nil {
			fmt.Printf("Parse error: %s\n", err)
			continue
		}

		// Try to evaluate the expression
		result, err := starlark.EvalExprOptions(&opts, thread, expr, globals)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			continue
		}

		// Print the result if it's not None
		if result != starlark.None {
			fmt.Println(result)
		}
	}

	return scanner.Err()
}

func init() {
	RootCmd.AddCommand(ReplCmd)
	AddRefFlags(ReplCmd, true)
	ReplCmd.Flags().StringP("command", "c", "", "Execute a single command and exit (non-interactive mode)")
}
