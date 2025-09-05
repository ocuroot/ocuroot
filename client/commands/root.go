package commands

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/about"
	"github.com/ocuroot/ocuroot/client"
	"github.com/spf13/cobra"
)

var cleanup func()

// rootCmd represents the base command for the client
// RootCmd represents the base command for the client
var RootCmd = &cobra.Command{
	Use:   "ocuroot",
	Short: "Ocuroot client CLI",
	Long: `Ocuroot client provides command-line tools for interacting 
with the Ocuroot release orchestration platform.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		homeDir := client.HomeDir()
		initLogs(homeDir, os.Args[1:])

		log.Info("Starting ocuroot", "version", about.Version, "args", os.Args[1:], "home", homeDir)
		cleanup = setupTelemetry()
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		cleanup()

		if logCloser != nil {
			logCloser.Close()
			log.SetOutput(os.Stderr)
			fmt.Fprintf(os.Stderr, "Logs at: %v\n", logPath)
		}
	},
}

var (
	logCloser io.WriteCloser
	logPath   string
)

func initLogs(homeDir string, args []string) {
	logDir := path.Join(homeDir, "logs")
	err := os.MkdirAll(logDir, os.ModeDir|os.ModePerm)
	if err != nil {
		log.Error("Could not create log directory. Logs will be discarded.", "error", err)
		log.SetOutput(io.Discard)
		return
	}
	logPath = path.Join(
		logDir,
		fmt.Sprintf(
			"%v-%v.log",
			time.Now().UnixNano(),
			strings.ReplaceAll(strings.Join(args, "_"), "/", "_"),
		),
	)
	logFile, err := os.Create(logPath)
	if err != nil {
		log.Error("Could not create log file. Logs will be discarded", "error", err)
		log.SetOutput(io.Discard)
		return
	}

	logCloser = logFile
	log.SetOutput(logFile)
	log.SetReportCaller(true)
}

// GetRootCommand returns the root Cobra command for the client
func GetRootCommand() *cobra.Command {
	return RootCmd
}

// Execute runs the root command
func Execute() error {
	return RootCmd.Execute()
}
