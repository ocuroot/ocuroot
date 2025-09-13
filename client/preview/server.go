package preview

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/fsnotify/fsnotify"
	"github.com/ocuroot/ocuroot/client/local"
	"github.com/ocuroot/ocuroot/client/release"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/ocuroot/ocuroot/store/models"
	"github.com/ocuroot/ocuroot/ui/components/pipeline"
	"github.com/ocuroot/ui/assets"
	"github.com/ocuroot/ui/css"
	"github.com/ocuroot/ui/js"
)

// StartPreviewServer starts a web server to preview the package configuration
func StartPreviewServer(tc release.TrackerConfig, previewPort int) {
	// Find available port if one wasn't specified
	port := previewPort
	if port == 0 {
		var err error
		port, err = findAvailablePort(3000, 3100) // Try ports 3000-3100
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding available port: %v\n", err)
			os.Exit(1)
		}
	}

	http.HandleFunc("/", MakeServePreview(tc))

	// Initialize the unified CSS and JS services
	http.HandleFunc(css.Default().GetVersionedURL(), css.Default().Serve)
	http.HandleFunc(js.Default().GetVersionedURL(), js.Default().Serve)

	http.HandleFunc("/static/logo.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write([]byte(assets.Logo))
	})
	http.HandleFunc("/static/anon_user.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write(assets.AnonUser)
	})
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/x-icon")
		w.Write(assets.Favicon)
	})

	http.HandleFunc("/watch", makeWatchHandler(tc.RepoPath, []string{
		tc.Ref.Filename,
	}))

	// Print URL to stdout
	url := fmt.Sprintf("http://localhost:%d", port)
	fmt.Printf("Preview server started at: %s\n", url)
	fmt.Printf("Press Ctrl+C to stop the server\n")

	// Start server
	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", port),
	}
	fmt.Printf("Listening on port %d\n", port)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Println(err)
	}
}

// findAvailablePort tries to find an available port within the given range
func findAvailablePort(start, end int) (int, error) {
	for port := start; port <= end; port++ {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			listener.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports found in range %d-%d", start, end)
}

// makeWatchHandler creates an SSE handler to notify the client when the package file changes
func makeWatchHandler(rootPath string, packageFilePaths []string) http.HandlerFunc {
	watchedFiles := make([]string, len(packageFilePaths))
	for i, file := range packageFilePaths {
		watchedFiles[i] = filepath.Join(rootPath, file)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Watching file", "file", watchedFiles)

		// Set headers for SSE
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		// Create new file watcher
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return
		}
		defer watcher.Close()

		// Add file to watch
		for _, file := range watchedFiles {
			if err := watcher.Add(file); err != nil {
				log.Error("Failed to add file to watch", "file", file, "error", err)
				return
			}
		}

		// Channel to notify of connection close
		done := r.Context().Done()

		// Send initial message
		fmt.Fprintf(w, "data: connected\n\n")
		w.(http.Flusher).Flush()

		// Watch for changes
		for {
			select {
			case <-done:
				return
			case event := <-watcher.Events:
				if event.Has(fsnotify.Write) {
					// Small delay to ensure file write is complete
					time.Sleep(100 * time.Millisecond)
					fmt.Fprintf(w, "data: reload\n\n")
					w.(http.Flusher).Flush()
				}
			case err := <-watcher.Errors:
				fmt.Fprintf(os.Stderr, "Watcher error: %v\n", err)
			}
		}
	}
}

func MakeServePreview(tc release.TrackerConfig) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		backend, _ := local.NewBackend(tc.Ref)
		backend.Environments = &release.EnvironmentBackend{
			State: tc.State,
		}
		config, err := local.ExecutePackage(r.Context(), tc.RepoPath, tc.Ref, backend)
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to load config %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			errorComp := ErrorPage(errorMsg)
			errorComp.Render(r.Context(), w)
			return
		}

		var pkg sdk.Package
		if config != nil && config.Package != nil {
			pkg = *config.Package
		}
		summary := pipeline.SDKPackageToReleaseSummary(
			models.ReleaseID("preview"),
			"preview",
			&pkg,
		)
		comp := PreviewPage(summary, tc.Ref.Filename, pkg)
		err = comp.Render(r.Context(), w)
		if err != nil {
			log.Error("Failed to render preview", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			errorComp := ErrorPage(err.Error())
			errorComp.Render(r.Context(), w)
			return
		}
	}
}
