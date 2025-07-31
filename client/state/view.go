package state

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path"
	"sort"
	"strings"

	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ui/assets"
	"github.com/ocuroot/ui/css"
	"github.com/ocuroot/ui/js"
)

func View(ctx context.Context, store refstore.Store) error {
	return StartViewServer(ctx, store, 0)
}

func StartViewServer(ctx context.Context, store refstore.Store, port int) error {
	if port == 0 {
		var err error
		port, err = findAvailablePort(3000, 3100) // Try ports 3000-3100
		if err != nil {
			return fmt.Errorf("failed to find available port: %w", err)
		}
	}

	// Initialize the unified CSS and JS services
	cssService := css.NewService()
	jsService := js.NewService()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		childRefs, err := store.Match(ctx, "**")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		childRefs = collapseRefs(childRefs)

		content := ViewPage(ViewPageProps{
			Ref:         "",
			ResolvedRef: "",
			Content:     nil,
			ChildRefs:   childRefs,
		})
		content.Render(ctx, w)
	})
	http.HandleFunc("/ref/", func(w http.ResponseWriter, r *http.Request) {
		refStr := strings.TrimPrefix(r.URL.Path, "/ref/")
		resolvedRef, err := store.ResolveLink(ctx, refStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		childRefs, err := store.Match(ctx, fmt.Sprintf("%s/**", resolvedRef))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		childRefs = collapseRefs(childRefs)

		var doc any
		if err := store.Get(ctx, resolvedRef, &doc); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		content := ViewPage(ViewPageProps{
			Ref:         refStr,
			ResolvedRef: resolvedRef,
			Content:     doc,
			ChildRefs:   childRefs,
		})
		content.Render(ctx, w)
	})
	http.HandleFunc("/style.css", cssService.ServeCSS)
	http.HandleFunc("/script.js", jsService.ServeJS)
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

	// Start server
	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", port),
	}
	fmt.Printf("Listening on port %d\n", port)
	if err := srv.ListenAndServe(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
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

func normalizeRef(parent string, ref string) string {
	return strings.TrimPrefix(ref, parent+"/")
}

func collapseRefs(refs []string) []string {
	var refSet = make(map[string]struct{})
	for _, ref := range refs {
		refSet[ref] = struct{}{}
	}
	var out []string
	for ref := range refSet {
		isSubRef := false
		cr := path.Dir(ref)
		for cr != "." && cr != "" {
			if _, exists := refSet[cr]; exists {
				isSubRef = true
				break
			}
			cr = path.Dir(cr)
		}
		if !isSubRef {
			out = append(out, ref)
		}
	}
	sort.Strings(out)
	return out
}
