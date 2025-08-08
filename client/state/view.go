package state

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/ocuroot/ocuroot/lib/release"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ui/assets"
	"github.com/ocuroot/ui/css"
	"github.com/ocuroot/ui/js"
)

func View(ctx context.Context, store refstore.Store) error {
	return StartViewServer(ctx, store, 0)
}

type server struct {
	store refstore.Store
}

func StartViewServer(ctx context.Context, store refstore.Store, port int) error {

	if port == 0 {
		var err error
		port, err = findAvailablePort(3000, 3100) // Try ports 3000-3100
		if err != nil {
			return fmt.Errorf("failed to find available port: %w", err)
		}
	}

	s := &server{
		store: store,
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		environmentRefs, err := store.Match(ctx, "@/environment/*")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		allReleaseRefs, err := store.Match(ctx, "**/-/**@*")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Filter out paths containing repo.ocu.star
		var releaseRefs []string
		for _, ref := range allReleaseRefs {
			if !strings.Contains(ref, "repo.ocu.star") {
				releaseRefs = append(releaseRefs, ref)
			}
		}

		deploymentRefs, err := store.Match(ctx, "**/@*/deploy/*")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		customStateRefs, err := store.Match(ctx, "**/@*/custom/*")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		index := Index(len(environmentRefs), len(releaseRefs), len(deploymentRefs), len(customStateRefs))
		index.Render(ctx, w)
	})
	http.HandleFunc("/match/", s.handleMatch)
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

		finalRef, err := refs.Parse(resolvedRef)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		doc, err := release.LoadRef(ctx, store, finalRef)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if r.URL.Query().Get("partial") == "true" {
			content := StateContent(RefPageProps{
				Ref:         refStr,
				ResolvedRef: resolvedRef,
				Content:     doc,
				ChildRefs:   childRefs,
			})
			content.Render(ctx, w)
			return
		}

		content := RefPage(RefPageProps{
			Ref:         refStr,
			ResolvedRef: resolvedRef,
			Content:     doc,
			ChildRefs:   childRefs,
		})
		content.Render(ctx, w)
	})

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

	// Start server
	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", port),
	}

	go func() {
		// Wait until the server returns 2xx on the relevant port
		// then output instructions
		client := &http.Client{
			Timeout: 1 * time.Second,
		}
		url := fmt.Sprintf("http://localhost:%d", port)

		for {
			resp, err := client.Get(url)
			if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
				resp.Body.Close()
				fmt.Printf("Your state view is ready and waiting!\n")
				fmt.Printf("Open your browser to: %s\n", url)
				return
			}
			if resp != nil {
				resp.Body.Close()
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

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

type RefMap map[string]RefMap

func (rm RefMap) OrderedKeys() []string {
	var keys []string
	for k := range rm {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (rm RefMap) AddAll(refs []string) {
	// Sort by length so prefixes get into the tree first
	sort.Slice(refs, func(i, j int) bool {
		return len(refs[i]) < len(refs[j])
	})
	for _, ref := range refs {
		rm.AddRef(ref)
	}
}

func (rm RefMap) AddRef(ref string) {
	for cr := path.Dir(ref); cr != "." && cr != ""; cr = path.Dir(cr) {
		if t, exists := rm[cr]; exists {
			t.AddRef(ref)
			return
		}
	}
	rm[ref] = RefMap{}
}

func BuildRefTree(refs []string) RefMap {
	var tree = make(RefMap)
	tree.AddAll(refs)
	return tree
}
