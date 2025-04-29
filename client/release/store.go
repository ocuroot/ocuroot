package release

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/ocuroot/ocuroot/client"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
)

func NewRefStore(
	storeConfig *sdk.Store,
	repoURL string,
	repoPath string,
) (refstore.Store, error) {
	if storeConfig == nil {
		return nil, fmt.Errorf("state store config has not been set")
	}

	if storeConfig.Intent != nil {
		stateStore, err := newRefStoreFromBackend(&storeConfig.State, repoURL, repoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create state store: %w", err)
		}
		intentStore, err := newRefStoreFromBackend(storeConfig.Intent, repoURL, repoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create intent store: %w", err)
		}
		return &CombinedRefStore{
			stateStore:  stateStore,
			intentStore: intentStore,
		}, nil
	}

	b, err := newRefStoreFromBackend(&storeConfig.State, repoURL, repoPath)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func newRefStoreFromBackend(
	storeConfig *sdk.StorageBackend,
	repoURL string,
	repoPath string,
) (refstore.Store, error) {
	if storeConfig.Fs != nil {
		statePath := filepath.Join(repoPath, storeConfig.Fs.Path)
		return refstore.NewFSRefStore(statePath)
	}
	if storeConfig.Git != nil {
		return refstore.NewGitRefStore(
			filepath.Join(client.HomeDir(), "state", repoURL),
			storeConfig.Git.RemoteURL,
			storeConfig.Git.Branch,
		)
	}
	return nil, fmt.Errorf("only filesystem and git state stores are currently supported")
}

var _ refstore.Store = &CombinedRefStore{}

type CombinedRefStore struct {
	stateStore  refstore.Store
	intentStore refstore.Store
}

func (s *CombinedRefStore) StartTransaction(ctx context.Context) error {
	if err := s.stateStore.StartTransaction(ctx); err != nil {
		return err
	}
	if err := s.intentStore.StartTransaction(ctx); err != nil {
		return err
	}
	return nil
}

func (s *CombinedRefStore) CommitTransaction(ctx context.Context, message string) error {
	if err := s.stateStore.CommitTransaction(ctx, message); err != nil {
		return err
	}
	if err := s.intentStore.CommitTransaction(ctx, message); err != nil {
		return err
	}
	return nil
}

func (s *CombinedRefStore) storeForRef(ref string) refstore.Store {
	if isIntentRef(ref) {
		return s.intentStore
	}
	return s.stateStore
}

func isIntentRef(ref string) bool {
	pr, err := refs.Parse(ref)
	if err != nil {
		return false
	}
	return pr.ReleaseOrIntent.Type == refs.Intent
}

// Close implements release.RefStore
func (s *CombinedRefStore) Close() error {
	var err error
	if s.stateStore != nil {
		se := s.stateStore.Close()
		if se != nil {
			err = fmt.Errorf("failed to close state store: %w", se)
		}
	}
	if s.intentStore != nil {
		ie := s.intentStore.Close()
		if ie != nil {
			if err != nil {
				err = fmt.Errorf("%w\nfailed to close intent store: %w", err, ie)
			} else {
				err = fmt.Errorf("failed to close intent store: %w", ie)
			}
		}
	}
	return err
}

// Delete implements release.RefStore.
func (s *CombinedRefStore) Delete(ctx context.Context, ref string) error {
	return s.storeForRef(ref).Delete(ctx, ref)
}

// Get implements release.RefStore.
func (s *CombinedRefStore) Get(ctx context.Context, ref string, v any) error {
	return s.storeForRef(ref).Get(ctx, ref, v)
}

// GetDependants implements release.RefStore.
func (s *CombinedRefStore) GetDependants(ctx context.Context, ref string) ([]string, error) {
	return s.storeForRef(ref).GetDependants(ctx, ref)
}

// GetDependencies implements release.RefStore.
func (s *CombinedRefStore) GetDependencies(ctx context.Context, ref string) ([]string, error) {
	return s.storeForRef(ref).GetDependencies(ctx, ref)
}

// Link implements release.RefStore.
func (s *CombinedRefStore) Link(ctx context.Context, ref string, target string) error {
	return s.storeForRef(ref).Link(ctx, ref, target)
}

// Unlink implements release.RefStore.
func (s *CombinedRefStore) Unlink(ctx context.Context, ref string) error {
	return s.storeForRef(ref).Unlink(ctx, ref)
}

// GetLinks implements release.RefStore.
func (s *CombinedRefStore) GetLinks(ctx context.Context, ref string) ([]string, error) {
	return s.storeForRef(ref).GetLinks(ctx, ref)
}

// ResolveLink implements release.RefStore.
func (s *CombinedRefStore) ResolveLink(ctx context.Context, ref string) (string, error) {
	return s.storeForRef(ref).ResolveLink(ctx, ref)
}

// Match implements release.RefStore.
func (s *CombinedRefStore) Match(ctx context.Context, glob ...string) ([]string, error) {
	stateRefs, err := s.stateStore.Match(ctx, glob...)
	if err != nil {
		return nil, err
	}
	intentRefs, err := s.intentStore.Match(ctx, glob...)
	if err != nil {
		return nil, err
	}
	out := append(stateRefs, intentRefs...)
	sort.Strings(out)
	return out, nil
}

// RemoveDependency implements release.RefStore.
func (s *CombinedRefStore) RemoveDependency(ctx context.Context, ref string, dependency string) error {
	return s.storeForRef(ref).RemoveDependency(ctx, ref, dependency)
}

// Set implements release.RefStore.
func (s *CombinedRefStore) Set(ctx context.Context, ref string, v any) error {
	return s.storeForRef(ref).Set(ctx, ref, v)
}

func (s *CombinedRefStore) AddDependency(ctx context.Context, ref string, dependency string) error {
	return s.storeForRef(ref).AddDependency(ctx, ref, dependency)
}
