package refstore

import (
	"context"
	"fmt"
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
	
	be, err := NewGitBackend(ctx, bareRepoPath, remote, branch, cfg.PathPrefix, cfg.GitUserName, cfg.GitUserEmail)
	if err != nil {
		return nil, err
	}
	
	// Write support files if provided (at repository root, not under path prefix)
	if len(cfg.SupportFiles) > 0 {
		gitBe, ok := be.(*gitBackend)
		if !ok {
			return nil, fmt.Errorf("support files are only supported for git backends")
		}
		for path, content := range cfg.SupportFiles {
			if err := gitBe.SetBytesAtRoot(ctx, path, []byte(content)); err != nil {
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
