package refstore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

var _ DocumentBackend = (*inMemoryBackend)(nil)

type inMemoryBackend struct {
	info    *StoreInfo
	storage map[string]json.RawMessage
}

func NewInMemoryBackend() DocumentBackend {
	return &inMemoryBackend{
		storage: make(map[string]json.RawMessage),
	}
}

// GetInfo implements DocumentBackend.
func (i *inMemoryBackend) GetInfo(ctx context.Context) (*StoreInfo, error) {
	return i.info, nil
}

// SetInfo implements DocumentBackend.
func (i *inMemoryBackend) SetInfo(ctx context.Context, info *StoreInfo) error {
	i.info = info
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

// Marker implements DocumentBackend.
func (i *inMemoryBackend) Marker() ([]byte, error) {
	return nil, nil
}

// Match implements DocumentBackend.
func (i *inMemoryBackend) Match(ctx context.Context, reqs []MatchRequest) ([]string, error) {
	var compiledReqs []compiledMatchReq
	compiledReqs, err := compileMatchRequests(reqs)
	if err != nil {
		return nil, err
	}

	var out []string
	for ref := range i.storage {
		for _, req := range compiledReqs {
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
				if strings.HasSuffix(p, suffix) && req.compiledGlob.Match(strings.TrimSuffix(p, suffix)) {
					out = append(out, ref)
				}
			}
		}
	}

	return out, nil
}

// Set implements DocumentBackend.
func (i *inMemoryBackend) Set(ctx context.Context, marker []byte, message string, reqs []SetRequest) error {
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
