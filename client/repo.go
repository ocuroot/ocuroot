package client

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ocuroot/gittools"
	"github.com/ocuroot/ocuroot/refs/refstore"
)

var (
	ErrRootNotFound = errors.New("root not found")
)

type RepoInfo struct {
	Root     string
	IsSource bool
}

func (r RepoInfo) GetReleaseConfigFiles() ([]string, error) {
	files := []string{}
	err := filepath.Walk(r.Root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), "ocu.star") && info.Name() != "repo.ocu.star" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil

}

func GetRepoInfo(path string) (RepoInfo, error) {
	sourceRoot, err := FindSourceRepoRoot(path)
	if err != nil && !errors.Is(err, ErrRootNotFound) {
		return RepoInfo{}, err
	}
	stateRoot, err := FindStateStoreRoot(path)
	if err != nil && !errors.Is(err, ErrRootNotFound) {
		return RepoInfo{}, err
	}

	if sourceRoot == "" && stateRoot == "" {
		return RepoInfo{}, ErrRootNotFound
	}

	root := sourceRoot
	if len(stateRoot) >= len(sourceRoot) {
		root = stateRoot
	}

	return RepoInfo{
		Root:     root,
		IsSource: sourceRoot == root,
	}, nil
}

func FindSourceRepoRoot(path string) (string, error) {
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

func GetRepoURL(repoRootPath string) (string, error) {
	var (
		repo *gittools.Repo
		err  error
	)
	repoURL := os.Getenv("OCU_REPO_URL_OVERRIDE")
	if repoURL != "" {
		return repoURL, nil
	}

	repo, err = gittools.Open(repoRootPath)
	if err != nil {
		return "", fmt.Errorf("failed to open repo: %w", err)
	}
	repoURL, err = repo.RemoteURL("origin", false)
	if err != nil {
		return "", fmt.Errorf("failed to get repo URL: %w", err)
	}
	repoURL = strings.TrimRight(repoURL, "\n")
	repoURL = refstore.GitURLToValidPath(repoURL)
	return repoURL, nil
}

func GetRepoCommit(repoRootPath string) (string, error) {
	var (
		repo *gittools.Repo
		err  error
	)

	commit := os.Getenv("OCU_REPO_COMMIT_OVERRIDE")
	if commit != "" {
		return commit, nil
	}

	repo, err = gittools.Open(repoRootPath)
	if err != nil {
		return "", fmt.Errorf("failed to open repo: %w", err)
	}

	// TODO: This should be built into gittools
	commitB, stderr, err := repo.Client.Exec("rev-parse", "HEAD")
	if err != nil {
		// This implies that there are no commits on this branch
		if strings.Contains(string(stderr), "unknown revision or path not in the working tree.") {
			return "null", nil
		}

		return "", fmt.Errorf("failed to get commit hash: %w\n%s", err, stderr)
	}
	commit = strings.TrimRight(string(commitB), "\n")
	return commit, nil
}
