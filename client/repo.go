package client

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ocuroot/gittools"
)

var (
	ErrRootNotFound = errors.New("root not found")
)

func FindRepoRoot(path string) (string, error) {
	return findRoot(path, "repo.ocu.star")
}

func FindStateStoreRoot(path string) (string, error) {
	return findRoot(path, ".ocuroot-store")
}

func findRoot(path string, markerFile string) (string, error) {
	dir, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	for {
		repoPath := filepath.Join(dir, markerFile)
		if _, err := os.Stat(repoPath); err == nil {
			return dir, nil
		}

		// Stop if we've reached the root directory
		parentDir := filepath.Dir(dir)
		if parentDir == dir {
			return "", ErrRootNotFound
		}
		dir = parentDir
	}
}

// GetRepoInfo returns the repo URL and commit hash for the given repo root path.
// If the environment variables OCU_REPO_URL_OVERRIDE and OCU_REPO_COMMIT_OVERRIDE are set, they will be used.
func GetRepoInfo(repoRootPath string) (string, string, error) {
	var (
		repo *gittools.Repo
		err  error
	)

	repoURL := os.Getenv("OCU_REPO_URL_OVERRIDE")
	commit := os.Getenv("OCU_REPO_COMMIT_OVERRIDE")

	if repoURL == "" || commit == "" {
		repo, err = gittools.Open(repoRootPath)
		if err != nil {
			return "", "", fmt.Errorf("failed to open repo: %w", err)
		}
	}

	if repoURL == "" {
		repoURL, err = repo.RemoteURL("origin", false)
		if err != nil {
			return "", "", fmt.Errorf("failed to get repo URL: %w", err)
		}
		repoURL = strings.TrimRight(repoURL, "\n")
	}

	if commit == "" {
		// TODO: This should be built into gittools
		commitB, _, err := repo.Client.Exec("rev-parse", "HEAD")
		if err != nil {
			return "", "", fmt.Errorf("failed to get commit hash: %w", err)
		}
		commit = strings.TrimRight(string(commitB), "\n")
	}
	return repoURL, commit, nil
}
