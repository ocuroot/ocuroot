package refstore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
)

var _ DocumentBackend = (*inMemoryBackend)(nil)

type inMemoryBackend struct {
	storage map[string]json.RawMessage
}

func NewInMemoryBackend() DocumentBackend {
	return &inMemoryBackend{
		storage: make(map[string]json.RawMessage),
	}
}

// GetBytes implements DocumentBackend.
func (i *inMemoryBackend) GetBytes(ctx context.Context, path string) ([]byte, error) {
	if data, ok := i.storage[path]; ok {
		return []byte(data), nil
	}
	return nil, nil
}

// SetBytes implements DocumentBackend.
func (i *inMemoryBackend) SetBytes(ctx context.Context, path string, content []byte) error {
	i.storage[path] = json.RawMessage(content)
	return nil
}

// Get implements DocumentBackend.
func (i *inMemoryBackend) Get(ctx context.Context, refs []string) ([]GetResult, error) {
	var out []GetResult
	for _, ref := range refs {
		if obj, ok := i.storage[ref]; ok {
			var doc StorageObject
			if err := json.Unmarshal(obj, &doc); err != nil {
				return nil, fmt.Errorf("unmarshal: %w", err)
			}
			out = append(out, GetResult{Path: ref, Doc: &doc})
		} else {
			out = append(out, GetResult{Path: ref})
		}
	}
	return out, nil
}

// Match implements DocumentBackend.
func (i *inMemoryBackend) Match(ctx context.Context, reqs []MatchRequest) ([]string, error) {
	log.Info("inMemoryBackend.Match", "reqs", reqs)

	var compiledReqs []compiledMatchReq
	compiledReqs, err := compileMatchRequests(reqs)
	if err != nil {
		return nil, err
	}

	var out []string
	for ref := range i.storage {
		for _, req := range compiledReqs {
			log.Info("inMemoryBackend.Match inner", "ref", ref, "req", req)
			p := ref
			if req.prefix != "" {
				if !strings.HasPrefix(p, req.prefix) {
					continue
				}
				p = strings.TrimPrefix(p, req.prefix)
			}
			var suffixes = req.suffixes
			if len(suffixes) == 0 {
				suffixes = []string{""}
			}

			for _, suffix := range suffixes {
				log.Info("inMemoryBackend.Match suffix check", "ref", ref, "req", req, "suffix", suffix, "trim_suffix", strings.TrimSuffix(p, suffix))
				if strings.HasSuffix(p, suffix) && req.compiledGlob.Match(strings.TrimSuffix(p, suffix)) {
					out = append(out, ref)
				}
			}
		}
	}

	return out, nil
}

// Set implements DocumentBackend.
func (i *inMemoryBackend) Set(ctx context.Context, message string, reqs []SetRequest) error {
	for _, req := range reqs {
		if req.Doc == nil {
			delete(i.storage, req.Path)
		} else {
			content, err := json.Marshal(req.Doc)
			if err != nil {
				return fmt.Errorf("marshal: %w", err)
			}

			i.storage[req.Path] = content
		}
	}
	return nil
}
