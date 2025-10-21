package refstore

import (
	"context"

	"github.com/ocuroot/ocuroot/git"
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
	r, err := git.NewRemoteGitWithUser(remote, &git.GitUser{
		Name:  cfg.GitUserName,
		Email: cfg.GitUserEmail,
	})
	if err != nil {
		return nil, err
	}
	be, err := NewGitBackend(context.Background(), r, branch)
	if err != nil {
		return nil, err
	}
	return NewRefStore(context.Background(), be, tags)
}
