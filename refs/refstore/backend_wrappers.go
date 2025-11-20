package refstore

import (
	"context"
	"path"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
)

var _ DocumentBackend = &backendWithPrefix{}
var _ DocumentBackend = &backendWithLogging{}

type backendWithPrefix struct {
	backend DocumentBackend
	prefix  string
}

type backendWithLogging struct {
	backend DocumentBackend
}

// Get implements DocumentBackend.
func (b *backendWithLogging) Get(ctx context.Context, paths []string) ([]GetResult, error) {
	log.Debug("backend.Get", "paths", paths)
	return b.backend.Get(ctx, paths)
}

// GetBytes implements DocumentBackend.
func (b *backendWithLogging) GetBytes(ctx context.Context, path string) ([]byte, error) {
	log.Debug("backend.GetBytes", "path", path)
	return b.backend.GetBytes(ctx, path)
}

// Match implements DocumentBackend.
func (b *backendWithLogging) Match(ctx context.Context, reqs []MatchRequest) ([]string, error) {
	log.Debug("backend.Match", "reqs", reqs)
	res, err := b.backend.Match(ctx, reqs)
	log.Debug("backend.Match", "res", res, "err", err)
	return res, err
}

// Set implements DocumentBackend.
func (b *backendWithLogging) Set(ctx context.Context, message string, reqs []SetRequest) error {
	log.Debug("backend.Set", "message", message, "reqs", reqs)
	return b.backend.Set(ctx, message, reqs)
}

// SetBytes implements DocumentBackend.
func (b *backendWithLogging) SetBytes(ctx context.Context, path string, content []byte) error {
	log.Debug("backend.SetBytes", "path", path, "content", content)
	return b.backend.SetBytes(ctx, path, content)
}

// Get implements DocumentBackend.
func (b *backendWithPrefix) Get(ctx context.Context, paths []string) ([]GetResult, error) {
	var prefixedPaths []string
	for _, p := range paths {
		prefixedPaths = append(prefixedPaths, filepath.Join(b.prefix, p))
	}

	results, err := b.backend.Get(ctx, prefixedPaths)
	if err != nil {
		return nil, err
	}

	var out []GetResult
	for _, r := range results {
		r.Path = removePrefix(r.Path, b.prefix)
		out = append(out, r)
	}

	return out, nil
}

// GetBytes implements DocumentBackend.
func (b *backendWithPrefix) GetBytes(ctx context.Context, path string) ([]byte, error) {
	return b.backend.GetBytes(ctx, filepath.Join(b.prefix, path))
}

// Match implements DocumentBackend.
func (b *backendWithPrefix) Match(ctx context.Context, reqs []MatchRequest) ([]string, error) {
	var prefixedReqs []MatchRequest
	for _, r := range reqs {
		pfx := path.Join(b.prefix, r.Prefix)
		if len(pfx) > 0 {
			pfx = pfx + "/"
		}

		mr := r
		mr.Prefix = pfx
		prefixedReqs = append(prefixedReqs, mr)
	}

	results, err := b.backend.Match(ctx, prefixedReqs)
	if err != nil {
		return nil, err
	}

	var out []string
	for _, r := range results {
		out = append(out, removePrefix(r, b.prefix))
	}

	return out, nil
}

// Set implements DocumentBackend.
func (b *backendWithPrefix) Set(ctx context.Context, message string, reqs []SetRequest) error {
	var prefixedReqs []SetRequest
	for _, r := range reqs {
		prefixedReqs = append(prefixedReqs, SetRequest{
			Path: path.Join(b.prefix, r.Path),
			Doc:  r.Doc,
		})
	}

	return b.backend.Set(ctx, message, prefixedReqs)
}

// SetBytes implements DocumentBackend.
func (b *backendWithPrefix) SetBytes(ctx context.Context, path string, content []byte) error {
	return b.backend.SetBytes(ctx, filepath.Join(b.prefix, path), content)
}

func removePrefix(path, prefix string) string {
	out := strings.TrimPrefix(path, prefix)
	out = strings.TrimLeft(out, "/")
	return out
}
