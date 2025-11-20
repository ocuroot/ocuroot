package refstore

import (
	"context"
	"strings"

	libglob "github.com/gobwas/glob"
)

// DocumentBackend defines functions to interact with a storage backend
// Documents are stored as StorageObject values against string refs.
type DocumentBackend interface {
	// GetBytes retrieves a file explicitly as bytes rather than batched documents
	GetBytes(ctx context.Context, path string) ([]byte, error)
	// SetBytes sets the contents of a file at a path to explicit bytes
	SetBytes(ctx context.Context, path string, content []byte) error

	// Lock(ctx context.Context, path string) (Lock, error)

	// Get returns the document content for the given refs
	Get(ctx context.Context, paths []string) ([]GetResult, error)
	// Set updates the document content for given ref/content pairs
	Set(ctx context.Context, message string, reqs []SetRequest) error
	// Match returns all refs that match the given glob patterns
	// MatchRequest contains a prefix value that may be used to optimize queries
	Match(ctx context.Context, reqs []MatchRequest) ([]string, error)
}

// type Lock interface {
// 	Unlock() error
// }

type SetRequest struct {
	Path string
	Doc  *StorageObject
}

func Paths(sr []SetRequest) []string {
	var out []string
	for _, r := range sr {
		out = append(out, r.Path)
	}
	return out
}

type GetResult struct {
	Path string
	Doc  *StorageObject
}

type MatchRequest struct {
	Prefix   string
	Suffixes []string
	Glob     string
}

func (m MatchRequest) Matches(in string) bool {
	if !strings.HasPrefix(in, m.Prefix) {
		return false
	}

	in = strings.TrimPrefix(in, m.Prefix)

	if len(m.Suffixes) > 0 {
		var hasSuffix bool
		for _, suffix := range m.Suffixes {
			if strings.HasSuffix(in, suffix) {
				in = strings.TrimSuffix(in, suffix)
				hasSuffix = true
				break
			}
		}
		if !hasSuffix {
			return false
		}
	}

	compiledGlob, err := libglob.Compile(m.Glob, '/')
	if err != nil {
		return false
	}

	return compiledGlob.Match(in)
}

type compiledMatchReq struct {
	prefix       string
	suffixes     []string
	compiledGlob libglob.Glob
}

func compileMatchRequests(reqs []MatchRequest) ([]compiledMatchReq, error) {
	var compiledReqs []compiledMatchReq
	for _, req := range reqs {
		compiledGlob, err := libglob.Compile(req.Glob, '/')
		if err != nil {
			return nil, err
		}
		compiledReqs = append(compiledReqs, compiledMatchReq{
			prefix:       req.Prefix,
			suffixes:     req.Suffixes,
			compiledGlob: compiledGlob,
		})
	}
	return compiledReqs, nil
}
