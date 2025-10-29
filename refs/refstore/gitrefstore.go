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

	PathPrefix   string
	SupportFiles map[string]string
}

func NewGitRefStore(
	baseDir string,
	tags map[string]struct{},
	remote string,
	branch string,
	cfg GitRefStoreConfig,
) (Store, error) {
	ctx := context.Background()
	
	// Create bare repo path under baseDir
	bareRepoPath := filepath.Join(baseDir, "git-repos", sanitizeRepoName(remote))
	
	be, err := NewGitBackend(ctx, bareRepoPath, remote, branch, cfg.GitUserName, cfg.GitUserEmail)
	if err != nil {
		return nil, err
	}
	
	// Write support files if provided
	if len(cfg.SupportFiles) > 0 {
		for path, content := range cfg.SupportFiles {
			if err := be.SetBytes(ctx, path, []byte(content)); err != nil {
				return nil, err
			}
		}
	}
	
	return NewRefStore(ctx, be, tags)
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
