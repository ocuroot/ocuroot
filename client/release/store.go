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

	if storeConfig.Intent != nil {
		stateStore, err := newRefStoreFromBackend(&storeConfig.State, repoURL, repoPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create state store: %w", err)
		}
		intentStore, err := newRefStoreFromBackend(storeConfig.Intent, repoURL, repoPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create intent store: %w", err)
		}
		return stateStore, intentStore, nil
	}

	b, err := newRefStoreFromBackend(&storeConfig.State, repoURL, repoPath)
	if err != nil {
		return nil, nil, err
	}

	// TODO: Set paths
	return b, b, nil
}

func newRefStoreFromBackend(
	storeConfig *sdk.StorageBackend,
	repoURL string,
	repoPath string,
) (refstore.Store, error) {
	var (
		store refstore.Store
		err   error
	)
	if storeConfig.Fs != nil {
		statePath := filepath.Join(repoPath, storeConfig.Fs.Path)
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
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create state store: %w", err)
		}
	}
	store = refstore.StoreWithOtel(store)

	return store, nil
}
