package release

import (
	"fmt"
	"path/filepath"

	"github.com/ocuroot/ocuroot/client"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
)

func NewRefStore(
	storeConfig *sdk.Store,
	repoURL string,
	repoPath string,
) (refstore.Store, refstore.Store, error) {
	if storeConfig == nil {
		return nil, nil, fmt.Errorf("state store config has not been set")
	}

	// Prefixes for state and intent
	var statePrefix, intentPrefix string

	if storeConfig.State.Git != nil {
		statePrefix = storeConfig.State.Git.PathPrefix
	}
	if storeConfig.Intent != nil && storeConfig.Intent.Git != nil {
		intentPrefix = storeConfig.Intent.Git.PathPrefix
	}

	if storeConfig.Intent == nil {
		if intentPrefix == "" {
			intentPrefix = "intent"
		}
		if statePrefix == "" {
			statePrefix = "state"
		}
	}

	stateStore, err := newRefStoreFromBackend(&storeConfig.State, repoURL, repoPath, statePrefix)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create state store: %w", err)
	}

	if storeConfig.Intent != nil {
		intentStore, err := newRefStoreFromBackend(storeConfig.Intent, repoURL, repoPath, intentPrefix)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create intent store: %w", err)
		}
		return stateStore, intentStore, nil
	}

	intentStore, err := newRefStoreFromBackend(&storeConfig.State, repoURL, repoPath, intentPrefix)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create intent store: %w", err)
	}
	return stateStore, intentStore, nil
}

func newRefStoreFromBackend(
	storeConfig *sdk.StorageBackend,
	repoURL string,
	repoPath string,
	pathPrefix string,
) (refstore.Store, error) {
	var (
		store refstore.Store
		err   error
	)
	if storeConfig.Fs != nil {
		statePath := filepath.Join(repoPath, storeConfig.Fs.Path, pathPrefix)
		store, err = refstore.NewFSRefStore(statePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create state store: %w", err)
		}
	}
	if storeConfig.Git != nil {
		store, err = refstore.NewGitRefStore(
			filepath.Join(client.HomeDir(), "state", repoURL),
			storeConfig.Git.RemoteURL,
			storeConfig.Git.Branch,
			pathPrefix,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create state store: %w", err)
		}
	}
	store = refstore.StoreWithOtel(store)

	return store, nil
}
