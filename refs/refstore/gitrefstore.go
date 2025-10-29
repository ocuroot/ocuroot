package refstore

import (
	"context"
	"path/filepath"
)

type GitRepoConfig struct {
	CreateBranch bool

	GitUserName  string
	GitUserEmail string
}

type GitRefStoreConfig struct {
	GitRepoConfig

	PathPrefix string
}

func NewGitRefStore(
	baseDir string,
	tags map[string]struct{},
	remote string,
	branch string,
	cfg GitRefStoreConfig,
) (Store, error) {
	// Create bare repo path under baseDir
	bareRepoPath := filepath.Join(baseDir, "git-repos", sanitizeRepoName(remote))
	
	be, err := NewGitBackend(context.Background(), bareRepoPath, remote, branch)
	if err != nil {
		return nil, err
	}
	return NewRefStore(context.Background(), be, tags)
}

// sanitizeRepoName creates a safe directory name from a git remote URL
func sanitizeRepoName(remote string) string {
	// Simple sanitization - replace special chars with underscores
	safe := ""
	for _, r := range remote {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '.' {
			safe += string(r)
		} else {
			safe += "_"
		}
	}
	return safe
}
