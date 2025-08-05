package state

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path"
	"sort"
	"strings"

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

func StartViewServer(ctx context.Context, store refstore.Store, port int) error {
	if port == 0 {
		var err error
		port, err = findAvailablePort(3000, 3100) // Try ports 3000-3100
		if err != nil {
			return fmt.Errorf("failed to find available port: %w", err)
		}
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		environmentRefs, err := store.Match(ctx, "@/environment/*")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		releaseRefs, err := store.Match(ctx, "**/@*")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
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
	http.HandleFunc("/match/", func(w http.ResponseWriter, r *http.Request) {
		query := strings.TrimPrefix(r.URL.Path, "/match/")
		refs, err := store.Match(ctx, query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if r.URL.Query().Get("partial") == "true" {
			content := Match(query, refs)
			content.Render(ctx, w)
			return
		}
		MatchPage(query, refs).Render(ctx, w)
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
