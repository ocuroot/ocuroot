package release

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ocuroot/ocuroot/client"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
)

var (
	intentTags = map[string]struct{}{
		"intent": {},
	}
	stateTags = map[string]struct{}{
		"state": {},
	}
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

	stateStore, err := newRefStoreFromBackend(&storeConfig.State, stateTags, repoURL, repoPath, statePrefix)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create state store: %w", err)
	}

	if storeConfig.Intent != nil {
		intentStore, err := newRefStoreFromBackend(storeConfig.Intent, intentTags, repoURL, repoPath, intentPrefix)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create intent store: %w", err)
		}
		return stateStore, intentStore, nil
	}

	intentStore, err := newRefStoreFromBackend(&storeConfig.State, intentTags, repoURL, repoPath, intentPrefix)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create intent store: %w", err)
	}
	return stateStore, intentStore, nil
}

func newRefStoreFromBackend(
	storeConfig *sdk.StorageBackend,
	tags map[string]struct{},
	repoURL string,
	repoPath string,
	pathPrefix string,
) (refstore.Store, error) {
	var (
		store refstore.Store
		err   error
	)
	if storeConfig.Fs != nil {
		statePath := filepath.Join(storeConfig.Fs.Path, pathPrefix)
		if !filepath.IsAbs(statePath) {
			statePath = filepath.Join(repoPath, statePath)
		}
		store, err = refstore.NewFSRefStore(statePath, tags)
		if err != nil {
			return nil, fmt.Errorf("failed to create state store: %w", err)
		}
	}

	gitUserName := "Ocuroot"
	gitUserEmail := "contact@ocuroot.com"
	if os.Getenv("OCUROOT_GIT_USER_NAME") != "" {
		gitUserName = os.Getenv("OCUROOT_GIT_USER_NAME")
	}
	if os.Getenv("OCUROOT_GIT_USER_EMAIL") != "" {
		gitUserEmail = os.Getenv("OCUROOT_GIT_USER_EMAIL")
	}

	if storeConfig.Local != nil {
		statePath := filepath.Join(client.HomeDir(), "local_state", storeConfig.Local.ID, pathPrefix)

		// The git backend will handle initialization and branch creation
		store, err = refstore.NewGitRefStore(
			filepath.Join(client.HomeDir(), "state"),
			tags,
			"file://"+statePath,
			"main",
			refstore.GitRefStoreConfig{
				PathPrefix: pathPrefix,
				GitRepoConfig: refstore.GitRepoConfig{
					CreateBranch: true,
					GitUserName:  gitUserName,
					GitUserEmail: gitUserEmail,
				},
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create state store: %w", err)
		}
	}

	if storeConfig.Git != nil {
		store, err = refstore.NewGitRefStore(
			filepath.Join(client.HomeDir(), "state"),
			tags,
			storeConfig.Git.RemoteURL,
			storeConfig.Git.Branch,
			refstore.GitRefStoreConfig{
				PathPrefix:   pathPrefix,
				SupportFiles: storeConfig.Git.SupportFiles,
				GitRepoConfig: refstore.GitRepoConfig{
					CreateBranch: storeConfig.Git.CreateBranch,
					GitUserName:  gitUserName,
					GitUserEmail: gitUserEmail,
				},
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create state store: %w", err)
		}
	}
	store = refstore.StoreWithOtel(store)

	return store, nil
}
