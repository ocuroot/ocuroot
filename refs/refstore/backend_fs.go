package refstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func NewFsBackend(root string) DocumentBackend {
	return &fsBackend{
		root: root,
	}
}

var _ DocumentBackend = (*fsBackend)(nil)

type fsBackend struct {
	root string
}

// GetBytes implements DocumentBackend.
func (f *fsBackend) GetBytes(ctx context.Context, path string) ([]byte, error) {
	filePath := filepath.Join(f.root, path)

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, err
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return content, nil
}

// SetBytes implements DocumentBackend.
func (f *fsBackend) SetBytes(ctx context.Context, path string, content []byte) error {
	filePath := filepath.Join(f.root, path)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	return os.WriteFile(filePath, content, 0644)
}

// Get implements DocumentBackend.
func (f *fsBackend) Get(ctx context.Context, paths []string) ([]GetResult, error) {
	var out []GetResult
	for _, p := range paths {
		fp := filepath.Join(f.root, p)
		content, err := os.ReadFile(fp)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				out = append(out, GetResult{Path: p})
				continue
			}
			return nil, fmt.Errorf("read: %w", err)
		}
		var result StorageObject
		if err := json.Unmarshal(content, &result); err != nil {
			return nil, fmt.Errorf("unmarshal %q: %w", string(content), err)
		}
		out = append(out, GetResult{Path: p, Doc: &result})

	}
	return out, nil
}

// Marker implements DocumentBackend.
func (f *fsBackend) Marker() ([]byte, error) {
	return nil, nil
}

// Match implements DocumentBackend.
func (f *fsBackend) Match(ctx context.Context, reqs []MatchRequest) ([]string, error) {
	compiledReqs, err := compileMatchRequests(reqs)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(f.root); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var matchingRefs []string
	err = filepath.WalkDir(f.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(f.root, p)
		if err != nil {
			return err
		}
		candidate := relPath
		for _, g := range compiledReqs {
			c := candidate
			if g.prefix != "" {
				if !strings.HasPrefix(c, g.prefix) {
					continue
				}
				c = strings.TrimPrefix(c, g.prefix)
			}
			suffixes := g.suffixes
			if len(suffixes) == 0 {
				suffixes = []string{""}
			}
			for _, suffix := range suffixes {
				if strings.HasSuffix(c, suffix) && g.compiledGlob.Match(strings.TrimSuffix(c, suffix)) {
					matchingRefs = append(matchingRefs, candidate)
					return nil
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matchingRefs, nil
}

// Set implements DocumentBackend.
func (f *fsBackend) Set(ctx context.Context, marker []byte, message string, reqs []SetRequest) error {
	for _, req := range reqs {
		rPath := filepath.Join(f.root, req.Path)
		if req.Doc == nil {
			if err := os.Remove(rPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove %v: %w", rPath, err)
			}
			continue
		}

		// Create all necessary parent directories.
		if err := os.MkdirAll(filepath.Dir(rPath), 0755); err != nil {
			return fmt.Errorf("mkdir %v: %w", rPath, err)
		}

		content, err := json.Marshal(req.Doc)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		if err := os.WriteFile(rPath, content, 0644); err != nil {
			return fmt.Errorf("write %v: %w", rPath, err)
		}
	}
	return nil
}
